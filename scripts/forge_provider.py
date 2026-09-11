#!/usr/bin/env python3
"""forge_provider.py — frontière d'adaptation vers la forge (GitHub, Forgejo, GitLab).

Pourquoi ce module existe : l'organisation a migré sur une forge Forgejo
souveraine et doit aussi travailler avec GitLab ; GitHub ne doit plus être
privilégié (orientation du 11.09.2026). Les scripts sidecar de NOMOS
appelaient la CLI `gh` directement, ce qui liait chaque appel d'API à un
seul fournisseur. Ce module isole les opérations dont les scripts ont
besoin à l'exécution derrière un protocole `Provider`, avec une
implémentation par forge et une implémentation en mémoire pour les tests.

Périmètre volontairement étroit (première tranche) : les commentaires
d'issue/PR/MR (lister, créer, mettre à jour) et un GET générique en
lecture seule. Rien d'autre n'est promis.

Configuration par l'environnement :

    NOMOS_FORGE_PROVIDER    github | forgejo | gitlab | fake
                            (défaut : github si GITHUB_TOKEN, GH_TOKEN ou
                            GITHUB_REPOSITORY est défini ; sinon erreur)
    NOMOS_FORGE_URL         base de l'API (défaut https://api.github.com
                            pour github ; obligatoire pour forgejo/gitlab)
    NOMOS_FORGE_TOKEN_FILE  chemin d'un fichier contenant le jeton
                            (repli : NOMOS_FORGE_TOKEN ; pour github,
                            repli supplémentaire GITHUB_TOKEN puis GH_TOKEN)

Doctrine (docs/43 §2.8, « ce qui se tait ment ») : un jeton ou une URL
manquants lèvent une erreur nommée, jamais un repli silencieux vers un
no-op. Le jeton n'est jamais journalisé ni inclus dans un message
d'erreur.

Bibliothèque standard uniquement : urllib, json, os, subprocess, shutil.
"""

from __future__ import annotations

import json
import os
import re
import shutil
import subprocess
import urllib.error
import urllib.parse
import urllib.request
from typing import Any, Callable, Protocol

PROVIDER_NAMES = ("github", "forgejo", "gitlab", "fake")

ENV_PROVIDER = "NOMOS_FORGE_PROVIDER"
ENV_URL = "NOMOS_FORGE_URL"
ENV_TOKEN_FILE = "NOMOS_FORGE_TOKEN_FILE"
ENV_TOKEN = "NOMOS_FORGE_TOKEN"

GITHUB_DEFAULT_URL = "https://api.github.com"
GITHUB_PAGE_SIZE = 100
FORGEJO_PAGE_SIZE = 50
GITLAB_PAGE_SIZE = 100
MAX_PAGES = 100
HTTP_TIMEOUT_SECONDS = 60.0
GH_TIMEOUT_SECONDS = 60.0
USER_AGENT = "nomos-forge-provider"


# ---------------------------------------------------------------------------
# Erreurs nommées
# ---------------------------------------------------------------------------


class ForgeError(RuntimeError):
    """Échec d'un appel à la forge. Porte le statut HTTP et l'endpoint, jamais le jeton."""

    def __init__(self, message: str, *, status: int | None = None, endpoint: str = "") -> None:
        super().__init__(message)
        self.status = status
        self.endpoint = endpoint


class ForgeConfigError(ForgeError):
    """Configuration incomplète (variable d'environnement manquante ou invalide)."""


class NotSupported(ForgeError):
    """Opération non prise en charge par ce fournisseur — on refuse plutôt que deviner."""


# ---------------------------------------------------------------------------
# Protocole
# ---------------------------------------------------------------------------


class Provider(Protocol):
    """Opérations que les scripts sidecar attendent d'une forge.

    `repo` est toujours donné sous la forme `owner/name`. Les commentaires
    renvoyés sont normalisés en `{"id": int, "body": str, "raw": original}`
    pour que les appelants voient une seule forme quel que soit le
    fournisseur.
    """

    name: str

    def list_issue_comments(self, repo: str, number: int) -> list[dict]: ...

    def create_issue_comment(self, repo: str, number: int, body: str) -> dict: ...

    def update_issue_comment(
        self, repo: str, comment_id: int, body: str, *, number: int | None = None
    ) -> dict: ...

    def get_json(self, path: str) -> Any: ...


# ---------------------------------------------------------------------------
# Aides communes
# ---------------------------------------------------------------------------


def split_repo(repo: str) -> tuple[str, str]:
    """Découpe `owner/name` ; refuse toute autre forme."""
    parts = (repo or "").strip("/").split("/")
    if len(parts) != 2 or not parts[0] or not parts[1]:
        raise ValueError(f"repo must be given as owner/name, got {repo!r}")
    return parts[0], parts[1]


def normalise_comment(raw: Any) -> dict:
    """Ramène un commentaire GitHub/Forgejo ou une note GitLab à une forme unique."""
    if not isinstance(raw, dict):
        raise ForgeError(f"comment payload is not an object: {type(raw).__name__}")
    ident = raw.get("id")
    try:
        ident = int(ident)
    except (TypeError, ValueError) as exc:
        raise ForgeError(f"comment id is not an integer: {ident!r}") from exc
    body = raw.get("body")
    if body is None:
        body = ""
    if not isinstance(body, str):
        body = str(body)
    return {"id": ident, "body": body, "raw": raw}


def _join(base: str, path: str) -> str:
    return base.rstrip("/") + "/" + path.lstrip("/")


class _RestProvider:
    """Socle HTTP partagé : requête JSON via urllib, erreurs nommées, pagination."""

    name = "rest"
    page_size = 100

    def __init__(self, base_url: str, token: str | None, *, opener: Callable | None = None) -> None:
        if not base_url:
            raise ForgeConfigError(f"{ENV_URL} is required for provider {self.name}")
        self.base_url = base_url.rstrip("/")
        self._token = token
        self._opener = opener or urllib.request.urlopen

    # Chaque fournisseur nomme son en-tête d'authentification.
    def _auth_headers(self) -> dict[str, str]:  # pragma: no cover - surchargé
        return {}

    def _request(self, method: str, path: str, body: Any = None) -> Any:
        url = _join(self.base_url, path)
        headers = {"Accept": "application/json", "User-Agent": USER_AGENT}
        headers.update(self._auth_headers())
        data = None
        if body is not None:
            data = json.dumps(body).encode("utf-8")
            headers["Content-Type"] = "application/json"
        req = urllib.request.Request(url, data=data, headers=headers, method=method)
        try:
            with self._opener(req, timeout=HTTP_TIMEOUT_SECONDS) as resp:
                status = getattr(resp, "status", 200)
                payload = resp.read()
        except urllib.error.HTTPError as exc:
            raise ForgeError(
                f"{self.name}: {method} {path} failed with HTTP {exc.code}",
                status=exc.code,
                endpoint=path,
            ) from None
        except urllib.error.URLError as exc:
            raise ForgeError(
                f"{self.name}: {method} {path} unreachable: {exc.reason}", endpoint=path
            ) from None
        if status and status >= 400:
            raise ForgeError(
                f"{self.name}: {method} {path} failed with HTTP {status}",
                status=status,
                endpoint=path,
            )
        return _decode_json(payload, self.name, path)

    def _paginate(self, path_for_page: Callable[[int], str]) -> list[Any]:
        """Enchaîne les pages jusqu'à une page courte ou vide."""
        items: list[Any] = []
        for page in range(1, MAX_PAGES + 1):
            chunk = self._request("GET", path_for_page(page))
            if not isinstance(chunk, list):
                raise ForgeError(
                    f"{self.name}: expected a JSON array from {path_for_page(page)}",
                    endpoint=path_for_page(page),
                )
            items.extend(chunk)
            if len(chunk) < self.page_size:
                break
        return items

    def get_json(self, path: str) -> Any:
        return self._request("GET", self._map_path(path))

    def _map_path(self, path: str) -> str:
        return path


def _decode_json(payload: bytes | str, name: str, path: str) -> Any:
    if isinstance(payload, bytes):
        payload = payload.decode("utf-8")
    text = (payload or "").strip()
    if not text:
        return None
    try:
        return json.loads(text)
    except json.JSONDecodeError as exc:
        raise ForgeError(f"{name}: {path} returned invalid JSON: {exc}", endpoint=path) from exc


# ---------------------------------------------------------------------------
# GitHub
# ---------------------------------------------------------------------------


class GitHubProvider(_RestProvider):
    """REST GitHub avec `Authorization: Bearer` ; sans jeton, repli sur `gh api`.

    Le repli conserve le dessein existant des workflows GitHub Actions : le
    GITHUB_TOKEN du runner (exposé à `gh` via GH_TOKEN) reste la seule
    frontière d'identification. Sans jeton ET sans binaire `gh`, on lève
    une erreur nommée.
    """

    name = "github"
    page_size = GITHUB_PAGE_SIZE

    def __init__(
        self,
        base_url: str = GITHUB_DEFAULT_URL,
        token: str | None = None,
        *,
        opener: Callable | None = None,
        runner: Callable | None = None,
        gh_path: str | None = None,
    ) -> None:
        super().__init__(base_url or GITHUB_DEFAULT_URL, token, opener=opener)
        self._runner = runner or subprocess.run
        self._gh_path = gh_path
        if not self._token and not self._gh_path:
            raise ForgeConfigError(
                f"github provider needs a token ({ENV_TOKEN_FILE}, {ENV_TOKEN}, "
                "GITHUB_TOKEN or GH_TOKEN) or the `gh` binary on PATH"
            )

    @property
    def uses_gh(self) -> bool:
        return not self._token and bool(self._gh_path)

    def _auth_headers(self) -> dict[str, str]:
        return {
            "Authorization": f"Bearer {self._token}",
            "X-GitHub-Api-Version": "2022-11-28",
        }

    def _request(self, method: str, path: str, body: Any = None) -> Any:
        if not self.uses_gh:
            return super()._request(method, path, body)
        cmd = [self._gh_path, "api", "-X", method, path.lstrip("/")]
        stdin = None
        if body is not None:
            cmd.extend(["--input", "-"])
            stdin = json.dumps(body)
        try:
            result = self._runner(
                cmd,
                input=stdin,
                text=True,
                capture_output=True,
                check=False,
                timeout=GH_TIMEOUT_SECONDS,
            )
        except subprocess.TimeoutExpired:
            raise ForgeError(
                f"github: gh api {method} {path} timed out after {GH_TIMEOUT_SECONDS:.0f}s",
                endpoint=path,
            ) from None
        if result.returncode != 0:
            # gh écrit « HTTP 404 » dans stderr ; on en extrait le statut sans
            # rejouer stderr entier, qui pourrait contenir l'URL avec des
            # paramètres sensibles.
            status = _status_from_gh_stderr(result.stderr or "")
            raise ForgeError(
                f"github: gh api {method} {path} failed (exit {result.returncode}"
                + (f", HTTP {status}" if status else "")
                + ")",
                status=status,
                endpoint=path,
            )
        return _decode_json(result.stdout or "", self.name, path)

    def list_issue_comments(self, repo: str, number: int) -> list[dict]:
        owner, name = split_repo(repo)
        raw = self._paginate(
            lambda page: f"/repos/{owner}/{name}/issues/{int(number)}/comments"
            f"?per_page={self.page_size}&page={page}"
        )
        return [normalise_comment(c) for c in raw]

    def create_issue_comment(self, repo: str, number: int, body: str) -> dict:
        owner, name = split_repo(repo)
        raw = self._request(
            "POST", f"/repos/{owner}/{name}/issues/{int(number)}/comments", {"body": body}
        )
        return normalise_comment(raw)

    def update_issue_comment(
        self, repo: str, comment_id: int, body: str, *, number: int | None = None
    ) -> dict:
        owner, name = split_repo(repo)
        raw = self._request(
            "PATCH", f"/repos/{owner}/{name}/issues/comments/{int(comment_id)}", {"body": body}
        )
        return normalise_comment(raw)


def _status_from_gh_stderr(stderr: str) -> int | None:
    """`gh` écrit « ... (HTTP 404) » : on n'extrait que ce statut."""
    match = re.search(r"HTTP (\d{3})", stderr or "")
    return int(match.group(1)) if match else None


# ---------------------------------------------------------------------------
# Forgejo
# ---------------------------------------------------------------------------


class ForgejoProvider(_RestProvider):
    """REST Forgejo (`/api/v1`) avec `Authorization: token <t>`."""

    name = "forgejo"
    page_size = FORGEJO_PAGE_SIZE
    api_prefix = "/api/v1"

    def __init__(self, base_url: str, token: str | None, *, opener: Callable | None = None) -> None:
        super().__init__(base_url, token, opener=opener)
        if not self._token:
            raise ForgeConfigError(
                f"forgejo provider needs a token ({ENV_TOKEN_FILE} or {ENV_TOKEN})"
            )
        # On accepte l'URL de l'instance ou déjà celle de l'API.
        if self.base_url.endswith(self.api_prefix):
            self.base_url = self.base_url[: -len(self.api_prefix)]

    def _auth_headers(self) -> dict[str, str]:
        return {"Authorization": f"token {self._token}"}

    def _map_path(self, path: str) -> str:
        """`/repos/...` (forme GitHub) devient `/api/v1/repos/...`."""
        clean = "/" + path.lstrip("/")
        if clean.startswith(self.api_prefix + "/"):
            return clean
        return self.api_prefix + clean

    def list_issue_comments(self, repo: str, number: int) -> list[dict]:
        owner, name = split_repo(repo)
        raw = self._paginate(
            lambda page: f"{self.api_prefix}/repos/{owner}/{name}/issues/{int(number)}/comments"
            f"?limit={self.page_size}&page={page}"
        )
        return [normalise_comment(c) for c in raw]

    def create_issue_comment(self, repo: str, number: int, body: str) -> dict:
        owner, name = split_repo(repo)
        raw = self._request(
            "POST",
            f"{self.api_prefix}/repos/{owner}/{name}/issues/{int(number)}/comments",
            {"body": body},
        )
        return normalise_comment(raw)

    def update_issue_comment(
        self, repo: str, comment_id: int, body: str, *, number: int | None = None
    ) -> dict:
        owner, name = split_repo(repo)
        raw = self._request(
            "PATCH",
            f"{self.api_prefix}/repos/{owner}/{name}/issues/comments/{int(comment_id)}",
            {"body": body},
        )
        return normalise_comment(raw)


# ---------------------------------------------------------------------------
# GitLab
# ---------------------------------------------------------------------------


class GitLabProvider(_RestProvider):
    """REST GitLab (`/api/v4`) avec `PRIVATE-TOKEN` ; les commentaires sont des notes de MR.

    Conventions reprises de `ci/gitlab/annotate-mr.py` : projet encodé en
    URL (`owner%2Fname`), notes créées via
    `/projects/{id}/merge_requests/{iid}/notes`.
    """

    name = "gitlab"
    page_size = GITLAB_PAGE_SIZE
    api_prefix = "/api/v4"

    def __init__(self, base_url: str, token: str | None, *, opener: Callable | None = None) -> None:
        super().__init__(base_url, token, opener=opener)
        if not self._token:
            raise ForgeConfigError(
                f"gitlab provider needs a token ({ENV_TOKEN_FILE} or {ENV_TOKEN})"
            )
        if self.base_url.endswith(self.api_prefix):
            self.base_url = self.base_url[: -len(self.api_prefix)]

    def _auth_headers(self) -> dict[str, str]:
        return {"PRIVATE-TOKEN": self._token or ""}

    @staticmethod
    def project_id(repo: str) -> str:
        owner, name = split_repo(repo)
        return urllib.parse.quote(f"{owner}/{name}", safe="")

    def _notes_path(self, repo: str, number: int) -> str:
        return f"{self.api_prefix}/projects/{self.project_id(repo)}/merge_requests/{int(number)}/notes"

    def list_issue_comments(self, repo: str, number: int) -> list[dict]:
        base = self._notes_path(repo, number)
        raw = self._paginate(lambda page: f"{base}?per_page={self.page_size}&page={page}")
        return [normalise_comment(c) for c in raw]

    def create_issue_comment(self, repo: str, number: int, body: str) -> dict:
        raw = self._request("POST", self._notes_path(repo, number), {"body": body})
        return normalise_comment(raw)

    def update_issue_comment(
        self, repo: str, comment_id: int, body: str, *, number: int | None = None
    ) -> dict:
        if number is None:
            raise ForgeError(
                "gitlab: updating a note requires the merge request iid (pass number=...)"
            )
        raw = self._request(
            "PUT", f"{self._notes_path(repo, number)}/{int(comment_id)}", {"body": body}
        )
        return normalise_comment(raw)

    def get_json(self, path: str) -> Any:
        raise NotSupported(
            f"gitlab: generic GET is not supported for {path!r}; "
            "no GitHub-shaped read endpoint is mapped onto GitLab",
            endpoint=path,
        )


# ---------------------------------------------------------------------------
# Fournisseur en mémoire (tests)
# ---------------------------------------------------------------------------


class FakeProvider:
    """Forge en mémoire : aucun binaire, aucun réseau ; enregistre chaque appel dans `.calls`.

    `responses` alimente `get_json` (clé = chemin exact). Les commentaires
    peuvent être amorcés par `seed_comments`.
    """

    name = "fake"

    def __init__(self, responses: dict[str, Any] | None = None) -> None:
        self.calls: list[tuple[str, dict[str, Any]]] = []
        self.responses: dict[str, Any] = dict(responses or {})
        self._comments: dict[tuple[str, int], list[dict]] = {}
        self._next_id = 1

    def seed_comments(self, repo: str, number: int, comments: list[dict]) -> None:
        stored = [normalise_comment(c) for c in comments]
        for c in stored:
            self._next_id = max(self._next_id, c["id"] + 1)
        self._comments[(repo, int(number))] = stored

    def _record(self, op: str, **kwargs: Any) -> None:
        self.calls.append((op, kwargs))

    def list_issue_comments(self, repo: str, number: int) -> list[dict]:
        self._record("list_issue_comments", repo=repo, number=int(number))
        return [dict(c) for c in self._comments.get((repo, int(number)), [])]

    def create_issue_comment(self, repo: str, number: int, body: str) -> dict:
        self._record("create_issue_comment", repo=repo, number=int(number), body=body)
        raw = {"id": self._next_id, "body": body}
        self._next_id += 1
        comment = normalise_comment(raw)
        self._comments.setdefault((repo, int(number)), []).append(comment)
        return dict(comment)

    def update_issue_comment(
        self, repo: str, comment_id: int, body: str, *, number: int | None = None
    ) -> dict:
        self._record(
            "update_issue_comment", repo=repo, comment_id=int(comment_id), body=body, number=number
        )
        for comments in self._comments.values():
            for comment in comments:
                if comment["id"] == int(comment_id):
                    comment["body"] = body
                    comment["raw"] = {"id": comment["id"], "body": body}
                    return dict(comment)
        raise ForgeError(f"fake: comment {comment_id} not found", status=404, endpoint=repo)

    def get_json(self, path: str) -> Any:
        self._record("get_json", path=path)
        if path not in self.responses:
            raise ForgeError(f"fake: no response registered for {path}", status=404, endpoint=path)
        return self.responses[path]


# ---------------------------------------------------------------------------
# Résolution de la configuration
# ---------------------------------------------------------------------------


def read_token(environ: dict[str, str], *fallback_vars: str) -> str | None:
    """Jeton depuis NOMOS_FORGE_TOKEN_FILE, puis NOMOS_FORGE_TOKEN, puis les variables de repli."""
    token_file = environ.get(ENV_TOKEN_FILE, "").strip()
    if token_file:
        if not os.path.isfile(token_file):
            raise ForgeConfigError(f"{ENV_TOKEN_FILE} points to a missing file")
        with open(token_file, "r", encoding="utf-8") as fh:
            token = fh.read().strip()
        if not token:
            raise ForgeConfigError(f"{ENV_TOKEN_FILE} points to an empty file")
        return token
    for var in (ENV_TOKEN, *fallback_vars):
        value = environ.get(var, "").strip()
        if value:
            return value
    return None


def resolve_provider_name(environ: dict[str, str]) -> str:
    name = environ.get(ENV_PROVIDER, "").strip().lower()
    if name:
        if name not in PROVIDER_NAMES:
            raise ForgeConfigError(
                f"{ENV_PROVIDER}={name!r} is not one of {', '.join(PROVIDER_NAMES)}"
            )
        return name
    if any(environ.get(v) for v in ("GITHUB_TOKEN", "GH_TOKEN", "GITHUB_REPOSITORY")):
        return "github"
    raise ForgeConfigError(
        f"{ENV_PROVIDER} is not set and no GitHub context (GITHUB_TOKEN, GH_TOKEN or "
        "GITHUB_REPOSITORY) is present; refusing to guess the forge"
    )


def make_provider(name: str, **kw: Any) -> Provider:
    """Construit un fournisseur par son nom ; les kwargs vont au constructeur."""
    if name == "github":
        return GitHubProvider(**kw)
    if name == "forgejo":
        return ForgejoProvider(**kw)
    if name == "gitlab":
        return GitLabProvider(**kw)
    if name == "fake":
        return FakeProvider(**kw)
    raise ForgeConfigError(f"unknown provider {name!r}; expected one of {', '.join(PROVIDER_NAMES)}")


def provider_from_env(
    environ: dict[str, str] | None = None,
    which: Callable[[str], str | None] | None = None,
    **kw: Any,
) -> Provider:
    """Résout le fournisseur depuis l'environnement (`os.environ` par défaut).

    `which` (défaut `shutil.which`) sert à détecter le binaire `gh` pour le
    repli GitHub ; les kwargs supplémentaires (`opener`, `runner`) sont
    transmis au constructeur, ce qui permet aux tests de stubber le réseau.
    """
    env = dict(os.environ if environ is None else environ)
    which = which or shutil.which
    name = resolve_provider_name(env)

    if name == "fake":
        return FakeProvider()

    url = env.get(ENV_URL, "").strip()
    if name == "github":
        token = read_token(env, "GITHUB_TOKEN", "GH_TOKEN")
        gh_path = None if token else which("gh")
        return GitHubProvider(url or GITHUB_DEFAULT_URL, token, gh_path=gh_path, **kw)

    if not url:
        raise ForgeConfigError(f"{ENV_URL} is required when {ENV_PROVIDER}={name}")
    token = read_token(env)
    if not token:
        raise ForgeConfigError(
            f"{ENV_TOKEN_FILE} or {ENV_TOKEN} is required when {ENV_PROVIDER}={name}"
        )
    return make_provider(name, base_url=url, token=token, **kw)


__all__ = [
    "ForgeError",
    "ForgeConfigError",
    "NotSupported",
    "Provider",
    "GitHubProvider",
    "ForgejoProvider",
    "GitLabProvider",
    "FakeProvider",
    "normalise_comment",
    "split_repo",
    "read_token",
    "resolve_provider_name",
    "make_provider",
    "provider_from_env",
]
