"""Tests of check_api.py (0039-PLAN-five-adopter-additions-on-schema-1-25-0.md, 2e).

Each test builds a scratch module, a git repository with two v1 tags, and
compares its working tree with the newer one through the pinned apidiff, run
with `go run` as the gate runs it: they need Go, and the network only until
the tool is in the module cache.
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
import check_api  # noqa: E402

MODULE = "example.invalid/m"


def go_line() -> str:
    """The go line of this repository's go.mod, so the scratch module builds with the toolchain the gate uses."""
    for line in (check_api.ROOT / "go.mod").read_text(encoding="utf-8").splitlines():
        if line.startswith("go "):
            return line
    raise AssertionError("go.mod has no go line")


class CompareTest(unittest.TestCase):
    def setUp(self) -> None:
        self.root = Path(tempfile.mkdtemp())
        self.addCleanup(lambda: subprocess.run(["rm", "-rf", str(self.root)], check=False))
        self.git("init", "-q")
        self.git("config", "user.email", "test@example.invalid")
        self.git("config", "user.name", "test")
        self.write("go.mod", f"module {MODULE}\n\n{go_line()}\n")
        self.write("m.go", "package m\n\nfunc A() {}\n")
        self.write("rep/rep.go", "package rep\n\nfunc R() {}\n")
        self.commit("v1.9.0")
        self.write("m.go", "package m\n\nfunc A() {}\n\n// T is a type.\ntype T struct{ X int }\n")
        self.commit("v1.10.0")

    def git(self, *args: str) -> None:
        subprocess.run(["git", "-C", str(self.root), *args], check=True, capture_output=True)

    def write(self, rel: str, text: str) -> None:
        path = self.root / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")

    def commit(self, tag: str) -> None:
        self.git("add", "-A")
        self.git("commit", "-q", "-m", tag)
        self.git("tag", tag)

    def compare(self, gated: tuple[str, ...], reported: tuple[str, ...]) -> tuple[int, str]:
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            status = check_api.compare(self.root, check_api.latest_release(self.root), gated, reported)
        return status, out.getvalue()

    def test_newest_tag_by_number(self) -> None:
        self.assertEqual(check_api.latest_release(self.root), "v1.10.0")

    def test_compatible_change_passes(self) -> None:
        self.write("m.go", "package m\n\nfunc A() {}\n\n// T is a type.\ntype T struct{ X int }\n\n// B is new.\nfunc B() {}\n")
        status, out = self.compare(("",), ("rep",))
        self.assertEqual(status, 0, out)
        self.assertIn("check_api: compatible: B: added\n", out)
        self.assertIn("check_api: against v1.10.0: 0 incompatible, 1 compatible\n", out)

    def test_incompatible_change_in_a_gated_package_fails(self) -> None:
        self.write("m.go", "package m\n\n// T is a type.\ntype T struct{ X int }\n")
        status, out = self.compare(("",), ("rep",))
        self.assertEqual(status, 1, out)
        self.assertIn("check_api: incompatible: A: removed\n", out)

    def test_incompatible_change_in_a_reported_package_is_listed_and_passes(self) -> None:
        self.write("rep/rep.go", "package rep\n\n// S replaces R.\nfunc S() {}\n")
        status, out = self.compare(("",), ("rep",))
        self.assertEqual(status, 0, out)
        self.assertIn("check_api: incompatible, reported, not gated: rep: R: removed\n", out)
        self.assertIn("check_api: compatible: rep: S: added\n", out)
        self.assertIn("check_api: against v1.10.0: 0 incompatible, 1 compatible\n", out)
        self.assertIn("check_api: reported packages rep: 1 incompatible, not gated\n", out)

    def test_the_same_change_gated_fails(self) -> None:
        self.write("rep/rep.go", "package rep\n\n// S replaces R.\nfunc S() {}\n")
        status, out = self.compare(("", "rep"), ())
        self.assertEqual(status, 1, out)
        self.assertIn("check_api: incompatible: rep: R: removed\n", out)
        self.assertNotIn("reported", out)

    def test_package_new_since_the_tag_is_compatible(self) -> None:
        self.write("neu/neu.go", "package neu\n\n// N is new.\nfunc N() {}\n")
        status, out = self.compare(("", "neu"), ("rep",))
        self.assertEqual(status, 0, out)
        self.assertIn("check_api: compatible: neu: N: added\n", out)

    def test_tool_errors(self) -> None:
        for name, gated, reported in (("in both lists", ("", "rep"), ("rep",)), ("missing from the tree", ("", "gone"), ())):
            with self.subTest(name=name), contextlib.redirect_stderr(io.StringIO()):
                with self.assertRaises(SystemExit) as raised:
                    self.compare(gated, reported)
                self.assertEqual(raised.exception.code, 2)


class PackagesTest(unittest.TestCase):
    def test_unstable_is_reported_and_not_gated(self) -> None:
        # Package unstable may change in a minor release
        # (0039-PLAN-five-adopter-additions-on-schema-1-25-0.md, 2f).
        self.assertIn("unstable", check_api.REPORTED)
        self.assertNotIn("unstable", check_api.PACKAGES)
        self.assertIn("", check_api.PACKAGES)
        self.assertIn("agentproc", check_api.PACKAGES)


if __name__ == "__main__":
    unittest.main()
