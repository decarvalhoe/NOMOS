#!/usr/bin/env python3
"""unshipped_surfaces_guard.py — les surfaces non livrées (`sdk/`, `policies/`,
`examples/<nom>/`) sont déclarées telles quelles dans les README, jamais
présentées comme livrées.

Constat du 2026-09-11 (REP-1, #747, `docs/52` §0.6) : `sdk/` et `policies/`
ne contiennent qu'un README, `examples/clinical/` et `examples/fiscal-tax/`
aussi ; les tableaux d'arborescence des trois README décrivaient `examples/`
comme « exemples de domaines », `policies/` comme « placeholder », et `sdk/`
n'y figurait pas. Doctrine `docs/43` §2.2 : une revendication non prouvée est
downgradée, jamais maquillée ; §2.8 : ce qui se tait ment.

Ce garde CALCULE depuis l'arbre quelles surfaces sont « README seul » (aucun
fichier autre que `README*.md`), puis lit chaque `README*.md` à la racine :

    python3 scripts/unshipped_surfaces_guard.py --root . --check
    python3 scripts/unshipped_surfaces_guard.py --root . --report reports/unshipped.json

Il est rouge (code 1, fichier et ligne nommés) quand :

* une phrase ou une ligne de tableau qui nomme une surface README seul la dit
  livrée, disponible, exécutable ou opérationnelle (petite liste de motifs
  FR/EN/DE ; une clause niée — « aucun », « no », « kein », « nicht »… — n'est
  pas une revendication) ;
* un README qui porte un tableau d'arborescence (lignes « | `dossier/` | »)
  omet la ligne d'une surface README seul de premier niveau (`sdk/`,
  `policies/`).

Dès qu'un vrai fichier apparaît sous la surface, la contrainte tombe d'elle-
même : le garde ne juge que l'écart entre l'arbre et la prose.

Claim boundary : ce garde ne prouve rien sur la qualité d'un SDK, d'une policy
ou d'un exemple ; il garantit seulement qu'un dossier vide n'est pas décrit
comme rempli.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

README_GLOB = "README*.md"
README_NAME = re.compile(r"^README.*\.md$", re.IGNORECASE)
TOP_LEVEL_SURFACES: tuple[str, ...] = ("sdk", "policies")
EXAMPLES_DIR = "examples"

# Une ligne de tableau d'arborescence : « | `cli/` | … ».
TABLE_DIR_ROW = re.compile(r"^\|\s*`(?P<dir>[^`]+/)`\s*\|")
SENTENCE_SPLIT = re.compile(r"(?<=[.!?])\s+")
FENCE = re.compile(r"^\s*(```|~~~)")

# Une clause niée n'affirme rien : « aucun SDK livré », « no SDK is shipped »,
# « kein SDK ausgeliefert », « n'est pas livré ».
NEGATION = re.compile(
    r"\b(?:no|not|never|none|without|aucun|aucune|ne|n|pas|jamais|sans|kein|keine|keinen|nicht|ohne)\b",
    re.IGNORECASE,
)

# Motifs de revendication par surface : (motif qui nomme la surface, motifs
# qui la disent livrée). Tous insensibles à la casse ; accents et graphies
# sans accent acceptés (les README EN/DE sont écrits sans diacritiques).
_SHIPPED = (
    r"livr[ée]e?s?|shipped|disponibles?|available|verf(?:ü|ue)gbar|ausgeliefert"
    r"|op[ée]rationnel(?:le)?s?|operational|operativ|stable|stabil"
)
_EXECUTABLE = (
    r"ex[ée]cutables?|executable|op[ée]rationnel(?:le)?s?|operational|operativ"
    r"|ausf(?:ü|ue)hrbare?|einsatzbereit|enforced|appliqu[ée]e?s?|livr[ée]e?s?|shipped|ausgeliefert"
)
_TOOLED = (
    r"outill[ée]s?|tooled|ex[ée]cutables?|executable|runnable|lauff(?:ä|ae)hig"
    r"|ausger(?:ü|ue)stet|complets?|complete|vollst(?:ä|ae)ndig"
)
_VERB = r"(?:est|is|ist|sont|are|sind|wird|was|has been|a [ée]t[ée])"

SDK_NAME = re.compile(r"\bSDK\b|`sdk/`", re.IGNORECASE)
SDK_CLAIMS: tuple[re.Pattern[str], ...] = (
    re.compile(rf"\bSDK\b[^,;]*?\s{_VERB}\s+(?:{_SHIPPED})\b", re.IGNORECASE),
    re.compile(rf"\b(?:ships?|shipped|livre|livrons|liefert|liefern)\s+(?:\w+\s+){{0,2}}SDK\b", re.IGNORECASE),
    re.compile(rf"\bSDK\s+(?:{_SHIPPED})\b", re.IGNORECASE),
)
POLICIES_NAME = re.compile(r"\bpolic(?:y|ies)\b|\bpolitiques?\b|`policies/`", re.IGNORECASE)
POLICIES_CLAIMS: tuple[re.Pattern[str], ...] = (
    re.compile(rf"\b(?:polic(?:y|ies)|politiques?)\b[^,;]*?\s{_VERB}\s+(?:{_EXECUTABLE})\b", re.IGNORECASE),
    re.compile(rf"\b(?:{_EXECUTABLE})\s+(?:polic(?:y|ies)|politiques?)\b", re.IGNORECASE),
    re.compile(rf"\b(?:politiques?|policies?/?)\s+(?:{_EXECUTABLE})\b", re.IGNORECASE),
)


def example_patterns(name: str) -> tuple[re.Pattern[str], tuple[re.Pattern[str], ...]]:
    """Motifs pour `examples/<name>/` : le nom, puis « outillé/tooled/… » dans la même clause."""
    quoted = re.escape(name)
    return (
        re.compile(rf"\b{quoted}\b", re.IGNORECASE),
        (
            re.compile(rf"\b{quoted}\b[^,;]*?\b(?:{_TOOLED})\b", re.IGNORECASE),
            re.compile(rf"\b(?:{_TOOLED})\b[^;]*?\b{quoted}\b", re.IGNORECASE),
        ),
    )


def is_readme_only(directory: Path) -> tuple[bool, list[str]]:
    """Rend (README seul ?, fichiers autres que README*.md) pour un dossier."""
    if not directory.is_dir():
        return False, []
    others = sorted(
        str(p.relative_to(directory))
        for p in directory.rglob("*")
        if p.is_file() and not README_NAME.match(p.name)
    )
    return not others, others


def compute_surfaces(root: Path) -> dict[str, dict]:
    """Calcule depuis l'arbre l'état de chaque surface : `sdk/`, `policies/`, `examples/<nom>/`."""
    surfaces: dict[str, dict] = {}
    for name in TOP_LEVEL_SURFACES:
        directory = root / name
        if not directory.is_dir():
            continue
        readme_only, others = is_readme_only(directory)
        surfaces[f"{name}/"] = {"readme_only": readme_only, "files": others, "top_level": True}
    examples = root / EXAMPLES_DIR
    if examples.is_dir():
        for child in sorted(p for p in examples.iterdir() if p.is_dir()):
            readme_only, others = is_readme_only(child)
            surfaces[f"{EXAMPLES_DIR}/{child.name}/"] = {
                "readme_only": readme_only,
                "files": others,
                "top_level": False,
            }
    return surfaces


def _rules(surfaces: dict[str, dict]) -> list[tuple[str, re.Pattern[str], tuple[re.Pattern[str], ...]]]:
    rules = []
    for key, state in surfaces.items():
        if not state["readme_only"]:
            continue
        if key == "sdk/":
            rules.append((key, SDK_NAME, SDK_CLAIMS))
        elif key == "policies/":
            rules.append((key, POLICIES_NAME, POLICIES_CLAIMS))
        elif key.startswith(f"{EXAMPLES_DIR}/"):
            name = key[len(EXAMPLES_DIR) + 1 : -1]
            rules.append((key, *example_patterns(name)))
    return rules


def units(text: str) -> list[tuple[int, str]]:
    """Découpe un README en unités (ligne, texte) : lignes de tableau telles
    quelles, prose par phrase ; les blocs de code sont ignorés."""
    out: list[tuple[int, str]] = []
    paragraph: list[tuple[int, str]] = []

    def flush() -> None:
        if not paragraph:
            return
        start = paragraph[0][0]
        joined = "\n".join(line for _, line in paragraph)
        position = 0
        for sentence in SENTENCE_SPLIT.split(joined):
            offset = joined.find(sentence, position)
            line_no = start + joined[:offset].count("\n")
            out.append((line_no, sentence.strip()))
            position = offset + len(sentence)
        paragraph.clear()

    in_fence = False
    for index, line in enumerate(text.split("\n"), start=1):
        if FENCE.match(line):
            in_fence = not in_fence
            flush()
            continue
        if in_fence:
            continue
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            flush()
            continue
        if stripped.startswith("|"):
            flush()
            out.append((index, stripped))
            continue
        paragraph.append((index, line))
    flush()
    return out


def scan_readme(text: str, surfaces: dict[str, dict]) -> list[dict]:
    """Rend les revendications indues et les lignes de tableau manquantes d'un README."""
    failures: list[dict] = []
    rules = _rules(surfaces)
    for line_no, unit in units(text):
        for clause in re.split(r"\s*;\s*", unit):
            if NEGATION.search(clause):
                continue
            for key, name, claims in rules:
                if not name.search(clause):
                    continue
                if any(c.search(clause) for c in claims):
                    failures.append(
                        {"line": line_no, "surface": key, "reason": "revendiquée comme livrée", "text": clause}
                    )
    rows = [(no, TABLE_DIR_ROW.match(u).group("dir")) for no, u in units(text) if TABLE_DIR_ROW.match(u)]
    if rows:
        present = {d for _, d in rows}
        for key, state in surfaces.items():
            if state["top_level"] and state["readme_only"] and key not in present:
                failures.append(
                    {
                        "line": rows[0][0],
                        "surface": key,
                        "reason": "absente du tableau d'arborescence",
                        "text": f"aucune ligne « | `{key}` | »",
                    }
                )
    return failures


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    parser.add_argument("--root", default=".", help="racine du dépôt")
    parser.add_argument("--check", action="store_true", help="échoue à la moindre revendication indue (défaut)")
    parser.add_argument("--report", help="écrit un rapport JSON à ce chemin")
    args = parser.parse_args(argv)

    root = Path(args.root)
    surfaces = compute_surfaces(root)
    failures: list[dict] = []
    for readme in sorted(root.glob(README_GLOB)):
        for failure in scan_readme(readme.read_text(encoding="utf-8"), surfaces):
            failures.append({"file": readme.name, **failure})

    if args.report:
        report = {"root": str(root), "surfaces": surfaces, "failures": failures, "ok": not failures}
        report_path = Path(args.report)
        report_path.parent.mkdir(parents=True, exist_ok=True)
        report_path.write_text(json.dumps(report, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")

    readme_only = sorted(k for k, s in surfaces.items() if s["readme_only"])
    label = ", ".join(readme_only) or "aucune"
    if failures:
        print(f"unshipped surfaces: ROUGE (README seul : {label})", file=sys.stderr)
        for failure in failures:
            print(
                f"  {failure['file']} ligne {failure['line']}: {failure['surface']} {failure['reason']} — {failure['text']}",
                file=sys.stderr,
            )
        return 1
    print(f"unshipped surfaces: OK (README seul : {label})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
