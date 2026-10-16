#!/usr/bin/env python3
"""Record and docs-link tooling for the fixed docs tree.

Modes:
  --next         print the next record number: one more than the highest in
                 use anywhere in the repository, so a gap stays a gap
  --check        validate numbered records: filename shape, number pairing,
                 placement, and relative markdown links with their anchors
  --check-all    --check plus link checks for unnumbered docs
                 (docs/guides/**, docs/README.md, docs/architecture.md,
                 and the root README.md)

This copy does not generate docs/README.md. The index is written by hand
so Status and extra sections survive.

Errors print to stdout; exit status is 1 iff errors were found. Errors: a
number claimed by more than one MADR, a PLAN without a same-number MADR, a
GATES without a same-number PLAN, a record outside its kind directory, a
misnamed record (a markdown file in a docs/decisions or docs/reports
directory that is not NNNN-KIND-slug.md, or a file anywhere named like a
record in another case or width), a relative link to a missing file, and a
link whose #fragment names no heading or HTML anchor of its markdown target.
Several PLANs and REPORTs may share a number.

Anchors are computed as GitHub computes them from ATX headings (# ...):
inline markup removed, lowercased, characters other than letters, digits,
underscores, spaces and hyphens removed, spaces turned into hyphens, and a
repeated heading suffixed -1, -2, ... Headings in fenced code are ignored.
"""

from __future__ import annotations

import argparse
import os
import re
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
RECORD_RE = re.compile(r"^(\d{4})-(MADR|PLAN|REPORT|GATES)-(.+)\.md$")
LOOKALIKE_RE = re.compile(r"^\d+-(madr|plan|report|gates)-", re.IGNORECASE)
LINK_RE = re.compile(r"\]\(([^)]+)\)")
SCHEME_RE = re.compile(r"^[a-zA-Z][a-zA-Z0-9+.\-]*:")
FENCE_RE = re.compile(r"^\s*(```|~~~)")
HEADING_RE = re.compile(r"^ {0,3}#{1,6}(?:[ \t]+(.*?))?(?:[ \t]+#+)?[ \t]*$")
HTML_ANCHOR_RE = re.compile(r"<a\s+(?:[^>]*\s)?(?:id|name)\s*=\s*[\"']([^\"']+)[\"']", re.IGNORECASE)
KIND_SUFFIX = {"MADR": "docs/decisions", "PLAN": "docs/decisions",
               "REPORT": "docs/reports", "GATES": "docs/reports"}
RECORD_DIRS = ("docs/decisions", "docs/reports")
SKIP_PARTS = {".git", "node_modules", "vendor", "dist", ".pub-cache",
              ".pub", ".symlinks", "build", ".dart_tool", ".gradle",
              ".idea", ".vscode"}


def errors(msg: str) -> None:
    print(msg)


def markdown_files() -> list[Path]:
    out = []
    for p in sorted(REPO.rglob("*.md")):
        rel_parts = p.relative_to(REPO).parts
        if any(part in SKIP_PARTS for part in rel_parts[:-1]):
            continue
        out.append(p)
    return out


class Record:
    def __init__(self, path: Path) -> None:
        m = RECORD_RE.match(path.name)
        if not m:
            raise ValueError(path.name)
        self.path = path
        self.rel = path.relative_to(REPO).as_posix()
        self.num, self.kind, self.slug = m.group(1), m.group(2), m.group(3)


def find_records() -> list[Record]:
    records = []
    for p in markdown_files():
        if RECORD_RE.match(p.name):
            records.append(Record(p))
    return records


def next_number() -> int:
    return max((int(r.num) for r in find_records()), default=0) + 1


def in_record_dir(path: Path) -> bool:
    parent = path.parent.relative_to(REPO).as_posix()
    return any(parent == d or parent.endswith("/" + d) for d in RECORD_DIRS)


def check_names() -> int:
    found = 0
    for p in markdown_files():
        if RECORD_RE.match(p.name):
            continue
        if in_record_dir(p) or LOOKALIKE_RE.match(p.name):
            errors(f"misnamed record: {p.relative_to(REPO).as_posix()} "
                   "(expected NNNN-MADR|PLAN|REPORT|GATES-slug.md)")
            found += 1
    return found


def check_structure(records: list[Record]) -> int:
    found = 0
    by_num: dict[str, list[Record]] = {}
    for r in records:
        by_num.setdefault(r.num, []).append(r)
    for num in sorted(by_num):
        items = by_num[num]
        madrs = [r for r in items if r.kind == "MADR"]
        plans = [r for r in items if r.kind == "PLAN"]
        gates = [r for r in items if r.kind == "GATES"]
        if len(madrs) > 1:
            errors(f"number {num} is claimed by {len(madrs)} MADRs: "
                   + ", ".join(r.rel for r in madrs)
                   + " (a number is never reused)")
            found += 1
        if plans and not madrs:
            errors(f"number {num} has a PLAN but no MADR: "
                   + ", ".join(r.rel for r in plans))
            found += 1
        if gates and not plans:
            errors(f"number {num} has a GATES but no PLAN: "
                   + ", ".join(r.rel for r in gates))
            found += 1
    for r in records:
        expected = KIND_SUFFIX[r.kind]
        parent = r.path.parent.relative_to(REPO).as_posix()
        if parent != expected and not parent.endswith("/" + expected):
            errors(f"record outside its directory: {r.rel} "
                   f"(expected a path ending in {expected}/)")
            found += 1
    return found


def strip_code_spans(line: str) -> str:
    """Blank out `inline code` spans so example links inside them, like
    `[text](url)` or `SendRequest[T any](c *Connection, …)`, are not
    mistaken for live markdown links."""
    parts = line.split("`")
    return "".join(p for i, p in enumerate(parts) if i % 2 == 0)


def heading_slug(text: str) -> str:
    text = re.sub(r"!?\[([^\]]*)\]\([^)]*\)", r"\1", text)
    text = re.sub(r"<[^>]+>", "", text)
    text = text.replace("`", "").replace("*", "")
    text = re.sub(r"(?<!\w)_+|_+(?!\w)", "", text)
    text = re.sub(r"[^\w\- ]", "", text.strip().lower())
    return text.replace(" ", "-")


_anchor_cache: dict[Path, set[str]] = {}


def anchors(path: Path) -> set[str]:
    if path in _anchor_cache:
        return _anchor_cache[path]
    out: set[str] = set()
    seen: dict[str, int] = {}
    in_fence = False
    for line in path.read_text(encoding="utf-8").splitlines():
        if FENCE_RE.match(line):
            in_fence = not in_fence
            continue
        if in_fence:
            continue
        out.update(HTML_ANCHOR_RE.findall(line))
        m = HEADING_RE.match(line)
        if m:
            slug = heading_slug(m.group(1) or "")
            n = seen.get(slug, 0)
            seen[slug] = n + 1
            out.add(slug if n == 0 else f"{slug}-{n}")
    _anchor_cache[path] = out
    return out


def check_links(path: Path) -> int:
    rel = path.relative_to(REPO).as_posix()
    found = 0
    in_fence = False
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
    except OSError as exc:
        errors(f"unreadable file: {rel}: {exc}")
        return 1
    for i, line in enumerate(lines, 1):
        if FENCE_RE.match(line):
            in_fence = not in_fence
            continue
        if in_fence:
            continue
        for m in LINK_RE.finditer(strip_code_spans(line)):
            target = m.group(1).strip()
            if not target or SCHEME_RE.match(target):
                continue
            if any(c.isspace() for c in target):
                continue
            file_part, _, fragment = target.partition("#")
            if file_part:
                resolved = Path(os.path.normpath(path.parent / file_part))
                if not resolved.exists():
                    errors(f"broken relative link: {rel}:{i}: {target}")
                    found += 1
                    continue
            else:
                resolved = path
            if fragment and resolved.suffix == ".md" and resolved.is_file():
                if fragment not in anchors(resolved):
                    errors(f"broken anchor: {rel}:{i}: {target}")
                    found += 1
    return found


def unnumbered_docs() -> list[Path]:
    out = []
    for p in markdown_files():
        rel = p.relative_to(REPO).as_posix()
        if rel == "README.md" or rel in ("docs/README.md", "docs/architecture.md"):
            out.append(p)
        elif rel.startswith("docs/guides/"):
            out.append(p)
    return out


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--next", action="store_true", help="print the next record number")
    ap.add_argument("--check", action="store_true", help="validate numbered records")
    ap.add_argument("--check-all", action="store_true", help="--check plus unnumbered docs")
    args = ap.parse_args()
    if not (args.next or args.check or args.check_all):
        ap.print_help()
        return 2
    records = find_records()
    if args.next:
        print(f"{next_number():04d}")
    if args.check or args.check_all:
        found = check_names() + check_structure(records)
        found += sum(check_links(r.path) for r in records)
        if args.check_all:
            for p in unnumbered_docs():
                found += check_links(p)
        if found:
            print(f"{found} error(s)")
            return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
