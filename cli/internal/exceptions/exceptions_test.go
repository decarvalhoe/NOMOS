package exceptions

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func td(name string) string {
	return filepath.Join("testdata", name)
}

var futureDate = time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
var pastDate = time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

func TestValidException(t *testing.T) {
	result, err := CheckExceptions(td("valid.yaml"), futureDate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid, got errors: %v", result.Exceptions[0].Errors)
	}
	if len(result.Exceptions) != 1 {
		t.Fatalf("expected 1 exception, got %d", len(result.Exceptions))
	}
	if result.Exceptions[0].ID != "EXC-001" {
		t.Fatalf("expected EXC-001, got %q", result.Exceptions[0].ID)
	}
}

func TestExpiredActiveBlocks(t *testing.T) {
	result, err := CheckExceptions(td("expired-active.yaml"), futureDate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid for expired active exception")
	}
	assertError(t, result.Exceptions[0], "EXCEPTION_EXPIRED")
}

func TestExpiredActivePassesWhenNotYetExpired(t *testing.T) {
	// Use a date before the expiration
	early := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	result, err := CheckExceptions(td("expired-active.yaml"), early)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid before expiration date, got errors: %v", result.Exceptions[0].Errors)
	}
}

func TestNoOwner(t *testing.T) {
	result, err := CheckExceptions(td("no-owner.yaml"), futureDate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid")
	}
	assertError(t, result.Exceptions[0], "NO_OWNER")
}

func TestNoApprover(t *testing.T) {
	result, err := CheckExceptions(td("no-approver.yaml"), futureDate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid")
	}
	assertError(t, result.Exceptions[0], "NO_APPROVER")
}

func TestNoExpiration(t *testing.T) {
	result, err := CheckExceptions(td("no-expiration.yaml"), futureDate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid")
	}
	assertError(t, result.Exceptions[0], "MISSING_EXPIRATION")
}

func TestMultipleErrors(t *testing.T) {
	result, err := CheckExceptions(td("multiple-errors.yaml"), futureDate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid")
	}
	ec := result.Exceptions[0]
	assertError(t, ec, "MISSING_EXCEPTION_ID")
	assertError(t, ec, "MISSING_SUMMARY")
	assertError(t, ec, "NO_OWNER")
	assertError(t, ec, "NO_APPROVER")
	assertError(t, ec, "INVALID_SEVERITY")
	assertError(t, ec, "INVALID_STATUS")
	assertError(t, ec, "MISSING_REASON")
	assertError(t, ec, "INVALID_EXPIRATION")
}

func TestRevokedExpiredDoesNotBlock(t *testing.T) {
	// A revoked exception that is past its expiry should NOT trigger EXCEPTION_EXPIRED
	result, err := CheckExceptions(td("revoked-expired-ok.yaml"), futureDate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid for revoked exception, got errors: %v", result.Exceptions[0].Errors)
	}
}

func TestCheckExceptionsFromBytes(t *testing.T) {
	data := []byte(`
exceptions:
  - id: EXC-OK
    summary: "Temporary bypass"
    owner: dev@example.com
    approver: lead@example.com
    severity: low
    status: active
    created_at: "2026-04-01"
    expires_at: "2027-01-01"
    reason: "Waiting for fix."
`)
	result, err := CheckExceptionsFromBytes(data, futureDate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid, got errors: %v", result.Exceptions[0].Errors)
	}
}

func TestInvalidYAML(t *testing.T) {
	_, err := CheckExceptionsFromBytes([]byte(`{broken`), futureDate)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestNonexistentManifest(t *testing.T) {
	_, err := CheckExceptions("/nonexistent/exceptions.yaml", futureDate)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func assertError(t *testing.T, ec ExceptionCheck, code string) {
	t.Helper()
	for _, e := range ec.Errors {
		if e.Code == code {
			return
		}
	}
	var codes []string
	for _, e := range ec.Errors {
		codes = append(codes, e.Code)
	}
	t.Fatalf("expected error code %s in [%s]", code, strings.Join(codes, ", "))
}
