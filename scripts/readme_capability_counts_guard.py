#!/usr/bin/env python3
"""readme_capability_counts_guard.py — les compteurs de capacités des README
sont recopiés depuis la matrice de câblage, jamais déclarés à la main.

Constat du 2026-09-11 : `README.md` annonçait 57 capacités (44 réelles,
11 sidecar, 2 absentes), `README.en.md` en annonçait 40 (32/7/1), alors que
`.vrc-wiring-matrix/wiring-matrix.json` — le seul fichier calculé depuis
l'arbre — en comptait 66 (47/17/2). La matrice était juste ; la prose était
périmée. Doctrine `docs/43` §2.8 : une constante qui ne décrit plus la machine
est une déclaration périmée, et « ce qui se tait ment ».

Ce garde lit le bloc `summary` de la matrice générée et compare chaque nombre
que les README avancent sur ce sujet :

    python3 scripts/readme_capability_counts_guard.py --root . --check
    python3 scripts/readme_capability_counts_guard.py --root . --write

`--check` échoue (code 1) à la moindre dérive et nomme le fichier, la ligne et
les deux valeurs. `--write` réécrit les nombres en place. Une phrase absente
d'un README n'est pas une dérive : le garde ne juge que ce qui est écrit.

Claim boundary : ce garde ne prouve rien sur les capacités elles-mêmes ; il
garantit seulement que la prose recopie la matrice.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

MATRIX_PATH = Path(".vrc-wiring-matrix/wiring-matrix.json")
README_GLOB = "README*.md"

# Chaque motif capture les nombres dans l'ordre des clés annoncées. Les groupes
# sont nommés d'après `summary` : total, real, sidecar, absent.
PATTERNS: tuple[re.Pattern[str], ...] = (
    # FR : « 66 capacités déclarées dans … (47 réelles, 17 sidecar, 2 absentes … »
    re.compile(
        r"(?P<total>\d+) capacités déclarées dans `scripts/vrc_wiring_matrix_registry\.json`"
        r"[^(]*\((?P<real>\d+) réelles, (?P<sidecar>\d+) sidecar, (?P<absent>\d+) absentes"
    ),
    # EN : « 66 capabilities declared in … (47 real, 17 sidecar, 2 absent, … »
    re.compile(
        r"(?P<total>\d+) capabilities declared in `scripts/vrc_wiring_matrix_registry\.json`"
        r"[^(]*\((?P<real>\d+) real, (?P<sidecar>\d+) sidecar, (?P<absent>\d+) absent"
    ),
    # Lignes de tableau « Matrice de câblage (VRC-00) | 66 capacités, 0 écart »
    re.compile(r"\(VRC-00\) \| (?P<total>\d+) capacités, "),
    re.compile(r"\(VRC-00\) \| (?P<total>\d+) capabilities, "),
    re.compile(r"\(VRC-00\) \| (?P<total>\d+) Faehigkeiten, "),
)


def load_expected(root: Path) -> dict[str, int]:
    """Lit `summary` de la matrice générée et rend total/real/sidecar/absent."""
    matrix = json.loads((root / MATRIX_PATH).read_text(encoding="utf-8"))
    summary = matrix["summary"]
    computed = summary["computed"]
    return {
        "total": int(summary["capabilities"]),
        "real": int(computed["real"]),
        "sidecar": int(computed["sidecar"]),
        "absent": int(computed["absent"]),
    }


def scan(text: str, expected: dict[str, int]) -> tuple[str, list[str]]:
    """Rend (texte corrigé, liste des dérives « ligne: clé déclaré≠attendu »)."""
    drifts: list[str] = []
    lines = text.split("\n")
    for index, line in enumerate(lines, start=1):
        for pattern in PATTERNS:
            match = pattern.search(line)
            if not match:
                continue
            for key, declared in match.groupdict().items():
                wanted = expected[key]
                if int(declared) != wanted:
                    drifts.append(f"ligne {index}: {key} déclaré {declared}, matrice {wanted}")
            # Réécriture par remplacement des groupes, de droite à gauche pour
            # ne pas décaler les positions.
            for key in sorted(match.groupdict(), key=lambda k: match.start(k), reverse=True):
                start, end = match.span(key)
                line = line[:start] + str(expected[key]) + line[end:]
        lines[index - 1] = line
    return "\n".join(lines), drifts


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    parser.add_argument("--root", default=".", help="racine du dépôt")
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument("--check", action="store_true", help="échoue à la moindre dérive (défaut)")
    mode.add_argument("--write", action="store_true", help="réécrit les nombres dans les README")
    args = parser.parse_args(argv)

    root = Path(args.root)
    expected = load_expected(root)
    failures: list[str] = []
    rewritten: list[str] = []
    for readme in sorted(root.glob(README_GLOB)):
        text = readme.read_text(encoding="utf-8")
        fixed, drifts = scan(text, expected)
        if not drifts:
            continue
        if args.write:
            readme.write_text(fixed, encoding="utf-8")
            rewritten.append(readme.name)
        else:
            failures.extend(f"{readme.name} {d}" for d in drifts)

    label = ", ".join(f"{k}={v}" for k, v in expected.items())
    if failures:
        print(f"readme capability counts: DÉRIVE ({label})", file=sys.stderr)
        for failure in failures:
            print(f"  {failure}", file=sys.stderr)
        print("  remède : python3 scripts/readme_capability_counts_guard.py --root . --write", file=sys.stderr)
        return 1
    if rewritten:
        print(f"readme capability counts: réécrit {', '.join(rewritten)} ({label})")
    else:
        print(f"readme capability counts: OK ({label})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
