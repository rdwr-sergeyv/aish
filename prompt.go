package main

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"runtime"
	"strings"
)

type shellContext struct {
	OS         string
	Shell      string
	Cwd        string
	User       string
	Root       bool
	LastCmd    string
	LastStatus int
}

func gatherContext(cfg *Config, lastCmd string, lastStatus int) shellContext {
	ctx := shellContext{
		OS:    osName(),
		Shell: os.Getenv("AISH_SHELL"),
	}
	if ctx.Shell == "" {
		ctx.Shell = "bash"
	}
	ctx.Cwd, _ = os.Getwd()
	if u, err := user.Current(); err == nil {
		ctx.User = u.Username
		ctx.Root = u.Uid == "0"
	}
	if cfg.SendLast {
		ctx.LastCmd = strings.TrimSpace(lastCmd)
		ctx.LastStatus = lastStatus
	}
	return ctx
}

func osName() string {
	name := runtime.GOOS
	if f, err := os.Open("/etc/os-release"); err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if v, ok := strings.CutPrefix(sc.Text(), "PRETTY_NAME="); ok {
				name = strings.Trim(v, `"`)
				break
			}
		}
	}
	if b, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil &&
		strings.Contains(strings.ToLower(string(b)), "microsoft") {
		name += " (WSL)"
	}
	return name + ", " + runtime.GOARCH
}

func systemPrompt(ctx shellContext) string {
	return fmt.Sprintf(`You turn a request typed at a terminal prompt into one command for %s on %s.
Your reply is placed on the user's command line verbatim, where they review it and press Enter to run it.

Reply with the command only: no explanation, no markdown, no code fences, no leading "$".
- Prefer one line; chain steps with &&, | or ; when needed. Use a here-document or a multi-line command only when one line would be unreadable.
- Prefer tools present on a stock install of this OS. If a tool must be installed first, chain the install in front.
- Where a value you can't know is required (a host, a file, a PID), write an obvious placeholder such as <host>.
- Add sudo only when the command needs it and the user is not root.
- If the request can't be done with a command, or asks for information that isn't a command, reply with one line starting with "# " that answers briefly.
- Use the previous command and its exit status only when the request refers to them ("fix that", "again but...").`, ctx.Shell, ctx.OS)
}

func explainPrompt(ctx shellContext) string {
	return fmt.Sprintf(`You explain a command for %s on %s to the person about to run it. Your reply is printed in their terminal, above their prompt.

Write plain text: no markdown, no code fences, no headings, no bold.
- First line: what the command does, in one sentence.
- Then, unless the command is trivial, one short line per notable part (a flag, a pipe stage, an expansion), as "  <part>  <what it does>".
- If running it can delete or overwrite data, change system, service or network state, needs root, or does something other than it appears to, add a line starting with "Warning: ".
- If it is wrong or won't work here, say why and give the corrected command on a line starting with "Fix: ".
Stay under 12 lines.`, ctx.Shell, ctx.OS)
}

func userPrompt(ctx shellContext, label, text string) string {
	var b strings.Builder
	b.WriteString("<context>\n")
	fmt.Fprintf(&b, "cwd: %s\n", ctx.Cwd)
	who := ctx.User
	if ctx.Root {
		who += " (root)"
	}
	fmt.Fprintf(&b, "user: %s\n", who)
	if ctx.LastCmd != "" {
		fmt.Fprintf(&b, "previous command: %s\nits exit status: %d\n", ctx.LastCmd, ctx.LastStatus)
	}
	fmt.Fprintf(&b, "</context>\n\n%s: %s", label, text)
	return b.String()
}

// cleanExplanation drops code-fence lines, which models add despite instructions.
func cleanExplanation(s string) string {
	var keep []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "```") {
			keep = append(keep, strings.TrimRight(line, " \t\r"))
		}
	}
	return strings.TrimSpace(strings.Join(keep, "\n"))
}

// cleanAnswer strips what models add despite instructions: code fences
// and a leading "$ " prompt marker.
func cleanAnswer(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		lines = lines[1:]
		if n := len(lines); n > 0 && strings.HasPrefix(strings.TrimSpace(lines[n-1]), "```") {
			lines = lines[:n-1]
		}
		s = strings.TrimSpace(strings.Join(lines, "\n"))
	}
	if strings.HasPrefix(s, "`") && strings.HasSuffix(s, "`") && !strings.Contains(s, "\n") {
		s = strings.Trim(s, "`")
	}
	s = strings.TrimPrefix(s, "$ ")
	return strings.TrimSpace(s)
}
