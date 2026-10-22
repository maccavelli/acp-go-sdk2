"""Scan files for the identifiers no commit may carry
(docs/decisions/0031-MADR-verification-tools-in-repository.md, question 2).

The patterns are the push guard's deny list, never this repository's: the file
DISCLOSURE_DENY names, or ~/.config/git/disclosure-deny. Each line that is not
blank or a comment is "<regex> TAB <placeholder> [TAB identity]"; every
line's regex is used, identity lines included. A match is reported by its
place, path:line, never by the text it matched.

Usage:
  idscan.py                 the changed files: modified, staged and untracked
  idscan.py --all           every tracked file
  idscan.py --range A..B    the files the commits in A..B touch, as at B
  idscan.py --paths FILE..  the files named, inside the repository or not

Exit 0 when nothing matches, 1 when something does, 2 when the deny list is
missing or holds no pattern.
"""
from __future__ import annotations

import argparse
import os
import re
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _common  # noqa: E402


def deny_list() -> Path:
    """Where the push guard reads its deny list."""
    override = os.environ.get("DISCLOSURE_DENY")
    return Path(override) if override else Path.home() / ".config" / "git" / "disclosure-deny"


def patterns(path: Path) -> list[re.Pattern[str]]:
    """The deny list's regexes; empty when the file is missing or holds none."""
    if not path.is_file():
        return []
    out = []
    for line in path.read_text(encoding="utf-8").splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        out.append(re.compile(line.split("\t")[0]))
    return out


def git(root: Path, *args: str) -> str:
    return subprocess.run(["git", *args], cwd=root, capture_output=True, text=True, check=True).stdout


def scan_text(name: str, text: str, pats: list[re.Pattern[str]]) -> list[str]:
    """The places, name:line, where any pattern matches text."""
    return [f"{name}:{n}" for n, line in enumerate(text.splitlines(), 1) if any(p.search(line) for p in pats)]


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0], formatter_class=argparse.RawDescriptionHelpFormatter)
    mode = ap.add_mutually_exclusive_group()
    mode.add_argument("--all", action="store_true", help="every tracked file")
    mode.add_argument("--range", metavar="A..B", help="the files the commits in A..B touch, as at B")
    mode.add_argument("--paths", nargs="+", metavar="FILE", help="the files named")
    args = ap.parse_args(argv)

    source = deny_list()
    pats = patterns(source)
    if not pats:
        print(f"idscan: no pattern: the deny list is missing or empty (DISCLOSURE_DENY, or {Path('~/.config/git/disclosure-deny')})",
              file=sys.stderr)
        return 2

    places: list[str] = []
    if args.paths:
        for f in args.paths:
            p = Path(f)
            places += scan_text(f, p.read_text(encoding="utf-8", errors="replace"), pats)
        scanned = len(args.paths)
    else:
        root = _common.repo(Path.cwd())
        if args.range:
            end = args.range.split("..", 1)[1] or "HEAD"
            names = sorted(set(git(root, "diff", "--name-only", "--diff-filter=d", args.range).split()))
            for n in names:
                blob = subprocess.run(["git", "show", f"{end}:{n}"], cwd=root, capture_output=True, check=True).stdout
                places += scan_text(n, blob.decode(errors="replace"), pats)
        else:
            if args.all:
                names = git(root, "ls-files", "-z").split("\0")
            else:
                names = set(git(root, "diff", "--name-only", "HEAD").split()) | set(git(root, "ls-files", "--others", "--exclude-standard").split())
            names = sorted(n for n in names if n and (root / n).is_file())
            for n in names:
                places += scan_text(n, (root / n).read_text(encoding="utf-8", errors="replace"), pats)
        scanned = len(names)
    for place in places:
        print(f"idscan: identifier at {place}")
    print(f"idscan: {scanned} file(s), {len(pats)} pattern(s), {len(places)} match(es)")
    return 1 if places else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
