package app

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/RBOKproject/Nomos/cli/internal/attestation"
)

// attestCommand is `nomos attest`: emit and verify REAL cryptographic signatures
// (ECDSA P-256 DSSE) over NOMOS attestation predicates (CKM-H1 / #519). This is
// the CLI surface that earns the word "signed": keygen → sign → verify, with a
// tamper-evident envelope. Keyless Sigstore (Fulcio/Rekor) is a documented
// network follow-up; this is real, offline, key-based DSSE.
func attestCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		attestUsage(stdout)
		return 0
	}
	switch args[0] {
	case "keygen":
		return attestKeygen(args[1:], stdout, stderr)
	case "sign":
		return attestSign(args[1:], stdout, stderr)
	case "create":
		return AttestCommand(args[1:], stdout, stderr)
	case "supply-chain":
		return attestSupplyChainCommand(args[1:], stdout, stderr)
	case "verify":
		return attestVerify(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		attestUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown attest subcommand %q\n\n", args[0])
		attestUsage(stderr)
		return 2
	}
}

func attestUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  nomos attest keygen --out <priv.pem> --pub-out <pub.pem>")
	fmt.Fprintln(w, "  nomos attest sign --project-id <id> --verdict <v> [--subject <artifact>] --key <priv.pem> --out <envelope.json>")
	fmt.Fprintln(w, "  nomos attest sign --statement <statement.json> --key <priv.pem> --out <envelope.json>")
	fmt.Fprintln(w, "  nomos attest create --project-id <id> --verdict <v> [--subject <artifact>] [--key <priv.pem>] [--output <envelope.json>]")
	fmt.Fprintln(w, "  nomos attest supply-chain --project-id <id> --corpus-id <id> --snapshot <f> --manifest <f> --feed <f> [--rag <f>] [--sign] [--out <f>]")
	fmt.Fprintln(w, "  nomos attest supply-chain --verify <statement.json> [--artifact <name>=<path>]...")
	fmt.Fprintln(w, "  nomos attest verify --envelope <envelope.json> --pub <pub.pem>")
}

func attestKeygen(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("attest keygen", flag.ContinueOnError)
	flags.SetOutput(stderr)
	out := flags.String("out", "", "private key output path (PKCS#8 PEM, required)")
	pubOut := flags.String("pub-out", "", "public key output path (PKIX PEM, required)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *out == "" || *pubOut == "" {
		fmt.Fprintln(stderr, "attest keygen: --out and --pub-out are required")
		return 2
	}
	signer, err := attestation.GenerateSigner()
	if err != nil {
		fmt.Fprintf(stderr, "attest keygen: %v\n", err)
		return 1
	}
	priv, err := signer.PrivateKeyPEM()
	if err != nil {
		fmt.Fprintf(stderr, "attest keygen: %v\n", err)
		return 1
	}
	pub, err := signer.PublicKeyPEM()
	if err != nil {
		fmt.Fprintf(stderr, "attest keygen: %v\n", err)
		return 1
	}
	if err := os.WriteFile(*out, priv, 0o600); err != nil {
		fmt.Fprintf(stderr, "attest keygen: write private key: %v\n", err)
		return 1
	}
	if err := os.WriteFile(*pubOut, pub, 0o644); err != nil {
		fmt.Fprintf(stderr, "attest keygen: write public key: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "generated ECDSA P-256 key (keyid %s)\n", signer.KeyID())
	return 0
}

func attestSign(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("attest sign", flag.ContinueOnError)
	flags.SetOutput(stderr)
	statementPath := flags.String("statement", "", "path to an existing in-toto statement JSON to sign")
	projectID := flags.String("project-id", "", "project identifier (when building a statement)")
	verdict := flags.String("verdict", "", "attestation verdict (when building a statement)")
	subjectPath := flags.String("subject", "", "artifact to attest (when building a statement)")
	keyPath := flags.String("key", "", "ECDSA private key PEM; if omitted an ephemeral key is generated and --pub-out is required")
	out := flags.String("out", "", "DSSE envelope output path (default: stdout)")
	pubOut := flags.String("pub-out", "", "write the public key needed to verify (required with an ephemeral key)")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	stmt, code := attestBuildOrLoadStatement(*statementPath, *projectID, *verdict, *subjectPath, stderr)
	if code != 0 {
		return code
	}

	signer, ephemeral, code := attestLoadSigner(*keyPath, stderr)
	if code != 0 {
		return code
	}
	if ephemeral && *pubOut == "" {
		fmt.Fprintln(stderr, "attest sign: --pub-out is required with an ephemeral key (otherwise the signature is unverifiable)")
		return 2
	}

	env, err := signer.SignStatement(stmt)
	if err != nil {
		fmt.Fprintf(stderr, "attest sign: %v\n", err)
		return 1
	}

	if *pubOut != "" {
		pub, err := signer.PublicKeyPEM()
		if err != nil {
			fmt.Fprintf(stderr, "attest sign: %v\n", err)
			return 1
		}
		if err := os.WriteFile(*pubOut, pub, 0o644); err != nil {
			fmt.Fprintf(stderr, "attest sign: write public key: %v\n", err)
			return 1
		}
	}

	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "attest sign: %v\n", err)
		return 1
	}
	if *out == "" {
		if _, err := stdout.Write(append(data, '\n')); err != nil {
			fmt.Fprintf(stderr, "attest sign: %v\n", err)
			return 1
		}
		return 0
	}
	if err := os.WriteFile(*out, append(data, '\n'), 0o644); err != nil {
		fmt.Fprintf(stderr, "attest sign: write envelope: %v\n", err)
		return 1
	}
	fmt.Fprintf(stderr, "signed with keyid %s\n", signer.KeyID())
	return 0
}

func attestVerify(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("attest verify", flag.ContinueOnError)
	flags.SetOutput(stderr)
	envelopePath := flags.String("envelope", "", "DSSE envelope path (required)")
	pubPath := flags.String("pub", "", "public key PEM (required)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *envelopePath == "" || *pubPath == "" {
		fmt.Fprintln(stderr, "attest verify: --envelope and --pub are required")
		return 2
	}
	envData, err := os.ReadFile(*envelopePath)
	if err != nil {
		fmt.Fprintf(stderr, "attest verify: read envelope: %v\n", err)
		return 1
	}
	var env attestation.DSSEEnvelope
	if err := json.Unmarshal(envData, &env); err != nil {
		fmt.Fprintf(stderr, "attest verify: parse envelope: %v\n", err)
		return 1
	}
	pubData, err := os.ReadFile(*pubPath)
	if err != nil {
		fmt.Fprintf(stderr, "attest verify: read public key: %v\n", err)
		return 1
	}
	pub, err := attestation.ParsePublicKeyPEM(pubData)
	if err != nil {
		fmt.Fprintf(stderr, "attest verify: %v\n", err)
		return 1
	}
	if err := attestation.VerifyEnvelope(env, pub); err != nil {
		fmt.Fprintf(stdout, "INVALID: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "OK: signature verified")
	return 0
}

func attestBuildOrLoadStatement(statementPath, projectID, verdict, subjectPath string, stderr io.Writer) (attestation.InTotoStatement, int) {
	if statementPath != "" {
		data, err := os.ReadFile(statementPath)
		if err != nil {
			fmt.Fprintf(stderr, "attest sign: read statement: %v\n", err)
			return attestation.InTotoStatement{}, 1
		}
		var stmt attestation.InTotoStatement
		if err := json.Unmarshal(data, &stmt); err != nil {
			fmt.Fprintf(stderr, "attest sign: parse statement: %v\n", err)
			return attestation.InTotoStatement{}, 1
		}
		return stmt, 0
	}

	if projectID == "" || verdict == "" {
		fmt.Fprintln(stderr, "attest sign: provide --statement, or --project-id and --verdict to build one")
		return attestation.InTotoStatement{}, 2
	}
	var subjects []attestation.Subject
	if subjectPath != "" {
		data, err := os.ReadFile(subjectPath)
		if err != nil {
			fmt.Fprintf(stderr, "attest sign: read subject: %v\n", err)
			return attestation.InTotoStatement{}, 1
		}
		subjects = append(subjects, attestation.SubjectFromBytes(subjectPath, data))
	} else {
		fmt.Fprintln(stderr, "attest sign: --subject is required when building a statement")
		return attestation.InTotoStatement{}, 2
	}
	stmt, err := attestation.GenerateStatement(attestation.NomosAttestation{
		ProjectID: projectID,
		Verdict:   verdict,
	}, subjects)
	if err != nil {
		fmt.Fprintf(stderr, "attest sign: generate statement: %v\n", err)
		return attestation.InTotoStatement{}, 1
	}
	return stmt, 0
}

func attestLoadSigner(keyPath string, stderr io.Writer) (*attestation.Signer, bool, int) {
	if keyPath == "" {
		signer, err := attestation.GenerateSigner()
		if err != nil {
			fmt.Fprintf(stderr, "attest sign: %v\n", err)
			return nil, false, 1
		}
		return signer, true, 0
	}
	data, err := os.ReadFile(keyPath)
	if err != nil {
		fmt.Fprintf(stderr, "attest sign: read key: %v\n", err)
		return nil, false, 1
	}
	signer, err := attestation.SignerFromPEM(data)
	if err != nil {
		fmt.Fprintf(stderr, "attest sign: %v\n", err)
		return nil, false, 1
	}
	return signer, false, 0
}
