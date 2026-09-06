"""NRT-023 (#676) — the contract registry is complete and its fixtures vet.

The Go verifier (cli/internal/contracts) checks hashes, versions, fixtures and
compatibility reads. This sidecar test adds what only `cue` can say: every
registered valid fixture vets against the registered definition and every
registered invalid fixture does NOT. Fixtures listed under reader_valid /
reader_invalid belong to a reader's contract (a script), not to cue, and are
exercised by that reader's own tests. Skipped, named, when cue is absent.

A contract file may reference definitions declared in sibling files of the
`specs` package; vetting the whole package at once unifies unrelated
definitions that share a name across files, so the file set is resolved per
contract: the file plus, transitively, the files declaring what it references."""

from __future__ import annotations

import hashlib
import re
import shutil
import subprocess
import unittest
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
REG = ROOT / "specs" / "contract-registry.yaml"
CUE_FILES = sorted(ROOT.glob("specs/*.cue")) + [ROOT / "attestations" / "nomos-attestation.cue"]
DEF_DECL = re.compile(r"^(#[A-Za-z][A-Za-z0-9_]*)\s*:", re.M)
DEF_REF = re.compile(r"(#[A-Za-z][A-Za-z0-9_]*)")


def registry() -> dict:
    return yaml.safe_load(REG.read_text(encoding="utf-8"))


def declarations() -> dict[str, Path]:
    decl: dict[str, Path] = {}
    for f in CUE_FILES:
        for name in DEF_DECL.findall(f.read_text(encoding="utf-8")):
            decl.setdefault(name, f)
    return decl


def vet_files(contract: Path, decl: dict[str, Path]) -> list[str]:
    files, todo = [], [contract]
    while todo:
        f = todo.pop()
        if f in files:
            continue
        files.append(f)
        text = f.read_text(encoding="utf-8")
        declared = set(DEF_DECL.findall(text))
        for ref in set(DEF_REF.findall(text)) - declared:
            if ref in decl and decl[ref] not in files:
                todo.append(decl[ref])
    return [str(f.relative_to(ROOT)) for f in files]


class ContractRegistryTests(unittest.TestCase):
    def test_registry_covers_every_contract_file(self) -> None:
        reg = registry()
        self.assertEqual(reg["schema_version"], "nomos-contract-registry-v1")
        paths = {c["path"] for c in reg["contracts"]}
        files = {f"specs/{p.name}" for p in (ROOT / "specs").glob("*.cue")} | {f"specs/{p.name}" for p in (ROOT / "specs").glob("*.schema.json")}
        self.assertEqual(files - paths, set(), "contract files without a registry entry")
        self.assertEqual(paths - files, set(), "registry entries without a file")
        for c in reg["contracts"]:
            digest = "sha256:" + hashlib.sha256((ROOT / c["path"]).read_bytes()).hexdigest()
            self.assertEqual(c["sha256"], digest, f"{c['id']}: hash drift — bump and accept")
            if c["stability"] == "stable":
                self.assertTrue(c["fixtures"]["valid"], f"{c['id']} is stable without a valid fixture")
                self.assertTrue(c["schema_version"], f"{c['id']} is stable without a version")
            for f in c["fixtures"]["valid"] + c["fixtures"]["invalid"]:
                self.assertTrue((ROOT / f).exists(), f"{c['id']}: fixture {f} missing")
            for rf in c["fixtures"].get("reader_valid", []) + c["fixtures"].get("reader_invalid", []):
                self.assertTrue((ROOT / rf["path"]).exists() and (ROOT / rf["reader"]).exists(), f"{c['id']}: reader fixture {rf}")

    def test_every_registered_fixture_vets_against_its_definition(self) -> None:
        cue = shutil.which("cue")
        if not cue:
            self.skipTest("cue not installed; the Go verifier still checks hashes, versions and reads")
        reg = registry()
        decl = declarations()
        checked, problems = 0, []
        for c in reg["contracts"]:
            if not c["path"].endswith(".cue") or not c.get("definition"):
                continue
            files = vet_files(ROOT / c["path"], decl)
            overrides = c.get("definition_overrides") or {}
            for f in c["fixtures"]["valid"]:
                d = overrides.get(f, c["definition"])
                r = subprocess.run([cue, "vet", "-d", d, *files, f], cwd=ROOT, capture_output=True, text=True)
                checked += 1
                if r.returncode != 0:
                    problems.append(f"{c['id']}: valid fixture {f} does not vet against {d} with {files}:\n{r.stderr.strip()}")
            for f in c["fixtures"]["invalid"]:
                d = overrides.get(f, c["definition"])
                r = subprocess.run([cue, "vet", "-d", d, *files, f], cwd=ROOT, capture_output=True, text=True)
                checked += 1
                if r.returncode == 0:
                    problems.append(f"{c['id']}: invalid fixture {f} vets against {d} — it proves nothing (register it under reader_invalid if a reader rejects it)")
        self.maxDiff = None
        self.assertEqual(problems, [], "\n\n".join(problems))
        self.assertGreater(checked, 40)


if __name__ == "__main__":
    unittest.main()
