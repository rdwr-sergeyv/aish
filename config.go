package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Provider  string // anthropic | openai | claude
	Model     string
	Effort    string // anthropic and claude only
	BaseURL   string
	APIKey    string
	ClaudeBin string
	SendLast  bool // send the previous command and its exit status
	Fallbacks bool // anthropic only: server-side refusal fallback
	Timeout   time.Duration
	Path      string // the config file read, if any
}

var defaultModels = map[string]string{
	"anthropic": "claude-opus-5-5",
	"openai":    "gpt-5-mini",
	"claude":    "", // the CLI's own default
}

func configPath() string {
	if p := os.Getenv("AISH_CONFIG"); p != "" {
		return p
	}
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "aish", "config")
}

// loadConfig reads defaults, then the config file, then AISH_* variables.
func loadConfig() (*Config, error) {
	cfg := &Config{
		Provider:  "anthropic",
		Effort:    "low",
		ClaudeBin: "claude",
		SendLast:  true,
		Fallbacks: true,
		Timeout:   60 * time.Second,
	}
	values := map[string]string{}

	path := configPath()
	if f, err := os.Open(path); err == nil {
		defer f.Close()
		cfg.Path = path
		sc := bufio.NewScanner(f)
		for n := 1; sc.Scan(); n++ {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			k, v, ok := strings.Cut(line, "=")
			if !ok {
				return nil, fmt.Errorf("%s:%d: expected key = value", path, n)
			}
			values[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	for _, k := range []string{"provider", "model", "effort", "base_url", "api_key", "claude_bin", "send_last", "fallbacks", "timeout"} {
		if v, ok := os.LookupEnv("AISH_" + strings.ToUpper(k)); ok {
			values[k] = v
		}
	}

	for k, v := range values {
		var err error
		switch k {
		case "provider":
			cfg.Provider = strings.ToLower(v)
		case "model":
			cfg.Model = v
		case "effort":
			cfg.Effort = strings.ToLower(v)
		case "base_url":
			cfg.BaseURL = v
		case "api_key":
			cfg.APIKey = v
		case "claude_bin":
			cfg.ClaudeBin = v
		case "send_last":
			cfg.SendLast, err = strconv.ParseBool(v)
		case "fallbacks":
			cfg.Fallbacks, err = strconv.ParseBool(v)
		case "timeout":
			cfg.Timeout, err = time.ParseDuration(v)
		default:
			return nil, fmt.Errorf("%s: unknown key %q", path, k)
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %s: %v", path, k, err)
		}
	}

	def, ok := defaultModels[cfg.Provider]
	if !ok {
		return nil, fmt.Errorf("unknown provider %q (anthropic, openai or claude)", cfg.Provider)
	}
	if cfg.Model == "" {
		cfg.Model = def
	}
	return cfg, nil
}

func (c *Config) describe() string {
	var b strings.Builder
	file := c.Path
	if file == "" {
		file = configPath() + " (not present)"
	}
	key := "(from environment)"
	if c.APIKey != "" {
		key = "(set in config)"
	}
	model := c.Model
	if model == "" {
		model = "(CLI default)"
	}
	fmt.Fprintf(&b, "config     %s\n", file)
	fmt.Fprintf(&b, "provider   %s\n", c.Provider)
	fmt.Fprintf(&b, "model      %s\n", model)
	if c.Provider != "openai" {
		fmt.Fprintf(&b, "effort     %s\n", c.Effort)
	}
	if c.BaseURL != "" {
		fmt.Fprintf(&b, "base_url   %s\n", c.BaseURL)
	}
	if c.Provider == "claude" {
		fmt.Fprintf(&b, "claude_bin %s\n", c.ClaudeBin)
	} else {
		fmt.Fprintf(&b, "api_key    %s\n", key)
	}
	if c.Provider == "anthropic" {
		fmt.Fprintf(&b, "fallbacks  %t\n", c.Fallbacks)
	}
	fmt.Fprintf(&b, "send_last  %t\n", c.SendLast)
	fmt.Fprintf(&b, "timeout    %s\n", c.Timeout)
	return b.String()
}
