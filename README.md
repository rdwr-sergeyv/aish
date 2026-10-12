# aish

Plain language in, one shell command out, on your own prompt line. Nothing runs
until you press Enter.

```
$ ? find files over 100MB changed this week      ← press Enter (or type it bare and press Ctrl+G)
$ find . -type f -size +100M -mtime -7           ← the line is replaced; Enter runs it, Ctrl+C drops it
```

| Keys | Effect |
|---|---|
| `? <request>` Enter | replace the line with a command |
| `<request>` Ctrl+G | the same, without the `?` |
| `?` Enter | fix the previous command (uses its text and exit status) |
| Ctrl+J | plain accept-line, bypassing the hook |

When there's no command to give, the answer is a `# ...` comment line, so Enter is
harmless. Errors arrive the same way: `# aish: ...`.

No agent loop, no tools, no confirmations. One model call per request. The
generated command goes into history like anything you typed.

## Install

```bash
./build.sh                    # static binaries: dist/aish-linux-{amd64,arm64}
./install.sh                  # this machine (inside WSL for WSL)
./install.sh user@host        # a remote machine over ssh
```

`install.sh` copies the binary to `~/.local/bin/aish` and adds one line to
`~/.bashrc`:

```bash
[ -x ~/.local/bin/aish ] && eval "$(~/.local/bin/aish init bash)"
```

It copies no credentials. Set them on each machine (below).

## Providers

The binary picks the provider and makes the call. The bash hook only calls the binary.

| `provider` | Call | Credentials | Default model |
|---|---|---|---|
| `anthropic` (default) | Messages API | `ANTHROPIC_API_KEY` | `claude-opus-5-5` |
| `openai` | `/chat/completions` (OpenAI or any compatible endpoint, such as Ollama) | `OPENAI_API_KEY` | `gpt-5-mini` |
| `claude` | `claude -p` (Claude Code CLI, print mode, no tools, no MCP, no session saved) | the CLI's own login | the CLI's default |

Configure in `~/.config/aish/config` (see [config.example](config.example)) or with
`AISH_<KEY>` variables, which win. `aish config` prints what's in effect.

```bash
AISH_PROVIDER=claude aish ask -- list listening ports    # one-off, outside the hook
AISH_DEBUG=1 aish ask -- ...                             # print provider, model, latency
```

Speed matters most here. `effort = low` is the default. With the `claude` provider,
CLI startup adds a few seconds to every request. The API providers are faster.

## What is sent

Your OS and distro, the shell, the current directory, your user name, the request,
and (with `send_last = true`) the previous command line and its exit status. No file
contents and no environment variables. Set `send_last = false` if your commands
carry secrets.

## Tests

```bash
go test ./...
python3 test/hook_test.py      # drives a real interactive bash through a PTY, with a stub model
```
