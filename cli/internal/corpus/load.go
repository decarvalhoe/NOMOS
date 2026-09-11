package corpus

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/RBOKproject/Nomos/cli/internal/docload"
)

// LoadCorpusBodyLedger is the engine's loader for a corpus body ledger
// (specs/corpus-body-ledger.cue): YAML or JSON, format identifier checked.
// The strict gate and `nomos corpus` read ledgers through it.
func LoadCorpusBodyLedger(path string) (CorpusBodyLedger, error) {
	var ledger CorpusBodyLedger
	if err := docload.Load(path, &ledger); err != nil {
		return CorpusBodyLedger{}, err
	}
	if ledger.Format != BodyLedgerFormat {
		return CorpusBodyLedger{}, fmt.Errorf("body ledger %s: format %q, this engine reads %s", path, ledger.Format, BodyLedgerFormat)
	}
	return ledger, nil
}

// LoadIntegrityReportFile is the engine's loader for an integrity report
// (specs/corpus-integrity-report.cue): either the aggregate
// {source_integrity, feed_quality}, a bare source-integrity report, or a bare
// feed-quality report; YAML or JSON. Moved from the strict gate so that the
// gate and the contract registry read the same way.
func LoadIntegrityReportFile(path string) (*IntegrityReport, *FeedQualityReport, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", path, err)
	}
	var agg struct {
		SourceIntegrity *IntegrityReport   `json:"source_integrity"`
		FeedQuality     *FeedQualityReport `json:"feed_quality"`
	}
	if err := docload.Decode(raw, path, &agg); err == nil && (agg.SourceIntegrity != nil || agg.FeedQuality != nil) {
		return agg.SourceIntegrity, agg.FeedQuality, nil
	}
	var probe map[string]any
	if err := docload.Decode(raw, path, &probe); err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if _, ok := probe["source_count"]; ok {
		var r IntegrityReport
		if err := docload.Decode(raw, path, &r); err != nil {
			return nil, nil, fmt.Errorf("parse source integrity report %s: %w", path, err)
		}
		return &r, nil, nil
	}
	if _, ok := probe["feed_unit_count"]; ok {
		var r FeedQualityReport
		if err := docload.Decode(raw, path, &r); err != nil {
			return nil, nil, fmt.Errorf("parse feed quality report %s: %w", path, err)
		}
		return nil, &r, nil
	}
	_ = json.Valid
	return nil, nil, fmt.Errorf("integrity report %s: shape not recognised (no source_integrity, feed_quality, source_count, or feed_unit_count keys)", path)
}
