"""Tests of check_ci_filters.py (0037-PLAN-align-docs-ci-and-disclosure-gates.md, phase 2)."""

from __future__ import annotations

import contextlib
import io
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import check_ci_filters as c  # noqa: E402

PIN = "actions/checkout@0123456789abcdef0123456789abcdef01234567 # v7.0.1"
CI = """name: CI
on:
  push:
    branches: [main]
    tags: ['v*']
    paths-ignore:
      - 'docs/**'
      - 'AGENTS.md'
  pull_request:
    paths-ignore:
      - 'docs/**'
      - 'AGENTS.md'
  schedule:
    - cron: '17 6 * * 1'
permissions:
  contents: read
jobs:
  validate:
    runs-on: ubuntu-24.04
    steps:
      - uses: {pin}
      - run: go test ./...
      - name: docs
        run: |
          set -euo pipefail
          make markdownlint
          make check-records
          make disclosure-check
"""
DOCS = """name: Docs
on:
  push:
    branches: [main]
    paths:
      - 'docs/**'
      - 'AGENTS.md'
      - '.github/workflows/docs.yml'
  pull_request:
    paths:
      - 'docs/**'
      - 'AGENTS.md'
      - '.github/workflows/docs.yml'
permissions:
  contents: read
jobs:
  docs:
    runs-on: ubuntu-24.04
    steps:
      - uses: {pin}
      - run: |
          make markdownlint
          make check-records
          make disclosure-check
"""
FILES = {"docs/a.md": "# a\n", "AGENTS.md": "# agents\n", "README.md": "# readme\n",
         "docs_test.go": 'package x\n\nvar _ = "README.md"\n'}


class LoadTest(unittest.TestCase):
    def test_subset(self) -> None:
        got = c.load("a: [x, 'y z', \"w\"]  # note\nb:\n  - k: 1\n    m: two # c\n  - plain\nrun: |\n  one\n  # kept\n\n  two\nq: 'it''s'\n")
        self.assertEqual(got, {"a": ["x", "y z", "w"], "b": [{"k": "1", "m": "two"}, "plain"], "run": "one\n# kept\n\ntwo\n",
                               "q": "it's"})

    def test_sequence_at_key_indent(self) -> None:
        self.assertEqual(c.load("a:\n- x\n- y\nb: z\n"), {"a": ["x", "y"], "b": "z"})

    def test_refused(self) -> None:
        for text in ("a: &x 1\n", "a: *x\n", "a: {b: 1}\n", "---\na: 1\n", "a: 1\na: 2\n", "a: !tag x\n", "a:\n\t- x\n",
                     "just text\n"):
            with self.subTest(text=text), self.assertRaises(c.YAMLError):
                c.load(text)


class GlobTest(unittest.TestCase):
    def test_globs(self) -> None:
        self.assertTrue(c.glob_re("docs/**").match("docs/x/y.md"))
        self.assertFalse(c.glob_re("docs/**").match("docsx/y.md"))
        self.assertTrue(c.glob_re("*.md").match("a.md"))
        self.assertFalse(c.glob_re("*.md").match("d/a.md"))
        self.assertFalse(c.glob_re("AGENTS.md").match("AGENTS.mdx"))


class CheckTest(unittest.TestCase):
    def setUp(self) -> None:
        self.dir = Path(tempfile.mkdtemp())
        self.addCleanup(shutil.rmtree, self.dir, True)

    def repo(self, ci: str = CI, docs: str = DOCS, files: dict[str, str] | None = None) -> list[str]:
        wf = self.dir / ".github" / "workflows"
        wf.mkdir(parents=True, exist_ok=True)
        (wf / "ci.yml").write_text(ci.replace("{pin}", PIN), encoding="utf-8")
        (wf / "docs.yml").write_text(docs.replace("{pin}", PIN), encoding="utf-8")
        for name, text in (files or FILES).items():
            (self.dir / name).parent.mkdir(parents=True, exist_ok=True)
            (self.dir / name).write_text(text, encoding="utf-8")
        return sorted([*(files or FILES), ".github/workflows/ci.yml", ".github/workflows/docs.yml"])

    def problems(self, **kw) -> list[str]:
        return c.check(self.dir, self.repo(**kw))

    def assert_one(self, part: str, **kw) -> None:
        """The contract fails in part; one break may fail another part too."""
        got = self.problems(**kw)
        self.assertTrue(any(p.startswith(part + ":") for p in got), got)

    def test_docs_message_names_what_is_missing(self) -> None:
        got = self.problems(docs=DOCS.replace("      - 'AGENTS.md'\n", ""))
        self.assertIn("docs: Docs' paths are not CI's ignored paths and .github/workflows/docs.yml: missing ['AGENTS.md'], extra []",
                      got)

    def test_clean(self) -> None:
        self.assertEqual(self.problems(), [])

    def test_mismatched_ignore(self) -> None:
        self.assert_one("filters", ci=CI.replace("      - 'AGENTS.md'\n  pull_request:", "  pull_request:"))

    def test_tags_dropped(self) -> None:
        self.assert_one("filters", ci=CI.replace("    tags: ['v*']\n", ""))

    def test_docs_path_missing(self) -> None:
        self.assert_one("docs", docs=DOCS.replace("      - 'AGENTS.md'\n", ""))

    def test_dead_pattern(self) -> None:
        files = {k: v for k, v in FILES.items() if k != "AGENTS.md"}
        self.assert_one("tracked", files=files)

    def test_reader_input_ignored(self) -> None:
        ci = CI.replace("      - 'AGENTS.md'\n", "      - 'AGENTS.md'\n      - 'README.md'\n")
        docs = DOCS.replace("      - 'AGENTS.md'\n", "      - 'AGENTS.md'\n      - 'README.md'\n")
        self.assertEqual(self.problems(ci=ci, docs=docs), ["readers: CI ignores README.md, which Go code reads"])

    def test_gate_missing(self) -> None:
        self.assertEqual(self.problems(docs=DOCS.replace("          make disclosure-check\n", "")),
                         ["gates: Docs does not run make disclosure-check"])

    def test_markdownlint_pinned_in_workflow(self) -> None:
        self.assert_one("gates", docs=DOCS.replace("make markdownlint", "make markdownlint\n          npx --yes markdownlint-cli2@0.23.2"))

    def test_pin_differs(self) -> None:
        self.assert_one("pins", docs=DOCS.replace("{pin}", "actions/checkout@ffffffffffffffffffffffffffffffffffffffff"))

    def test_permissions(self) -> None:
        self.assert_one("pins", docs=DOCS.replace("  contents: read\n", "  contents: write\n"))


class RepositoryTest(unittest.TestCase):
    def test_this_repository_holds(self) -> None:
        root = Path(__file__).resolve().parent.parent
        if subprocess.run(["git", "-C", str(root), "rev-parse"], capture_output=True, check=False).returncode != 0:
            self.skipTest("not a git checkout")
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            status = c.main(["--root", str(root)])
        self.assertEqual(status, 0, out.getvalue())


if __name__ == "__main__":
    unittest.main()
