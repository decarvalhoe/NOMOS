package connector

import (
	"context"
	"os"
	"strings"
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

// TestLive_FedlexELI performs a REAL negotiated fetch of the Fedlex/ELI entry
// for the federal spatial-planning act (LAT, RS 700) — the built-environment
// authority register source (VRC-32 / #569). A plain GET on Fedlex serves the
// Angular app shell, so the machine representation (RDF/XML) is requested via
// ELI content negotiation; the evidence then records BOTH the URL and the
// Accept that produced the hashed bytes. Gated like the OFS live test; no
// specific hash asserted (upstream metadata legitimately evolves).
//
// Run with:
//
//	NOMOS_LIVE_CH_FETCH=1 go test ./internal/connector/ -run TestLive -v
func TestLive_FedlexELI(t *testing.T) {
	if os.Getenv("NOMOS_LIVE_CH_FETCH") != "1" {
		t.Skip("set NOMOS_LIVE_CH_FETCH=1 to run the live Swiss-source fetch")
	}
	const url = "https://fedlex.data.admin.ch/eli/cc/1979/1573_1573_1573"
	const accept = "application/rdf+xml"

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	content, result, err := Fetch(ctx, url, FetchOptions{Accept: accept})
	if err != nil {
		t.Fatalf("live fetch failed: %v", err)
	}
	if result.ByteCount == 0 || len(content) == 0 {
		t.Fatal("live fetch returned no bytes")
	}
	if len(result.SHA256) != len("sha256:")+64 {
		t.Fatalf("live fetch produced a non-real hash %q", result.SHA256)
	}
	// The negotiation must have produced the machine representation, not the
	// Angular shell: RDF/XML content type and an RDF payload.
	if !strings.Contains(result.ContentType, "rdf+xml") {
		t.Fatalf("expected RDF/XML, got content type %q (app shell?)", result.ContentType)
	}
	if !strings.Contains(string(content[:min(512, len(content))]), "rdf:RDF") {
		t.Fatal("payload does not look like RDF/XML — negotiation regressed to the app shell")
	}
	if result.Accept != accept {
		t.Fatalf("provenance lost the negotiation: Accept=%q", result.Accept)
	}

	ledger := BuildLineLedger(content)
	if !ledger.IsFullyCovered() {
		t.Fatalf("live content left %d bytes uncovered", ledger.UncoveredBytes)
	}
	ev := BuildEvidence("ch-fedlex-eli", content, result, ledger, 3)
	if err := ev.Validate(); err != nil {
		t.Fatalf("live evidence invalid: %v", err)
	}
	if err := ev.VerifyContentHash(content); err != nil {
		t.Fatalf("recomputed hash does not match recorded: %v", err)
	}
	t.Logf("live Fedlex fetch: %d bytes, %s, %d atoms, 0 uncovered", result.ByteCount, result.SHA256, ev.AtomCount)
}

// TestLive_SwisstopoSTAC fetches the STAC collection document of a federal
// built-environment geodataset (swissBUILDINGS3D 3.0) on data.geo.admin.ch
// (VRC-32 / #569). Plain JSON GET — no negotiation needed. Asserts the payload
// is a real STAC collection, not an error page.
func TestLive_SwisstopoSTAC(t *testing.T) {
	if os.Getenv("NOMOS_LIVE_CH_FETCH") != "1" {
		t.Skip("set NOMOS_LIVE_CH_FETCH=1 to run the live Swiss-source fetch")
	}
	const url = "https://data.geo.admin.ch/api/stac/v0.9/collections/ch.swisstopo.swissbuildings3d_3_0"

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	content, result, err := Fetch(ctx, url, FetchOptions{})
	if err != nil {
		t.Fatalf("live fetch failed: %v", err)
	}
	if len(result.SHA256) != len("sha256:")+64 {
		t.Fatalf("live fetch produced a non-real hash %q", result.SHA256)
	}
	if !strings.Contains(string(content), "stac_version") || !strings.Contains(string(content), "ch.swisstopo.swissbuildings3d_3_0") {
		t.Fatal("payload does not look like the STAC collection document")
	}
	ledger := BuildLineLedger(content)
	if !ledger.IsFullyCovered() {
		t.Fatalf("live content left %d bytes uncovered", ledger.UncoveredBytes)
	}
	ev := BuildEvidence("ch-swisstopo-stac", content, result, ledger, 3)
	if err := ev.Validate(); err != nil {
		t.Fatalf("live evidence invalid: %v", err)
	}
	t.Logf("live swisstopo fetch: %d bytes, %s, %d atoms, 0 uncovered", result.ByteCount, result.SHA256, ev.AtomCount)
}

// TestLive_RDPPFOEREB fetches the official OEREB v2 capabilities document of a
// cantonal RDPPF webservice (Zurich — the federal standard's GetCapabilities,
// listing the restriction themes incl. Nutzungsplanung) (VRC-32 / #569).
// Asserts the payload is the capabilities response, not an HTML error page.
func TestLive_RDPPFOEREB(t *testing.T) {
	if os.Getenv("NOMOS_LIVE_CH_FETCH") != "1" {
		t.Skip("set NOMOS_LIVE_CH_FETCH=1 to run the live Swiss-source fetch")
	}
	const url = "https://maps.zh.ch/oereb/v2/capabilities/json"

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	content, result, err := Fetch(ctx, url, FetchOptions{})
	if err != nil {
		t.Fatalf("live fetch failed: %v", err)
	}
	if len(result.SHA256) != len("sha256:")+64 {
		t.Fatalf("live fetch produced a non-real hash %q", result.SHA256)
	}
	if !strings.Contains(string(content), "GetCapabilitiesResponse") {
		t.Fatal("payload does not look like an OEREB capabilities response")
	}
	ledger := BuildLineLedger(content)
	if !ledger.IsFullyCovered() {
		t.Fatalf("live content left %d bytes uncovered", ledger.UncoveredBytes)
	}
	ev := BuildEvidence("ch-rdppf-oereb", content, result, ledger, 3)
	if err := ev.Validate(); err != nil {
		t.Fatalf("live evidence invalid: %v", err)
	}
	t.Logf("live RDPPF fetch: %d bytes, %s, %d atoms, 0 uncovered", result.ByteCount, result.SHA256, ev.AtomCount)
}

// TestLive_RDPPFExtractByEGRID proves the per-parcel RDPPF extract — the
// document Aedifica's domain actually runs on (Nutzungsplanung restrictions of
// ONE parcel). No parcel is hardcoded: each run DISCOVERS its parcels through
// the standard's own getegrid endpoint (LV95 coordinates → EGRID), across
// several distinct locations, then fetches each full extract. So the proof
// holds for arbitrary parcels, not a curated one (VRC-32 / #569).
func TestLive_RDPPFExtractByEGRID(t *testing.T) {
	if os.Getenv("NOMOS_LIVE_CH_FETCH") != "1" {
		t.Skip("set NOMOS_LIVE_CH_FETCH=1 to run the live Swiss-source fetch")
	}
	const base = "https://maps.zh.ch/oereb/v2"
	// Three unrelated locations (LV95): Zurich centre, Zurich-Oerlikon, and a
	// third district — whatever parcel happens to sit there today.
	coords := []string{"2683100,1248100", "2683600,1252000", "2697000,1262000"}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	parcels := 0
	for _, en := range coords {
		egridDoc, _, err := Fetch(ctx, base+"/getegrid/json?EN="+en, FetchOptions{})
		if err != nil {
			t.Fatalf("getegrid(%s): %v", en, err)
		}
		// Pull the first "egrid":"CH…" out of the discovery response — the
		// standard's own answer, never an invented identifier.
		marker := `"egrid":"`
		idx := strings.Index(string(egridDoc), marker)
		if idx < 0 {
			t.Fatalf("getegrid(%s) returned no parcel: %s", en, string(egridDoc[:min(200, len(egridDoc))]))
		}
		rest := string(egridDoc[idx+len(marker):])
		egrid := rest[:strings.Index(rest, `"`)]
		if !strings.HasPrefix(egrid, "CH") {
			t.Fatalf("discovered EGRID %q does not look like an EGRID", egrid)
		}

		content, result, err := Fetch(ctx, base+"/extract/json?EGRID="+egrid, FetchOptions{})
		if err != nil {
			t.Fatalf("extract(%s): %v", egrid, err)
		}
		if len(result.SHA256) != len("sha256:")+64 {
			t.Fatalf("extract(%s) produced a non-real hash %q", egrid, result.SHA256)
		}
		payload := string(content)
		if !strings.Contains(payload, "GetExtractByIdResponse") {
			t.Fatalf("extract(%s) is not an OEREB extract response", egrid)
		}
		if !strings.Contains(payload, "ch.Nutzungsplanung") {
			t.Fatalf("extract(%s) carries no Nutzungsplanung theme — wrong document?", egrid)
		}
		if !strings.Contains(payload, egrid) {
			t.Fatalf("extract(%s) does not reference its own EGRID", egrid)
		}
		ledger := BuildLineLedger(content)
		if !ledger.IsFullyCovered() {
			t.Fatalf("extract(%s) left %d bytes uncovered", egrid, ledger.UncoveredBytes)
		}
		ev := BuildEvidence("ch-rdppf-oereb", content, result, ledger, 2)
		if err := ev.Validate(); err != nil {
			t.Fatalf("extract(%s) evidence invalid: %v", egrid, err)
		}
		parcels++
		t.Logf("parcel %s: %d bytes, %s, %d atoms, 0 uncovered", egrid, result.ByteCount, result.SHA256, ev.AtomCount)
	}
	if parcels < 3 {
		t.Fatalf("only %d parcels proven, want 3", parcels)
	}
}

// TestLive_GeoportailCantonal fetches the WMS GetCapabilities of an official
// cantonal geoportal (Basel-Stadt — its full layer register, incl. the
// Zonenplan/Nutzungsplan layers) (W23-2 / #591). Asserts the payload is the
// genuine capabilities catalogue carrying zoning layers, not an error page.
func TestLive_GeoportailCantonal(t *testing.T) {
	if os.Getenv("NOMOS_LIVE_CH_FETCH") != "1" {
		t.Skip("set NOMOS_LIVE_CH_FETCH=1 to run the live Swiss-source fetch")
	}
	const url = "https://wms.geo.bs.ch/?SERVICE=WMS&REQUEST=GetCapabilities"

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	content, result, err := Fetch(ctx, url, FetchOptions{})
	if err != nil {
		t.Fatalf("live fetch failed: %v", err)
	}
	if len(result.SHA256) != len("sha256:")+64 {
		t.Fatalf("live fetch produced a non-real hash %q", result.SHA256)
	}
	payload := string(content)
	if !strings.Contains(payload, "WMS_Capabilities") {
		t.Fatal("payload is not a WMS capabilities document")
	}
	if !strings.Contains(payload, "Zonenplan") && !strings.Contains(payload, "Nutzungsplan") {
		t.Fatal("capabilities carry no zoning layer — wrong service for the built-environment register")
	}
	ledger := BuildLineLedger(content)
	if !ledger.IsFullyCovered() {
		t.Fatalf("live content left %d bytes uncovered", ledger.UncoveredBytes)
	}
	ev := BuildEvidence("ch-geoportail-cantonal", content, result, ledger, 3)
	if err := ev.Validate(); err != nil {
		t.Fatalf("live evidence invalid: %v", err)
	}
	t.Logf("live geoportail fetch: %d bytes, %s, %d atoms, 0 uncovered", result.ByteCount, result.SHA256, ev.AtomCount)
}
