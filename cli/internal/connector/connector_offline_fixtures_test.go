package connector

import (
	"strings"
	"testing"
)

// W23-2 (#591) — the per-connector OFFLINE fixtures the #569 DoD asks for:
// every known source family has a realistic embedded payload snippet (heads
// only — never full documents) proving, with ZERO network, that (a) the
// descriptor resolves, (b) the family's payload marker is what the live tests
// assert, (c) the evidence pipeline (ledger + samples + no-full-text) holds on
// that payload shape. The live tests stay the reality check; these fixtures
// keep the contract visible when the network is off.
var offlineFixtures = []struct {
	connectorID string
	marker      string
	payload     string
}{
	{
		connectorID: "ch-ofs-commune-register",
		marker:      "HistoricalCode",
		payload:     "HistoricalCode,BfsCode,ValidFrom,ValidTo,Level,Parent,Name\n11000,1,1960-01-01,,3,100,Aeugst am Albis\n11001,2,1960-01-01,,3,100,Affoltern am Albis\n",
	},
	{
		connectorID: "ch-fedlex-eli",
		marker:      "rdf:RDF",
		payload: `<?xml version="1.0" encoding="utf-8" ?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns:jolux="http://data.legilux.public.lu/resource/ontology/jolux#">
  <rdf:Description rdf:about="https://fedlex.data.admin.ch/eli/cc/9999/0000_0000_0000">
    <jolux:title>Loi exemple sur les espaces construits</jolux:title>
  </rdf:Description>
</rdf:RDF>`,
	},
	{
		connectorID: "ch-swisstopo-stac",
		marker:      "stac_version",
		payload:     `{"stac_version":"0.9.0","id":"ch.swisstopo.swissbuildings3d_3_0","title":"swissBUILDINGS3D 3.0 Beta","links":[{"rel":"self","href":"https://data.geo.admin.ch/api/stac/v0.9/collections/ch.swisstopo.swissbuildings3d_3_0"}]}`,
	},
	{
		connectorID: "ch-rdppf-oereb",
		marker:      "GetCapabilitiesResponse",
		payload:     `{"GetCapabilitiesResponse":{"topic":[{"code":"ch.Nutzungsplanung","Text":[{"Language":"de","Text":"Nutzungsplanung"}]},{"code":"ch.Planungszonen","Text":[{"Language":"de","Text":"Planungszonen"}]}]}}`,
	},
	{
		connectorID: "ch-geoportail-cantonal",
		marker:      "WMS_Capabilities",
		payload: `<?xml version="1.0" encoding="utf-8"?>
<WMS_Capabilities version="1.3.0"><Capability><Layer>
  <Layer><Name>ch.bs.zonenplan_stadt_basel</Name><Title>Zonenplan Stadt Basel</Title></Layer>
</Layer></Capability></WMS_Capabilities>`,
	},
}

func TestOffline_EveryKnownSourceHasAFixture(t *testing.T) {
	covered := map[string]bool{}
	for _, f := range offlineFixtures {
		covered[f.connectorID] = true
	}
	for id := range KnownSources {
		if !covered[id] {
			t.Fatalf("known source %q has no offline fixture — the #569 DoD requires one per connector", id)
		}
	}
}

func TestOffline_ConnectorFixtures(t *testing.T) {
	for _, f := range offlineFixtures {
		t.Run(f.connectorID, func(t *testing.T) {
			desc, ok := KnownSources[f.connectorID]
			if !ok {
				t.Fatalf("descriptor missing for %s", f.connectorID)
			}
			if desc.Jurisdiction != "CH" || desc.Authority == "" || desc.LicenseNote == "" {
				t.Fatalf("descriptor incomplete: %+v", desc)
			}
			// The marker the live test asserts must hold on the fixture too —
			// one contract, offline and online.
			if !strings.Contains(f.payload, f.marker) {
				t.Fatalf("fixture payload lost its marker %q", f.marker)
			}
			content := []byte(f.payload)
			ledger := BuildLineLedger(content)
			if !ledger.IsFullyCovered() {
				t.Fatalf("fixture left %d bytes uncovered", ledger.UncoveredBytes)
			}
			ev := BuildEvidence(f.connectorID, content, FetchResult{
				URL:        "https://offline.fixture/" + f.connectorID,
				FetchedAt:  "2026-06-11T00:00:00Z",
				StatusCode: 200,
				ByteCount:  len(content),
				SHA256:     sha256Prefixed(content),
			}, ledger, 2)
			if err := ev.Validate(); err != nil {
				t.Fatalf("evidence invalid on fixture: %v", err)
			}
			if !ev.NoFullText {
				t.Fatal("no_full_text discipline lost")
			}
			if ev.Source.ID != f.connectorID {
				t.Fatalf("descriptor did not resolve: %+v", ev.Source)
			}
		})
	}
}
