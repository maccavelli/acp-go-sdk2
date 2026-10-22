"""Search without grep (docs/decisions/0031-MADR-verification-tools-in-repository.md).

Usage:
  find.py lines PATH_OR_GLOB... -- TEXT...
      each line, with its file and number, of the files named or matched
      (globs may use **) that contains any of the texts; plain substrings
  find.py files ROOT PATTERN...
      the paths under ROOT whose name matches any of the glob patterns, .git
      left out

Exit 0 when something is found, 1 when nothing is, or when the reader of its
output closes early (`| head`), 2 on a usage error.
"""
from __future__ import annotations

import glob
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _common  # noqa: E402


def lines(args: list[str]) -> int:
    if "--" not in args:
        print("find.py lines: missing -- before the texts", file=sys.stderr)
        return 2
    sep = args.index("--")
    sources, needles = args[:sep], args[sep + 1:]
    if not sources or not needles:
        print("find.py lines: need at least one path or glob and one text", file=sys.stderr)
        return 2
    files: list[str] = []
    for s in sources:
        matched = sorted(glob.glob(s, recursive=True)) if glob.has_magic(s) else [s]
        files += [f for f in matched if Path(f).is_file()]
    found = 0
    for f in files:
        for n, line in enumerate(Path(f).read_text(encoding="utf-8", errors="replace").splitlines(), 1):
            if any(t in line for t in needles):
                print(f"{f}:{n}: {line.strip()[:200]}")
                found += 1
    return 0 if found else 1


def files(args: list[str]) -> int:
    if len(args) < 2:
        print("find.py files: need ROOT and at least one pattern", file=sys.stderr)
        return 2
    root, pats = Path(args[0]), args[1:]
    seen: set[Path] = set()
    for pat in pats:
        for p in sorted(root.rglob(pat)):
            if ".git" not in p.relative_to(root).parts and p not in seen:
                seen.add(p)
                print(p.relative_to(root))
    return 0 if seen else 1


def main(argv: list[str]) -> int:
    if not argv or argv[0] in ("-h", "--help"):
        print(__doc__)
        return 0 if argv else 2
    cmd, rest = argv[0], argv[1:]
    if cmd == "lines":
        return lines(rest)
    if cmd == "files":
        return files(rest)
    print(f"find.py: unknown command {cmd!r}; see --help", file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(_common.main_guard(main, sys.argv[1:]))
