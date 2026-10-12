#!/usr/bin/env bash
# Install aish into ~/.local/bin and hook it into ~/.bashrc.
#   ./install.sh              this Linux machine (run it inside WSL for WSL)
#   ./install.sh user@host    a remote machine over ssh
# It copies no credentials: set ANTHROPIC_API_KEY or ~/.config/aish/config there.
set -euo pipefail
cd "$(dirname "$0")"
host=${1:-}

setup='mkdir -p ~/.local/bin && chmod +x ~/.local/bin/aish && { grep -q "aish init bash" ~/.bashrc 2>/dev/null || printf "%s\n" "[ -x ~/.local/bin/aish ] && eval \"\$(~/.local/bin/aish init bash)\"" >> ~/.bashrc; }'

if [[ -n $host ]]; then arch=$(ssh "$host" uname -m); else arch=$(uname -m); fi
case $arch in
  x86_64) bin=dist/aish-linux-amd64 ;;
  aarch64 | arm64) bin=dist/aish-linux-arm64 ;;
  *) echo "unsupported architecture: $arch" >&2; exit 1 ;;
esac
[[ -f $bin ]] || ./build.sh

if [[ -n $host ]]; then
  ssh "$host" 'mkdir -p ~/.local/bin'
  scp -q "$bin" "$host:.local/bin/aish"
  ssh "$host" "$setup"
else
  mkdir -p ~/.local/bin
  cp "$bin" ~/.local/bin/aish
  bash -c "$setup"
fi
echo "installed on ${host:-this machine}; open a new shell, type a request and press Ctrl+G"
