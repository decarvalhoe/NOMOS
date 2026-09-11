"""Tests for scripts/forge_provider.py — the forge adapter boundary.

No network, no binary: HTTP is replaced by a stubbed `urlopen`, and the
`gh` fallback by a stubbed `subprocess.run`. Doctrine §2.8: a missing
token or URL must surface as a named error, never as a silent no-op, so
the configuration tests assert on the variable named in the message.
"""

from __future__ import annotations

import io
import json
import subprocess
import sys
import tempfile
import unittest
import urllib.error
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
SCRIPTS_DIR = REPO_ROOT / "scripts"
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

import forge_provider as fp  # noqa: E402


class _Response(io.BytesIO):
    """Réponse minimale compatible `with urlopen(...) as resp`."""

    def __init__(self, payload, status: int = 200) -> None:
        super().__init__(json.dumps(payload).encode("utf-8"))
        self.status = status

    def __enter__(self):
        return self

    def __exit__(self, *exc):
        self.close()
        return False


class StubOpener:
    """`urlopen` de substitution : enregistre les requêtes, répond par URL ou par ordre."""

    def __init__(self, routes=None, *, error=None) -> None:
        self.routes = routes or {}
        self.error = error
        self.requests = []

    def __call__(self, req, timeout=None):
        self.requests.append(req)
        if self.error is not None:
            raise self.error
        key = req.full_url
        if key in self.routes:
            payload = self.routes[key]
        else:
            raise AssertionError(f"unexpected request {req.get_method()} {key}")
        if callable(payload):
            payload = payload(req)
        return _Response(payload)


def _pages(size: int, total: int) -> list[list[dict]]:
    """`total` commentaires découpés en pages de `size` (dernière page courte)."""
    items = [{"id": i, "body": f"c{i}"} for i in range(1, total + 1)]
    return [items[i : i + size] for i in range(0, len(items), size)] or [[]]


# ---------------------------------------------------------------------------
# Résolution de l'environnement
# ---------------------------------------------------------------------------


class EnvResolutionTests(unittest.TestCase):
    def test_no_provider_and_no_github_context_names_the_variable(self):
        with self.assertRaises(fp.ForgeConfigError) as ctx:
            fp.provider_from_env({}, which=lambda _: None)
        self.assertIn("NOMOS_FORGE_PROVIDER", str(ctx.exception))

    def test_unknown_provider_name_is_refused(self):
        with self.assertRaises(fp.ForgeConfigError) as ctx:
            fp.provider_from_env({"NOMOS_FORGE_PROVIDER": "bitbucket"}, which=lambda _: None)
        self.assertIn("bitbucket", str(ctx.exception))

    def test_github_is_default_when_github_repository_is_set(self):
        provider = fp.provider_from_env(
            {"GITHUB_REPOSITORY": "o/r"}, which=lambda name: "/usr/bin/gh"
        )
        self.assertIsInstance(provider, fp.GitHubProvider)
        self.assertTrue(provider.uses_gh)
        self.assertEqual(provider.base_url, fp.GITHUB_DEFAULT_URL)

    def test_github_token_env_uses_rest_not_gh(self):
        provider = fp.provider_from_env(
            {"GITHUB_TOKEN": "ghs_x"}, which=lambda name: "/usr/bin/gh"
        )
        self.assertIsInstance(provider, fp.GitHubProvider)
        self.assertFalse(provider.uses_gh)

    def test_github_without_token_and_without_gh_is_an_error(self):
        with self.assertRaises(fp.ForgeConfigError) as ctx:
            fp.provider_from_env({"GITHUB_REPOSITORY": "o/r"}, which=lambda _: None)
        self.assertIn("gh", str(ctx.exception))
        self.assertIn("GITHUB_TOKEN", str(ctx.exception))

    def test_github_custom_url_is_honoured(self):
        provider = fp.provider_from_env(
            {"GITHUB_TOKEN": "t", "NOMOS_FORGE_URL": "https://ghe.example/api/v3/"},
            which=lambda _: None,
        )
        self.assertEqual(provider.base_url, "https://ghe.example/api/v3")

    def test_forgejo_requires_url(self):
        with self.assertRaises(fp.ForgeConfigError) as ctx:
            fp.provider_from_env(
                {"NOMOS_FORGE_PROVIDER": "forgejo", "NOMOS_FORGE_TOKEN": "t"},
                which=lambda _: None,
            )
        self.assertIn("NOMOS_FORGE_URL", str(ctx.exception))

    def test_forgejo_requires_token(self):
        with self.assertRaises(fp.ForgeConfigError) as ctx:
            fp.provider_from_env(
                {"NOMOS_FORGE_PROVIDER": "forgejo", "NOMOS_FORGE_URL": "https://forge.example"},
                which=lambda _: None,
            )
        self.assertIn("NOMOS_FORGE_TOKEN", str(ctx.exception))

    def test_forgejo_github_token_is_not_a_fallback(self):
        # A GitHub token must never be sent to another forge.
        with self.assertRaises(fp.ForgeConfigError):
            fp.provider_from_env(
                {
                    "NOMOS_FORGE_PROVIDER": "forgejo",
                    "NOMOS_FORGE_URL": "https://forge.example",
                    "GITHUB_TOKEN": "ghs_x",
                },
                which=lambda _: None,
            )

    def test_gitlab_resolves_with_url_and_token(self):
        provider = fp.provider_from_env(
            {
                "NOMOS_FORGE_PROVIDER": "gitlab",
                "NOMOS_FORGE_URL": "https://gitlab.example/api/v4",
                "NOMOS_FORGE_TOKEN": "glpat",
            },
            which=lambda _: None,
        )
        self.assertIsInstance(provider, fp.GitLabProvider)
        # The API prefix is normalised away and re-added per request.
        self.assertEqual(provider.base_url, "https://gitlab.example")

    def test_token_file_wins_over_env_token(self):
        with tempfile.TemporaryDirectory() as tmp:
            token_path = Path(tmp) / "token"
            token_path.write_text("from-file\n", encoding="utf-8")
            token = fp.read_token(
                {"NOMOS_FORGE_TOKEN_FILE": str(token_path), "NOMOS_FORGE_TOKEN": "from-env"}
            )
        self.assertEqual(token, "from-file")

    def test_token_file_missing_or_empty_is_named(self):
        with self.assertRaises(fp.ForgeConfigError) as ctx:
            fp.read_token({"NOMOS_FORGE_TOKEN_FILE": "/nonexistent/token"})
        self.assertIn("NOMOS_FORGE_TOKEN_FILE", str(ctx.exception))
        with tempfile.TemporaryDirectory() as tmp:
            empty = Path(tmp) / "empty"
            empty.write_text("", encoding="utf-8")
            with self.assertRaises(fp.ForgeConfigError):
                fp.read_token({"NOMOS_FORGE_TOKEN_FILE": str(empty)})

    def test_token_file_feeds_forgejo(self):
        with tempfile.TemporaryDirectory() as tmp:
            token_path = Path(tmp) / "token"
            token_path.write_text("secret-forgejo", encoding="utf-8")
            provider = fp.provider_from_env(
                {
                    "NOMOS_FORGE_PROVIDER": "forgejo",
                    "NOMOS_FORGE_URL": "https://forge.example",
                    "NOMOS_FORGE_TOKEN_FILE": str(token_path),
                },
                which=lambda _: None,
            )
        self.assertIsInstance(provider, fp.ForgejoProvider)
        self.assertEqual(provider._auth_headers()["Authorization"], "token secret-forgejo")

    def test_fake_from_env_and_make_provider(self):
        self.assertIsInstance(
            fp.provider_from_env({"NOMOS_FORGE_PROVIDER": "fake"}), fp.FakeProvider
        )
        self.assertIsInstance(fp.make_provider("fake"), fp.FakeProvider)
        with self.assertRaises(fp.ForgeConfigError):
            fp.make_provider("nope")


# ---------------------------------------------------------------------------
# Normalisation
# ---------------------------------------------------------------------------


class NormalisationTests(unittest.TestCase):
    def test_comment_shape_is_id_body_raw(self):
        raw = {"id": "42", "body": "hello", "user": {"login": "x"}}
        norm = fp.normalise_comment(raw)
        self.assertEqual(norm, {"id": 42, "body": "hello", "raw": raw})

    def test_missing_body_becomes_empty_string(self):
        self.assertEqual(fp.normalise_comment({"id": 1})["body"], "")

    def test_non_integer_id_is_refused(self):
        with self.assertRaises(fp.ForgeError):
            fp.normalise_comment({"id": "abc", "body": ""})
        with self.assertRaises(fp.ForgeError):
            fp.normalise_comment(["not", "an", "object"])

    def test_split_repo_refuses_other_shapes(self):
        self.assertEqual(fp.split_repo("o/r"), ("o", "r"))
        for bad in ("", "o", "o/r/x", "/r"):
            with self.assertRaises(ValueError):
                fp.split_repo(bad)


# ---------------------------------------------------------------------------
# GitHub (REST et repli gh)
# ---------------------------------------------------------------------------


class GitHubRestTests(unittest.TestCase):
    def _provider(self, routes, **kw):
        opener = StubOpener(routes, **kw)
        provider = fp.GitHubProvider(token="ghs_secret", opener=opener)
        return provider, opener

    def test_pagination_stops_on_short_page(self):
        pages = _pages(100, 150)
        base = "https://api.github.com/repos/o/r/issues/12/comments"
        provider, opener = self._provider(
            {f"{base}?per_page=100&page=1": pages[0], f"{base}?per_page=100&page=2": pages[1]}
        )
        comments = provider.list_issue_comments("o/r", 12)
        self.assertEqual(len(comments), 150)
        self.assertEqual(len(opener.requests), 2)
        self.assertEqual(comments[-1], {"id": 150, "body": "c150", "raw": pages[1][-1]})

    def test_exact_full_page_fetches_one_more_empty_page(self):
        base = "https://api.github.com/repos/o/r/issues/12/comments"
        provider, opener = self._provider(
            {f"{base}?per_page=100&page=1": _pages(100, 100)[0], f"{base}?per_page=100&page=2": []}
        )
        self.assertEqual(len(provider.list_issue_comments("o/r", 12)), 100)
        self.assertEqual(len(opener.requests), 2)

    def test_bearer_header_and_json_body_on_create(self):
        url = "https://api.github.com/repos/o/r/issues/12/comments"
        provider, opener = self._provider({url: {"id": 7, "body": "hi"}})
        created = provider.create_issue_comment("o/r", 12, "hi")
        req = opener.requests[0]
        self.assertEqual(req.get_method(), "POST")
        self.assertEqual(req.get_header("Authorization"), "Bearer ghs_secret")
        self.assertEqual(json.loads(req.data.decode("utf-8")), {"body": "hi"})
        self.assertEqual(created["id"], 7)

    def test_update_uses_patch_on_comment_endpoint(self):
        url = "https://api.github.com/repos/o/r/issues/comments/99"
        provider, opener = self._provider({url: {"id": 99, "body": "new"}})
        updated = provider.update_issue_comment("o/r", 99, "new")
        self.assertEqual(opener.requests[0].get_method(), "PATCH")
        self.assertEqual(updated["body"], "new")

    def test_http_error_carries_status_and_endpoint_but_not_token(self):
        err = urllib.error.HTTPError(
            "https://api.github.com/x", 404, "Not Found", hdrs=None, fp=None
        )
        provider, _ = self._provider({}, error=err)
        with self.assertRaises(fp.ForgeError) as ctx:
            provider.get_json("/repos/o/r/actions/runs/1/artifacts")
        self.assertEqual(ctx.exception.status, 404)
        self.assertIn("/repos/o/r/actions/runs/1/artifacts", ctx.exception.endpoint)
        self.assertNotIn("ghs_secret", str(ctx.exception))

    def test_network_error_is_a_forge_error(self):
        provider, _ = self._provider({}, error=urllib.error.URLError("refused"))
        with self.assertRaises(fp.ForgeError):
            provider.get_json("/rate_limit")

    def test_get_json_is_relative_to_base(self):
        url = "https://api.github.com/repos/o/r/actions/runs/1/artifacts?per_page=100"
        provider, opener = self._provider({url: {"artifacts": []}})
        self.assertEqual(
            provider.get_json("/repos/o/r/actions/runs/1/artifacts?per_page=100"),
            {"artifacts": []},
        )
        self.assertEqual(opener.requests[0].get_method(), "GET")


class GitHubGhFallbackTests(unittest.TestCase):
    def _runner(self, outputs):
        calls = []

        def run(cmd, **kwargs):
            calls.append((cmd, kwargs))
            spec = outputs.pop(0)
            if isinstance(spec, BaseException):
                raise spec
            return subprocess.CompletedProcess(cmd, spec.get("rc", 0), spec.get("out", ""), spec.get("err", ""))

        return run, calls

    def test_list_paginates_through_gh_api(self):
        pages = _pages(100, 120)
        run, calls = self._runner([{"out": json.dumps(pages[0])}, {"out": json.dumps(pages[1])}])
        provider = fp.GitHubProvider(token=None, gh_path="/usr/bin/gh", runner=run)
        self.assertTrue(provider.uses_gh)
        comments = provider.list_issue_comments("o/r", 12)
        self.assertEqual(len(comments), 120)
        self.assertEqual(
            calls[0][0],
            ["/usr/bin/gh", "api", "-X", "GET", "repos/o/r/issues/12/comments?per_page=100&page=1"],
        )
        self.assertEqual(calls[1][0][-1], "repos/o/r/issues/12/comments?per_page=100&page=2")

    def test_create_sends_json_on_stdin(self):
        run, calls = self._runner([{"out": json.dumps({"id": 3, "body": "b"})}])
        provider = fp.GitHubProvider(token=None, gh_path="/usr/bin/gh", runner=run)
        created = provider.create_issue_comment("o/r", 12, "b")
        cmd, kwargs = calls[0]
        self.assertEqual(cmd[:4], ["/usr/bin/gh", "api", "-X", "POST"])
        self.assertIn("--input", cmd)
        self.assertEqual(json.loads(kwargs["input"]), {"body": "b"})
        self.assertEqual(created["id"], 3)

    def test_update_uses_patch(self):
        run, calls = self._runner([{"out": json.dumps({"id": 9, "body": "n"})}])
        provider = fp.GitHubProvider(token=None, gh_path="/usr/bin/gh", runner=run)
        provider.update_issue_comment("o/r", 9, "n")
        self.assertEqual(calls[0][0][3], "PATCH")
        self.assertEqual(calls[0][0][4], "repos/o/r/issues/comments/9")

    def test_gh_failure_is_a_forge_error_with_status(self):
        run, _ = self._runner([{"rc": 1, "err": "gh: Not Found (HTTP 404)"}])
        provider = fp.GitHubProvider(token=None, gh_path="/usr/bin/gh", runner=run)
        with self.assertRaises(fp.ForgeError) as ctx:
            provider.get_json("/repos/o/r/actions/runs/1/artifacts")
        self.assertEqual(ctx.exception.status, 404)
        self.assertEqual(ctx.exception.endpoint, "/repos/o/r/actions/runs/1/artifacts")

    def test_gh_timeout_is_a_forge_error(self):
        run, _ = self._runner([subprocess.TimeoutExpired(cmd="gh", timeout=1)])
        provider = fp.GitHubProvider(token=None, gh_path="/usr/bin/gh", runner=run)
        with self.assertRaises(fp.ForgeError):
            provider.get_json("/rate_limit")

    def test_gh_invalid_json_is_a_forge_error(self):
        run, _ = self._runner([{"out": "not json"}])
        provider = fp.GitHubProvider(token=None, gh_path="/usr/bin/gh", runner=run)
        with self.assertRaises(fp.ForgeError):
            provider.get_json("/rate_limit")

    def test_token_present_never_shells_out(self):
        run, calls = self._runner([])
        opener = StubOpener({"https://api.github.com/rate_limit": {"ok": True}})
        provider = fp.GitHubProvider(token="t", gh_path="/usr/bin/gh", runner=run, opener=opener)
        self.assertEqual(provider.get_json("/rate_limit"), {"ok": True})
        self.assertEqual(calls, [])


# ---------------------------------------------------------------------------
# Forgejo
# ---------------------------------------------------------------------------


class ForgejoTests(unittest.TestCase):
    BASE = "https://forge.example/api/v1"

    def _provider(self, routes, **kw):
        opener = StubOpener(routes, **kw)
        return fp.ForgejoProvider("https://forge.example", "fj_secret", opener=opener), opener

    def test_pagination_uses_limit_50(self):
        pages = _pages(50, 70)
        base = f"{self.BASE}/repos/o/r/issues/5/comments"
        provider, opener = self._provider(
            {f"{base}?limit=50&page=1": pages[0], f"{base}?limit=50&page=2": pages[1]}
        )
        comments = provider.list_issue_comments("o/r", 5)
        self.assertEqual(len(comments), 70)
        self.assertEqual(len(opener.requests), 2)
        self.assertEqual(opener.requests[0].get_header("Authorization"), "token fj_secret")

    def test_create_and_update_endpoints(self):
        provider, opener = self._provider(
            {
                f"{self.BASE}/repos/o/r/issues/5/comments": {"id": 1, "body": "a"},
                f"{self.BASE}/repos/o/r/issues/comments/1": {"id": 1, "body": "b"},
            }
        )
        self.assertEqual(provider.create_issue_comment("o/r", 5, "a")["id"], 1)
        self.assertEqual(provider.update_issue_comment("o/r", 1, "b")["body"], "b")
        self.assertEqual([r.get_method() for r in opener.requests], ["POST", "PATCH"])

    def test_get_json_maps_repos_onto_api_v1(self):
        url = f"{self.BASE}/repos/o/r/actions/runs/1/artifacts?per_page=100"
        provider, opener = self._provider({url: {"artifacts": []}})
        provider.get_json("/repos/o/r/actions/runs/1/artifacts?per_page=100")
        self.assertEqual(opener.requests[0].full_url, url)
        # A path already under /api/v1 is not prefixed twice.
        provider.get_json("/api/v1/repos/o/r/actions/runs/1/artifacts?per_page=100")
        self.assertEqual(opener.requests[1].full_url, url)

    def test_instance_url_with_api_suffix_is_accepted(self):
        opener = StubOpener({f"{self.BASE}/version": {"version": "9"}})
        provider = fp.ForgejoProvider("https://forge.example/api/v1", "t", opener=opener)
        self.assertEqual(provider.get_json("/version"), {"version": "9"})

    def test_missing_url_or_token_is_named(self):
        with self.assertRaises(fp.ForgeConfigError) as ctx:
            fp.ForgejoProvider("", "t")
        self.assertIn("NOMOS_FORGE_URL", str(ctx.exception))
        with self.assertRaises(fp.ForgeConfigError) as ctx:
            fp.ForgejoProvider("https://forge.example", None)
        self.assertIn("NOMOS_FORGE_TOKEN", str(ctx.exception))


# ---------------------------------------------------------------------------
# GitLab
# ---------------------------------------------------------------------------


class GitLabTests(unittest.TestCase):
    NOTES = "https://gitlab.example/api/v4/projects/o%2Fr/merge_requests/8/notes"

    def _provider(self, routes, **kw):
        opener = StubOpener(routes, **kw)
        return fp.GitLabProvider("https://gitlab.example", "glpat_secret", opener=opener), opener

    def test_pagination_over_mr_notes(self):
        pages = _pages(100, 101)
        provider, opener = self._provider(
            {f"{self.NOTES}?per_page=100&page=1": pages[0], f"{self.NOTES}?per_page=100&page=2": pages[1]}
        )
        notes = provider.list_issue_comments("o/r", 8)
        self.assertEqual(len(notes), 101)
        self.assertEqual(opener.requests[0].get_header("Private-token"), "glpat_secret")

    def test_project_id_is_url_encoded(self):
        self.assertEqual(fp.GitLabProvider.project_id("group/sub"), "group%2Fsub")

    def test_create_note(self):
        provider, opener = self._provider({self.NOTES: {"id": 11, "body": "n"}})
        self.assertEqual(provider.create_issue_comment("o/r", 8, "n")["id"], 11)
        self.assertEqual(opener.requests[0].get_method(), "POST")

    def test_update_requires_number(self):
        provider, _ = self._provider({})
        with self.assertRaises(fp.ForgeError) as ctx:
            provider.update_issue_comment("o/r", 11, "x")
        self.assertIn("number", str(ctx.exception))

    def test_update_with_number_puts_the_note(self):
        provider, opener = self._provider({f"{self.NOTES}/11": {"id": 11, "body": "x"}})
        updated = provider.update_issue_comment("o/r", 11, "x", number=8)
        self.assertEqual(opener.requests[0].get_method(), "PUT")
        self.assertEqual(updated["id"], 11)

    def test_get_json_is_not_supported(self):
        provider, opener = self._provider({})
        with self.assertRaises(fp.NotSupported):
            provider.get_json("/repos/o/r/actions/runs/1/artifacts")
        self.assertEqual(opener.requests, [])


# ---------------------------------------------------------------------------
# Fake
# ---------------------------------------------------------------------------


class FakeProviderTests(unittest.TestCase):
    def test_records_every_call_in_order(self):
        fake = fp.FakeProvider(responses={"/rate_limit": {"ok": 1}})
        fake.list_issue_comments("o/r", 1)
        created = fake.create_issue_comment("o/r", 1, "a")
        fake.update_issue_comment("o/r", created["id"], "b", number=1)
        fake.get_json("/rate_limit")
        self.assertEqual(
            [op for op, _ in fake.calls],
            ["list_issue_comments", "create_issue_comment", "update_issue_comment", "get_json"],
        )
        self.assertEqual(fake.calls[3][1], {"path": "/rate_limit"})
        self.assertEqual(fake.list_issue_comments("o/r", 1)[0]["body"], "b")

    def test_seeded_comments_are_normalised(self):
        fake = fp.FakeProvider()
        fake.seed_comments("o/r", 2, [{"id": "10", "body": "x"}])
        self.assertEqual(fake.list_issue_comments("o/r", 2)[0]["id"], 10)
        # New comments do not collide with seeded ids.
        self.assertEqual(fake.create_issue_comment("o/r", 2, "y")["id"], 11)

    def test_unknown_comment_and_path_raise(self):
        fake = fp.FakeProvider()
        with self.assertRaises(fp.ForgeError):
            fake.update_issue_comment("o/r", 404, "x")
        with self.assertRaises(fp.ForgeError) as ctx:
            fake.get_json("/missing")
        self.assertEqual(ctx.exception.status, 404)


if __name__ == "__main__":
    unittest.main()
