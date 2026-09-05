package portfolio

import (
	"encoding/json"
	"fmt"
	"io"
)

// WriteJSON writes the status as indented JSON.
func WriteJSON(w io.Writer, st Status) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(st)
}

// WriteMarkdown renders a table per section from the SAME computed values —
// prose is never added, only the numbers and their sources.
func WriteMarkdown(w io.Writer, st Status) error {
	fmt.Fprintf(w, "# Portfolio status — %s\n\nDigest `%s` · %d section(s) unavailable · %d stale · stale after %d days.\n\n", st.GeneratedAt, st.StatusDigest, st.SectionsUnavailable, st.SectionsStale, st.FreshnessPolicy.StaleAfterDays)
	fmt.Fprintln(w, "| Section | Available | Key figures | Source |")
	fmt.Fprintln(w, "|---|---|---|---|")
	row := func(name string, section any) {
		switch s := section.(type) {
		case Unavailable:
			fmt.Fprintf(w, "| %s | no | %s | — |\n", name, s.Reason)
		case Capabilities:
			fmt.Fprintf(w, "| %s | yes | %d capabilities: real %d, sidecar %d, partial %d, stub %d, absent %d; mismatches %d; registry/matrix agree %v | `%s` (%s) |\n", name, s.Total, s.Computed.Real, s.Computed.Sidecar, s.Computed.Partial, s.Computed.Stub, s.Computed.Absent, s.Mismatches, s.ExpectedVsComputedAgree, s.Matrix.Path, s.Matrix.Sha256[:19])
		case Roadmap:
			fmt.Fprintf(w, "| %s | yes | product open %d / closed %d (queue %v); devops open %d / closed %d (queue %v); regulated passive %d, human %d, external %d | `%s` (%s) |\n", name, s.Lanes.Product.AutonomousOpen, s.Lanes.Product.AutonomousClosed, s.Lanes.Product.Queue, s.Lanes.Devops.AutonomousOpen, s.Lanes.Devops.AutonomousClosed, s.Lanes.Devops.Queue, s.Lanes.Regulated.Passive, s.Lanes.Regulated.Human, s.Lanes.Regulated.External, s.Source.Path, s.Source.Freshness)
		case Gaps:
			fmt.Fprintf(w, "| %s | yes | %d blocking gaps, %d open | `%s` (%s) |\n", name, s.Total, s.Open, s.Source.Path, s.Source.Freshness)
		case CapaSection:
			fmt.Fprintf(w, "| %s | yes | %d records, %d open | `%s` |\n", name, s.Total, s.Open, s.Directory)
		case Reviews:
			fmt.Fprintf(w, "| %s | yes | %d records | `%s` |\n", name, s.Total, s.Directory)
		case RepeatedCI:
			fmt.Fprintf(w, "| %s | yes | %d/%d consecutive green, claim_unlocked %v | `%s` (%s) |\n", name, s.ConsecutiveGreenRuns, s.TargetConsecutiveGreenRuns, s.ClaimUnlocked, s.Source.Path, s.Source.Freshness)
		case PraxisGate:
			fmt.Fprintf(w, "| %s | yes | %s, %d/%d unmet | `%s` |\n", name, s.Status, s.UnmetCount, s.Checks, s.Record.Path)
		case Competence:
			fmt.Fprintf(w, "| %s | yes | %d attestation file(s), %d waived record(s); role status by `%s` | `%s` |\n", name, s.AttestationFiles, s.WaivedRecords, s.RoleStatusComputedBy, s.Waiver.Path)
		case DomainPacks:
			fmt.Fprintf(w, "| %s | yes | %d pack(s) | `%s` |\n", name, s.Total, s.Directory)
		case PublicSources:
			fmt.Fprintf(w, "| %s | yes | %d source(s): %v | `%s` (%s) |\n", name, s.Total, s.ByStatus, s.Source.Path, s.Source.Freshness)
		case ReleaseCandidate:
			fmt.Fprintf(w, "| %s | yes | %s %s, approval %s, %d open gap(s) | `%s` |\n", name, s.Version, s.Verdict, s.ApprovalStatus, s.OpenGaps, s.Source.Path)
		default:
			fmt.Fprintf(w, "| %s | ? | unrenderable section | — |\n", name)
		}
	}
	row("capabilities", st.Capabilities)
	row("roadmap", st.Roadmap)
	row("gaps", st.Gaps)
	row("capa", st.Capa)
	row("reviews", st.Reviews)
	row("repeated_ci", st.RepeatedCI)
	row("praxis_gate", st.PraxisGate)
	row("competence", st.Competence)
	row("domain_packs", st.DomainPacks)
	row("public_sources", st.PublicSources)
	row("release_candidate", st.ReleaseCandidate)
	fmt.Fprintf(w, "\n%s\n", st.ClaimBoundary)
	return nil
}
