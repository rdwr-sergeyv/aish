// aish turns a plain-language request into one shell command.
// It only prints the command; the bash hook puts it on the prompt line,
// and the user decides whether to run it.
package main

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

//go:embed shell/aish.bash
var bashHook string

const usage = `aish - plain language in, one shell command out

Usage:
  aish ask [--last-cmd CMD --last-status N] -- <request...>
  aish init bash        print the bash hook: eval "$(aish init bash)"
  aish config           print the effective configuration
  aish <request...>     shorthand for "ask"

In bash, type a request and press Ctrl+G, or start the line with "? " and
press Enter. The line is replaced with a command; Enter runs it.
`

func main() {
	args := os.Args[1:]
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Print(usage)
		return
	}
	switch args[0] {
	case "init":
		os.Exit(runInit(args[1:]))
	case "config":
		os.Exit(runConfig())
	case "ask":
		os.Exit(runAsk(args[1:]))
	default:
		os.Exit(runAsk(args))
	}
}

func runInit(args []string) int {
	if len(args) != 1 || args[0] != "bash" {
		fmt.Fprintln(os.Stderr, "aish init: only bash is supported: aish init bash")
		return 2
	}
	self, err := os.Executable()
	if err != nil {
		self = "aish"
	}
	fmt.Printf("__aish_bin=%s\n%s", shellQuote(self), bashHook)
	return 0
}

func runConfig() int {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "aish:", err)
		return 1
	}
	fmt.Print(cfg.describe())
	return 0
}

func runAsk(args []string) int {
	fs := flag.NewFlagSet("ask", flag.ContinueOnError)
	lastCmd := fs.String("last-cmd", "", "the previous command line, for context")
	lastStatus := fs.Int("last-status", 0, "exit status of the previous command")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	request := strings.TrimSpace(strings.Join(fs.Args(), " "))

	cfg, err := loadConfig()
	if err != nil {
		return fail(err)
	}
	ctx := gatherContext(cfg, *lastCmd, *lastStatus)
	if request == "" {
		if ctx.LastCmd == "" {
			return fail(errors.New("empty request"))
		}
		request = "Fix the previous command."
	}

	p, err := newProvider(cfg)
	if err != nil {
		return fail(err)
	}
	c, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	start := time.Now()
	raw, err := p.Complete(c, systemPrompt(ctx), userPrompt(ctx, request))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			err = fmt.Errorf("no answer within %s (timeout in config)", cfg.Timeout)
		}
		return fail(err)
	}
	cmd := cleanAnswer(raw)
	if cmd == "" {
		return fail(errors.New("the model returned an empty answer"))
	}
	if os.Getenv("AISH_DEBUG") != "" {
		model := cfg.Model
		if model == "" {
			model = "default"
		}
		fmt.Fprintf(os.Stderr, "aish: %s/%s in %s\n", cfg.Provider, model, time.Since(start).Round(time.Millisecond))
	}
	fmt.Println(cmd)
	return 0
}

// fail prints the error as a comment line so that, once it lands on the
// prompt, pressing Enter is harmless.
func fail(err error) int {
	msg := strings.ReplaceAll(err.Error(), "\n", " ")
	fmt.Println("# aish: " + msg)
	return 1
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
