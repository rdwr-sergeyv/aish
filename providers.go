package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
)

type Provider interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

func newProvider(cfg *Config) (Provider, error) {
	switch cfg.Provider {
	case "anthropic":
		return &anthropicProvider{cfg}, nil
	case "openai":
		return &openaiProvider{cfg}, nil
	case "claude":
		return &claudeCLIProvider{cfg}, nil
	}
	return nil, fmt.Errorf("unknown provider %q", cfg.Provider)
}

// --- Anthropic Messages API ---

type anthropicProvider struct{ cfg *Config }

// Server-side refusal fallback ("default" routing) is accepted on these models only.
var fallbackModels = map[string]bool{
	"claude-fable-5-1":  true,
	"claude-opus-5-5":   true,
	"claude-opus-5":     true,
	"claude-sonnet-5-5": true,
}

func (p *anthropicProvider) Complete(ctx context.Context, system, user string) (string, error) {
	var opts []option.RequestOption
	if p.cfg.APIKey != "" {
		opts = append(opts, option.WithAPIKey(p.cfg.APIKey))
	}
	if p.cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(p.cfg.BaseURL))
	}
	client := anthropic.NewClient(opts...)

	params := anthropic.BetaMessageNewParams{
		Model:     anthropic.Model(p.cfg.Model),
		MaxTokens: 4096,
		System:    []anthropic.BetaTextBlockParam{{Text: system}},
		Messages:  []anthropic.BetaMessageParam{anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(user))},
	}
	if p.cfg.Effort != "" {
		params.OutputConfig = anthropic.BetaOutputConfigParam{Effort: anthropic.BetaOutputConfigEffort(p.cfg.Effort)}
	}
	if p.cfg.Fallbacks && fallbackModels[p.cfg.Model] {
		params.Betas = []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01}
		params.Fallbacks = anthropic.BetaFallbacksParamUnion{OfDefault: constant.ValueOf[constant.Default]()}
	}

	resp, err := client.Beta.Messages.New(ctx, params)
	if err != nil {
		var apierr *anthropic.Error
		if errors.As(err, &apierr) {
			switch apierr.StatusCode {
			case 401:
				return "", errors.New("anthropic: not authenticated; set ANTHROPIC_API_KEY or api_key in config")
			case 429:
				return "", errors.New("anthropic: rate limited, try again shortly")
			}
			return "", fmt.Errorf("anthropic: HTTP %d: %s", apierr.StatusCode, firstLine(apierr.Error()))
		}
		if strings.Contains(err.Error(), "no Anthropic credentials") {
			return "", errors.New("anthropic: no API key; set ANTHROPIC_API_KEY, or provider = claude in config")
		}
		return "", fmt.Errorf("anthropic: %s", firstLine(err.Error()))
	}
	if resp.StopReason == anthropic.BetaStopReasonRefusal {
		return "", fmt.Errorf("the model declined this request (%s)", resp.StopDetails.Category)
	}
	var out strings.Builder
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.BetaTextBlock); ok {
			out.WriteString(t.Text)
		}
	}
	return out.String(), nil
}

// --- OpenAI Chat Completions (also any compatible endpoint via base_url) ---

type openaiProvider struct{ cfg *Config }

func (p *openaiProvider) Complete(ctx context.Context, system, user string) (string, error) {
	base := p.cfg.BaseURL
	if base == "" {
		base = os.Getenv("OPENAI_BASE_URL")
	}
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	key := p.cfg.APIKey
	if key == "" {
		key = os.Getenv("OPENAI_API_KEY")
	}

	body, _ := json.Marshal(map[string]any{
		"model": p.cfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
	})
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(base, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == 401 {
		return "", errors.New("openai: not authenticated; set OPENAI_API_KEY or api_key in config")
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("openai: HTTP %d: %s", resp.StatusCode, firstLine(string(data)))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
				Refusal string `json:"refusal"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("openai: unreadable response: %v", err)
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("openai: no choices in response")
	}
	if r := parsed.Choices[0].Message.Refusal; r != "" {
		return "", fmt.Errorf("the model declined: %s", r)
	}
	return parsed.Choices[0].Message.Content, nil
}

// --- Claude Code CLI in print mode ---

type claudeCLIProvider struct{ cfg *Config }

func (p *claudeCLIProvider) Complete(ctx context.Context, system, user string) (string, error) {
	args := []string{
		"-p",
		"--output-format", "text",
		"--tools", "",
		"--strict-mcp-config",
		"--no-session-persistence",
		"--system-prompt", system,
	}
	if p.cfg.Model != "" {
		args = append(args, "--model", p.cfg.Model)
	}
	if p.cfg.Effort != "" {
		args = append(args, "--effort", p.cfg.Effort)
	}
	args = append(args, user)

	cmd := exec.CommandContext(ctx, p.cfg.ClaudeBin, args...)
	// Run outside the project so its CLAUDE.md and settings don't steer the answer.
	cmd.Dir = os.TempDir()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if errors.Is(err, exec.ErrNotFound) {
			return "", fmt.Errorf("claude CLI not found (%s); set claude_bin in config", p.cfg.ClaudeBin)
		}
		msg := firstLine(stderr.String())
		if msg == "" {
			msg = firstLine(string(out))
		}
		return "", fmt.Errorf("claude: %v: %s", err, msg)
	}
	return string(out), nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}
