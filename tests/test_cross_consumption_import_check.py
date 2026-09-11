"""NRT-029 (#702) — the cross-consumption import proof, and the proofs are the
refusals (doctrine §2.3):

* an index that is exactly the export passes, in the neutral shape, in the
  langchain projection and as a hashes-only dump, and the digest recomputed
  from the consumer's records equals the manifest's;
* a missing chunk, an extra chunk, a duplicated chunk, an altered body, an
  altered embedding text (the digest moves), a forged source_hash, an empty
  index and a wrong manifest schema are red and named;
* a line nobody can read is a named finding, never a skipped line;
* citations that name a chunk the manifest does not contain, or carry another
  source_hash, are red;
* an unreadable manifest is a usage error, not a pass;
* docs/50 replays end to end against the fixtures when Go is available.
"""

from __future__ import annotations

import hashlib
import json
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "cross_consumption_import_check.py"
REPLAY = ROOT / "scripts" / "integration_guide_replay.py"
GUIDE = ROOT / "docs" / "50-cross-consumption-proof-kit.md"

SOURCES = {"a.md": "sha256:" + "a" * 64, "b.md": "sha256:" + "b" * 64}
CHUNKS = [
    ("chunk:1", "a.md", "a.md · Un\n\nPremier corps.", "Premier corps."),
    ("chunk:2", "a.md", "a.md · Deux\n\nDeuxième corps.", "Deuxième corps."),
    ("chunk:3", "b.md", "b.md · Trois\n\nTroisième corps.", "Troisième corps."),
]


def sha(text: str) -> str:
    return "sha256:" + hashlib.sha256(text.encode("utf-8")).hexdigest()


def fingerprints() -> list[dict]:
    return [
        {"chunk_id": cid, "source_id": sid, "source_hash": SOURCES[sid], "embedding_hash": sha(emb), "body_hash": sha(body)}
        for cid, sid, emb, body in CHUNKS
    ]


def digest(prints: list[dict]) -> str:
    h = hashlib.sha256()
    for p in sorted(prints, key=lambda p: p["chunk_id"]):
        h.update(p["chunk_id"].encode())
        h.update(b"\0")
        h.update(p["source_hash"].encode())
        h.update(b"\0")
        h.update(p["embedding_hash"].removeprefix("sha256:").encode())
        h.update(b"\0")
    return "sha256:" + h.hexdigest()


def manifest() -> dict:
    prints = fingerprints()
    return {
        "schema_version": "nomos-rag-index-manifest-v1",
        "chunk_count": len(prints),
        "chunk_digest": digest(prints),
        "sources": [
            {"source_id": "a.md", "source_hash": SOURCES["a.md"], "chunk_count": 2},
            {"source_id": "b.md", "source_hash": SOURCES["b.md"], "chunk_count": 1},
        ],
        "chunks": prints,
    }


def neutral_records() -> list[dict]:
    return [
        {"schema_version": "nomos-rag-chunk-v1", "chunk_id": cid, "embedding_text": emb, "body_text": body, "provenance": {"source_id": sid, "source_hash": SOURCES[sid]}}
        for cid, sid, emb, body in CHUNKS
    ]


def langchain_records() -> list[dict]:
    return [
        {"page_content": emb, "metadata": {"chunk_id": cid, "source_id": sid, "source_hash": SOURCES[sid], "body_text": body}}
        for cid, sid, emb, body in CHUNKS
    ]


def hashes_only_records() -> list[dict]:
    return [
        {"chunk_id": cid, "source_hash": SOURCES[sid], "embedding_hash": sha(emb), "body_hash": sha(body)}
        for cid, sid, emb, body in CHUNKS
    ]


def answers(spans: list[dict]) -> dict:
    return {"answers": [{"answer_id": "A-1", "answer": "x", "source_spans": spans, "retrieved_chunks": []}]}


class Harness(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.dir = Path(self._tmp.name)

    def write_manifest(self, doc: dict | None = None, raw: str | None = None) -> Path:
        path = self.dir / "manifest.json"
        path.write_text(raw if raw is not None else json.dumps(doc or manifest()), encoding="utf-8")
        return path

    def write_index(self, records: list[dict], raw_lines: list[str] | None = None) -> Path:
        path = self.dir / "index.jsonl"
        lines = [json.dumps(r, ensure_ascii=False) for r in records] + (raw_lines or [])
        path.write_text("\n".join(lines) + "\n", encoding="utf-8")
        return path

    def run_check(self, manifest_path: Path, index_path: Path, *extra: str) -> tuple[int, dict, str]:
        report = self.dir / "verdict.json"
        proc = subprocess.run(
            [sys.executable, str(SCRIPT), "--manifest", str(manifest_path), "--index", str(index_path), "--report", str(report), *extra],
            capture_output=True, text=True, check=False,
        )
        verdict = json.loads(report.read_text(encoding="utf-8")) if report.exists() else {}
        return proc.returncode, verdict, proc.stderr


class IdenticalIndexTests(Harness):
    def test_neutral_records_pass_and_the_digest_is_the_manifests(self) -> None:
        code, verdict, stderr = self.run_check(self.write_manifest(), self.write_index(neutral_records()))
        self.assertEqual(code, 0, stderr)
        self.assertEqual(verdict["status"], "pass")
        detail = verdict["index_check"]
        self.assertEqual(detail["matched"], 3)
        self.assertEqual(detail["missing"], [])
        self.assertEqual(detail["extra"], [])
        self.assertEqual(detail["index_digest"], manifest()["chunk_digest"])

    def test_langchain_projection_passes(self) -> None:
        code, verdict, stderr = self.run_check(self.write_manifest(), self.write_index(langchain_records()))
        self.assertEqual(code, 0, stderr)
        self.assertEqual(verdict["index_check"]["matched"], 3)

    def test_hashes_only_dump_passes(self) -> None:
        code, verdict, stderr = self.run_check(self.write_manifest(), self.write_index(hashes_only_records()))
        self.assertEqual(code, 0, stderr)
        self.assertEqual(verdict["index_check"]["matched"], 3)


class RefusalTests(Harness):
    def test_missing_chunk_is_red(self) -> None:
        code, verdict, _ = self.run_check(self.write_manifest(), self.write_index(neutral_records()[:2]))
        self.assertEqual(code, 1)
        self.assertEqual(verdict["index_check"]["missing"], ["chunk:3"])
        self.assertTrue(any("chunk:3: in the manifest, not in the index" in f for f in verdict["findings"]), verdict["findings"])
        self.assertTrue(any("source b.md: 0 chunk(s) indexed, manifest says 1" in f for f in verdict["findings"]), verdict["findings"])

    def test_extra_chunk_is_red(self) -> None:
        records = neutral_records() + [{"chunk_id": "chunk:9", "embedding_text": "x", "body_text": "x", "provenance": {"source_hash": SOURCES["a.md"]}}]
        code, verdict, _ = self.run_check(self.write_manifest(), self.write_index(records))
        self.assertEqual(code, 1)
        self.assertEqual(verdict["index_check"]["extra"], ["chunk:9"])
        self.assertTrue(any("chunk:9: in the index, not in the manifest" in f for f in verdict["findings"]), verdict["findings"])

    def test_duplicated_chunk_is_red(self) -> None:
        records = neutral_records() + neutral_records()[:1]
        code, verdict, _ = self.run_check(self.write_manifest(), self.write_index(records))
        self.assertEqual(code, 1)
        self.assertTrue(any("chunk:1: indexed twice" in f for f in verdict["findings"]), verdict["findings"])

    def test_altered_body_is_red_and_named(self) -> None:
        records = neutral_records()
        records[1]["body_text"] = "Deuxième corps, modifié."
        code, verdict, _ = self.run_check(self.write_manifest(), self.write_index(records))
        self.assertEqual(code, 1)
        self.assertEqual(verdict["index_check"]["mismatched"], ["chunk:2"])
        self.assertTrue(any(f.startswith("chunk:2: body_hash ") for f in verdict["findings"]), verdict["findings"])

    def test_altered_embedding_text_moves_the_digest(self) -> None:
        records = neutral_records()
        records[0]["embedding_text"] = "a.md · Un\n\nPremier corps réécrit."
        code, verdict, _ = self.run_check(self.write_manifest(), self.write_index(records))
        self.assertEqual(code, 1)
        self.assertTrue(any(f.startswith("chunk:1: embedding_hash ") for f in verdict["findings"]), verdict["findings"])
        self.assertTrue(any("index digest" in f and "the index is not the export" in f for f in verdict["findings"]), verdict["findings"])

    def test_forged_source_hash_is_red(self) -> None:
        records = neutral_records()
        records[2]["provenance"]["source_hash"] = "sha256:" + "f" * 64
        code, verdict, _ = self.run_check(self.write_manifest(), self.write_index(records))
        self.assertEqual(code, 1)
        self.assertTrue(any(f.startswith("chunk:3: source_hash ") for f in verdict["findings"]), verdict["findings"])

    def test_empty_index_is_red(self) -> None:
        code, verdict, _ = self.run_check(self.write_manifest(), self.write_index([]))
        self.assertEqual(code, 1)
        self.assertTrue(any("the index is empty" in f for f in verdict["findings"]), verdict["findings"])

    def test_unreadable_line_is_named_not_skipped(self) -> None:
        code, verdict, _ = self.run_check(self.write_manifest(), self.write_index(neutral_records(), raw_lines=["{not json", json.dumps({"embedding_text": "x"})]))
        self.assertEqual(code, 1)
        self.assertTrue(any("index line 4: not JSON" in f for f in verdict["findings"]), verdict["findings"])
        self.assertTrue(any("index line 5: record carries no chunk_id" in f for f in verdict["findings"]), verdict["findings"])
        self.assertEqual(verdict["index_check"]["matched"], 3, "the readable records are still compared")

    def test_wrong_manifest_schema_is_red(self) -> None:
        doc = manifest()
        doc["schema_version"] = "nomos-rag-index-manifest-v2"
        code, verdict, _ = self.run_check(self.write_manifest(doc), self.write_index(neutral_records()))
        self.assertEqual(code, 1)
        self.assertTrue(any("schema_version" in f and "refuse" in f for f in verdict["findings"]), verdict["findings"])

    def test_unreadable_manifest_is_a_usage_error(self) -> None:
        code, verdict, stderr = self.run_check(self.write_manifest(raw="{"), self.write_index(neutral_records()))
        self.assertEqual(code, 2)
        self.assertEqual(verdict, {})
        self.assertIn("manifest unreadable", stderr)


class CitationTests(Harness):
    def write_citations(self, spans: list[dict]) -> Path:
        path = self.dir / "answers.yaml"
        path.write_text(yaml.safe_dump(answers(spans), sort_keys=False, allow_unicode=True), encoding="utf-8")
        return path

    def test_resolvable_citations_pass(self) -> None:
        citations = self.write_citations([{"source_id": "a.md", "source_hash": SOURCES["a.md"], "span": "L1-L2", "chunk_id": "chunk:1", "text": "Premier corps."}])
        code, verdict, stderr = self.run_check(self.write_manifest(), self.write_index(neutral_records()), "--citations", str(citations))
        self.assertEqual(code, 0, stderr)
        self.assertEqual(verdict["citations"][0]["references_checked"], 1)

    def test_citation_of_an_unknown_chunk_is_red(self) -> None:
        citations = self.write_citations([{"source_id": "a.md", "source_hash": SOURCES["a.md"], "span": "L1", "chunk_id": "chunk:404", "text": "x"}])
        code, verdict, _ = self.run_check(self.write_manifest(), self.write_index(neutral_records()), "--citations", str(citations))
        self.assertEqual(code, 1)
        self.assertTrue(any("A-1: source_spans cites chunk:404, which is not in the manifest" in f for f in verdict["findings"]), verdict["findings"])

    def test_citation_with_another_source_hash_is_red(self) -> None:
        citations = self.write_citations([{"source_id": "a.md", "source_hash": "sha256:" + "9" * 64, "span": "L1", "chunk_id": "chunk:1", "text": "x"}])
        code, verdict, _ = self.run_check(self.write_manifest(), self.write_index(neutral_records()), "--citations", str(citations))
        self.assertEqual(code, 1)
        self.assertTrue(any("A-1: source_spans chunk:1 carries source_hash" in f for f in verdict["findings"]), verdict["findings"])

    def test_citation_without_chunk_id_is_red(self) -> None:
        citations = self.write_citations([{"source_id": "a.md", "source_hash": SOURCES["a.md"], "span": "L1", "text": "x"}])
        code, verdict, _ = self.run_check(self.write_manifest(), self.write_index(neutral_records()), "--citations", str(citations))
        self.assertEqual(code, 1)
        self.assertTrue(any("names no chunk_id" in f for f in verdict["findings"]), verdict["findings"])


@unittest.skipIf(shutil.which("go") is None, "go not available — the kit guide replay builds cli/")
class KitGuideReplayTests(unittest.TestCase):
    def test_the_kit_guide_replays_against_the_fixtures(self) -> None:
        proc = subprocess.run([sys.executable, str(REPLAY), "--root", str(ROOT), "--guide", str(GUIDE)], capture_output=True, text=True, check=False)
        self.assertEqual(proc.returncode, 0, proc.stderr + proc.stdout)
        self.assertIn("replay: OK", proc.stdout)


if __name__ == "__main__":
    unittest.main()
