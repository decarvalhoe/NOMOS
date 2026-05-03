#!/usr/bin/env python3
"""Tests for regulated_audit_log_export.py."""

import json
import sys
import tempfile
from pathlib import Path
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).parent))
from regulated_audit_log_export import (
    compute_sha256,
    extract_next_link,
    load_manifest,
    parse_args,
    resolve_date_range,
    write_export,
    main,
)


def test_resolve_date_range_defaults():
    since, until = resolve_date_range(None, None)
    assert since is not None
    assert until is not None
    assert len(since) == 10  # YYYY-MM-DD
    assert len(until) == 10


def test_resolve_date_range_explicit():
    since, until = resolve_date_range("2026-04-01", "2026-04-30")
    assert since == "2026-04-01"
    assert until == "2026-04-30"


def test_compute_sha256():
    data = b"test data"
    result = compute_sha256(data)
    assert result.startswith("sha256:")
    assert len(result) == 7 + 64  # "sha256:" + 64 hex chars


def test_compute_sha256_deterministic():
    data = b"deterministic"
    assert compute_sha256(data) == compute_sha256(data)


def test_extract_next_link_present():
    header = '<https://api.github.com/next?page=2>; rel="next", <https://api.github.com/last>; rel="last"'
    result = extract_next_link(header)
    assert result == "https://api.github.com/next?page=2"


def test_extract_next_link_absent():
    header = '<https://api.github.com/last>; rel="last"'
    result = extract_next_link(header)
    assert result is None


def test_extract_next_link_empty():
    assert extract_next_link("") is None
    assert extract_next_link(None) is None


def test_write_export():
    with tempfile.TemporaryDirectory() as tmpdir:
        output_dir = Path(tmpdir)
        events = [
            {"action": "repo.create", "actor": "alice", "@timestamp": 1714500000000},
            {"action": "org.update_member", "actor": "bob", "@timestamp": 1714500001000},
        ]
        result = write_export(events, output_dir, "2026-04-26", "2026-05-03")

        assert result["event_count"] == 2
        assert result["hash"].startswith("sha256:")
        assert Path(result["path"]).exists()

        # Verify file content
        export_data = json.loads(Path(result["path"]).read_text())
        assert export_data["event_count"] == 2
        assert len(export_data["events"]) == 2
        assert export_data["date_range"]["since"] == "2026-04-26"

        # Verify manifest
        manifest = json.loads((output_dir / "export-manifest.json").read_text())
        assert manifest["schema_version"] == "0.1.0"
        assert manifest["policy_ref"] == "RCP-004"
        assert len(manifest["exports"]) == 1
        assert manifest["exports"][0]["hash"] == result["hash"]


def test_write_export_appends_manifest():
    with tempfile.TemporaryDirectory() as tmpdir:
        output_dir = Path(tmpdir)
        write_export([{"event": 1}], output_dir, "2026-04-01", "2026-04-07")
        write_export([{"event": 2}], output_dir, "2026-04-08", "2026-04-14")

        manifest = json.loads((output_dir / "export-manifest.json").read_text())
        assert len(manifest["exports"]) == 2


def test_write_export_hash_integrity():
    with tempfile.TemporaryDirectory() as tmpdir:
        output_dir = Path(tmpdir)
        events = [{"action": "test"}]
        result = write_export(events, output_dir, "2026-05-01", "2026-05-02")

        file_bytes = Path(result["path"]).read_bytes()
        actual_hash = compute_sha256(file_bytes)
        assert actual_hash == result["hash"]


def test_load_manifest_new():
    with tempfile.TemporaryDirectory() as tmpdir:
        path = Path(tmpdir) / "manifest.json"
        manifest = load_manifest(path)
        assert manifest["schema_version"] == "0.1.0"
        assert manifest["exports"] == []


def test_load_manifest_existing():
    with tempfile.TemporaryDirectory() as tmpdir:
        path = Path(tmpdir) / "manifest.json"
        existing = {"schema_version": "0.1.0", "policy_ref": "RCP-004", "exports": [{"test": True}]}
        path.write_text(json.dumps(existing))
        manifest = load_manifest(path)
        assert len(manifest["exports"]) == 1


def test_parse_args():
    args = parse_args(["--org", "TestOrg", "--output", "/tmp/out", "--dry-run"])
    assert args.org == "TestOrg"
    assert args.output == "/tmp/out"
    assert args.dry_run is True


def test_main_dry_run():
    code = main(["--org", "TestOrg", "--output", "/tmp/test-audit", "--dry-run"])
    assert code == 0


def test_main_no_token():
    with patch.dict("os.environ", {"GITHUB_TOKEN": ""}, clear=False):
        code = main(["--org", "TestOrg", "--output", "/tmp/test-audit"])
        assert code == 1


def test_retention_policy_valid_yaml():
    """Verify the retention policy YAML is valid and has required fields."""
    import yaml  # noqa: delayed import for test isolation

    policy_path = Path(__file__).parent.parent / "docs" / "regulated" / "operations" / "artifact-retention-policy.yaml"
    data = yaml.safe_load(policy_path.read_text())

    assert data["policy_id"] == "RCP-004"
    assert data["schema_version"] == "0.1.0"
    assert len(data["retention_rules"]) >= 5
    assert data["audit_log_export"]["schedule"] == "weekly"
    assert data["audit_log_export"]["format"] == "json"

    # Verify each rule has required fields
    for rule in data["retention_rules"]:
        assert "id" in rule
        assert "category" in rule
        assert "description" in rule
        assert "storage" in rule
        assert "disposal_method" in rule


if __name__ == "__main__":
    tests = [v for k, v in globals().items() if k.startswith("test_") and callable(v)]
    passed = 0
    failed = 0
    for test in tests:
        try:
            test()
            passed += 1
            print(f"  PASS: {test.__name__}")
        except Exception as e:
            failed += 1
            print(f"  FAIL: {test.__name__}: {e}")
    print(f"\n{passed} passed, {failed} failed")
    sys.exit(1 if failed else 0)
