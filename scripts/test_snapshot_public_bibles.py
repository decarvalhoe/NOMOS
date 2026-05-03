#!/usr/bin/env python3
"""Tests for snapshot_public_bibles.py."""

import json
import sys
import tempfile
from pathlib import Path
from unittest.mock import MagicMock, patch
from io import BytesIO

sys.path.insert(0, str(Path(__file__).parent))
from snapshot_public_bibles import (
    PUBLIC_BIBLES,
    PublicBible,
    compute_sha256_from_url,
    generate_sidecar,
    get_bibles,
    main,
    parse_args,
    write_sidecar,
)


def test_public_bibles_defined():
    """All expected public bibles are defined."""
    assert len(PUBLIC_BIBLES) >= 5
    ids = {b.ref_id for b in PUBLIC_BIBLES}
    assert "NIST-SSDF-1.1" in ids
    assert "NIST-CSF-2.0" in ids
    assert "NASA-NPR-7150.2D" in ids
    assert "FDA-GPSV-2002" in ids
    assert "FDA-CSA-2023" in ids


def test_public_bibles_have_required_fields():
    """Each bible has all required fields populated."""
    for bible in PUBLIC_BIBLES:
        assert bible.ref_id, f"missing ref_id"
        assert bible.title, f"{bible.ref_id}: missing title"
        assert bible.authority, f"{bible.ref_id}: missing authority"
        assert bible.url.startswith("https://"), f"{bible.ref_id}: URL must be HTTPS"
        assert bible.version, f"{bible.ref_id}: missing version"
        assert bible.category, f"{bible.ref_id}: missing category"


def test_generate_sidecar_content():
    bible = PublicBible(
        ref_id="TEST-001",
        title="Test Reference",
        authority="Test Authority",
        url="https://example.com/test.pdf",
        version="1.0",
        category="test",
        notes="Test note",
    )
    content = generate_sidecar(bible, "sha256:aabbccdd", 12345)

    assert 'ref_id: "TEST-001"' in content
    assert 'title: "Test Reference"' in content
    assert 'authority: "Test Authority"' in content
    assert 'snapshot_hash: "sha256:aabbccdd"' in content
    assert "snapshot_size_bytes: 12345" in content
    assert 'stored_in_git: false' in content
    assert 'storage_policy: "hash_only_no_full_text"' in content
    assert 'notes: "Test note"' in content


def test_generate_sidecar_no_full_text():
    """Sidecar NEVER contains document content."""
    bible = PublicBible(
        ref_id="TEST-002", title="Big Doc", authority="Auth",
        url="https://example.com/big.pdf", version="2.0", category="test",
    )
    content = generate_sidecar(bible, "sha256:ffff", 999999)
    # Should not contain anything that looks like document content
    assert "stored_in_git: false" in content
    assert "hash_only_no_full_text" in content


def test_write_sidecar():
    with tempfile.TemporaryDirectory() as tmpdir:
        bible = PublicBible(
            ref_id="WRITE-TEST", title="Write Test", authority="Auth",
            url="https://example.com/doc.pdf", version="1.0", category="test",
        )
        output_dir = Path(tmpdir)
        path = write_sidecar(bible, "sha256:1234", 5000, output_dir)

        assert Path(path).exists()
        assert path.endswith("write-test.sidecar.yaml")
        content = Path(path).read_text()
        assert 'snapshot_hash: "sha256:1234"' in content


def test_write_sidecar_creates_directory():
    with tempfile.TemporaryDirectory() as tmpdir:
        nested = Path(tmpdir) / "deep" / "nested"
        bible = PublicBible(
            ref_id="DIR-TEST", title="Dir", authority="A",
            url="https://example.com/x.pdf", version="1", category="t",
        )
        path = write_sidecar(bible, "sha256:abcd", 100, nested)
        assert Path(path).exists()


def test_get_bibles_no_filter():
    result = get_bibles(None)
    assert len(result) == len(PUBLIC_BIBLES)


def test_get_bibles_with_filter():
    result = get_bibles(["NIST-SSDF-1.1", "FDA-GPSV-2002"])
    assert len(result) == 2
    ids = {b.ref_id for b in result}
    assert "NIST-SSDF-1.1" in ids
    assert "FDA-GPSV-2002" in ids


def test_get_bibles_case_insensitive():
    result = get_bibles(["nist-ssdf-1.1"])
    assert len(result) == 1


def test_parse_args_basic():
    args = parse_args(["--output", "/tmp/out", "--dry-run"])
    assert args.output == "/tmp/out"
    assert args.dry_run is True
    assert args.timeout == 30


def test_parse_args_ref_filter():
    args = parse_args(["--output", "/tmp/out", "--ref", "NIST-SSDF-1.1", "--ref", "FDA-CSA-2023"])
    assert args.ref == ["NIST-SSDF-1.1", "FDA-CSA-2023"]


def test_main_dry_run():
    code = main(["--output", "/tmp/test-bibles", "--dry-run"])
    assert code == 0


def test_main_dry_run_with_filter():
    code = main(["--output", "/tmp/test-bibles", "--dry-run", "--ref", "NIST-SSDF-1.1"])
    assert code == 0


def test_compute_sha256_from_url_mock():
    """Test hash computation with mocked URL response."""
    test_content = b"This is test content for hashing"
    expected_hash = "sha256:" + __import__("hashlib").sha256(test_content).hexdigest()

    mock_response = MagicMock()
    mock_response.read = BytesIO(test_content).read
    mock_response.__enter__ = lambda s: s
    mock_response.__exit__ = MagicMock(return_value=False)

    with patch("urllib.request.urlopen", return_value=mock_response):
        result_hash, result_size = compute_sha256_from_url("https://example.com/test", timeout=5)

    assert result_hash == expected_hash
    assert result_size == len(test_content)


def test_sidecar_filename_lowercase():
    """Sidecar filenames are always lowercase."""
    bible = PublicBible(
        ref_id="NASA-NPR-7150.2D", title="Test", authority="NASA",
        url="https://example.com/doc.pdf", version="D", category="eng",
    )
    with tempfile.TemporaryDirectory() as tmpdir:
        path = write_sidecar(bible, "sha256:ee", 100, Path(tmpdir))
        assert "nasa-npr-7150.2d.sidecar.yaml" in path


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
