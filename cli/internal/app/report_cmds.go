package app

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/RBOKproject/Nomos/cli/internal/attestation"
	"github.com/RBOKproject/Nomos/cli/internal/corpus"
	"github.com/RBOKproject/Nomos/cli/internal/detect"
	"github.com/RBOKproject/Nomos/cli/internal/export"
	"github.com/RBOKproject/Nomos/cli/internal/report"
)

// ReportCommand implements "nomos report".
func ReportCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("report", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root to inspect")
	projectID := flags.String("project-id", "", "project identifier")
	projectName := flags.String("project-name", "", "project name")
	domain := flags.String("domain", "", "project domain")
	riskLevel := flags.String("risk-level", "", "project risk level")
	outputPath := flags.String("output", "", "write report to file (default: stdout)")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	if flags.NArg() == 1 && *root == "." {
		*root = flags.Arg(0)
	}

	dr, err := detect.Detect(*root)
	if err != nil {
		fmt.Fprintf(stderr, "report: detect failed: %v\n", err)
		return 1
	}

	nr := report.Generate(dr, report.Options{
		ProjectID:   *projectID,
		ProjectName: *projectName,
		Domain:      *domain,
		RiskLevel:   *riskLevel,
		ToolVersion: Version,
		Mode:        "report",
	})

	w := stdout
	if *outputPath != "" {
		f, err := os.Create(*outputPath)
		if err != nil {
			fmt.Fprintf(stderr, "report: create output file: %v\n", err)
			return 1
		}
		defer f.Close()
		w = f
	}

	if err := report.WriteJSON(w, nr); err != nil {
		fmt.Fprintf(stderr, "report: write: %v\n", err)
		return 1
	}
	return 0
}

// exportCommand is `nomos export`: BOM exports of the project report. The two
// exporters below (and their internal/export engine) were implemented and
// tested but unreachable until the wiring matrix caught them (VRC-09 #555);
// wiring them is the groundwork of the evidence-BOM lane (VRC-23 #566).
func exportCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		exportUsage(stdout)
		return 0
	}
	switch args[0] {
	case "spdx":
		return ExportSPDXCommand(args[1:], stdout, stderr)
	case "cyclonedx":
		return ExportCycloneDXCommand(args[1:], stdout, stderr)
	case "evidence-bom":
		return ExportEvidenceBOMCommand(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		exportUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown export subcommand %q\n\n", args[0])
		exportUsage(stderr)
		return 2
	}
}

func exportUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  nomos export spdx [--root <dir>] [--project-id <id>] [--output <file>]")
	fmt.Fprintln(w, "  nomos export cyclonedx [--root <dir>] [--project-id <id>] [--output <file>]")
	fmt.Fprintln(w, "  nomos export evidence-bom --body-ledger <file> [--format cyclonedx|spdx] [--output <file>]")
}

// uuidFromRoot derives a deterministic UUID-shaped id from a Merkle root hex,
// so the BOM serial number is reproducible (no randomness, no wall clock).
func uuidFromRoot(root string) string {
	hex := root
	if i := strings.Index(hex, ":"); i >= 0 {
		hex = hex[i+1:]
	}
	hex = strings.ToLower(hex)
	for len(hex) < 32 {
		hex += "0"
	}
	hex = hex[:32]
	return fmt.Sprintf("%s-%s-%s-%s-%s", hex[0:8], hex[8:12], hex[12:16], hex[16:20], hex[20:32])
}

// ExportEvidenceBOMCommand implements "nomos export evidence-bom" (VRC-23 #566):
// emit a CycloneDX/SPDX BOM for a corpus evidence pack, with each source's
// sha256 cross-checked against the Merkle-verified body ledger. Fails closed
// (exit 1) if verification fails — a tampered hash never produces a BOM.
func ExportEvidenceBOMCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("export evidence-bom", flag.ContinueOnError)
	flags.SetOutput(stderr)
	ledgerPath := flags.String("body-ledger", "", "corpus body-ledger JSON path (required)")
	format := flags.String("format", "cyclonedx", "BOM format: cyclonedx or spdx")
	outputPath := flags.String("output", "", "write BOM to file (default: stdout)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*ledgerPath) == "" {
		fmt.Fprintln(stderr, "export evidence-bom: --body-ledger is required")
		return 2
	}
	raw, err := os.ReadFile(*ledgerPath)
	if err != nil {
		fmt.Fprintf(stderr, "export evidence-bom: read ledger: %v\n", err)
		return 1
	}
	var ledger corpus.CorpusBodyLedger
	if err := json.Unmarshal(raw, &ledger); err != nil {
		fmt.Fprintf(stderr, "export evidence-bom: parse ledger: %v\n", err)
		return 1
	}

	var payload any
	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "cyclonedx", "cdx":
		serial := "urn:uuid:" + uuidFromRoot(ledgerMerkleRoot(ledger))
		bom, gErr := export.GenerateEvidenceCycloneDX(ledger, serial)
		if gErr != nil {
			fmt.Fprintf(stderr, "%v\n", gErr)
			return 1
		}
		payload = bom
	case "spdx":
		ns := "https://nomos.dev/spdx/evidence-pack/" + uuidFromRoot(ledgerMerkleRoot(ledger))
		doc, gErr := export.GenerateEvidenceSPDX(ledger, ns)
		if gErr != nil {
			fmt.Fprintf(stderr, "%v\n", gErr)
			return 1
		}
		payload = doc
	default:
		fmt.Fprintf(stderr, "export evidence-bom: unknown format %q (want cyclonedx or spdx)\n", *format)
		return 2
	}

	w := stdout
	if *outputPath != "" {
		f, ferr := os.Create(*outputPath)
		if ferr != nil {
			fmt.Fprintf(stderr, "export evidence-bom: create output file: %v\n", ferr)
			return 1
		}
		defer f.Close()
		w = f
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		fmt.Fprintf(stderr, "export evidence-bom: write: %v\n", err)
		return 1
	}
	return 0
}

func ledgerMerkleRoot(ledger corpus.CorpusBodyLedger) string {
	if ledger.Merkle == nil {
		return ""
	}
	return ledger.Merkle.Root
}

// ExportSPDXCommand implements "nomos export spdx".
func ExportSPDXCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("export spdx", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root to inspect")
	projectID := flags.String("project-id", "", "project identifier")
	outputPath := flags.String("output", "", "write SPDX to file (default: stdout)")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	nr, err := generateReport(*root, *projectID)
	if err != nil {
		fmt.Fprintf(stderr, "export spdx: %v\n", err)
		return 1
	}

	doc := export.GenerateSPDX(nr)

	w := stdout
	if *outputPath != "" {
		f, err := os.Create(*outputPath)
		if err != nil {
			fmt.Fprintf(stderr, "export spdx: create output file: %v\n", err)
			return 1
		}
		defer f.Close()
		w = f
	}

	if err := export.WriteSPDX(w, doc); err != nil {
		fmt.Fprintf(stderr, "export spdx: write: %v\n", err)
		return 1
	}
	return 0
}

// ExportCycloneDXCommand implements "nomos export cyclonedx".
func ExportCycloneDXCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("export cyclonedx", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root to inspect")
	projectID := flags.String("project-id", "", "project identifier")
	outputPath := flags.String("output", "", "write CycloneDX to file (default: stdout)")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	nr, err := generateReport(*root, *projectID)
	if err != nil {
		fmt.Fprintf(stderr, "export cyclonedx: %v\n", err)
		return 1
	}

	bom := export.GenerateCycloneDX(nr)

	w := stdout
	if *outputPath != "" {
		f, err := os.Create(*outputPath)
		if err != nil {
			fmt.Fprintf(stderr, "export cyclonedx: create output file: %v\n", err)
			return 1
		}
		defer f.Close()
		w = f
	}

	if err := export.WriteCycloneDX(w, bom); err != nil {
		fmt.Fprintf(stderr, "export cyclonedx: write: %v\n", err)
		return 1
	}
	return 0
}

// AttestCommand implements "nomos attest create" as a one-shot REAL signing path: it
// builds an in-toto statement and emits a genuinely signed DSSE envelope (ECDSA
// P-256 over the DSSE PAE, via internal/attestation/signing.go). The legacy fake
// path (WrapCosignEnvelope, which hard-coded Sig:"") was removed in CKM-H1-FU
// (#537): every envelope this command produces carries a real signature and a
// public key with which it can be verified.
//
// By default an ephemeral key is generated and its public key is written next to
// the envelope (or to --pub-out) so the signature is verifiable; pass --key to
// sign with a stable private key instead.
func AttestCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("attest", flag.ContinueOnError)
	flags.SetOutput(stderr)
	projectID := flags.String("project-id", "", "project identifier (required)")
	verdict := flags.String("verdict", "", "attestation verdict (required)")
	subjectPath := flags.String("subject", "", "path to artifact to attest")
	keyPath := flags.String("key", "", "ECDSA private key PEM to sign with (default: ephemeral key)")
	pubOut := flags.String("pub-out", "", "write the public key needed to verify (default: <output>.pub.pem)")
	outputPath := flags.String("output", "", "write attestation to file (default: stdout)")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	if *projectID == "" {
		fmt.Fprintln(stderr, "attest: --project-id is required")
		return 2
	}
	if *verdict == "" {
		fmt.Fprintln(stderr, "attest: --verdict is required")
		return 2
	}

	var subjects []attestation.Subject
	if *subjectPath != "" {
		data, err := os.ReadFile(*subjectPath)
		if err != nil {
			fmt.Fprintf(stderr, "attest: read subject: %v\n", err)
			return 1
		}
		subjects = append(subjects, attestation.SubjectFromBytes(*subjectPath, data))
	}

	att := attestation.NomosAttestation{
		ProjectID: *projectID,
		Verdict:   *verdict,
	}

	stmt, err := attestation.GenerateStatement(att, subjects)
	if err != nil {
		fmt.Fprintf(stderr, "attest: generate statement: %v\n", err)
		return 1
	}

	signer, err := loadOrGenerateSigner(*keyPath)
	if err != nil {
		fmt.Fprintf(stderr, "attest: %v\n", err)
		return 1
	}

	envelope, err := signer.SignStatement(stmt)
	if err != nil {
		fmt.Fprintf(stderr, "attest: sign statement: %v\n", err)
		return 1
	}

	// Write the public key so the real signature is verifiable. With an
	// ephemeral key this is mandatory; default to <output>.pub.pem.
	pubPath := *pubOut
	if pubPath == "" && *outputPath != "" {
		pubPath = *outputPath + ".pub.pem"
	}
	if pubPath != "" {
		pub, err := signer.PublicKeyPEM()
		if err != nil {
			fmt.Fprintf(stderr, "attest: encode public key: %v\n", err)
			return 1
		}
		if err := os.WriteFile(pubPath, pub, 0o644); err != nil {
			fmt.Fprintf(stderr, "attest: write public key: %v\n", err)
			return 1
		}
	}

	w := stdout
	if *outputPath != "" {
		f, err := os.Create(*outputPath)
		if err != nil {
			fmt.Fprintf(stderr, "attest: create output file: %v\n", err)
			return 1
		}
		defer f.Close()
		w = f
	}

	if err := attestation.WriteJSON(w, envelope); err != nil {
		fmt.Fprintf(stderr, "attest: write: %v\n", err)
		return 1
	}
	return 0
}

// loadOrGenerateSigner returns a signer from the given private-key PEM path, or a
// fresh ephemeral signer when the path is empty.
func loadOrGenerateSigner(keyPath string) (*attestation.Signer, error) {
	if keyPath == "" {
		return attestation.GenerateSigner()
	}
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read key: %w", err)
	}
	return attestation.SignerFromPEM(data)
}

func generateReport(root string, projectID string) (report.NomosReport, error) {
	dr, err := detect.Detect(root)
	if err != nil {
		return report.NomosReport{}, fmt.Errorf("detect: %w", err)
	}
	return report.Generate(dr, report.Options{
		ProjectID:   projectID,
		ToolVersion: Version,
		Mode:        "report",
	}), nil
}
