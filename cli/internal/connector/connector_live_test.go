package connector

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestLive_OFSCommuneRegister performs a REAL fetch of the open OFS commune
// register and proves the full pipeline end to end: real hash, real byte count,
// and a body ledger with zero uncovered bytes. It is gated behind
// NOMOS_LIVE_CH_FETCH=1 so offline CI stays deterministic; it does not assert a
// specific hash because the upstream content legitimately changes over time.
//
// Run with:
//
//	NOMOS_LIVE_CH_FETCH=1 go test ./internal/connector/ -run TestLive -v
func TestLive_OFSCommuneRegister(t *testing.T) {
	if os.Getenv("NOMOS_LIVE_CH_FETCH") != "1" {
		t.Skip("set NOMOS_LIVE_CH_FETCH=1 to run the live Swiss-source fetch")
	}
	const url = "https://www.agvchapp.bfs.admin.ch/api/communes/snapshot?date=01-01-2026"

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	content, result, err := Fetch(ctx, url, FetchOptions{})
	if err != nil {
		t.Fatalf("live fetch failed: %v", err)
	}
	if result.ByteCount == 0 || len(content) == 0 {
		t.Fatal("live fetch returned no bytes")
	}
	if len(result.SHA256) != len("sha256:")+64 {
		t.Fatalf("live fetch produced a non-real hash %q", result.SHA256)
	}

	ledger := BuildLineLedger(content)
	if !ledger.IsFullyCovered() {
		t.Fatalf("live content left %d bytes uncovered", ledger.UncoveredBytes)
	}
	ev := BuildEvidence("ch-ofs-commune-register", content, result, ledger, 3)
	if err := ev.Validate(); err != nil {
		t.Fatalf("live evidence invalid: %v", err)
	}
	if err := ev.VerifyContentHash(content); err != nil {
		t.Fatalf("recomputed hash does not match recorded: %v", err)
	}
	t.Logf("live OFS fetch: %d bytes, %s, %d atoms, 0 uncovered", result.ByteCount, result.SHA256, ev.AtomCount)
}
