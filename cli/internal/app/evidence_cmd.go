package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

const evidenceBundleSchemaVersion = "nomos-evidence-bundle-v1"

type evidenceArtifactHash struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type evidenceBundle struct {
	SchemaVersion               string                 `json:"schema_version"`
	BundleID                    string                 `json:"bundle_id"`
	BundleVersion               string                 `json:"bundle_version"`
	Issuer                      string                 `json:"issuer"`
	Subject                     string                 `json:"subject"`
	GeneratedAtUTC              string                 `json:"generated_at_utc"`
	ArtifactHashes              []evidenceArtifactHash `json:"artifact_hashes"`
	ALCOAEnvelopeHash           string                 `json:"alcoa_envelope_hash"`
	SourceCommit                string                 `json:"source_commit"`
	SignatureAlgorithm          string                 `json:"signature_algorithm"`
	SignatureValueOrExternalRef string                 `json:"signature_value_or_external_ref"`
	SignatureMode               string                 `json:"signature_mode"`
	VerificationStatus          string                 `json:"verification_status"`
	VerificationInstructions    string                 `json:"verification_instructions"`
	TransparencyEntryRef        string                 `json:"transparency_entry_ref"`
	ClaimBoundary               string                 `json:"claim_boundary"`
	Warnings                    []string               `json:"warnings,omitempty"`
}

type evidenceVerifyReport struct {
	SchemaVersion    string   `json:"schema_version"`
	Status           string   `json:"status"`
	BundleID         string   `json:"bundle_id"`
	ArtifactsChecked int      `json:"artifacts_checked"`
	SignatureMode    string   `json:"signature_mode"`
	Findings         []string `json:"findings,omitempty"`
	Warnings         []string `json:"warnings,omitempty"`
	ClaimBoundary    string   `json:"claim_boundary"`
}

type repeatStringFlag []string

func (v *repeatStringFlag) String() string {
	if v == nil {
		return ""
	}
	return fmt.Sprint([]string(*v))
}

func (v *repeatStringFlag) Set(value string) error {
	if value == "" {
		return fmt.Errorf("empty value")
	}
	*v = append(*v, value)
	return nil
}

func evidenceCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		printEvidenceUsage(stdout)
		return 0
	}
	switch args[0] {
	case "hash":
		return evidenceHashCommand(args[1:], stdout, stderr)
	case "sign":
		return evidenceSignCommand(args[1:], stdout, stderr)
	case "verify":
		return evidenceVerifyCommand(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		printEvidenceUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown evidence subcommand %q\n\n", args[0])
		printEvidenceUsage(stderr)
		return 2
	}
}

func printEvidenceUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  nomos evidence hash --artifact <path>")
	fmt.Fprintln(w, "  nomos evidence sign --artifact <path> --out <bundle.json> --bundle-id <id>")
	fmt.Fprintln(w, "  nomos evidence verify --bundle <bundle.json>")
}

func evidenceHashCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("evidence hash", flag.ContinueOnError)
	flags.SetOutput(stderr)
	artifact := flags.String("artifact", "", "artifact path to hash")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *artifact == "" {
		fmt.Fprintln(stderr, "evidence hash: --artifact is required")
		return 2
	}
	digest, err := fileSHA256(*artifact)
	if err != nil {
		fmt.Fprintf(stderr, "evidence hash: %v\n", err)
		return 1
	}
	return writeEvidenceJSON(stdout, map[string]string{
		"schema_version": evidenceBundleSchemaVersion,
		"path":           *artifact,
		"sha256":         digest,
	}, stderr)
}

func evidenceSignCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("evidence sign", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var artifacts repeatStringFlag
	flags.Var(&artifacts, "artifact", "artifact path to include; may be repeated")
	outPath := flags.String("out", "", "bundle output path")
	bundleID := flags.String("bundle-id", "", "bundle identifier")
	bundleVersion := flags.String("bundle-version", "0.1.0", "bundle version")
	issuer := flags.String("issuer", "not_assigned", "bundle issuer")
	subject := flags.String("subject", "evidence-bundle", "bundle subject")
	signatureRef := flags.String("signature-ref", "", "external signature reference")
	signatureAlgorithm := flags.String("signature-algorithm", "external-detached", "signature algorithm or external mode")
	alcoaHash := flags.String("alcoa-envelope-hash", "", "ALCOA envelope hash")
	sourceCommit := flags.String("source-commit", "", "source commit bound to the bundle")
	transparencyRef := flags.String("transparency-entry-ref", "", "optional transparency entry reference")
	unsigned := flags.Bool("unsigned", false, "allow unsigned weaker bundle mode")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if len(artifacts) == 0 {
		fmt.Fprintln(stderr, "evidence sign: at least one --artifact is required")
		return 2
	}
	if *outPath == "" {
		fmt.Fprintln(stderr, "evidence sign: --out is required")
		return 2
	}
	if *bundleID == "" {
		fmt.Fprintln(stderr, "evidence sign: --bundle-id is required")
		return 2
	}
	if *unsigned && *signatureRef != "" {
		fmt.Fprintln(stderr, "evidence sign: --unsigned cannot be combined with --signature-ref")
		return 2
	}

	hashes := make([]evidenceArtifactHash, 0, len(artifacts))
	for _, artifact := range artifacts {
		digest, err := fileSHA256(artifact)
		if err != nil {
			fmt.Fprintf(stderr, "evidence sign: %v\n", err)
			return 1
		}
		hashes = append(hashes, evidenceArtifactHash{Path: artifact, SHA256: digest})
	}

	mode := "external_signature_reference"
	status := "prepared_for_external_signature"
	algorithm := *signatureAlgorithm
	warnings := []string(nil)
	if *unsigned {
		mode = "unsigned_weaker"
		status = "unsigned_weaker"
		algorithm = "none"
		warnings = append(warnings, "unsigned mode is available but weaker than detached or external signatures")
	}

	bundle := evidenceBundle{
		SchemaVersion:               evidenceBundleSchemaVersion,
		BundleID:                    *bundleID,
		BundleVersion:               *bundleVersion,
		Issuer:                      *issuer,
		Subject:                     *subject,
		GeneratedAtUTC:              time.Now().UTC().Format(time.RFC3339),
		ArtifactHashes:              hashes,
		ALCOAEnvelopeHash:           *alcoaHash,
		SourceCommit:                *sourceCommit,
		SignatureAlgorithm:          algorithm,
		SignatureValueOrExternalRef: *signatureRef,
		SignatureMode:               mode,
		VerificationStatus:          status,
		VerificationInstructions:    "Run nomos evidence verify --bundle <bundle.json> before trusting artifact integrity.",
		TransparencyEntryRef:        *transparencyRef,
		ClaimBoundary:               "Tamper-evidence only; signatures and hashes do not prove semantic correctness.",
		Warnings:                    warnings,
	}
	if err := writeEvidenceJSONFile(*outPath, bundle); err != nil {
		fmt.Fprintf(stderr, "evidence sign: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "wrote evidence bundle %s\n", *outPath)
	return 0
}

func evidenceVerifyCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("evidence verify", flag.ContinueOnError)
	flags.SetOutput(stderr)
	bundlePath := flags.String("bundle", "", "bundle path to verify")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *bundlePath == "" {
		fmt.Fprintln(stderr, "evidence verify: --bundle is required")
		return 2
	}

	var bundle evidenceBundle
	raw, err := os.ReadFile(*bundlePath)
	if err != nil {
		fmt.Fprintf(stderr, "evidence verify: read bundle: %v\n", err)
		return 1
	}
	if err := json.Unmarshal(raw, &bundle); err != nil {
		fmt.Fprintf(stderr, "evidence verify: parse bundle: %v\n", err)
		return 1
	}

	report := evidenceVerifyReport{
		SchemaVersion: evidenceBundleSchemaVersion,
		Status:        "verified",
		BundleID:      bundle.BundleID,
		SignatureMode: bundle.SignatureMode,
		ClaimBoundary: "Hash verification only; this does not prove semantic correctness.",
	}
	if bundle.SignatureMode == "unsigned_weaker" {
		report.Status = "verified_unsigned_weaker"
		report.Warnings = append(report.Warnings, "bundle is unsigned and weaker than detached or external signatures")
	}

	for _, artifact := range bundle.ArtifactHashes {
		actual, err := fileSHA256(artifact.Path)
		if err != nil {
			report.Findings = append(report.Findings, fmt.Sprintf("read artifact %s: %v", artifact.Path, err))
			continue
		}
		report.ArtifactsChecked++
		if actual != artifact.SHA256 {
			report.Findings = append(report.Findings, fmt.Sprintf("artifact hash mismatch for %s", artifact.Path))
		}
	}

	if len(report.Findings) > 0 {
		report.Status = "failed"
		_ = writeEvidenceJSON(stdout, report, stderr)
		for _, finding := range report.Findings {
			fmt.Fprintf(stderr, "evidence verify: %s\n", finding)
		}
		return 1
	}
	return writeEvidenceJSON(stdout, report, stderr)
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func writeEvidenceJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func writeEvidenceJSON(stdout io.Writer, value any, stderr io.Writer) int {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "evidence: marshal json: %v\n", err)
		return 1
	}
	if _, err := stdout.Write(append(data, '\n')); err != nil {
		fmt.Fprintf(stderr, "evidence: write json: %v\n", err)
		return 1
	}
	return 0
}
