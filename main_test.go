package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanAnswer(t *testing.T) {
	cases := map[string]string{
		"ls -la":                        "ls -la",
		"  ls -la\n":                    "ls -la",
		"```bash\nls -la\n```":          "ls -la",
		"```\nls -la\n```":              "ls -la",
		"`ls -la`":                      "ls -la",
		"$ ls -la":                      "ls -la",
		"# not possible from the shell": "# not possible from the shell",
		"cat <<'EOF' > f\na\nEOF":       "cat <<'EOF' > f\na\nEOF",
	}
	for in, want := range cases {
		if got := cleanAnswer(in); got != want {
			t.Errorf("cleanAnswer(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCleanExplanation(t *testing.T) {
	in := "```\nLists files.\n  -l  long format   \n```\n"
	want := "Lists files.\n  -l  long format"
	if got := cleanExplanation(in); got != want {
		t.Errorf("cleanExplanation = %q, want %q", got, want)
	}
}

func TestConfigPrecedence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	os.WriteFile(path, []byte("# comment\nprovider = openai\nmodel = \"m1\"\ntimeout = 5s\n"), 0o600)
	t.Setenv("AISH_CONFIG", path)
	t.Setenv("AISH_MODEL", "m2")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != "openai" || cfg.Model != "m2" || cfg.Timeout.Seconds() != 5 {
		t.Fatalf("got %+v", cfg)
	}
}

func TestConfigDefaultsModelPerProvider(t *testing.T) {
	t.Setenv("AISH_CONFIG", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("AISH_PROVIDER", "claude")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "" {
		t.Fatalf("claude provider should leave the model to the CLI, got %q", cfg.Model)
	}
}

func TestConfigRejectsUnknownKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	os.WriteFile(path, []byte("modle = x\n"), 0o600)
	t.Setenv("AISH_CONFIG", path)
	if _, err := loadConfig(); err == nil {
		t.Fatal("expected an error for an unknown key")
	}
}
