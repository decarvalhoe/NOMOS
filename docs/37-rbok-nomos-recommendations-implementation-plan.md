# RBOK NOMOS Recommendations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make RBOK consume NOMOS outputs as a living, source-backed doctrine for import, RAG, conversation runtime, YAML/JSON parcours objects, and admin traceability.

**Architecture:** NOMOS remains the authority-to-product transformer. RBOK consumes a read-only output bundle, validates it, imports immutable projections, activates one feed version in dev, then uses that active version for doctrine retrieval and conversation traceability. `realisons-business` remains source of truth and must not be mutated by RBOK or NOMOS output publication.

**Tech Stack:** RBOK FastAPI, SQLAlchemy 2.0 async, Alembic, pytest/pytest-asyncio, Next.js admin surfaces, GitHub PR workflow to `develop`, NOMOS output bundle artifacts.

---

## Non-Negotiable Constraints

- Work in `RBOKproject/RBOK`, from `develop`, on feature branches.
- PR target is always `develop` for this POC.
- No direct push to `develop`, `staging`, or `main`.
- Do not write to `RBOKproject/realisons-business`.
- Do not treat `rag-metadata.json` as full text.
- Do not treat `engine-import.json` as full text.
- Do not activate a feed when `strict-gate.json` is missing or failed.
- Do not close `RBOK#2704` or `RBOK#2711` again until their tests pass against the current NOMOS format.
- Keep the first POC on dev. Production activation is out of scope.

## Input Bundle Contract

The importer must support the current NOMOS bundle shape used by the POC.

Required files for the POC:

```text
feed.json or rbok-lawbook-feed.json
rag-metadata.json or rbok-rag-metadata.json
strict-gate.json or rbok-strict-fidelity-gate.json
attestation.json
source-manifest.yaml or equivalent manifest
corpus-body-ledger.json when available
```

Supported feed content locations:

```text
# Structured universal feed
feed.json
  format: nomos.corpus-feed.v1
  content_hash
  source_count
  unit_count
  units[]
    unit_id
    name
    unit_type
    business_rule
    source_id
    source_path
    start_byte
    end_byte
    start_line
    end_line
    normalized_text_hash
    heading_path
    body_ledger_segment_ids

# Historical lawbook feed
rbok-lawbook-feed.json
  feeds[]
    source_path
    source_hash
    nodes[]
      node_id
      node_type
      title
      text
      span
      canonical_ref
      display_ref
      parent_chain
```

RAG metadata is downstream metadata only:

```text
rag-metadata.json
  chunk_id
  source_id
  source_path
  source_hash
  unit_ids
  locator
  priority
  status
  semantic_tags
  token_count
  char_count
  start_byte
  end_byte
  start_line
  end_line
```

## Implementation Sequence

Do the work in this order. Each task has a single dependency path and can be reviewed independently.

```text
Task 1 Bundle fixtures and contract tests
  -> Task 2 Feed parser and bundle validator
    -> Task 3 Importer persistence and activation gate
      -> Task 4 Traceability API repair
        -> Task 5 Runtime DoctrineRAGRetriever wiring
          -> Task 6 Conversation behavior policy tests
            -> Task 7 YAML/JSON parcours gap handling
              -> Task 8 POC workflow and evidence dossier
```

## Task 1 - Bundle Fixtures And Contract Tests

**Purpose:** Lock the real NOMOS bundle shape before changing importer code.

**Files:**

- Create: `backend/tests/fixtures/nomos/current_bundle.py`
- Create: `backend/tests/unit/nomos/test_current_bundle_contract.py`
- Do not modify production code in this task.

- [ ] **Step 1: Create a minimal current NOMOS bundle fixture**

Create `backend/tests/fixtures/nomos/current_bundle.py` with this shape:

```python
from __future__ import annotations

from copy import deepcopy


def current_structured_feed() -> dict:
    return {
        "format": "nomos.corpus-feed.v1",
        "generated_at": "2026-05-04T10:02:22Z",
        "content_hash": "sha256:feedhash",
        "unit_count": 2,
        "source_count": 1,
        "units": [
            {
                "unit_id": "RBOK-MD-DEMO-001",
                "name": "Introduction",
                "domain": "rbok",
                "unit_type": "rule",
                "criticality": "medium",
                "status": "partial",
                "business_rule": "La doctrine RBOK doit rester source-backed.",
                "source_id": "CORPUS-RBOK-DEMO",
                "source_path": "00_meta/demo.md",
                "start_byte": 10,
                "end_byte": 55,
                "start_line": 2,
                "end_line": 2,
                "normalized_text_hash": "hash-unit-001",
                "heading_path": ["Demo", "Introduction"],
                "body_ledger_segment_ids": ["seg:CORPUS-RBOK-DEMO:10-55:paragraph"],
            },
            {
                "unit_id": "RBOK-MD-DEMO-002",
                "name": "Question active",
                "domain": "rbok",
                "unit_type": "rule",
                "criticality": "high",
                "status": "active",
                "business_rule": "Le runtime ne doit poser que la question du step courant.",
                "source_id": "CORPUS-RBOK-DEMO",
                "source_path": "03_parcours/demo.md",
                "start_byte": 100,
                "end_byte": 170,
                "start_line": 8,
                "end_line": 8,
                "normalized_text_hash": "hash-unit-002",
                "heading_path": ["Demo", "Parcours"],
                "body_ledger_segment_ids": ["seg:CORPUS-RBOK-DEMO:100-170:paragraph"],
            },
        ],
    }


def current_rag_metadata() -> list[dict]:
    return [
        {
            "chunk_id": "chunk:CORPUS-RBOK-DEMO:10-55",
            "source_id": "CORPUS-RBOK-DEMO",
            "source_path": "00_meta/demo.md",
            "source_hash": "sha256:sourcehash",
            "unit_ids": ["RBOK-MD-DEMO-001"],
            "locator": "00_meta/demo.md:L2-L2",
            "priority": "primary",
            "status": "active",
            "semantic_tags": ["rbok", "Demo", "Introduction"],
            "token_count": 7,
            "char_count": 45,
            "start_byte": 10,
            "end_byte": 55,
            "start_line": 2,
            "end_line": 2,
        },
        {
            "chunk_id": "chunk:CORPUS-RBOK-DEMO:100-170",
            "source_id": "CORPUS-RBOK-DEMO",
            "source_path": "03_parcours/demo.md",
            "source_hash": "sha256:sourcehash",
            "unit_ids": ["RBOK-MD-DEMO-002"],
            "locator": "03_parcours/demo.md:L8-L8",
            "priority": "primary",
            "status": "active",
            "semantic_tags": ["rbok", "Demo", "Parcours"],
            "token_count": 10,
            "char_count": 70,
            "start_byte": 100,
            "end_byte": 170,
            "start_line": 8,
            "end_line": 8,
        },
    ]


def strict_gate_pass() -> dict:
    return {"status": "pass", "findings": [], "blocking_findings": []}


def strict_gate_fail() -> dict:
    return {
        "status": "fail",
        "findings": [{"severity": "blocking", "code": "SOURCE_GAP"}],
        "blocking_findings": [{"code": "SOURCE_GAP"}],
    }


def bundle() -> dict:
    return {
        "feed": current_structured_feed(),
        "rag_metadata": current_rag_metadata(),
        "strict_gate": strict_gate_pass(),
        "attestation": {"predicateType": "nomos.attestation.v1"},
        "manifest": {"sources": [{"path": "00_meta/demo.md"}]},
    }


def bundle_without_text() -> dict:
    data = deepcopy(bundle())
    for unit in data["feed"]["units"]:
        unit.pop("business_rule", None)
    return data
```

- [ ] **Step 2: Write contract tests that fail against current importer assumptions**

Create `backend/tests/unit/nomos/test_current_bundle_contract.py`:

```python
from tests.fixtures.nomos.current_bundle import (
    bundle,
    bundle_without_text,
    strict_gate_fail,
)


def test_current_bundle_contains_text_in_feed_units():
    data = bundle()
    texts = [unit["business_rule"] for unit in data["feed"]["units"]]
    assert texts == [
        "La doctrine RBOK doit rester source-backed.",
        "Le runtime ne doit poser que la question du step courant.",
    ]


def test_current_bundle_rag_metadata_is_not_text_source():
    data = bundle()
    assert "business_rule" not in data["rag_metadata"][0]
    assert "text" not in data["rag_metadata"][0]


def test_bundle_without_feed_text_is_invalid_for_runtime_import():
    data = bundle_without_text()
    assert all("business_rule" not in unit for unit in data["feed"]["units"])


def test_failed_strict_gate_must_block_activation():
    data = bundle()
    data["strict_gate"] = strict_gate_fail()
    assert data["strict_gate"]["status"] == "fail"
    assert data["strict_gate"]["blocking_findings"]
```

- [ ] **Step 3: Run the contract tests**

Run:

```bash
cd backend
DATABASE_URL="postgresql+asyncpg://dummy:dummy@localhost/dummy" pytest tests/unit/nomos/test_current_bundle_contract.py -v --no-cov
```

Expected: pass, because this task only verifies fixture shape.

- [ ] **Step 4: Commit**

```bash
git add backend/tests/fixtures/nomos/current_bundle.py backend/tests/unit/nomos/test_current_bundle_contract.py
git commit -m "test: lock current Nomos bundle contract"
```

## Task 2 - Feed Parser And Bundle Validator

**Purpose:** Introduce a pure parser that supports the current NOMOS bundle without DB writes.

**Files:**

- Create: `backend/app/services/nomos_bundle_parser.py`
- Create: `backend/tests/test_nomos_bundle_parser.py`
- Do not modify `backend/app/services/nomos_importer.py` in this task.

- [ ] **Step 1: Write parser tests first**

Create `backend/tests/test_nomos_bundle_parser.py`:

```python
import pytest

from app.services.nomos_bundle_parser import (
    NomosBundleError,
    parse_nomos_bundle,
)
from tests.fixtures.nomos.current_bundle import (
    bundle,
    bundle_without_text,
    strict_gate_fail,
)


def test_parse_current_structured_feed_units():
    parsed = parse_nomos_bundle(bundle())
    assert parsed.feed_format == "nomos.corpus-feed.v1"
    assert parsed.content_hash == "sha256:feedhash"
    assert len(parsed.units) == 2
    assert parsed.units[0].unit_id == "RBOK-MD-DEMO-001"
    assert parsed.units[0].text == "La doctrine RBOK doit rester source-backed."
    assert parsed.units[0].source_path == "00_meta/demo.md"
    assert parsed.units[0].start_line == 2


def test_parse_joins_rag_metadata_by_unit_ids():
    parsed = parse_nomos_bundle(bundle())
    assert parsed.units[0].chunks[0].chunk_id == "chunk:CORPUS-RBOK-DEMO:10-55"
    assert parsed.units[0].chunks[0].locator == "00_meta/demo.md:L2-L2"


def test_parse_rejects_missing_text():
    with pytest.raises(NomosBundleError, match="missing text"):
        parse_nomos_bundle(bundle_without_text())


def test_parse_rejects_failed_strict_gate():
    data = bundle()
    data["strict_gate"] = strict_gate_fail()
    with pytest.raises(NomosBundleError, match="strict gate"):
        parse_nomos_bundle(data)
```

- [ ] **Step 2: Run parser tests and verify they fail**

Run:

```bash
cd backend
DATABASE_URL="postgresql+asyncpg://dummy:dummy@localhost/dummy" pytest tests/test_nomos_bundle_parser.py -v --no-cov
```

Expected: fail with `ModuleNotFoundError: No module named 'app.services.nomos_bundle_parser'`.

- [ ] **Step 3: Implement the pure parser**

Create `backend/app/services/nomos_bundle_parser.py`:

```python
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any


class NomosBundleError(ValueError):
    pass


@dataclass(frozen=True)
class ParsedNomosChunk:
    chunk_id: str
    locator: str | None
    source_hash: str | None
    metadata: dict[str, Any]


@dataclass(frozen=True)
class ParsedNomosUnit:
    unit_id: str
    unit_type: str
    title: str | None
    text: str
    source_id: str | None
    source_path: str
    start_byte: int | None
    end_byte: int | None
    start_line: int | None
    end_line: int | None
    normalized_text_hash: str | None
    heading_path: list[str]
    chunks: list[ParsedNomosChunk] = field(default_factory=list)


@dataclass(frozen=True)
class ParsedNomosBundle:
    feed_format: str
    content_hash: str
    unit_count: int
    source_count: int
    units: list[ParsedNomosUnit]
    warnings: list[str]


def parse_nomos_bundle(bundle: dict[str, Any]) -> ParsedNomosBundle:
    if not isinstance(bundle, dict):
        raise NomosBundleError("bundle must be a dict")

    strict_gate = bundle.get("strict_gate")
    if not isinstance(strict_gate, dict):
        raise NomosBundleError("strict gate is missing")
    if strict_gate.get("status") != "pass":
        raise NomosBundleError("strict gate did not pass")
    if strict_gate.get("blocking_findings"):
        raise NomosBundleError("strict gate has blocking findings")

    feed = bundle.get("feed")
    if not isinstance(feed, dict):
        raise NomosBundleError("feed is missing")

    rag_metadata = bundle.get("rag_metadata") or []
    if not isinstance(rag_metadata, list):
        raise NomosBundleError("rag_metadata must be a list")

    units = _parse_structured_feed(feed, rag_metadata)
    if not units:
        raise NomosBundleError("feed has no importable units")

    return ParsedNomosBundle(
        feed_format=str(feed.get("format", "unknown")),
        content_hash=str(feed.get("content_hash", "")),
        unit_count=int(feed.get("unit_count") or len(units)),
        source_count=int(feed.get("source_count") or 0),
        units=units,
        warnings=[],
    )


def _parse_structured_feed(
    feed: dict[str, Any],
    rag_metadata: list[dict[str, Any]],
) -> list[ParsedNomosUnit]:
    metadata_by_unit: dict[str, list[dict[str, Any]]] = {}
    for item in rag_metadata:
        for unit_id in item.get("unit_ids") or []:
            metadata_by_unit.setdefault(str(unit_id), []).append(item)

    parsed: list[ParsedNomosUnit] = []
    for unit in feed.get("units") or []:
        unit_id = str(unit.get("unit_id") or "")
        text = str(unit.get("business_rule") or "").strip()
        source_path = str(unit.get("source_path") or "")

        if not unit_id:
            raise NomosBundleError("unit missing unit_id")
        if not text:
            raise NomosBundleError(f"unit {unit_id} missing text")
        if not source_path:
            raise NomosBundleError(f"unit {unit_id} missing source_path")

        chunks = [
            ParsedNomosChunk(
                chunk_id=str(meta.get("chunk_id") or ""),
                locator=meta.get("locator"),
                source_hash=meta.get("source_hash"),
                metadata=meta,
            )
            for meta in metadata_by_unit.get(unit_id, [])
            if meta.get("chunk_id")
        ]

        parsed.append(
            ParsedNomosUnit(
                unit_id=unit_id,
                unit_type=str(unit.get("unit_type") or "unknown"),
                title=unit.get("name"),
                text=text,
                source_id=unit.get("source_id"),
                source_path=source_path,
                start_byte=unit.get("start_byte"),
                end_byte=unit.get("end_byte"),
                start_line=unit.get("start_line"),
                end_line=unit.get("end_line"),
                normalized_text_hash=unit.get("normalized_text_hash"),
                heading_path=list(unit.get("heading_path") or []),
                chunks=chunks,
            )
        )
    return parsed
```

- [ ] **Step 4: Run parser tests**

Run:

```bash
cd backend
DATABASE_URL="postgresql+asyncpg://dummy:dummy@localhost/dummy" pytest tests/test_nomos_bundle_parser.py -v --no-cov
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add backend/app/services/nomos_bundle_parser.py backend/tests/test_nomos_bundle_parser.py
git commit -m "feat: parse current Nomos bundle format"
```

## Task 3 - Importer Persistence And Activation Gate

**Purpose:** Replace the old flat importer contract with current bundle import into existing doctrine projection models.

**Files:**

- Modify: `backend/app/services/nomos_importer.py`
- Modify: `backend/tests/test_nomos_importer_service.py`
- Review: `backend/app/models/nomos_doctrine.py`

- [ ] **Step 1: Write importer tests for the current bundle**

Add tests to `backend/tests/test_nomos_importer_service.py`:

```python
import pytest
from sqlalchemy import select

from app.models.nomos_doctrine import (
    NomosDoctrineChunk,
    NomosDoctrineSource,
    NomosDoctrineUnit,
    NomosFeedVersion,
)
from app.services.nomos_importer import import_nomos_feed
from tests.fixtures.nomos.current_bundle import bundle, strict_gate_fail


@pytest.mark.asyncio
async def test_import_current_nomos_bundle_dry_run(db_session):
    result = await import_nomos_feed(bundle(), db_session, dry_run=True)
    assert result.success is True
    assert result.dry_run is True
    assert result.unit_count == 2
    assert result.chunk_count == 2


@pytest.mark.asyncio
async def test_import_current_nomos_bundle_persists_projection(db_session):
    result = await import_nomos_feed(bundle(), db_session)
    assert result.success is True

    feeds = (await db_session.execute(select(NomosFeedVersion))).scalars().all()
    sources = (await db_session.execute(select(NomosDoctrineSource))).scalars().all()
    units = (await db_session.execute(select(NomosDoctrineUnit))).scalars().all()
    chunks = (await db_session.execute(select(NomosDoctrineChunk))).scalars().all()

    assert len(feeds) == 1
    assert len(sources) == 1
    assert len(units) == 2
    assert len(chunks) == 2
    assert units[0].body
    assert chunks[0].content


@pytest.mark.asyncio
async def test_import_current_nomos_bundle_blocks_failed_strict_gate(db_session):
    data = bundle()
    data["strict_gate"] = strict_gate_fail()
    result = await import_nomos_feed(data, db_session)
    assert result.success is False
    assert any("strict gate" in error.lower() for error in result.errors)
```

- [ ] **Step 2: Run importer tests and verify current failures**

Run:

```bash
cd backend
DATABASE_URL="postgresql+asyncpg://dummy:dummy@localhost/dummy" pytest tests/test_nomos_importer_service.py -v --no-cov
```

Expected: fail before implementation because the importer still expects `source_hash`, `units`, and `chunks` at the old locations.

- [ ] **Step 3: Update importer to call parser first**

In `backend/app/services/nomos_importer.py`, replace top-level validation of old fields with:

```python
from app.services.nomos_bundle_parser import NomosBundleError, parse_nomos_bundle
```

Then at the start of `import_nomos_feed`:

```python
try:
    parsed = parse_nomos_bundle(feed_data)
except NomosBundleError as exc:
    return ImportResult(success=False, errors=[str(exc)])
```

Use:

```python
source_hash = parsed.content_hash
unit_count = len(parsed.units)
chunk_count = sum(len(unit.chunks) for unit in parsed.units)
```

- [ ] **Step 4: Persist into existing projection models**

The importer must create:

```text
NomosFeedVersion
NomosDoctrineSource
NomosDoctrineUnit
NomosDoctrineChunk
NomosRagMetadata when chunk metadata exists
AuditLog
```

Use one `NomosDoctrineSource` per unique `source_path`.

Map fields:

```text
ParsedNomosBundle.content_hash -> NomosFeedVersion.version_tag or artifact metadata
ParsedNomosUnit.source_path -> NomosDoctrineSource.path
ParsedNomosUnit.source_id -> NomosDoctrineSource.metadata_["source_id"]
ParsedNomosUnit.text -> NomosDoctrineUnit.body
ParsedNomosUnit.unit_type -> NomosDoctrineUnit.unit_type
ParsedNomosUnit.title -> NomosDoctrineUnit.title
ParsedNomosUnit.heading_path -> NomosDoctrineUnit.metadata_["heading_path"]
ParsedNomosUnit.start_line/end_line/start_byte/end_byte -> NomosDoctrineLocator or unit metadata
ParsedNomosChunk.metadata -> NomosRagMetadata.extra
```

- [ ] **Step 5: Activation must be atomic**

If `activate=True`:

```python
await session.execute(
    update(NomosFeedVersion)
    .where(NomosFeedVersion.feed_name == feed_name)
    .values(is_active=False)
)
feed_version.is_active = True
```

Do not activate if parser validation fails.

- [ ] **Step 6: Run importer tests**

Run:

```bash
cd backend
DATABASE_URL="postgresql+asyncpg://dummy:dummy@localhost/dummy" pytest tests/test_nomos_importer_service.py -v --no-cov
```

Expected: pass.

- [ ] **Step 7: Commit**

```bash
git add backend/app/services/nomos_importer.py backend/tests/test_nomos_importer_service.py
git commit -m "feat: import current Nomos doctrine bundle"
```

## Task 4 - Traceability API Repair

**Purpose:** Align admin doctrine endpoints and schemas with the current projection models.

**Files:**

- Modify: `backend/app/api/v1/admin/doctrine_traceability.py`
- Modify: `backend/app/schemas/doctrine_traceability.py`
- Modify: `backend/tests/test_doctrine_traceability.py`

- [ ] **Step 1: Update tests to use the current model names**

Tests must use:

```text
NomosFeedVersion.id
NomosFeedVersion.feed_name
NomosFeedVersion.version_tag
NomosFeedVersion.is_active
NomosDoctrineSource.feed_version_id
NomosDoctrineUnit.source_id
NomosDoctrineChunk.unit_id
```

Do not use missing fields:

```text
unit_count
chunk_count
import_id on unit
unit_key
```

- [ ] **Step 2: Run the traceability test and verify failure before repair**

Run:

```bash
cd backend
DATABASE_URL="postgresql+asyncpg://dummy:dummy@localhost/dummy" pytest tests/test_doctrine_traceability.py -v --no-cov
```

Expected: fail before endpoint/schema repair.

- [ ] **Step 3: Repair health endpoint**

Compute counts from related tables instead of missing fields:

```text
unit_count = count(NomosDoctrineUnit.id) joined through source/feed
chunk_count = count(NomosDoctrineChunk.id) joined through unit/source/feed
```

Active feed query:

```text
NomosFeedVersion.is_active == True
```

- [ ] **Step 4: Repair import detail/list vocabulary**

Use "feed version" language in schemas if the DB model is `NomosFeedVersion`.
Keep compatibility aliases only if tests prove clients need them.

- [ ] **Step 5: Run traceability test**

Run:

```bash
cd backend
DATABASE_URL="postgresql+asyncpg://dummy:dummy@localhost/dummy" pytest tests/test_doctrine_traceability.py -v --no-cov
```

Expected: pass.

- [ ] **Step 6: Commit**

```bash
git add backend/app/api/v1/admin/doctrine_traceability.py backend/app/schemas/doctrine_traceability.py backend/tests/test_doctrine_traceability.py
git commit -m "fix: align doctrine traceability with Nomos projections"
```

## Task 5 - Runtime Doctrine Retriever Wiring

**Purpose:** Make conversation prompt assembly consume active NOMOS doctrine before generic RAG.

**Files:**

- Modify: `backend/app/services/module_prompt_builder.py`
- Create or modify: `backend/tests/test_module_prompt_builder_doctrine.py`

- [ ] **Step 1: Write prompt builder tests**

Test cases:

```text
1. Active feed + matching module/question binding -> prompt contains Doctrine RBOK (Nomos).
2. No active feed -> prompt still builds without doctrine context.
3. Doctrine result exists -> generic RAG does not erase doctrine context.
4. Supporting-only result -> not presented as primary doctrine.
```

- [ ] **Step 2: Wire DoctrineRAGRetriever**

In `ModulePromptBuilder.build_module_prompt`, load doctrine after behavior config and before generic RAG combination:

```python
from app.services.doctrine_rag_retrieval import DoctrineRAGRetriever

doctrine_context: str | None = None
try:
    retriever = DoctrineRAGRetriever()
    doctrine_result = await retriever.retrieve(
        db,
        module_id=ensure_uuid(module_id),
        question_key=None,
        query_hint=module.name,
    )
    if doctrine_result.has_results:
        doctrine_context = doctrine_result.format_context()
except Exception:
    logger.warning("Failed to load Nomos doctrine context", exc_info=True)
```

Combine in this order:

```python
combined_context_parts = [
    c for c in (doctrine_context, rbok_context, rag_context) if c
]
```

- [ ] **Step 3: Add runtime trace metadata**

Where the response trace is assembled, preserve:

```text
feed_name
feed_version
chunk_ids
source_paths
source_hashes
authorities
```

If the current response model cannot store it, add a dedicated internal trace object first; avoid frontend changes until backend trace is stable.

- [ ] **Step 4: Run focused tests**

```bash
cd backend
DATABASE_URL="postgresql+asyncpg://dummy:dummy@localhost/dummy" pytest tests/test_doctrine_retrieval.py tests/test_module_prompt_builder_doctrine.py -v --no-cov
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add backend/app/services/module_prompt_builder.py backend/tests/test_module_prompt_builder_doctrine.py
git commit -m "feat: wire Nomos doctrine into module prompts"
```

## Task 6 - Conversation Behavior Policy

**Purpose:** Prove the business rule: one concise question, exactly the current step question, no extra questions.

**Files:**

- Create or modify: `backend/tests/test_ai_step_question_policy.py`
- Modify: `backend/app/services/module_prompt_builder.py`
- Review: AI behavior policy service files; centralize the policy there only when RBOK already has an existing policy owner.

- [ ] **Step 1: Write policy tests**

Create tests asserting:

```text
prompt contains "one question only" equivalent policy
prompt contains current question text
prompt forbids additional questions
prompt forbids bullet lists of questions
doctrine context is supporting context, not conversational output
```

- [ ] **Step 2: Add policy text in the prompt builder**

The policy must be concise and testable:

```text
Conversation rule:
- Ask only the current step question.
- Do not add another question.
- Do not ask follow-up questions unless the current step explicitly is that follow-up.
- Keep the wording concise, precise, and kind.
- Use doctrine only to guide the answer, not to lecture the client.
```

- [ ] **Step 3: Add provider-independent output guard test**

If RBOK has a response post-processor, add a test that rejects model output containing more than one `?` when the step expects one question. If not, add the test as pending work in the issue and do not fake a post-processor.

- [ ] **Step 4: Run tests**

```bash
cd backend
DATABASE_URL="postgresql+asyncpg://dummy:dummy@localhost/dummy" pytest tests/test_ai_step_question_policy.py -v --no-cov
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add backend/tests/test_ai_step_question_policy.py backend/app/services/module_prompt_builder.py
git commit -m "test: enforce single-question conversation policy"
```

## Task 7 - YAML/JSON Parcours Gap Handling

**Purpose:** Make YAML/JSON parcours/modules/questions explicit instead of silently absent from runtime doctrine.

**Files:**

- Create: `backend/app/services/nomos_structured_gap_policy.py`
- Create: `backend/tests/test_nomos_structured_gap_policy.py`
- NOMOS-side work for `rbok-parcours-feed.json` is a separate upstream task; this RBOK task only blocks or classifies missing structured parcours coverage.

- [ ] **Step 1: Write gap policy tests**

```python
from app.services.nomos_structured_gap_policy import classify_structured_gap


def test_yaml_parcours_absent_blocks_full_runtime_claim():
    result = classify_structured_gap(
        source_path="01_rbok/03_parcours/demo.yaml",
        imported=False,
        explicit_status=None,
    )
    assert result.blocking is True
    assert result.code == "STRUCTURED_PARCOURS_NOT_IMPORTED"


def test_yaml_parcours_explicitly_out_of_scope_is_reviewable():
    result = classify_structured_gap(
        source_path="01_rbok/03_parcours/demo.yaml",
        imported=False,
        explicit_status="accepted_gap",
    )
    assert result.blocking is False
    assert result.review_required is True
```

- [ ] **Step 2: Implement simple classifier**

```python
from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class StructuredGapDecision:
    code: str
    blocking: bool
    review_required: bool


def classify_structured_gap(
    *,
    source_path: str,
    imported: bool,
    explicit_status: str | None,
) -> StructuredGapDecision:
    if imported:
        return StructuredGapDecision("STRUCTURED_SOURCE_IMPORTED", False, False)
    if explicit_status == "accepted_gap":
        return StructuredGapDecision("STRUCTURED_SOURCE_ACCEPTED_GAP", False, True)
    if source_path.endswith((".yaml", ".yml", ".json")) and "/03_parcours/" in source_path:
        return StructuredGapDecision("STRUCTURED_PARCOURS_NOT_IMPORTED", True, True)
    return StructuredGapDecision("STRUCTURED_SOURCE_NOT_IMPORTED", False, True)
```

- [ ] **Step 3: Surface the gap in import result**

Importer validation must return a warning/blocker when manifest lists YAML/JSON parcours files but the bundle has no corresponding structured feed.

- [ ] **Step 4: Run tests**

```bash
cd backend
DATABASE_URL="postgresql+asyncpg://dummy:dummy@localhost/dummy" pytest tests/test_nomos_structured_gap_policy.py tests/test_nomos_importer_service.py -v --no-cov
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add backend/app/services/nomos_structured_gap_policy.py backend/tests/test_nomos_structured_gap_policy.py backend/app/services/nomos_importer.py backend/tests/test_nomos_importer_service.py
git commit -m "feat: make structured parcours gaps explicit"
```

## Task 8 - POC Workflow And Evidence Dossier

**Purpose:** Prove the full dev POC without production activation.

**Files:**

- Create: `docs/nomos/rbok-develop-poc.md`
- Modify: the RBOK NOMOS workflow under `.github/workflows/` when it exists; otherwise create `.github/workflows/nomos-rbok-develop-poc.yml`.
- Create or modify: backend import command or management script that runs the dry-run and DB dev import.

- [ ] **Step 1: Create POC dossier**

Create `docs/nomos/rbok-develop-poc.md` with:

```markdown
# RBOK Develop NOMOS POC

## Source

- Source repo: RBOKproject/realisons-business
- Source branch/ref:
- Corpus scope: 01_rbok/**
- Source mutation before run:
- Source mutation after run:

## NOMOS Output

- Feed artifact:
- RAG metadata artifact:
- Strict gate:
- Attestation:
- Source manifest:
- Artifact hash:

## RBOK Import

- Import mode: dry-run / DB dev
- Import verdict:
- Feed version:
- Unit count:
- Chunk count:
- Structured YAML/JSON status:

## Runtime

- Active feed:
- Scenario:
- Module:
- Question:
- Doctrine chunks used:
- Source trace:
- Conversation policy result:

## Decision

- Verdict: go / conditional go / blocked
- Remaining blockers:
- Accepted warnings:
```

- [ ] **Step 2: Run required backend tests**

```bash
cd backend
DATABASE_URL="postgresql+asyncpg://dummy:dummy@localhost/dummy" pytest tests/test_nomos_importer_service.py tests/test_doctrine_traceability.py tests/test_doctrine_retrieval.py -v --no-cov
```

Expected: pass.

- [ ] **Step 3: Run broader backend suite**

```bash
cd backend
DATABASE_URL="postgresql+asyncpg://dummy:dummy@localhost/dummy" pytest --cov=app --cov-report=term --cov-fail-under=65
```

Expected: pass with coverage >= 65%.

- [ ] **Step 4: Run frontend/admin tests if admin UI changed**

```bash
cd frontend
npm test
npm run lint
npm run type-check
```

Expected: pass.

- [ ] **Step 5: Open PR to develop**

```bash
git push -u origin <feature-branch>
gh pr create --repo RBOKproject/RBOK --base develop --head <feature-branch> --title "feat: integrate Nomos doctrine bundle into RBOK develop" --body-file <pr-body.md>
```

Expected: PR targets `develop`.

- [ ] **Step 6: Commit**

```bash
git add docs/nomos/rbok-develop-poc.md .github/workflows backend frontend
git commit -m "docs: record RBOK Nomos develop POC"
```

## Review Checklist

Before considering the recommendations implemented:

- [ ] `RBOK#2895` has a linked PR.
- [ ] `RBOK#2896` has linked PRs or child issues for each task.
- [ ] `RBOK#2704` is re-opened or explicitly revalidated with current NOMOS format.
- [ ] `RBOK#2711` is re-opened or explicitly revalidated with current projection models.
- [ ] `tests/test_nomos_importer_service.py --no-cov` passes.
- [ ] `tests/test_doctrine_traceability.py --no-cov` passes.
- [ ] `tests/test_doctrine_retrieval.py --no-cov` passes.
- [ ] Runtime prompt uses `DoctrineRAGRetriever` when active feed exists.
- [ ] `rag-metadata.json` is never used as the sole text source.
- [ ] YAML/JSON parcours status is explicit.
- [ ] POC dossier states `go`, `conditional go`, or `blocked`.

## Dispatch Guidance

Suggested ownership if work is split across RBOK agents:

| Task | Owner domain | Rationale |
|---|---|---|
| Task 1-3 | Backend | Parser/importer/DB projection. |
| Task 4 | Backend + frontend if UI exists | Admin API and optional UI read-only views. |
| Task 5-6 | Backend | Prompt assembly and runtime behavior tests. |
| Task 7 | Backend + NOMOS coordination | YAML/JSON gap status; upstream structured artifact remains a separate NOMOS task. |
| Task 8 | Backend/DevOps | POC workflow, evidence dossier, CI. |

Keep one issue per agent and sequence tasks that touch the same files.
