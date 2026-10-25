"""Tests of check_disclosure.py (0037-PLAN-align-docs-ci-and-disclosure-gates.md, phase 1).

Every real-looking home path is built at run time from split strings, so this
file passes the check it tests.
"""

from __future__ import annotations

import contextlib
import io
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import check_disclosure  # noqa: E402

NAME = "ali" + "ce"
MAC = "/Us" + "ers/"
LINUX = "/ho" + "me/"
WIN = "C:" + "\\Us" + "ers\\"


class ScanTextTest(unittest.TestCase):
    def test_each_form_is_found_on_its_line(self) -> None:
        cases = [
            (f"x\n{MAC}{NAME}/src\n", 2, "macos-home"),
            (f"{LINUX}{NAME}/project", 1, "linux-home"),
            (f"a\nb\n{WIN}{NAME}\\repo", 3, "windows-home"),
            (f"c:/users/{NAME}/repo", 1, "windows-home"),
            ('{"cwd":"C:\\\\Users\\\\' + NAME + '\\\\repo"}', 1, "windows-home"),
            (f"file://{MAC}{NAME}/x", 1, "macos-home"),
        ]
        for text, line, category in cases:
            with self.subTest(text=text):
                self.assertEqual(check_disclosure.scan_text(text), [(line, category)])

    def test_placeholders_pass(self) -> None:
        for name in ("<user>", "\\u003cuser\\u003e", "$USER", "%USERNAME%", "user", "u", "<name>"):
            for root in (MAC, LINUX, WIN):
                with self.subTest(root=root, name=name):
                    self.assertEqual(check_disclosure.scan_text(f"{root}{name}/x"), [])

    def test_other_paths_pass(self) -> None:
        for text in (f"https://example.invalid{LINUX}{NAME}", f"/tmp{LINUX}{NAME}", f"docs{MAC}{NAME}", "/home", "/Users/"):
            with self.subTest(text=text):
                self.assertEqual(check_disclosure.scan_text(text), [])


class ScanTest(unittest.TestCase):
    def setUp(self) -> None:
        self.dir = Path(tempfile.mkdtemp())
        self.addCleanup(lambda: subprocess.run(["rm", "-rf", str(self.dir)], check=False))

    def test_unreadable_and_binary(self) -> None:
        (self.dir / "latin1.txt").write_bytes(b"caf\xe9\n")
        (self.dir / "bin.dat").write_bytes(b"\0" + f"{MAC}{NAME}".encode())
        found = check_disclosure.scan(self.dir, ["latin1.txt", "bin.dat", "missing.txt"])
        self.assertEqual(found, [("latin1.txt", 0, "unreadable"), ("missing.txt", 0, "unreadable")])

    def git_repo(self, files: dict[str, str]) -> Path:
        for name, text in files.items():
            (self.dir / name).write_text(text, encoding="utf-8")
        subprocess.run(["git", "init", "-q", str(self.dir)], check=True)
        subprocess.run(["git", "-C", str(self.dir), "add", "."], check=True)
        return self.dir

    def run_main(self, root: Path) -> tuple[int, str]:
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            status = check_disclosure.main(["--root", str(root)])
        return status, out.getvalue()

    def test_main_prints_file_line_category_only(self) -> None:
        root = self.git_repo({"a.md": f"ok\n{LINUX}{NAME}/x\n", "b.go": "package b\n"})
        status, out = self.run_main(root)
        self.assertEqual(status, 1)
        self.assertIn("a.md:2: linux-home\n", out)
        self.assertNotIn(NAME, out)
        self.assertIn("2 tracked file(s), 1 finding(s)", out)

    def test_main_passes_clean_tree(self) -> None:
        root = self.git_repo({"a.md": f"{LINUX}<user>/x\n"})
        self.assertEqual(self.run_main(root), (0, "check-disclosure: 1 tracked file(s), 0 finding(s)\n"))

    def test_main_reads_tracked_files_only(self) -> None:
        root = self.git_repo({"a.md": "ok\n"})
        (root / "untracked.md").write_text(f"{MAC}{NAME}\n", encoding="utf-8")
        self.assertEqual(self.run_main(root)[0], 0)


if __name__ == "__main__":
    unittest.main()
