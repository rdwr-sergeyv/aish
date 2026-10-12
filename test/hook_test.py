#!/usr/bin/env python3
"""Drive an interactive bash through a PTY to test the readline hook.

A stub stands in for the aish binary, so no model is called:
    python3 test/hook_test.py
"""
import os, pty, re, select, subprocess, sys, tempfile, time

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
HOOK = os.path.join(ROOT, "shell", "aish.bash")

STUB = r"""#!/bin/bash
# echo back what the hook passed, as a runnable command
if [[ $1 == explain ]]; then
  shift 2; printf 'EXPLAIN[%s]\nWarning: stub\n' "$*"; exit
fi
shift  # "ask"
while [[ $1 == --* && $1 != -- ]]; do
  case $1 in --last-cmd) last=$2;; --last-status) st=$2;; esac; shift 2
done
shift
if [[ $* == multi ]]; then printf 'echo one\necho two\n'; exit; fi
echo "echo REQ[$*] LAST[$last] ST[$st]"
"""


def run(keys_seq):
    tmp = tempfile.mkdtemp()
    stub = os.path.join(tmp, "aish")
    with open(stub, "w") as f:
        f.write(STUB)
    os.chmod(stub, 0o755)
    rc = os.path.join(tmp, "rc")
    with open(rc, "w") as f:
        f.write(f"PS1='PROMPT$ '\nHISTFILE=/dev/null\n__aish_bin={stub}\nsource {HOOK}\n")

    pid, fd = pty.fork()
    if pid == 0:
        os.execvpe("bash", ["bash", "--rcfile", rc, "-i"], {"TERM": "dumb", "PATH": os.environ["PATH"], "HOME": tmp})

    def read(timeout=1.5):
        out, end = b"", time.time() + timeout
        while time.time() < end:
            r, _, _ = select.select([fd], [], [], 0.1)
            if r:
                try:
                    out += os.read(fd, 4096)
                except OSError:
                    break
        return out.decode(errors="replace")

    read()
    transcript = ""
    for keys in keys_seq:
        os.write(fd, keys.encode())
        transcript += read()
    os.write(fd, b"exit\r")
    read(0.5)
    return re.sub(r"\x1b\[[0-9;]*[A-Za-z]", "", transcript)


def check(name, cond, transcript):
    print(("PASS " if cond else "FAIL ") + name)
    if not cond:
        print("  transcript:", repr(transcript[-600:]))
    return cond


ok = True
t = run(["echo plain-enter\r"])
ok &= check("plain Enter runs the line", "\nplain-enter" in t, t)

t = run(["? list ports \r"])
ok &= check("'? request' Enter fills the line, request kept as a comment",
            "echo REQ[list ports] LAST[] ST[0]  # list ports" in t, t)
ok &= check("...and does not run it", "\nREQ[" not in t, t)

t = run(["? list ports\r", "\r"])
ok &= check("second Enter runs the filled command", "\nREQ[list ports] LAST[] ST[0]\r" in t, t)

t = run(["? list ports\r", "X\r"])
ok &= check("cursor is at the end of the command, before the comment", "\nREQ[list ports] LAST[] ST[0]X\r" in t, t)

t = run(["? list ports\r", "\r", "fc -ln -2 | head -1\r"])
ok &= check("history keeps the request",
            "echo REQ[list ports] LAST[] ST[0]  # list ports\r\n" in t.split("head -1\r\n")[-1], t)

t = run(["? multi\r", "\r"])
ok &= check("multi-line answer gets the comment on its own first line",
            "# multi" in t and "\none\r\ntwo" in t, t)

t = run(["shopt -u interactive_comments\r", "? list ports\r", "\r"])
ok &= check("no comment without interactive_comments", "ST[0]  #" not in t and "\nREQ[list ports]" in t, t)

t = run(["export AISH_KEEP_REQUEST=0\r", "? list ports\r", "\r"])
ok &= check("AISH_KEEP_REQUEST=0 drops the comment", "ST[0]  #" not in t and "\nREQ[list ports]" in t, t)

t = run(["false\r", "?\r", "\r"])
ok &= check("'?' alone passes the previous command and status, no comment",
            "\nREQ[] LAST[false] ST[1]\r" in t and "ST[1]  #" not in t, t)

t = run(["list ports\x07", "\r"])
ok &= check("Ctrl+G fills, Enter runs", "\nREQ[list ports]" in t, t)

t = run(["\x07", "echo after-empty\r"])
ok &= check("Ctrl+G on an empty line does nothing", "REQ[" not in t and "\nafter-empty" in t, t)

t = run(["echo 'a?b'\r"])
ok &= check("'?' inside a command is left alone", "\na?b" in t, t)

t = run(["echo kept-line", "\x1be", "\r"])
ok &= check("Alt+E prints the explanation", "EXPLAIN[echo kept-line]\r\nWarning: stub" in t, t)
i = t.find("EXPLAIN[")
ok &= check("...then redraws the prompt with the line", "PROMPT$ echo kept-line" in t[i:], t)
ok &= check("...and Enter runs the unchanged line", "\nkept-line" in t[i:], t)

t = run(["\x1be", "echo after-empty\r"])
ok &= check("Alt+E on an empty line does nothing", "EXPLAIN[" not in t and "\nafter-empty" in t, t)

sys.exit(0 if ok else 1)
