#!/usr/bin/env python3
"""Check tracked text for real user-home paths (0037-MADR-align-docs-ci-and-disclosure-gates.md).

A portable disclosure check, for full CI and the Docs workflow: it needs no
private data. It reads every file `git ls-files` lists and refuses a home
path whose account is not a placeholder:

  macos-home    /Users/<name>/...
  linux-home    /home/<name>/...
  windows-home  C:\\Users\\<name>\\..., with any drive letter, either
                separator, and backslashes JSON-escaped or not

An account is a placeholder when it is written in angle brackets, such as
<user>, or JSON-escaped, \\u003cuser\\u003e; when it starts with $ or %, such
as $USER or %USERNAME%; or when it is one of PLACEHOLDERS. A tracked file that
cannot be read, or that is not UTF-8 and holds no NUL byte, is refused as
"unreadable"; a file with a NUL byte is binary and is skipped.

Each finding prints as FILE:LINE: CATEGORY, never the text it found.

The owner's private deny list is not here: tools/idscan.py and the global
pre-push guard (make push-guard) check for it before a push.

Usage: python3 scripts/check_disclosure.py [--root DIR]
Exit 0 when nothing is found, 1 when something is, 2 when git fails.
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
from pathlib import Path

# The generic account names the tree uses in examples and tests.
PLACEHOLDERS = frozenset({"user", "u"})

_NAME = r"([^\s/\\\"'`<>()\[\]{},;]+|<[^\s<>/\\]*>|\\u003c[^\s\\/]*?\\u003e)"
# One backslash is tried before two, so a JSON-escaped placeholder after a
# separator, \Users\\u003cuser>, keeps its own backslash.
_SEP = r"(?:\\|\\\\|/)"
FORMS = (
    ("windows-home", re.compile(r"(?<![\w])[A-Za-z]:" + _SEP + r"Users" + _SEP + _NAME, re.IGNORECASE)),
    ("macos-home", re.compile(r"(?<![A-Za-z]:)(?<![\w.-])/Users/" + _NAME)),
    ("linux-home", re.compile(r"(?<![\w.-])/home/" + _NAME)),
)


def placeholder(name: str) -> bool:
    """Whether an account name is a placeholder rather than a real account."""
    low = name.lower()
    return (low.startswith(("<", "\\u003c", "$", "%")) or low in PLACEHOLDERS)


def scan_text(text: str) -> list[tuple[int, str]]:
    """The line and category of each real home path in text."""
    found = []
    for category, rx in FORMS:
        for m in rx.finditer(text):
            if not placeholder(m.group(1)):
                found.append((text.count("\n", 0, m.start()) + 1, category))
    return sorted(set(found))


def scan(root: Path, names: list[str]) -> list[tuple[str, int, str]]:
    """The findings in the files names, relative to root."""
    out = []
    for name in names:
        try:
            data = (root / name).read_bytes()
        except OSError:
            out.append((name, 0, "unreadable"))
            continue
        if b"\0" in data:
            continue
        try:
            text = data.decode("utf-8")
        except UnicodeDecodeError:
            out.append((name, 0, "unreadable"))
            continue
        out.extend((name, line, category) for line, category in scan_text(text))
    return out


def tracked(root: Path) -> list[str]:
    """The files git tracks in root."""
    p = subprocess.run(["git", "-C", str(root), "ls-files", "-z"], capture_output=True, check=False)
    if p.returncode != 0:
        sys.exit(2)
    return [n for n in p.stdout.decode("utf-8", "surrogateescape").split("\0") if n]


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--root", type=Path, default=Path(__file__).resolve().parent.parent)
    args = ap.parse_args(argv)
    names = tracked(args.root)
    findings = scan(args.root, names)
    for name, line, category in findings:
        print(f"{name}:{line}: {category}")
    print(f"check-disclosure: {len(names)} tracked file(s), {len(findings)} finding(s)")
    return 1 if findings else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
