package app

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/RBOKproject/Nomos/cli/internal/attestation"
)

// attestSupplyChainCommand implements `nomos attest supply-chain` (VRC-08
// #554): the production caller of the CKM-05 supply-chain predicate, which was
// defined and tested but had zero production callers until the wiring matrix
// caught it. It builds the ingestion -> canon -> embedding steps from REAL
// pipeline artifacts — every digest is computed from the file bytes, never
// declared (doctrine §2.3) — emits the in-toto statement, optionally wraps it
// in a genuinely signed DSSE envelope (ECDSA P-256, signing.go), and verifies
// existing statements against artifact bytes.
//
// The predicate-level Signature stays mode=unsigned until keyless Sigstore
// lands (VRC-40 #576): the optional --key signing here is ENVELOPE-level DSSE
// and must not be presented as predicate-level Sigstore.
func attestSupplyChainCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("attest supply-chain", flag.ContinueOnError)
	flags.SetOutput(stderr)
	projectID := flags.String("project-id", "", "project identifier (required to emit)")
	corpusID := flags.String("corpus-id", "", "corpus identifier (required to emit)")
	snapshotPath := flags.String("snapshot", "", "ingestion material: corpus scan snapshot JSON (required to emit)")
	manifestPath := flags.String("manifest", "", "ingestion product / canon material: source manifest YAML (required to emit)")
	feedPath := flags.String("feed", "", "canon product / embedding material: feed JSON (required to emit)")
	ragPath := flags.String("rag", "", "embedding product: RAG metadata JSON (optional; adds the embedding step)")
	keyPath := flags.String("key", "", "wrap the statement in a signed DSSE envelope using this ECDSA private key PEM (empty = ephemeral key)")
	sign := flags.Bool("sign", false, "wrap the statement in a signed DSSE envelope (ephemeral key unless --key)")
	pubOut := flags.String("pub-out", "", "write the verifying public key (default with signing: <out>.pub.pem)")
	out := flags.String("out", "", "output path (default: stdout)")
	verifyPath := flags.String("verify", "", "verify an existing supply-chain statement JSON and exit")
	var artifacts multiFlag
	flags.Var(&artifacts, "artifact", "artifact bytes to verify, as <subject-name>=<path> (repeatable, used with --verify)")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	if *verifyPath != "" {
		return verifySupplyChainStatementFile(*verifyPath, artifacts, stdout, stderr)
	}

	if *projectID == "" || *corpusID == "" || *snapshotPath == "" || *manifestPath == "" || *feedPath == "" {
		fmt.Fprintln(stderr, "attest supply-chain: --project-id, --corpus-id, --snapshot, --manifest, and --feed are required to emit")
		return 2
	}

	snapshotSub, err := subjectFromFile(*snapshotPath)
	if err != nil {
		fmt.Fprintf(stderr, "attest supply-chain: %v\n", err)
		return 1
	}
	manifestSub, err := subjectFromFile(*manifestPath)
	if err != nil {
		fmt.Fprintf(stderr, "attest supply-chain: %v\n", err)
		return 1
	}
	feedSub, err := subjectFromFile(*feedPath)
	if err != nil {
		fmt.Fprintf(stderr, "attest supply-chain: %v\n", err)
		return 1
	}

	steps := []attestation.SupplyChainStep{
		{
			Name:      attestation.StepIngestion,
			Materials: []attestation.Subject{snapshotSub},
			Products:  []attestation.Subject{manifestSub},
		},
		{
			Name:      attestation.StepCanon,
			Materials: []attestation.Subject{manifestSub},
			Products:  []attestation.Subject{feedSub},
		},
	}
	if *ragPath != "" {
		ragSub, err := subjectFromFile(*ragPath)
		if err != nil {
			fmt.Fprintf(stderr, "attest supply-chain: %v\n", err)
			return 1
		}
		steps = append(steps, attestation.SupplyChainStep{
			Name:      attestation.StepEmbedding,
			Materials: []attestation.Subject{feedSub},
			Products:  []attestation.Subject{ragSub},
		})
	}

	stmt, err := attestation.GenerateSupplyChainStatement(attestation.SupplyChainPredicate{
		ProjectID: *projectID,
		CorpusID:  *corpusID,
		Steps:     steps,
	})
	if err != nil {
		fmt.Fprintf(stderr, "attest supply-chain: %v\n", err)
		return 1
	}

	w := stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			fmt.Fprintf(stderr, "attest supply-chain: create output file: %v\n", err)
			return 1
		}
		defer f.Close()
		w = f
	}

	if !*sign && *keyPath == "" {
		if err := attestation.WriteJSON(w, stmt); err != nil {
			fmt.Fprintf(stderr, "attest supply-chain: write: %v\n", err)
			return 1
		}
		return 0
	}

	signer, err := loadOrGenerateSigner(*keyPath)
	if err != nil {
		fmt.Fprintf(stderr, "attest supply-chain: %v\n", err)
		return 1
	}
	envelope, err := signer.SignStatement(stmt)
	if err != nil {
		fmt.Fprintf(stderr, "attest supply-chain: sign statement: %v\n", err)
		return 1
	}
	pubPath := *pubOut
	if pubPath == "" && *out != "" {
		pubPath = *out + ".pub.pem"
	}
	if pubPath != "" {
		pub, err := signer.PublicKeyPEM()
		if err != nil {
			fmt.Fprintf(stderr, "attest supply-chain: encode public key: %v\n", err)
			return 1
		}
		if err := os.WriteFile(pubPath, pub, 0o644); err != nil {
			fmt.Fprintf(stderr, "attest supply-chain: write public key: %v\n", err)
			return 1
		}
	}
	if err := attestation.WriteJSON(w, envelope); err != nil {
		fmt.Fprintf(stderr, "attest supply-chain: write: %v\n", err)
		return 1
	}
	return 0
}

func verifySupplyChainStatementFile(path string, artifactSpecs []string, stdout io.Writer, stderr io.Writer) int {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "attest supply-chain: read statement: %v\n", err)
		return 1
	}
	var stmt attestation.InTotoStatement
	if err := json.Unmarshal(data, &stmt); err != nil {
		fmt.Fprintf(stderr, "attest supply-chain: decode statement: %v\n", err)
		return 1
	}
	artifacts := make(map[string][]byte, len(artifactSpecs))
	for _, spec := range artifactSpecs {
		name, file, ok := strings.Cut(spec, "=")
		if !ok || name == "" || file == "" {
			fmt.Fprintf(stderr, "attest supply-chain: invalid --artifact %q, expected <subject-name>=<path>\n", spec)
			return 2
		}
		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(stderr, "attest supply-chain: read artifact %s: %v\n", name, err)
			return 1
		}
		artifacts[name] = content
	}
	if err := attestation.VerifySupplyChainStatement(stmt, artifacts); err != nil {
		fmt.Fprintf(stderr, "attest supply-chain: verification FAILED: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "supply-chain statement verification: ok (%d subjects, %d artifact(s) checked)\n",
		len(stmt.Subject), len(artifacts))
	return 0
}

// subjectFromFile computes a subject whose sha256 digest comes from the actual
// file bytes — calculated, never declared.
func subjectFromFile(path string) (attestation.Subject, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return attestation.Subject{}, fmt.Errorf("read %s: %w", path, err)
	}
	return attestation.SubjectFromBytes(filepath.Base(path), data), nil
}

// multiFlag collects a repeatable string flag.
type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}
