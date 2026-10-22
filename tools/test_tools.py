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

    def test_reader_that_closes_early(self) -> None:
        # Its output piped to a reader that takes one line and closes, as
        # `| head -n 1` does: no traceback (0031-PLAN, step 10).
        big = self.dir / "big.txt"
        big.write_text("needle\n" * 200_000)
        proc = subprocess.Popen([sys.executable, str(TOOLS / "find.py"), "lines", str(big), "--", "needle"], stdout=subprocess.PIPE,
                                stderr=subprocess.PIPE)
        self.assertTrue(proc.stdout.readline())
        proc.stdout.close()
        err = proc.stderr.read().decode()
        proc.stderr.close()
        status = proc.wait(timeout=60)
        self.assertNotIn("Traceback", err, err[-400:])
        self.assertNotIn("BrokenPipeError", err)
        self.assertEqual(status, 1, "a broken pipe exits 1, as Python's documentation advises")


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


class ReleaseTest(Temp):
    REPORT = ("# Release notes for v9.9.9, against v9.9.8\n\n## Release text\n\n### v9.9.9\n\n"
              "It is in [the record](../decisions/0001-MADR-x.md#more), and [elsewhere](https://example.invalid/x).\n{extra}\n"
              "## Sources not in the release text\n\n- not in the body\n")

    def make(self, extra: str = "") -> Path:
        repo = self.dir / "r"
        (repo / "docs" / "reports").mkdir(parents=True)
        (repo / "docs" / "decisions").mkdir()
        (repo / "docs" / "decisions" / "0001-MADR-x.md").write_text("# x\n")
        (repo / "docs" / "reports" / "0002-REPORT-release-notes-v9-9-9.md").write_text(self.REPORT.format(extra=extra))
        git(repo, "init", "-q")
        git(repo, "remote", "add", "origin", "https://github.com/example-owner/example-repo.git")
        git(repo, "add", ".")
        git(repo, "commit", "-q", "-m", "r")
        deny = self.dir / "deny"
        deny.write_text(DENY_LINE)
        self.env = {"DISCLOSURE_DENY": str(deny)}
        return repo

    def test_body_extracted_and_links_made_absolute(self) -> None:
        repo = self.make()
        r = tool("release_body", "v9.9.9", "HEAD", "--out", str(self.dir / "out"), cwd=repo, env=self.env)
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        body = (self.dir / "out" / "release-body-v9.9.9.md").read_text()
        self.assertEqual(body, "It is in [the record](https://github.com/example-owner/example-repo/blob/v9.9.9/docs/decisions/0001-MADR-x.md"
                               "#more), and [elsewhere](https://example.invalid/x).\n")

    def test_missing_target_refused(self) -> None:
        repo = self.make(extra="See [gone](0099-REPORT-gone.md).\n")
        r = tool("release_body", "v9.9.9", "HEAD", "--out", str(self.dir / "out"), cwd=repo, env=self.env)
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        self.assertIn("the link target docs/reports/0099-REPORT-gone.md does not exist", r.stdout)


class HelpersTest(Temp):
    def test_github_repo(self) -> None:
        for url in ("https://github.com/o/n.git", "https://github.com/o/n", "git@github.com:o/n.git", "ssh://git@github.com/o/n.git\n"):
            self.assertEqual(_common.github_repo(url), "o/n", url)
        with self.assertRaises(SystemExit):
            _common.github_repo("https://example.invalid/o/n.git")

    def test_module_path(self) -> None:
        (self.dir / "go.mod").write_text("// c\nmodule example.invalid/m/v2\n\ngo 1.26.10\n")
        self.assertEqual(_common.module_path(self.dir), "example.invalid/m/v2")

    def test_watch_ci_resolves_a_short_id(self) -> None:
        import watch_ci
        repo = self.dir / "r"
        repo.mkdir()
        git(repo, "init", "-q")
        (repo / "f").write_text("x")
        git(repo, "add", ".")
        git(repo, "commit", "-q", "-m", "c")
        full = subprocess.run(["git", "rev-parse", "HEAD"], cwd=repo, capture_output=True, text=True, check=True).stdout.strip()
        self.assertEqual(watch_ci.full_sha(repo, full[:7]), full)

    def test_consumer_go_mod(self) -> None:
        import consumer
        good = "module example.invalid/consumer\n\ngo 1.26.10\n\nrequire example.invalid/m v1.2.3\n"
        self.assertTrue(consumer.go_mod_ok(good, "1.26.10", "example.invalid/m", "v1.2.3"))
        self.assertFalse(consumer.go_mod_ok(good + "\ntoolchain go1.27.3\n", "1.26.10", "example.invalid/m", "v1.2.3"))
        self.assertFalse(consumer.go_mod_ok(good.replace("1.26.10", "1.27.3"), "1.26.10", "example.invalid/m", "v1.2.3"))

    def test_tag_on_main(self) -> None:
        import publication
        repo = self.dir / "a"
        repo.mkdir()
        git(repo, "init", "-q")
        sha = []
        for i in range(2):
            (repo / "f").write_text(str(i))
            git(repo, "add", ".")
            git(repo, "commit", "-q", "-m", str(i))
            sha.append(subprocess.run(["git", "rev-parse", "HEAD"], cwd=repo, capture_output=True, text=True, check=True).stdout.strip())
        self.assertTrue(publication.contains(repo, sha[0], sha[1]), "an older tag is on main")
        self.assertTrue(publication.contains(repo, sha[1], sha[1]), "the tag at main's head is on main")
        self.assertFalse(publication.contains(repo, sha[1], sha[0]), "a tag ahead of main is not on it")

    def test_same_body(self) -> None:
        import publication
        self.assertTrue(publication.same_body("text\n", "\ntext"))
        self.assertFalse(publication.same_body("text", "text."))


class WatchCiTest(unittest.TestCase):
    """watch_ci.py's verdict and the runs it reads
    (0031-MADR-verification-tools-in-repository.md, "Amendment 2026-10-22, after step 8")."""
    SHA = "b" * 40
    RUNS = [{"databaseId": 2, "headSha": SHA, "headBranch": "v9.9.9", "event": "push"},
            {"databaseId": 1, "headSha": SHA, "headBranch": "main", "event": "push"},
            {"databaseId": 3, "headSha": "c" * 40, "headBranch": "main", "event": "push"}]

    def test_every_run_of_the_commit(self) -> None:
        import watch_ci
        self.assertEqual([r["databaseId"] for r in watch_ci.runs_for(self.RUNS, self.SHA)], [2, 1], "both runs of the commit")
        self.assertEqual([r["databaseId"] for r in watch_ci.runs_for(self.RUNS, self.SHA, "main")], [1], "the branch's alone")
        self.assertEqual(watch_ci.runs_for(self.RUNS, self.SHA, "nope"), [])

    def test_verdict_reads_every_job(self) -> None:
        import watch_ci
        ok = {"status": "completed", "conclusion": "success"}
        self.assertTrue(watch_ci.verdict({"conclusion": "success"}, [ok, {"status": "completed", "conclusion": "skipped"}]))
        self.assertFalse(watch_ci.verdict({"conclusion": "success"}, [ok, {"status": "completed", "conclusion": "failure"}]),
                         "a job's failure under a run's success must fail")
        self.assertFalse(watch_ci.verdict({"conclusion": "failure"}, [ok]))
        self.assertFalse(watch_ci.verdict({"conclusion": "success"}, []), "no job read is no verdict")

    def test_wait_settled(self) -> None:
        import watch_ci
        now = [0.0]
        reads = []
        lagging = {"status": "completed", "jobs": [{"status": "in_progress"}]}
        done = {"status": "completed", "jobs": [{"status": "completed"}]}

        def fetch() -> dict:
            reads.append(now[0])
            return lagging if len(reads) < 3 else done

        info, calm = watch_ci.wait_settled(fetch, 180, 10, clock=lambda: now[0], sleep=lambda s: now.__setitem__(0, now[0] + s))
        self.assertTrue(calm)
        self.assertIs(info, done)
        self.assertEqual(reads, [0.0, 10.0, 20.0], "read again every 10 s until settled")

        now[0], reads[:] = 0.0, []
        info, calm = watch_ci.wait_settled(lambda: (reads.append(now[0]), lagging)[1], 180, 10, clock=lambda: now[0],
                                           sleep=lambda s: now.__setitem__(0, now[0] + s))
        self.assertFalse(calm, "unsettled at the bound")
        self.assertEqual(reads[-1], 180.0)

    def test_watch_timeout_is_a_verdict(self) -> None:
        import watch_ci

        def hangs(cmd, **kw):
            raise subprocess.TimeoutExpired(cmd, kw.get("timeout"))

        self.assertEqual(watch_ci.watch(["gh"], timeout=5, runner=hangs), (None, "did not finish within 5 s"))
        self.assertEqual(watch_ci.watch(["gh"], runner=lambda cmd, **kw: subprocess.CompletedProcess(cmd, 1)), (1, ""))

    def test_main_run_of_a_released_commit(self) -> None:
        import publication
        runs = [{"databaseId": 2, "headBranch": "v9.9.9"}, {"databaseId": 1, "headBranch": "main"}]
        self.assertEqual(publication.main_run(runs)["databaseId"], 1, "main's run, not the newer tag's")
        self.assertIsNone(publication.main_run(runs[:1]))


class ReadmeTest(unittest.TestCase):
    def test_relative_links_resolve(self) -> None:
        # scripts/check_records.py checks the docs tree's links; this README
        # is outside it.
        import re
        readme = TOOLS / "README.md"
        links = re.findall(r"\]\(((?!https?://|#)[^)\s]+)\)", readme.read_text(encoding="utf-8"))
        self.assertTrue(links, "the README links its record")
        for link in links:
            self.assertTrue((readme.parent / link.split("#")[0]).is_file(), f"tools/README.md links {link}, which does not exist")


class HostTest(unittest.TestCase):
    def test_unset_names_the_variable(self) -> None:
        saved = os.environ.pop("ACP_LINUX_HOST", None)
        self.addCleanup(lambda: saved is not None and os.environ.__setitem__("ACP_LINUX_HOST", saved))
        with self.assertRaises(SystemExit) as cm:
            _common.host("linux")
        self.assertIn("ACP_LINUX_HOST", str(cm.exception))


if __name__ == "__main__":
    unittest.main()
