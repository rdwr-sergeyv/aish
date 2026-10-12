# aish bash hook, loaded with: eval "$(aish init bash)"
#   Ctrl+G          replace the request on the line with a command
#   ? <request> ⏎   the same, from Enter
#   ? ⏎             fix the previous command
# Nothing runs until you press Enter on the command. Ctrl+J is a plain accept-line.
# Settings: AISH_KEY (default \C-g), AISH_ENTER=0 leaves Enter alone.

: "${__aish_bin:=aish}"
__aish_status=0

__aish_save_status() { __aish_status=$?; }

__aish_fill() {
  local line=$READLINE_LINE
  [[ $line == *[![:space:]]* ]] || return 0
  line=${line#"${line%%[![:space:]]*}"}
  line=${line#\?}

  local last
  last=$(fc -ln -1 2>/dev/null)
  last=${last#"${last%%[![:space:]]*}"}

  printf '\r\e[K\e[2maish …\e[0m' >&2
  local out
  out=$(AISH_SHELL="bash ${BASH_VERSION%%(*}" "$__aish_bin" ask \
        --last-cmd "$last" --last-status "$__aish_status" -- "$line" </dev/null)
  printf '\r\e[K' >&2

  READLINE_LINE=$out
  READLINE_POINT=${#out}
}

# Enter is a macro: \C-x\C-a runs __aish_enter, which rebinds \C-x\C-b to
# accept-line for ordinary lines or to a no-op after filling a "?" line.
__aish_enter() {
  local re='^[[:space:]]*[?]([[:space:]]|$)'
  if [[ $READLINE_LINE =~ $re ]]; then
    __aish_fill
    bind '"\C-x\C-b": redraw-current-line'
  else
    bind '"\C-x\C-b": accept-line'
  fi
}

if [[ $- == *i* ]]; then
  if [[ ${PROMPT_COMMAND[*]} != *__aish_save_status* ]]; then
    # First, so $? is still the status of the user's command.
    PROMPT_COMMAND="__aish_save_status${PROMPT_COMMAND:+;$PROMPT_COMMAND}"
  fi
  for __aish_km in emacs vi-insert; do
    bind -m "$__aish_km" -x "\"${AISH_KEY:-\\C-g}\": __aish_fill"
    if [[ ${AISH_ENTER:-1} != 0 ]]; then
      bind -m "$__aish_km" -x '"\C-x\C-a": __aish_enter'
      bind -m "$__aish_km" '"\C-x\C-b": accept-line'
      bind -m "$__aish_km" '"\C-m": "\C-x\C-a\C-x\C-b"'
    fi
  done
  unset __aish_km
fi
