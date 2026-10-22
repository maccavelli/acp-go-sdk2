"""Extract a GitHub release's body from its release-notes report
(docs/decisions/0031-MADR-verification-tools-in-repository.md).

The report is docs/reports/NNNN-REPORT-release-notes-vX-Y-Z.md as at REV. The
body is the text under its "### vX.Y.Z" heading, up to "## Sources not in the
release text", each relative link made absolute at the tag, on the origin
remote's GitHub repository; each link's target must exist at REV. The body is
then scanned with idscan.py, and written to <out>/release-body-vX.Y.Z.md.

Usage: release_body.py VERSION REV [--out DIR]
Exit 0 with the body written; 1 when a target is missing or the scan matches.
"""
from __future__ import annotations

import argparse
import posixpath
import re
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _common  # noqa: E402

TOOLS = Path(__file__).resolve().parent
END = "\n## Sources not in the release text"
LINK = re.compile(r"\]\(((?!https?://|#|mailto:)[^)\s]+)\)")


def git(root: Path, *args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(["git", *args], cwd=root, capture_output=True, text=True)


def report_path(root: Path, version: str, rev: str) -> str:
    slug = "release-notes-" + version.replace(".", "-")
    names = git(root, "ls-tree", "--name-only", rev, "docs/reports/").stdout.split()
    hits = [n for n in names if re.fullmatch(rf"docs/reports/\d{{4}}-REPORT-{re.escape(slug)}\.md", n)]
    if len(hits) != 1:
        raise SystemExit(f"release_body: want one report for {version} at {rev}, found {hits}")
    return hits[0]


def body(root: Path, version: str, rev: str, repo: str) -> tuple[str, list[str]]:
    """The body, and the link targets missing at rev."""
    rel = report_path(root, version, rev)
    text = git(root, "show", f"{rev}:{rel}").stdout
    head = f"### {version}\n"
    if head not in text or END not in text:
        raise SystemExit(f"release_body: {rel} has no {head.strip()!r} section ending at {END.strip()!r}")
    start = text.index(head) + len(head)
    out = text[start:text.index(END, start)].strip("\n") + "\n"
    base = posixpath.dirname(rel)
    missing: list[str] = []

    def absolute(m: re.Match[str]) -> str:
        target, _, frag = m.group(1).partition("#")
        path = posixpath.normpath(posixpath.join(base, target))
        if git(root, "cat-file", "-e", f"{rev}:{path}").returncode != 0:
            missing.append(path)
        return f"](https://github.com/{repo}/blob/{version}/{path}{'#' + frag if frag else ''})"

    return LINK.sub(absolute, out), missing


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("version")
    ap.add_argument("rev")
    ap.add_argument("--out", help="the output directory (default .cache/tools/release)")
    args = ap.parse_args(argv)
    root = _common.repo(Path.cwd())
    text, missing = body(root, args.version, args.rev, _common.origin_repo(root))
    for m in missing:
        print(f"release_body: the link target {m} does not exist at {args.rev}")
    out = _common.out_dir("release", args.out) / f"release-body-{args.version}.md"
    out.write_text(text, encoding="utf-8")
    scan = subprocess.run([sys.executable, "-B", str(TOOLS / "idscan.py"), "--paths", str(out)], capture_output=True, text=True)
    print(scan.stdout.strip())
    links = re.findall(r"\]\((https?://[^)\s]+)\)", text)
    print(f"release_body: {len(text.splitlines())} lines, {len(links)} links, written to {out}")
    return 1 if missing or scan.returncode else 0


if __name__ == "__main__":
    sys.exit(_common.main_guard(main, sys.argv[1:]))
