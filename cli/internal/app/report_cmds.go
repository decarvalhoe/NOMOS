package app

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/RBOKproject/Nomos/cli/internal/attestation"
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

// AttestCommand implements "nomos attest" as a one-shot REAL signing path: it
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
