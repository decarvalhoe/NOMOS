#!/usr/bin/env python3
"""forge_provider.py — frontière d'adaptation vers la forge (GitHub, Forgejo, GitLab).

Pourquoi ce module existe : l'organisation a migré sur une forge Forgejo
souveraine et doit aussi travailler avec GitLab ; GitHub ne doit plus être
privilégié (orientation du 11.09.2026). Les scripts sidecar de NOMOS
appelaient la CLI `gh` directement, ce qui liait chaque appel d'API à un
seul fournisseur. Ce module isole les opérations dont les scripts ont
besoin à l'exécution derrière un protocole `Provider`, avec une
implémentation par forge et une implémentation en mémoire pour les tests.

Périmètre (deuxième tranche, FN-2 de l'ADR-0006) : commentaires d'issue/PR,
GET générique en lecture seule, issues (lire, lister, créer, éditer),
étiquettes et jalons, pull requests (créer, mettre à jour), protection de
branche (lire, appliquer), environnements de déploiement (GitHub
seulement) et export du journal d'audit d'organisation (GitHub seulement).
Une opération qu'une forge ne sait pas exprimer lève `NotSupported` : on
refuse plutôt que deviner.

Configuration par l'environnement :

    NOMOS_FORGE_PROVIDER    github | forgejo | gitlab | fake
                            (défaut : github si GITHUB_TOKEN, GH_TOKEN ou
                            GITHUB_REPOSITORY est défini ; sinon erreur)
    NOMOS_FORGE_URL         base de l'API (défaut https://api.github.com
                            pour github ; obligatoire pour forgejo/gitlab)
    NOMOS_FORGE_TOKEN_FILE  chemin d'un fichier contenant le jeton
                            (repli : NOMOS_FORGE_TOKEN ; pour github,
                            repli supplémentaire GITHUB_TOKEN puis GH_TOKEN)
    NOMOS_FORGE_REPO        dépôt `owner/name` pour les scripts qui n'ont
                            pas de dépôt dans leur configuration (lu par
                            `repo_from_env`)

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
ENV_REPO = "NOMOS_FORGE_REPO"

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

    `repo` est toujours donné sous la forme `owner/name`. Les objets
    renvoyés sont normalisés (voir `normalise_*`) pour que les appelants
    voient une seule forme quel que soit le fournisseur ; l'original reste
    accessible sous la clé `raw`.
    """

    name: str

    # Commentaires (première tranche).
    def list_issue_comments(self, repo: str, number: int) -> list[dict]: ...

    def create_issue_comment(self, repo: str, number: int, body: str) -> dict: ...

    def update_issue_comment(
        self, repo: str, comment_id: int, body: str, *, number: int | None = None
    ) -> dict: ...

    def get_json(self, path: str) -> Any: ...

    # Issues, étiquettes, jalons.
    def get_issue(self, repo: str, number: int) -> dict: ...

    def list_issues(self, repo: str, *, state: str = "all") -> list[dict]: ...

    def create_issue(self, repo: str, title: str, body: str) -> dict: ...

    def edit_issue(
        self,
        repo: str,
        number: int,
        *,
        add_labels: list[str] | None = None,
        milestone: int | None = None,
    ) -> dict: ...

    def list_labels(self, repo: str) -> list[dict]: ...

    def create_label(self, repo: str, name: str, color: str, description: str = "") -> dict: ...

    def update_label(
        self, repo: str, name: str, *, color: str | None = None, description: str | None = None
    ) -> dict: ...

    def list_milestones(self, repo: str, *, state: str = "all") -> list[dict]: ...

    def create_milestone(self, repo: str, title: str, description: str = "") -> dict: ...

    # Pull requests / merge requests.
    def create_pull_request(
        self, repo: str, *, head: str, base: str, title: str, body: str
    ) -> dict: ...

    def update_pull_request(
        self, repo: str, number: int, *, title: str | None = None, body: str | None = None
    ) -> dict: ...

    # Protection de branche (forme commune = forme GitHub).
    def get_branch_protection(self, repo: str, branch: str) -> dict: ...

    def put_branch_protection(self, repo: str, branch: str, payload: dict) -> dict: ...

    # Environnements de déploiement (GitHub seulement).
    def get_environment(self, repo: str, name: str) -> dict: ...

    def put_environment(self, repo: str, name: str, payload: dict) -> dict: ...

    # Journal d'audit d'organisation (GitHub seulement).
    def export_org_audit_log(self, org: str, *, since: str, until: str) -> list[dict]: ...


# ---------------------------------------------------------------------------
# Aides communes
# ---------------------------------------------------------------------------


def split_repo(repo: str) -> tuple[str, str]:
    """Découpe `owner/name` ; refuse toute autre forme."""
    parts = (repo or "").strip("/").split("/")
    if len(parts) != 2 or not parts[0] or not parts[1]:
        raise ValueError(f"repo must be given as owner/name, got {repo!r}")
    return parts[0], parts[1]


def _as_int(value: Any, what: str) -> int:
    try:
        return int(value)
    except (TypeError, ValueError) as exc:
        raise ForgeError(f"{what} is not an integer: {value!r}") from exc


def _as_object(raw: Any, what: str) -> dict:
    if not isinstance(raw, dict):
        raise ForgeError(f"{what} payload is not an object: {type(raw).__name__}")
    return raw


def normalise_comment(raw: Any) -> dict:
    """Ramène un commentaire GitHub/Forgejo ou une note GitLab à une forme unique."""
    raw = _as_object(raw, "comment")
    ident = _as_int(raw.get("id"), "comment id")
    body = raw.get("body")
    if body is None:
        body = ""
    if not isinstance(body, str):
        body = str(body)
    return {"id": ident, "body": body, "raw": raw}


def normalise_state(value: Any) -> str:
    """`open`/`opened` → `open`, `closed`/`merged` → `closed` ; le reste est refusé."""
    state = str(value or "").strip().lower()
    if state in ("open", "opened", "reopened"):
        return "open"
    if state in ("closed", "merged", "locked"):
        return "closed"
    raise ForgeError(f"unknown issue state {value!r}")


def normalise_issue(raw: Any, *, number_key: str = "number", url_key: str = "html_url") -> dict:
    """Issue GitHub/Forgejo (`number`, `html_url`) ou GitLab (`iid`, `web_url`)."""
    raw = _as_object(raw, "issue")
    return {
        "number": _as_int(raw.get(number_key), "issue number"),
        "title": str(raw.get("title") or ""),
        "state": normalise_state(raw.get("state")),
        "url": str(raw.get(url_key) or ""),
        "raw": raw,
    }


def normalise_pull_request(
    raw: Any, *, number_key: str = "number", url_key: str = "html_url"
) -> dict:
    raw = _as_object(raw, "pull request")
    return {
        "number": _as_int(raw.get(number_key), "pull request number"),
        "title": str(raw.get("title") or ""),
        "url": str(raw.get(url_key) or ""),
        "raw": raw,
    }


def normalise_color(value: Any) -> str:
    """Couleur hexadécimale sans `#`, en minuscules (GitHub la veut sans, Forgejo l'accepte avec)."""
    return str(value or "").strip().lstrip("#").lower()


def normalise_label(raw: Any) -> dict:
    raw = _as_object(raw, "label")
    ident = raw.get("id")
    return {
        "id": _as_int(ident, "label id") if ident is not None else None,
        "name": str(raw.get("name") or ""),
        "color": normalise_color(raw.get("color")),
        "description": str(raw.get("description") or ""),
        "raw": raw,
    }


def normalise_milestone(raw: Any, *, id_key: str = "number") -> dict:
    """Jalon GitHub (`number`) ou Forgejo (`id`) ; l'identifiant sert à l'affectation."""
    raw = _as_object(raw, "milestone")
    return {
        "id": _as_int(raw.get(id_key), "milestone id"),
        "title": str(raw.get("title") or ""),
        "description": str(raw.get("description") or ""),
        "state": str(raw.get("state") or ""),
        "raw": raw,
    }


def _join(base: str, path: str) -> str:
    if path.startswith("http://") or path.startswith("https://"):
        return path
    return base.rstrip("/") + "/" + path.lstrip("/")


def _next_link(link_header: str) -> str | None:
    """URL `rel="next"` d'un en-tête Link (pagination par curseur de GitHub)."""
    for part in (link_header or "").split(","):
        if 'rel="next"' in part:
            return part.split(";")[0].strip().strip("<>")
    return None


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

    def _http(self, method: str, path: str, body: Any = None) -> tuple[Any, dict[str, str]]:
        """Une requête ; renvoie (JSON décodé, en-têtes de réponse)."""
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
                resp_headers = dict(getattr(resp, "headers", None) or {})
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
        return _decode_json(payload, self.name, path), resp_headers

    def _request(self, method: str, path: str, body: Any = None) -> Any:
        return self._http(method, path, body)[0]

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

    # Refus explicites, surchargés par les forges qui savent faire.
    def _refuse(self, what: str, endpoint: str = "") -> NotSupported:
        return NotSupported(
            f"{self.name}: {what} is not supported by this provider", endpoint=endpoint
        )

    def get_environment(self, repo: str, name: str) -> dict:
        raise self._refuse("deployment environments (a GitHub concept)", repo)

    def put_environment(self, repo: str, name: str, payload: dict) -> dict:
        raise self._refuse("deployment environments (a GitHub concept)", repo)

    def export_org_audit_log(self, org: str, *, since: str, until: str) -> list[dict]:
        raise self._refuse("the organisation audit log (a GitHub concept)", org)


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

    def _gh(self, args: list[str], *, stdin: str | None, endpoint: str, what: str) -> Any:
        """Exécute `gh api ...` et décode sa sortie ; jamais de jeton dans les erreurs."""
        cmd = [self._gh_path, "api", *args]
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
                f"github: gh api {what} timed out after {GH_TIMEOUT_SECONDS:.0f}s",
                endpoint=endpoint,
            ) from None
        if result.returncode != 0:
            # gh écrit « HTTP 404 » dans stderr ; on en extrait le statut sans
            # rejouer stderr entier, qui pourrait contenir l'URL avec des
            # paramètres sensibles.
            status = _status_from_gh_stderr(result.stderr or "")
            raise ForgeError(
                f"github: gh api {what} failed (exit {result.returncode}"
                + (f", HTTP {status}" if status else "")
                + ")",
                status=status,
                endpoint=endpoint,
            )
        return _decode_json(result.stdout or "", self.name, endpoint)

    def _request(self, method: str, path: str, body: Any = None) -> Any:
        if not self.uses_gh:
            return super()._request(method, path, body)
        args = ["-X", method, path.lstrip("/")]
        stdin = None
        if body is not None:
            args.extend(["--input", "-"])
            stdin = json.dumps(body)
        return self._gh(args, stdin=stdin, endpoint=path, what=f"{method} {path}")

    # -- commentaires -------------------------------------------------------

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

    # -- issues, étiquettes, jalons ------------------------------------------

    def get_issue(self, repo: str, number: int) -> dict:
        owner, name = split_repo(repo)
        return normalise_issue(self._request("GET", f"/repos/{owner}/{name}/issues/{int(number)}"))

    def list_issues(self, repo: str, *, state: str = "all") -> list[dict]:
        owner, name = split_repo(repo)
        raw = self._paginate(
            lambda page: f"/repos/{owner}/{name}/issues?state={state}"
            f"&per_page={self.page_size}&page={page}"
        )
        # L'endpoint GitHub mélange issues et pull requests ; on ne garde que les issues.
        return [normalise_issue(i) for i in raw if isinstance(i, dict) and "pull_request" not in i]

    def create_issue(self, repo: str, title: str, body: str) -> dict:
        owner, name = split_repo(repo)
        raw = self._request("POST", f"/repos/{owner}/{name}/issues", {"title": title, "body": body})
        return normalise_issue(raw)

    def edit_issue(
        self,
        repo: str,
        number: int,
        *,
        add_labels: list[str] | None = None,
        milestone: int | None = None,
    ) -> dict:
        owner, name = split_repo(repo)
        base = f"/repos/{owner}/{name}/issues/{int(number)}"
        if add_labels:
            # POST .../labels ajoute sans retirer les étiquettes existantes.
            self._request("POST", f"{base}/labels", {"labels": list(add_labels)})
        if milestone is not None:
            self._request("PATCH", base, {"milestone": int(milestone)})
        return normalise_issue(self._request("GET", base))

    def list_labels(self, repo: str) -> list[dict]:
        owner, name = split_repo(repo)
        raw = self._paginate(
            lambda page: f"/repos/{owner}/{name}/labels?per_page={self.page_size}&page={page}"
        )
        return [normalise_label(label) for label in raw]

    def create_label(self, repo: str, name: str, color: str, description: str = "") -> dict:
        owner, repo_name = split_repo(repo)
        raw = self._request(
            "POST",
            f"/repos/{owner}/{repo_name}/labels",
            {"name": name, "color": normalise_color(color), "description": description},
        )
        return normalise_label(raw)

    def update_label(
        self, repo: str, name: str, *, color: str | None = None, description: str | None = None
    ) -> dict:
        owner, repo_name = split_repo(repo)
        body: dict[str, Any] = {}
        if color is not None:
            body["color"] = normalise_color(color)
        if description is not None:
            body["description"] = description
        raw = self._request(
            "PATCH", f"/repos/{owner}/{repo_name}/labels/{urllib.parse.quote(name, safe='')}", body
        )
        return normalise_label(raw)

    def list_milestones(self, repo: str, *, state: str = "all") -> list[dict]:
        owner, name = split_repo(repo)
        raw = self._paginate(
            lambda page: f"/repos/{owner}/{name}/milestones?state={state}"
            f"&per_page={self.page_size}&page={page}"
        )
        return [normalise_milestone(m) for m in raw]

    def create_milestone(self, repo: str, title: str, description: str = "") -> dict:
        owner, name = split_repo(repo)
        raw = self._request(
            "POST", f"/repos/{owner}/{name}/milestones", {"title": title, "description": description}
        )
        return normalise_milestone(raw)

    # -- pull requests -------------------------------------------------------

    def create_pull_request(
        self, repo: str, *, head: str, base: str, title: str, body: str
    ) -> dict:
        owner, name = split_repo(repo)
        raw = self._request(
            "POST",
            f"/repos/{owner}/{name}/pulls",
            {"head": head, "base": base, "title": title, "body": body},
        )
        return normalise_pull_request(raw)

    def update_pull_request(
        self, repo: str, number: int, *, title: str | None = None, body: str | None = None
    ) -> dict:
        owner, name = split_repo(repo)
        payload: dict[str, Any] = {}
        if title is not None:
            payload["title"] = title
        if body is not None:
            payload["body"] = body
        raw = self._request("PATCH", f"/repos/{owner}/{name}/pulls/{int(number)}", payload)
        return normalise_pull_request(raw)

    # -- protection de branche, environnements, journal d'audit -------------

    def get_branch_protection(self, repo: str, branch: str) -> dict:
        owner, name = split_repo(repo)
        raw = self._request("GET", f"/repos/{owner}/{name}/branches/{branch}/protection")
        return _as_object(raw, "branch protection")

    def put_branch_protection(self, repo: str, branch: str, payload: dict) -> dict:
        owner, name = split_repo(repo)
        raw = self._request("PUT", f"/repos/{owner}/{name}/branches/{branch}/protection", payload)
        return _as_object(raw if raw is not None else {}, "branch protection")

    def get_environment(self, repo: str, name: str) -> dict:
        owner, repo_name = split_repo(repo)
        raw = self._request("GET", f"/repos/{owner}/{repo_name}/environments/{name}")
        return _as_object(raw, "environment")

    def put_environment(self, repo: str, name: str, payload: dict) -> dict:
        owner, repo_name = split_repo(repo)
        raw = self._request("PUT", f"/repos/{owner}/{repo_name}/environments/{name}", payload)
        return _as_object(raw if raw is not None else {}, "environment")

    def export_org_audit_log(self, org: str, *, since: str, until: str) -> list[dict]:
        """Événements du journal d'audit d'organisation, toutes pages confondues.

        Exige un jeton avec `read:audit_log` (le GITHUB_TOKEN d'un runner ne
        l'a pas) ; un refus HTTP remonte comme `ForgeError` avec son statut.
        """
        path = (
            f"/orgs/{org}/audit-log?include=all&per_page={self.page_size}"
            f"&phrase=created:{since}..{until}"
        )
        if self.uses_gh:
            # `--slurp` regroupe les pages en un tableau de tableaux.
            payload = self._gh(
                ["--paginate", "--slurp", path.lstrip("/")],
                stdin=None,
                endpoint=path,
                what=f"GET {path}",
            )
            pages = payload if isinstance(payload, list) else []
            events: list[dict] = []
            for page in pages:
                if isinstance(page, list):
                    events.extend(e for e in page if isinstance(e, dict))
                elif isinstance(page, dict):
                    events.append(page)
            return events
        events = []
        url: str | None = path
        for _ in range(MAX_PAGES):
            if not url:
                break
            chunk, headers = self._http("GET", url)
            if not isinstance(chunk, list):
                raise ForgeError(f"github: expected a JSON array from {path}", endpoint=path)
            events.extend(e for e in chunk if isinstance(e, dict))
            url = _next_link(headers.get("Link") or headers.get("link") or "")
        return events


def _status_from_gh_stderr(stderr: str) -> int | None:
    """`gh` écrit « ... (HTTP 404) » : on n'extrait que ce statut."""
    match = re.search(r"HTTP (\d{3})", stderr or "")
    return int(match.group(1)) if match else None


# ---------------------------------------------------------------------------
# Forgejo
# ---------------------------------------------------------------------------

# Clés de la forme commune (GitHub) qu'une protection de branche Forgejo ne
# sait pas exprimer. En lecture elles sont absentes de la forme normalisée
# (le comparateur les signale) ; en écriture, les demander lève NotSupported.
FORGEJO_UNMAPPED_PROTECTION_KEYS = ("required_linear_history", "lock_branch", "restrictions")


def forgejo_protection_to_common(raw: dict) -> dict:
    """Protection de branche Forgejo → forme commune (GitHub).

    Correspondances : `enable_push` faux ⇔ pull request obligatoire
    (`required_approvals`, `dismiss_stale_approvals`) ; `enable_status_check`
    ⇔ `required_status_checks` (`block_on_outdated_branch` ⇔ `strict`) ;
    `apply_to_admins` ⇔ `enforce_admins`. Une branche protégée Forgejo
    n'accepte ni force-push ni suppression. L'historique linéaire, le
    verrouillage et les restrictions de push n'ont pas d'équivalent et
    restent absents : le comparateur les remonte comme écarts plutôt que
    de les supposer satisfaits.
    """
    raw = _as_object(raw, "branch protection")
    common: dict[str, Any] = {
        "required_pull_request_reviews": None,
        "required_status_checks": None,
        "enforce_admins": {"enabled": bool(raw.get("apply_to_admins", False))},
        "allow_force_pushes": {"enabled": False},
        "allow_deletions": {"enabled": False},
    }
    if not raw.get("enable_push", False):
        common["required_pull_request_reviews"] = {
            "required_approving_review_count": int(raw.get("required_approvals") or 0),
            "dismiss_stale_reviews": bool(raw.get("dismiss_stale_approvals", False)),
            "require_code_owner_reviews": bool(raw.get("block_on_official_review_requests", False)),
        }
    if raw.get("enable_status_check", False):
        common["required_status_checks"] = {
            "strict": bool(raw.get("block_on_outdated_branch", False)),
            "contexts": list(raw.get("status_check_contexts") or []),
        }
    common["raw"] = raw
    return common


def common_protection_to_forgejo(payload: dict, branch: str) -> dict:
    """Forme commune (GitHub) → options de protection Forgejo ; refuse l'inexprimable."""
    payload = _as_object(payload, "branch protection")
    unmapped = [key for key in FORGEJO_UNMAPPED_PROTECTION_KEYS if payload.get(key)]
    if unmapped:
        raise NotSupported(
            "forgejo: branch protection cannot express " + ", ".join(unmapped) + "; "
            "drop them from the declared rule or apply them by hand",
            endpoint=branch,
        )
    reviews = payload.get("required_pull_request_reviews")
    checks = payload.get("required_status_checks")
    options: dict[str, Any] = {
        "branch_name": branch,
        "rule_name": branch,
        "enable_push": reviews is None,
        "required_approvals": int((reviews or {}).get("required_approving_review_count") or 0),
        "dismiss_stale_approvals": bool((reviews or {}).get("dismiss_stale_reviews", False)),
        "block_on_official_review_requests": bool(
            (reviews or {}).get("require_code_owner_reviews", False)
        ),
        "enable_status_check": checks is not None,
        "status_check_contexts": list((checks or {}).get("contexts") or []),
        "block_on_outdated_branch": bool((checks or {}).get("strict", False)),
        "apply_to_admins": bool(payload.get("enforce_admins", False)),
    }
    return options


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

    def _repo_path(self, repo: str) -> str:
        owner, name = split_repo(repo)
        return f"{self.api_prefix}/repos/{owner}/{name}"

    # -- commentaires -------------------------------------------------------

    def list_issue_comments(self, repo: str, number: int) -> list[dict]:
        base = f"{self._repo_path(repo)}/issues/{int(number)}/comments"
        raw = self._paginate(lambda page: f"{base}?limit={self.page_size}&page={page}")
        return [normalise_comment(c) for c in raw]

    def create_issue_comment(self, repo: str, number: int, body: str) -> dict:
        raw = self._request(
            "POST", f"{self._repo_path(repo)}/issues/{int(number)}/comments", {"body": body}
        )
        return normalise_comment(raw)

    def update_issue_comment(
        self, repo: str, comment_id: int, body: str, *, number: int | None = None
    ) -> dict:
        raw = self._request(
            "PATCH", f"{self._repo_path(repo)}/issues/comments/{int(comment_id)}", {"body": body}
        )
        return normalise_comment(raw)

    # -- issues, étiquettes, jalons ------------------------------------------

    def get_issue(self, repo: str, number: int) -> dict:
        return normalise_issue(self._request("GET", f"{self._repo_path(repo)}/issues/{int(number)}"))

    def list_issues(self, repo: str, *, state: str = "all") -> list[dict]:
        base = f"{self._repo_path(repo)}/issues"
        raw = self._paginate(
            lambda page: f"{base}?state={state}&type=issues&limit={self.page_size}&page={page}"
        )
        return [normalise_issue(i) for i in raw]

    def create_issue(self, repo: str, title: str, body: str) -> dict:
        raw = self._request(
            "POST", f"{self._repo_path(repo)}/issues", {"title": title, "body": body}
        )
        return normalise_issue(raw)

    def _label_ids(self, repo: str, names: list[str]) -> list[int]:
        """Forgejo affecte les étiquettes par identifiant ; un nom inconnu est une erreur."""
        by_name = {label["name"]: label["id"] for label in self.list_labels(repo)}
        missing = [n for n in names if n not in by_name or by_name[n] is None]
        if missing:
            raise ForgeError(
                f"forgejo: label(s) {', '.join(missing)} do not exist in {repo}; create them first",
                status=404,
                endpoint=repo,
            )
        return [int(by_name[n]) for n in names]

    def edit_issue(
        self,
        repo: str,
        number: int,
        *,
        add_labels: list[str] | None = None,
        milestone: int | None = None,
    ) -> dict:
        base = f"{self._repo_path(repo)}/issues/{int(number)}"
        if add_labels:
            self._request("POST", f"{base}/labels", {"labels": self._label_ids(repo, list(add_labels))})
        if milestone is not None:
            self._request("PATCH", base, {"milestone": int(milestone)})
        return normalise_issue(self._request("GET", base))

    def list_labels(self, repo: str) -> list[dict]:
        base = f"{self._repo_path(repo)}/labels"
        raw = self._paginate(lambda page: f"{base}?limit={self.page_size}&page={page}")
        return [normalise_label(label) for label in raw]

    def create_label(self, repo: str, name: str, color: str, description: str = "") -> dict:
        raw = self._request(
            "POST",
            f"{self._repo_path(repo)}/labels",
            {"name": name, "color": "#" + normalise_color(color), "description": description},
        )
        return normalise_label(raw)

    def update_label(
        self, repo: str, name: str, *, color: str | None = None, description: str | None = None
    ) -> dict:
        (label_id,) = self._label_ids(repo, [name])
        body: dict[str, Any] = {}
        if color is not None:
            body["color"] = "#" + normalise_color(color)
        if description is not None:
            body["description"] = description
        raw = self._request("PATCH", f"{self._repo_path(repo)}/labels/{label_id}", body)
        return normalise_label(raw)

    def list_milestones(self, repo: str, *, state: str = "all") -> list[dict]:
        base = f"{self._repo_path(repo)}/milestones"
        raw = self._paginate(lambda page: f"{base}?state={state}&limit={self.page_size}&page={page}")
        return [normalise_milestone(m, id_key="id") for m in raw]

    def create_milestone(self, repo: str, title: str, description: str = "") -> dict:
        raw = self._request(
            "POST", f"{self._repo_path(repo)}/milestones", {"title": title, "description": description}
        )
        return normalise_milestone(raw, id_key="id")

    # -- pull requests -------------------------------------------------------

    def create_pull_request(
        self, repo: str, *, head: str, base: str, title: str, body: str
    ) -> dict:
        raw = self._request(
            "POST",
            f"{self._repo_path(repo)}/pulls",
            {"head": head, "base": base, "title": title, "body": body},
        )
        return normalise_pull_request(raw)

    def update_pull_request(
        self, repo: str, number: int, *, title: str | None = None, body: str | None = None
    ) -> dict:
        payload: dict[str, Any] = {}
        if title is not None:
            payload["title"] = title
        if body is not None:
            payload["body"] = body
        raw = self._request("PATCH", f"{self._repo_path(repo)}/pulls/{int(number)}", payload)
        return normalise_pull_request(raw)

    # -- protection de branche ------------------------------------------------

    def get_branch_protection(self, repo: str, branch: str) -> dict:
        raw = self._request(
            "GET", f"{self._repo_path(repo)}/branch_protections/{urllib.parse.quote(branch, safe='')}"
        )
        return forgejo_protection_to_common(raw)

    def put_branch_protection(self, repo: str, branch: str, payload: dict) -> dict:
        options = common_protection_to_forgejo(payload, branch)
        path = f"{self._repo_path(repo)}/branch_protections"
        try:
            self._request("GET", f"{path}/{urllib.parse.quote(branch, safe='')}")
            exists = True
        except ForgeError as exc:
            if exc.status != 404:
                raise
            exists = False
        if exists:
            raw = self._request("PATCH", f"{path}/{urllib.parse.quote(branch, safe='')}", options)
        else:
            raw = self._request("POST", path, options)
        return forgejo_protection_to_common(raw if raw is not None else {})


# ---------------------------------------------------------------------------
# GitLab
# ---------------------------------------------------------------------------


class GitLabProvider(_RestProvider):
    """REST GitLab (`/api/v4`) avec `PRIVATE-TOKEN` ; les commentaires sont des notes de MR.

    Conventions reprises de `ci/gitlab/annotate-mr.py` : projet encodé en
    URL (`owner%2Fname`), notes créées via
    `/projects/{id}/merge_requests/{iid}/notes`. Issues et merge requests
    sont prises en charge ; étiquettes, jalons et protection de branche
    (modèle `protected_branches` différent) sont refusés explicitement.
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

    def _project_path(self, repo: str) -> str:
        return f"{self.api_prefix}/projects/{self.project_id(repo)}"

    def _notes_path(self, repo: str, number: int) -> str:
        return f"{self._project_path(repo)}/merge_requests/{int(number)}/notes"

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

    # -- issues -----------------------------------------------------------------

    def get_issue(self, repo: str, number: int) -> dict:
        raw = self._request("GET", f"{self._project_path(repo)}/issues/{int(number)}")
        return normalise_issue(raw, number_key="iid", url_key="web_url")

    def list_issues(self, repo: str, *, state: str = "all") -> list[dict]:
        base = f"{self._project_path(repo)}/issues"
        state_query = "" if state == "all" else f"&state={'opened' if state == 'open' else state}"
        raw = self._paginate(
            lambda page: f"{base}?scope=all{state_query}&per_page={self.page_size}&page={page}"
        )
        return [normalise_issue(i, number_key="iid", url_key="web_url") for i in raw]

    def create_issue(self, repo: str, title: str, body: str) -> dict:
        raw = self._request(
            "POST", f"{self._project_path(repo)}/issues", {"title": title, "description": body}
        )
        return normalise_issue(raw, number_key="iid", url_key="web_url")

    def edit_issue(self, repo: str, number: int, *, add_labels=None, milestone=None) -> dict:
        raise self._refuse("issue taxonomy (labels and milestones)", repo)

    def list_labels(self, repo: str) -> list[dict]:
        raise self._refuse("issue taxonomy (labels and milestones)", repo)

    def create_label(self, repo: str, name: str, color: str, description: str = "") -> dict:
        raise self._refuse("issue taxonomy (labels and milestones)", repo)

    def update_label(self, repo: str, name: str, *, color=None, description=None) -> dict:
        raise self._refuse("issue taxonomy (labels and milestones)", repo)

    def list_milestones(self, repo: str, *, state: str = "all") -> list[dict]:
        raise self._refuse("issue taxonomy (labels and milestones)", repo)

    def create_milestone(self, repo: str, title: str, description: str = "") -> dict:
        raise self._refuse("issue taxonomy (labels and milestones)", repo)

    # -- merge requests -----------------------------------------------------------

    def create_pull_request(
        self, repo: str, *, head: str, base: str, title: str, body: str
    ) -> dict:
        raw = self._request(
            "POST",
            f"{self._project_path(repo)}/merge_requests",
            {"source_branch": head, "target_branch": base, "title": title, "description": body},
        )
        return normalise_pull_request(raw, number_key="iid", url_key="web_url")

    def update_pull_request(
        self, repo: str, number: int, *, title: str | None = None, body: str | None = None
    ) -> dict:
        payload: dict[str, Any] = {}
        if title is not None:
            payload["title"] = title
        if body is not None:
            payload["description"] = body
        raw = self._request(
            "PUT", f"{self._project_path(repo)}/merge_requests/{int(number)}", payload
        )
        return normalise_pull_request(raw, number_key="iid", url_key="web_url")

    # -- protection de branche ------------------------------------------------------

    def get_branch_protection(self, repo: str, branch: str) -> dict:
        raise self._refuse("branch protection (GitLab uses protected_branches, not mapped)", repo)

    def put_branch_protection(self, repo: str, branch: str, payload: dict) -> dict:
        raise self._refuse("branch protection (GitLab uses protected_branches, not mapped)", repo)


# ---------------------------------------------------------------------------
# Fournisseur en mémoire (tests)
# ---------------------------------------------------------------------------


class FakeProvider:
    """Forge en mémoire : aucun binaire, aucun réseau ; enregistre chaque appel dans `.calls`.

    `responses` alimente `get_json` (clé = chemin exact). Les autres
    objets sont amorcés par `seed_*`. `fail(op, exc)` fait lever `exc` au
    prochain appel de `op`, pour jouer les refus (`NotSupported`, HTTP 404).
    """

    name = "fake"

    def __init__(self, responses: dict[str, Any] | None = None) -> None:
        self.calls: list[tuple[str, dict[str, Any]]] = []
        self.responses: dict[str, Any] = dict(responses or {})
        self._comments: dict[tuple[str, int], list[dict]] = {}
        self._next_id = 1
        self.issues: dict[str, list[dict]] = {}
        self.labels: dict[str, list[dict]] = {}
        self.milestones: dict[str, list[dict]] = {}
        self.pull_requests: dict[str, list[dict]] = {}
        self.branch_protection: dict[tuple[str, str], dict] = {}
        self.environments: dict[tuple[str, str], dict] = {}
        self.audit_events: dict[str, list[dict]] = {}
        self._failures: dict[str, BaseException] = {}

    # -- plomberie ----------------------------------------------------------

    def fail(self, op: str, exc: BaseException) -> None:
        self._failures[op] = exc

    def _record(self, op: str, **kwargs: Any) -> None:
        self.calls.append((op, kwargs))
        if op in self._failures:
            raise self._failures[op]

    def _new_id(self) -> int:
        ident = self._next_id
        self._next_id += 1
        return ident

    # -- commentaires -------------------------------------------------------

    def seed_comments(self, repo: str, number: int, comments: list[dict]) -> None:
        stored = [normalise_comment(c) for c in comments]
        for c in stored:
            self._next_id = max(self._next_id, c["id"] + 1)
        self._comments[(repo, int(number))] = stored

    def list_issue_comments(self, repo: str, number: int) -> list[dict]:
        self._record("list_issue_comments", repo=repo, number=int(number))
        return [dict(c) for c in self._comments.get((repo, int(number)), [])]

    def create_issue_comment(self, repo: str, number: int, body: str) -> dict:
        self._record("create_issue_comment", repo=repo, number=int(number), body=body)
        comment = normalise_comment({"id": self._new_id(), "body": body})
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

    # -- issues -------------------------------------------------------------

    def seed_issues(self, repo: str, issues: list[dict]) -> None:
        stored = []
        for raw in issues:
            raw = {"html_url": f"fake://{repo}/issues/{raw.get('number')}", **raw}
            item = normalise_issue(raw)
            self._next_id = max(self._next_id, item["number"] + 1)
            stored.append(item)
        self.issues[repo] = stored

    def _find_issue(self, repo: str, number: int) -> dict:
        for issue in self.issues.get(repo, []):
            if issue["number"] == int(number):
                return issue
        raise ForgeError(f"fake: issue #{number} not found in {repo}", status=404, endpoint=repo)

    def get_issue(self, repo: str, number: int) -> dict:
        self._record("get_issue", repo=repo, number=int(number))
        return dict(self._find_issue(repo, number))

    def list_issues(self, repo: str, *, state: str = "all") -> list[dict]:
        self._record("list_issues", repo=repo, state=state)
        return [
            dict(i) for i in self.issues.get(repo, []) if state == "all" or i["state"] == state
        ]

    def create_issue(self, repo: str, title: str, body: str) -> dict:
        self._record("create_issue", repo=repo, title=title, body=body)
        number = self._new_id()
        issue = normalise_issue(
            {"number": number, "title": title, "state": "open", "body": body,
             "html_url": f"fake://{repo}/issues/{number}", "labels": [], "milestone": None}
        )
        self.issues.setdefault(repo, []).append(issue)
        return dict(issue)

    def edit_issue(
        self,
        repo: str,
        number: int,
        *,
        add_labels: list[str] | None = None,
        milestone: int | None = None,
    ) -> dict:
        self._record(
            "edit_issue", repo=repo, number=int(number),
            add_labels=list(add_labels or []), milestone=milestone,
        )
        issue = self._find_issue(repo, number)
        raw = issue["raw"]
        labels = list(raw.get("labels") or [])
        for name in add_labels or []:
            if name not in labels:
                labels.append(name)
        raw["labels"] = labels
        if milestone is not None:
            raw["milestone"] = int(milestone)
        return dict(issue)

    # -- étiquettes ---------------------------------------------------------

    def seed_labels(self, repo: str, labels: list[dict]) -> None:
        self.labels[repo] = [normalise_label({"id": self._new_id(), **label}) for label in labels]

    def list_labels(self, repo: str) -> list[dict]:
        self._record("list_labels", repo=repo)
        return [dict(label) for label in self.labels.get(repo, [])]

    def create_label(self, repo: str, name: str, color: str, description: str = "") -> dict:
        self._record("create_label", repo=repo, name=name, color=color, description=description)
        label = normalise_label(
            {"id": self._new_id(), "name": name, "color": color, "description": description}
        )
        self.labels.setdefault(repo, []).append(label)
        return dict(label)

    def update_label(
        self, repo: str, name: str, *, color: str | None = None, description: str | None = None
    ) -> dict:
        self._record("update_label", repo=repo, name=name, color=color, description=description)
        for label in self.labels.get(repo, []):
            if label["name"] == name:
                if color is not None:
                    label["color"] = normalise_color(color)
                if description is not None:
                    label["description"] = description
                return dict(label)
        raise ForgeError(f"fake: label {name!r} not found in {repo}", status=404, endpoint=repo)

    # -- jalons -------------------------------------------------------------

    def seed_milestones(self, repo: str, milestones: list[dict]) -> None:
        self.milestones[repo] = [
            normalise_milestone({"number": self._new_id(), "state": "open", **m}) for m in milestones
        ]

    def list_milestones(self, repo: str, *, state: str = "all") -> list[dict]:
        self._record("list_milestones", repo=repo, state=state)
        return [dict(m) for m in self.milestones.get(repo, [])]

    def create_milestone(self, repo: str, title: str, description: str = "") -> dict:
        self._record("create_milestone", repo=repo, title=title, description=description)
        milestone = normalise_milestone(
            {"number": self._new_id(), "title": title, "description": description, "state": "open"}
        )
        self.milestones.setdefault(repo, []).append(milestone)
        return dict(milestone)

    # -- pull requests ------------------------------------------------------

    def create_pull_request(
        self, repo: str, *, head: str, base: str, title: str, body: str
    ) -> dict:
        self._record("create_pull_request", repo=repo, head=head, base=base, title=title, body=body)
        number = self._new_id()
        pr = normalise_pull_request(
            {"number": number, "title": title, "body": body, "head": head, "base": base,
             "html_url": f"fake://{repo}/pulls/{number}"}
        )
        self.pull_requests.setdefault(repo, []).append(pr)
        return dict(pr)

    def update_pull_request(
        self, repo: str, number: int, *, title: str | None = None, body: str | None = None
    ) -> dict:
        self._record("update_pull_request", repo=repo, number=int(number), title=title, body=body)
        for pr in self.pull_requests.get(repo, []):
            if pr["number"] == int(number):
                if title is not None:
                    pr["title"] = title
                    pr["raw"]["title"] = title
                if body is not None:
                    pr["raw"]["body"] = body
                return dict(pr)
        raise ForgeError(f"fake: pull request #{number} not found in {repo}", status=404, endpoint=repo)

    # -- protection de branche, environnements, audit -----------------------

    def seed_branch_protection(self, repo: str, branch: str, payload: dict) -> None:
        self.branch_protection[(repo, branch)] = dict(payload)

    def get_branch_protection(self, repo: str, branch: str) -> dict:
        self._record("get_branch_protection", repo=repo, branch=branch)
        if (repo, branch) not in self.branch_protection:
            raise ForgeError(
                f"fake: no branch protection for {repo}@{branch}", status=404, endpoint=branch
            )
        return dict(self.branch_protection[(repo, branch)])

    def put_branch_protection(self, repo: str, branch: str, payload: dict) -> dict:
        self._record("put_branch_protection", repo=repo, branch=branch, payload=payload)
        self.branch_protection[(repo, branch)] = dict(payload)
        return dict(payload)

    def seed_environment(self, repo: str, name: str, payload: dict) -> None:
        self.environments[(repo, name)] = dict(payload)

    def get_environment(self, repo: str, name: str) -> dict:
        self._record("get_environment", repo=repo, name=name)
        if (repo, name) not in self.environments:
            raise ForgeError(f"fake: no environment {name!r} in {repo}", status=404, endpoint=name)
        return dict(self.environments[(repo, name)])

    def put_environment(self, repo: str, name: str, payload: dict) -> dict:
        self._record("put_environment", repo=repo, name=name, payload=payload)
        self.environments[(repo, name)] = dict(payload)
        return dict(payload)

    def export_org_audit_log(self, org: str, *, since: str, until: str) -> list[dict]:
        self._record("export_org_audit_log", org=org, since=since, until=until)
        return [dict(e) for e in self.audit_events.get(org, [])]


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


def repo_from_env(environ: dict[str, str] | None = None, explicit: str | None = None) -> str:
    """Dépôt `owner/name` : l'argument explicite prime, sinon NOMOS_FORGE_REPO, sinon erreur nommée."""
    env = os.environ if environ is None else environ
    repo = (explicit or "").strip() or env.get(ENV_REPO, "").strip()
    if not repo:
        raise ForgeConfigError(f"{ENV_REPO} is not set and no --repo was given; refusing to guess")
    split_repo(repo)
    return repo


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
    "normalise_issue",
    "normalise_pull_request",
    "normalise_label",
    "normalise_milestone",
    "normalise_state",
    "normalise_color",
    "forgejo_protection_to_common",
    "common_protection_to_forgejo",
    "split_repo",
    "read_token",
    "repo_from_env",
    "resolve_provider_name",
    "make_provider",
    "provider_from_env",
]
