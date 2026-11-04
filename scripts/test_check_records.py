"""Tests of check_records.py: the files it reads, and the docs whose links it
checks (0049-PLAN-tooling-fixes.md, phase 2, T1 and T3).

Each test copies the script into a temporary repository of its own, since
the script reads the repository it sits in."""

from __future__ import annotations

import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).resolve().parent / "check_records.py"


class CheckRecordsTest(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.root = Path(self._tmp.name) / "repo"
        (self.root / "scripts").mkdir(parents=True)
        shutil.copy2(SCRIPT, self.root / "scripts" / "check_records.py")
        self.write("docs/decisions/0001-MADR-first.md", "# First\n")
        # No repository above the temporary one may answer for it.
        self.env = {**os.environ, "GIT_CEILING_DIRECTORIES": str(self.root.parent)}

    def write(self, rel: str, text: str) -> None:
        p = self.root / rel
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(text, encoding="utf-8")

    def git_init(self, ignore: str = "") -> None:
        subprocess.run(["git", "-c", "core.hooksPath=/dev/null", "init", "-q"], cwd=self.root, check=True, env=self.env)
        if ignore:
            self.write(".gitignore", ignore)

    def check(self, *args: str) -> subprocess.CompletedProcess[str]:
        return subprocess.run([sys.executable, "-B", str(self.root / "scripts" / "check_records.py"), *args], cwd=self.root,
                              capture_output=True, text=True, env=self.env, timeout=60)

    def test_links_of_the_docs_safe_docs(self) -> None:
        self.git_init()
        for rel in ("RELEASING.md", "tools/README.md", "AGENTS.md"):
            with self.subTest(doc=rel):
                self.write(rel, "See [the missing file](missing.md).\n")
                r = self.check("--check-all")
                self.assertEqual(r.returncode, 1, r.stdout)
                self.assertIn(f"broken relative link: {rel}:1: missing.md", r.stdout)
                self.write(rel, "No link.\n")
        self.assertEqual(self.check("--check-all").returncode, 0)

    def test_ignored_directories_are_not_read(self) -> None:
        self.git_init(".worktrees/\n.cache/\n")
        self.write(".worktrees/copy/docs/decisions/0001-MADR-first.md", "# First\n")
        self.write(".worktrees/copy/docs/decisions/0099-MADR-later.md", "# Later\n")
        self.write(".cache/notes/0098-PLAN-stray.md", "# Stray\n")
        r = self.check("--check-all", "--next")
        self.assertEqual(r.returncode, 0, r.stdout)
        self.assertNotIn("claimed by 2 MADRs", r.stdout)
        self.assertEqual(r.stdout.strip().splitlines()[0], "0002", "an ignored copy's numbers are not the repository's")

    def test_outside_a_work_tree(self) -> None:
        self.write("RELEASING.md", "See [the missing file](missing.md).\n")
        r = self.check("--check-all", "--next")
        self.assertEqual(r.returncode, 1)
        self.assertEqual(r.stdout.strip().splitlines()[0], "0002", "the records are still found")
        self.assertIn("broken relative link: RELEASING.md:1: missing.md", r.stdout)


if __name__ == "__main__":
    unittest.main()
