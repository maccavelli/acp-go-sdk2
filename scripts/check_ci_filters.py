#!/usr/bin/env python3
"""Check that CI's path filters and the Docs workflow agree (0037-PLAN-align-docs-ci-and-disclosure-gates.md).

It reads .github/workflows/ci.yml and docs.yml as YAML, with the reader below,
and requires:

  filters   CI's push and pull_request paths-ignore lists are equal and not
            empty; CI still runs on v* tags and on its schedule
  docs      Docs' push and pull_request paths are equal, and are CI's ignored
            paths and docs.yml itself, no more and no fewer
  tracked   each ignored path matches a tracked file, and each tracked file
            CI ignores, Docs runs on
  readers   no tracked Markdown file that a Go file names in a string literal,
            as docs_test.go names README.md, is ignored by CI
  gates     each of DOC_GATES runs in a step of both workflows, and neither
            workflow pins markdownlint-cli2 itself: the Makefile does
  pins      both workflows check out with the same actions/checkout pin, and
            read the repository only (permissions: contents: read)

The reader is the subset of YAML the workflows use, so the check needs only the
standard library: block mappings and sequences, plain and quoted scalars,
flow sequences of scalars, block scalars (| and >), and comments. Anything
else, an anchor, an alias, a tag, a flow mapping or a second document, is
refused rather than guessed at.

Usage: python3 scripts/check_ci_filters.py [--root DIR]
Exit 0 when the contract holds, 1 when it does not, 2 when a file cannot be read.
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
from pathlib import Path

DOC_GATES = ("make markdownlint", "make check-records", "make disclosure-check")
DOCS_WORKFLOW = ".github/workflows/docs.yml"


class YAMLError(ValueError):
    """A workflow outside the YAML subset the reader handles."""


def _strip_comment(s: str) -> str:
    """s without a comment: a # at its start or after a space, outside quotes."""
    quote = ""
    for i, c in enumerate(s):
        if quote:
            if c == quote:
                quote = ""
        elif c in "'\"" and (i == 0 or s[i - 1] in " [,:"):
            quote = c
        elif c == "#" and (i == 0 or s[i - 1] in " \t"):
            return s[:i].rstrip()
    return s.rstrip()


def _split_flow(s: str) -> list[str]:
    """The items of a flow sequence's inside, split at commas outside quotes."""
    items, cur, quote = [], "", ""
    for c in s:
        if quote:
            cur += c
            if c == quote:
                quote = ""
        elif c in "'\"":
            quote = c
            cur += c
        elif c == ",":
            items.append(cur.strip())
            cur = ""
        else:
            cur += c
    if quote:
        raise YAMLError(f"unclosed quote in [{s}]")
    if cur.strip():
        items.append(cur.strip())
    return items


def scalar(s: str):
    """A scalar or flow sequence of the subset."""
    s = s.strip()
    if s.startswith("[") and s.endswith("]"):
        return [scalar(x) for x in _split_flow(s[1:-1])]
    if s.startswith("'") and s.endswith("'") and len(s) >= 2:
        return s[1:-1].replace("''", "'")
    if s.startswith('"') and s.endswith('"') and len(s) >= 2:
        return s[1:-1].replace('\\"', '"').replace("\\\\", "\\")
    if s[:1] in ("{", "&", "*", "!", "%", "@", "`") or s.startswith(("'", '"')):
        raise YAMLError(f"outside the subset: {s[:40]}")
    return s


def _key(s: str) -> tuple[str, str] | None:
    """A mapping entry's key and rest, or None when s is no entry."""
    m = re.match(r"^([A-Za-z0-9_.-]+):(?:\s+(.*))?$", s)
    return (m.group(1), m.group(2) or "") if m else None


def load(text: str):
    """The value of a workflow file in the subset."""
    rows = []
    raw = text.split("\n")
    i = 0
    while i < len(raw):
        line = raw[i]
        if line.strip() in ("---", "..."):
            raise YAMLError("a document marker")
        if "\t" in line[: len(line) - len(line.lstrip())]:
            raise YAMLError(f"line {i + 1}: a tab in indentation")
        body = _strip_comment(line)
        if body.strip():
            rows.append([len(body) - len(body.lstrip()), body.strip(), i])
        i += 1
    value, end = _block(rows, 0, 0, raw)
    if end != len(rows):
        raise YAMLError(f"line {rows[end][2] + 1}: unexpected indentation")
    return value


def _block_scalar(raw: list[str], rows, j: int, indent: int, style: str) -> tuple[str, int]:
    """A block scalar after rows[j-1], whose lines are indented more than indent."""
    start = rows[j - 1][2] + 1
    k = start
    lines = []
    while k < len(raw) and (not raw[k].strip() or len(raw[k]) - len(raw[k].lstrip()) > indent):
        lines.append(raw[k])
        k += 1
    while lines and not lines[-1].strip():
        lines.pop()
    pad = min((len(x) - len(x.lstrip()) for x in lines if x.strip()), default=0)
    text = "\n".join(x[pad:] for x in lines)
    if style.startswith(">"):
        text = " ".join(x.strip() for x in text.split("\n"))
    while j < len(rows) and rows[j][2] < k:
        j += 1
    return text + ("" if style.endswith("-") else "\n"), j


def _value(rows, j: int, indent: int, rest: str, raw) -> tuple[object, int]:
    """The value after a key or a dash: rest, a block scalar, or a nested block."""
    if rest in ("|", "|-", ">", ">-"):
        return _block_scalar(raw, rows, j, indent, rest)
    if rest:
        return scalar(rest), j
    if j < len(rows) and rows[j][0] > indent:
        return _block(rows, j, rows[j][0], raw)
    if j < len(rows) and rows[j][0] == indent and rows[j][1].startswith("- "):
        return _block(rows, j, indent, raw)
    return None, j


def _block(rows, j: int, indent: int, raw) -> tuple[object, int]:
    """The block mapping or sequence at rows[j], indented indent."""
    if rows[j][1] == "-" or rows[j][1].startswith("- "):
        out = []
        while j < len(rows) and rows[j][0] == indent and (rows[j][1] == "-" or rows[j][1].startswith("- ")):
            rest = rows[j][1][1:].strip()
            kv = _key(rest)
            if kv and not rest.startswith(("'", '"', "[")):
                inner = indent + 2
                rows[j] = [inner, rest, rows[j][2]]
                item, j = _block(rows, j, inner, raw)
                out.append(item)
            else:
                item, j = _value(rows, j + 1, indent, rest, raw)
                out.append(item)
        return out, j
    out = {}
    while j < len(rows) and rows[j][0] == indent:
        kv = _key(rows[j][1])
        if not kv:
            raise YAMLError(f"line {rows[j][2] + 1}: not a mapping entry")
        key, rest = kv
        if key in out:
            raise YAMLError(f"line {rows[j][2] + 1}: duplicate key {key}")
        out[key], j = _value(rows, j + 1, indent, rest, raw)
    return out, j


def glob_re(pattern: str) -> re.Pattern:
    """A GitHub path filter as a regular expression: ** any path, * a name, ? a character."""
    out, i = "", 0
    while i < len(pattern):
        if pattern.startswith("**", i):
            out += ".*"
            i += 2
        elif pattern[i] == "*":
            out += "[^/]*"
            i += 1
        elif pattern[i] == "?":
            out += "[^/]"
            i += 1
        else:
            out += re.escape(pattern[i])
            i += 1
    return re.compile(out + r"\Z")


def _matches(patterns: list[str], path: str) -> bool:
    return any(glob_re(p).match(path) for p in patterns)


def _runs(workflow) -> list[str]:
    """Each line of each step's run, stripped."""
    out = []
    for job in (workflow.get("jobs") or {}).values():
        for step in job.get("steps") or []:
            if isinstance(step, dict) and isinstance(step.get("run"), str):
                out.extend(x.strip() for x in step["run"].split("\n"))
    return out


def _uses(workflow, action: str) -> set[str]:
    out = set()
    for job in (workflow.get("jobs") or {}).values():
        for step in job.get("steps") or []:
            if isinstance(step, dict) and str(step.get("uses", "")).startswith(action + "@"):
                out.add(step["uses"])
    return out


def code_read_markdown(root: Path, tracked: list[str]) -> set[str]:
    """Tracked Markdown files a tracked Go file names in a string literal."""
    md = {t for t in tracked if t.endswith(".md")}
    out = set()
    for t in tracked:
        if not t.endswith(".go"):
            continue
        for m in re.finditer(r'"([^"\n]+\.md)"', (root / t).read_text(encoding="utf-8")):
            name = (Path(t).parent / m.group(1)).as_posix().removeprefix("./")
            if name in md:
                out.add(name)
            elif m.group(1) in md:
                out.add(m.group(1))
    return out


def check(root: Path, tracked: list[str]) -> list[str]:
    """The contract's failures, each naming its part."""
    ci = load((root / ".github/workflows/ci.yml").read_text(encoding="utf-8"))
    docs = load((root / DOCS_WORKFLOW).read_text(encoding="utf-8"))
    problems = []
    on, don = ci.get("on") or {}, docs.get("on") or {}
    push, pr = on.get("push") or {}, on.get("pull_request") or {}
    ignore = push.get("paths-ignore") or []
    if not ignore or ignore != (pr.get("paths-ignore") or []):
        problems.append("filters: CI's push and pull_request paths-ignore differ, or are empty")
    if "paths" in push or "paths" in pr:
        problems.append("filters: CI has a paths filter as well as paths-ignore")
    if "v*" not in (push.get("tags") or []) or not on.get("schedule"):
        problems.append("filters: CI no longer runs on v* tags or on its schedule")
    dpush, dpr = (don.get("push") or {}).get("paths") or [], (don.get("pull_request") or {}).get("paths") or []
    if dpush != dpr:
        problems.append("docs: Docs' push and pull_request paths differ")
    want = set(ignore) | {DOCS_WORKFLOW}
    if set(dpush) != want:
        problems.append(f"docs: Docs' paths are not CI's ignored paths and {DOCS_WORKFLOW}: "
                        f"missing {sorted(want - set(dpush))}, extra {sorted(set(dpush) - want)}")
    for p in ignore:
        if not any(glob_re(p).match(t) for t in tracked):
            problems.append(f"tracked: CI ignores {p!r}, which matches no tracked file")
    for t in tracked:
        if _matches(ignore, t) and not _matches(dpush, t):
            problems.append(f"tracked: CI ignores {t}, and Docs does not run on it")
    for t in sorted(code_read_markdown(root, tracked)):
        if _matches(ignore, t):
            problems.append(f"readers: CI ignores {t}, which Go code reads")
    for name, wf in (("CI", ci), ("Docs", docs)):
        runs = _runs(wf)
        for gate in DOC_GATES:
            if gate not in runs:
                problems.append(f"gates: {name} does not run {gate}")
        if any("markdownlint-cli2@" in r for r in runs):
            problems.append(f"gates: {name} pins markdownlint-cli2 itself, not through make markdownlint")
        if wf.get("permissions") != {"contents": "read"}:
            problems.append(f"pins: {name}'s permissions are not contents: read")
    cpin, dpin = _uses(ci, "actions/checkout"), _uses(docs, "actions/checkout")
    if not cpin or cpin != dpin:
        problems.append(f"pins: actions/checkout differs, CI {sorted(cpin)}, Docs {sorted(dpin)}")
    return problems


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--root", type=Path, default=Path(__file__).resolve().parent.parent)
    args = ap.parse_args(argv)
    p = subprocess.run(["git", "-C", str(args.root), "ls-files", "-z"], capture_output=True, check=False)
    if p.returncode != 0:
        print("check-ci-filters: git ls-files failed")
        return 2
    tracked = [n for n in p.stdout.decode("utf-8").split("\0") if n]
    try:
        problems = check(args.root, tracked)
    except (OSError, UnicodeDecodeError, YAMLError) as e:
        print(f"check-ci-filters: {e}")
        return 2
    for x in problems:
        print(f"check-ci-filters: {x}")
    print(f"check-ci-filters: {len(problems)} problem(s)")
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
