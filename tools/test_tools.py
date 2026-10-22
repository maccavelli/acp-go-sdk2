"""Tests of the verification tools' parts that need no host and no network
(docs/decisions/0031-PLAN-verification-tools-in-repository.md, "Tests").
Run by `make check-tools`. Every pattern and name here is made up."""
from __future__ import annotations

import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

TOOLS = Path(__file__).resolve().parent
sys.path.insert(0, str(TOOLS))
import _common  # noqa: E402

# A made-up identifier, and a deny-list line for it as the guard writes them.
FAKE = "zq-example-host-7"
DENY_LINE = f"{FAKE}\t<host>\n"


def tool(name: str, *args: str, cwd: Path | None = None, env: dict[str, str] | None = None) -> subprocess.CompletedProcess[str]:
    return subprocess.run([sys.executable, str(TOOLS / f"{name}.py"), *args], cwd=cwd, env={**os.environ, **(env or {})},
                          capture_output=True, text=True, timeout=120)


def git(cwd: Path, *args: str) -> None:
    # Hooks off: a test repository must not run the user's global hooks.
    env = {**os.environ, "GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@example.invalid", "GIT_COMMITTER_NAME": "t",
           "GIT_COMMITTER_EMAIL": "t@example.invalid"}
    subprocess.run(["git", "-c", "core.hooksPath=/dev/null", "-c", "init.defaultBranch=main", *args], cwd=cwd, env=env, check=True,
                   capture_output=True)


class Temp(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.dir = Path(self._tmp.name)
        self.addCleanup(self._tmp.cleanup)


class IdscanTest(Temp):
    def setUp(self) -> None:
        super().setUp()
        self.repo = self.dir / "repo"
        self.repo.mkdir()
        git(self.repo, "init", "-q")
        (self.repo / "clean.txt").write_text("nothing to see\n")
        git(self.repo, "add", ".")
        git(self.repo, "commit", "-q", "-m", "first")
        self.deny = self.dir / "deny"
        self.deny.write_text("# a comment\n\n" + DENY_LINE)
        self.env = {"DISCLOSURE_DENY": str(self.deny)}

    def test_match_reported_by_place_only(self) -> None:
        (self.repo / "a.txt").write_text(f"one\ntwo\nsee {FAKE} here\n")
        git(self.repo, "add", "a.txt")
        r = tool("idscan", "--all", cwd=self.repo, env=self.env)
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        self.assertIn("identifier at a.txt:3", r.stdout)
        self.assertNotIn(FAKE, r.stdout + r.stderr, "the matched text must not be printed")

    def test_no_match(self) -> None:
        r = tool("idscan", "--all", cwd=self.repo, env=self.env)
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn("0 match(es)", r.stdout)

    def test_changed_files_include_untracked(self) -> None:
        (self.repo / "new.txt").write_text(f"{FAKE}\n")
        r = tool("idscan", cwd=self.repo, env=self.env)
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        self.assertIn("identifier at new.txt:1", r.stdout)

    def test_range_reads_the_commit(self) -> None:
        (self.repo / "b.txt").write_text(f"x\n{FAKE}\n")
        git(self.repo, "add", "b.txt")
        git(self.repo, "commit", "-q", "-m", "second")
        (self.repo / "b.txt").write_text("cleaned in the tree only\n")
        r = tool("idscan", "--range", "HEAD~1..HEAD", cwd=self.repo, env=self.env)
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        self.assertIn("identifier at b.txt:2", r.stdout)

    def test_paths(self) -> None:
        f = self.dir / "body.md"
        f.write_text(f"{FAKE}\n")
        r = tool("idscan", "--paths", str(f), cwd=self.repo, env=self.env)
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)

    def test_missing_or_empty_list(self) -> None:
        r = tool("idscan", "--all", cwd=self.repo, env={"DISCLOSURE_DENY": str(self.dir / "absent")})
        self.assertEqual(r.returncode, 2, r.stdout + r.stderr)
        self.deny.write_text("# only a comment\n")
        r = tool("idscan", "--all", cwd=self.repo, env=self.env)
        self.assertEqual(r.returncode, 2, r.stdout + r.stderr)


class FindTest(Temp):
    def test_lines(self) -> None:
        (self.dir / "x.go").write_text("package x\n\n// the needle here\n")
        r = tool("find", "lines", str(self.dir / "*.go"), "--", "needle")
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn("x.go:3: // the needle here", r.stdout)
        r = tool("find", "lines", str(self.dir / "*.go"), "--", "absent text")
        self.assertEqual(r.returncode, 1)

    def test_files(self) -> None:
        (self.dir / "sub").mkdir()
        (self.dir / "sub" / "a_test.go").write_text("")
        (self.dir / ".git").mkdir()
        (self.dir / ".git" / "b_test.go").write_text("")
        r = tool("find", "files", str(self.dir), "*_test.go")
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertEqual(r.stdout.split(), [str(Path("sub") / "a_test.go")])

    def test_usage(self) -> None:
        self.assertEqual(tool("find", "lines", "x").returncode, 2)


class ProcsTest(unittest.TestCase):
    def test_finds_a_process_and_not_itself(self) -> None:
        token = "zq-procs-token-41"
        r = tool("procs", token)
        self.assertEqual(r.returncode, 0, r.stdout)
        sleeper = subprocess.Popen([sys.executable, "-c", "import time; time.sleep(60)", token])
        self.addCleanup(sleeper.wait)
        self.addCleanup(sleeper.kill)
        r = tool("procs", token)
        self.assertEqual(r.returncode, 1, r.stdout)
        self.assertIn(str(sleeper.pid), r.stdout.split())


class GatesTest(unittest.TestCase):
    def test_output_fails_a_diff_gate(self) -> None:
        import gates
        self.assertEqual(gates.fails_on_output(0, ""), 0)
        self.assertEqual(gates.fails_on_output(0, "  \n"), 0)
        self.assertNotEqual(gates.fails_on_output(0, "--- a/x.go\n+++ b/x.go\n"), 0, "output on exit 0 must fail")
        self.assertEqual(gates.fails_on_output(3, ""), 3)

    def test_selection(self) -> None:
        import gates
        default = gates.selected([], False)
        self.assertNotIn("windows", default)
        self.assertNotIn("linux", default)
        self.assertEqual(gates.selected([], True), gates.ORDER)
        self.assertEqual(gates.selected(["api", "fix"], False), ["api", "fix"])
        with self.assertRaises(SystemExit):
            gates.selected(["nope"], False)

    def test_every_gate_is_defined(self) -> None:
        import gates
        for name in gates.ORDER:
            self.assertTrue(callable(getattr(gates.Gates, name, None)), name)


class CacheTest(Temp):
    def test_default_is_outside_the_repository(self) -> None:
        saved = os.environ.pop("ACP_TOOLS_CACHE", None)
        self.addCleanup(lambda: saved is not None and os.environ.__setitem__("ACP_TOOLS_CACHE", saved))
        d = _common.cache_dir("zq-test-probe")
        self.addCleanup(d.rmdir)
        self.assertFalse(d.resolve().is_relative_to(_common.repo().resolve()), f"{d} is inside the repository")
        self.assertEqual(d.name, "zq-test-probe")

    def test_override(self) -> None:
        saved = os.environ.get("ACP_TOOLS_CACHE")
        os.environ["ACP_TOOLS_CACHE"] = str(self.dir / "c")
        self.addCleanup(lambda: os.environ.__setitem__("ACP_TOOLS_CACHE", saved) if saved is not None else os.environ.pop("ACP_TOOLS_CACHE"))
        self.assertEqual(_common.cache_dir("floor"), self.dir / "c" / "floor")
        self.assertTrue((self.dir / "c" / "floor").is_dir())

    def test_floor_caches_in_the_cache(self) -> None:
        import floor
        env = floor.floor_env("1.26.10", self.dir / "cache", {"GOFLAGS": "-x", "GOTOOLCHAIN": "local", "PATH": "p"})
        self.assertEqual(env["GOTOOLCHAIN"], "go1.26.10")
        for key in ("GOPATH", "GOMODCACHE", "GOCACHE"):
            self.assertTrue(Path(env[key]).is_relative_to(self.dir / "cache"), f"{key}={env[key]}")
        self.assertEqual(env["GOFLAGS"], "-modcacherw")
        self.assertEqual(env["PATH"], "p")


class FloorTest(Temp):
    def test_go_line(self) -> None:
        import floor
        mod = self.dir / "go.mod"
        mod.write_text("module example.invalid/m\n\ngo 1.26.10\n\ntoolchain go1.27.3\n")
        self.assertEqual(floor.go_line(mod), "1.26.10")
        mod.write_text("module example.invalid/m\n")
        with self.assertRaises(SystemExit):
            floor.go_line(mod)


class HostTest(unittest.TestCase):
    def test_unset_names_the_variable(self) -> None:
        saved = os.environ.pop("ACP_LINUX_HOST", None)
        self.addCleanup(lambda: saved is not None and os.environ.__setitem__("ACP_LINUX_HOST", saved))
        with self.assertRaises(SystemExit) as cm:
            _common.host("linux")
        self.assertIn("ACP_LINUX_HOST", str(cm.exception))


if __name__ == "__main__":
    unittest.main()
