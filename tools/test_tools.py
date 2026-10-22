"""Tests of the verification tools' parts that need no host and no network
(docs/decisions/0031-PLAN-verification-tools-in-repository.md, "Tests").
Run by `make check-tools`. Every pattern and name here is made up."""
from __future__ import annotations

import json
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


class RemoteTest(unittest.TestCase):
    def test_flags_quoted_for_the_remote_shell(self) -> None:
        import remote
        cmd = remote.binary_command("acp-tools-ab12", "pkg.test", ["-test.run=TestKill$|TestKillAfterExit$", "-test.count=2"])
        self.assertEqual(cmd, "cd acp-tools-ab12 && ./pkg.test '-test.run=TestKill$|TestKillAfterExit$' -test.count=2")
        self.assertEqual(remote.binary_command("d", "pkg.test", []), "cd d && ./pkg.test")

    def test_fresh_relative_directory(self) -> None:
        import remote
        a, b = remote.remote_dir(), remote.remote_dir()
        self.assertNotEqual(a, b)
        self.assertTrue(a.startswith("acp-tools-") and "/" not in a)


class LeftoversTest(unittest.TestCase):
    LIST = ("Image Name                     PID Session Name        Session#    Mem Usage\n"
            "========================= ======== ================ =========== ============\n"
            "tasklist.exe                 4812 Services                   0     12,312 K\n"
            "svchost.exe                  2604 Services                   0     57,076 K\n"
            "agentproc.test.exe           7310 Services                   0     20,100 K\n"
            "t.exe                        7311 Services                   0      9,000 K\n")

    def test_exact_image_names(self) -> None:
        import leftovers
        rows = leftovers.leftover_rows(self.LIST, set())
        self.assertEqual([r.split()[0] for r in rows], ["agentproc.test.exe"])
        rows = leftovers.leftover_rows(self.LIST, {"t.exe"})
        self.assertEqual([r.split()[0] for r in rows], ["agentproc.test.exe", "t.exe"], "tasklist.exe must not match t.exe")


class PlantsTest(Temp):
    """plants.py against a one-test Go module."""

    def setUp(self) -> None:
        super().setUp()
        self.tree = self.dir / "tree"
        self.tree.mkdir()
        (self.tree / "go.mod").write_text("module example.invalid/m\n\ngo 1.26\n")
        (self.tree / "m.go").write_text("package m\n\n// Add adds.\nfunc Add(a, b int) int { return a + b }\n")
        (self.tree / "m_test.go").write_text('package m\n\nimport "testing"\n\nfunc TestAdd(t *testing.T) {\n'
                                             '\tif Add(2, 2) != 4 {\n\t\tt.Fatal("Add is wrong")\n\t}\n}\n')
        (self.tree / ".git").mkdir()
        (self.tree / ".git" / "HEAD").write_text("ref: refs/heads/main\n")
        self.env = {"ACP_TOOLS_CACHE": str(self.dir / "cache")}

    def plants(self, *plants: dict, extra: tuple[str, ...] = ()) -> subprocess.CompletedProcess[str]:
        spec = self.dir / "spec.json"
        spec.write_text(json.dumps(list(plants)))
        return tool("plants", str(spec), "--tree", str(self.tree), "--out", str(self.dir / "out"), *extra, env=self.env)

    @staticmethod
    def plant(name: str, old: str, new: str, want: str = "Add is wrong") -> dict:
        return {"name": name, "edits": [{"file": "m.go", "old": old, "new": new}], "pkg": ".", "run": "^TestAdd$", "want": want}

    def test_fails_as_required(self) -> None:
        r = self.plants(self.plant("subtract", "return a + b", "return a - b"))
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn("FAILED AS REQUIRED: subtract", r.stdout)
        self.assertIn("1/1 as required", r.stdout)

    def test_edit_that_does_not_apply(self) -> None:
        r = self.plants(self.plant("absent", "return a * b", "return 0"))
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        self.assertIn("refused: m.go: the text to replace occurs 0 time(s)", r.stdout)

    def test_test_that_still_passes(self) -> None:
        r = self.plants(self.plant("comment", "// Add adds.", "// Add sums."))
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        self.assertIn("NOT AS REQUIRED: comment", r.stdout)

    def test_want_text_absent(self) -> None:
        r = self.plants(self.plant("wrong text", "return a + b", "return a - b", want="some other failure"))
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)

    def test_spec_refused(self) -> None:
        r = self.plants({"name": "x", "edits": []})
        self.assertEqual(r.returncode, 2, r.stdout + r.stderr)
        self.assertIn("lacks", r.stdout)

    def test_copy_outside_the_tree_without_git(self) -> None:
        r = self.plants(self.plant("subtract", "return a + b", "return a - b"), extra=("--keep",))
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        copies = [p for p in (self.dir / "cache" / "plants").glob("run-*/subtract")]
        self.assertEqual(len(copies), 1, copies)
        self.assertFalse((copies[0] / ".git").exists(), "the copy must leave .git out")
        self.assertIn("return a - b", (copies[0] / "m.go").read_text())
        self.assertIn("return a + b", (self.tree / "m.go").read_text(), "the tree itself must be unchanged")


class HostTest(unittest.TestCase):
    def test_unset_names_the_variable(self) -> None:
        saved = os.environ.pop("ACP_LINUX_HOST", None)
        self.addCleanup(lambda: saved is not None and os.environ.__setitem__("ACP_LINUX_HOST", saved))
        with self.assertRaises(SystemExit) as cm:
            _common.host("linux")
        self.assertIn("ACP_LINUX_HOST", str(cm.exception))


if __name__ == "__main__":
    unittest.main()
