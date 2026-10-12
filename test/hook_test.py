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
shift  # "ask"
while [[ $1 == --* && $1 != -- ]]; do
  case $1 in --last-cmd) last=$2;; --last-status) st=$2;; esac; shift 2
done
shift
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

t = run(["? list ports\r"])
ok &= check("'? request' Enter fills the line", "echo REQ[ list ports]" in t, t)
ok &= check("...and does not run it", "\nREQ[" not in t, t)

t = run(["? list ports\r", "\r"])
ok &= check("second Enter runs the filled command", "\nREQ[ list ports]" in t, t)

t = run(["false\r", "?\r", "\r"])
ok &= check("'?' alone passes the previous command and status", "\nREQ[] LAST[false] ST[1]" in t, t)

t = run(["list ports\x07", "\r"])
ok &= check("Ctrl+G fills, Enter runs", "\nREQ[list ports]" in t, t)

t = run(["\x07", "echo after-empty\r"])
ok &= check("Ctrl+G on an empty line does nothing", "REQ[" not in t and "\nafter-empty" in t, t)

t = run(["echo 'a?b'\r"])
ok &= check("'?' inside a command is left alone", "\na?b" in t, t)

sys.exit(0 if ok else 1)
