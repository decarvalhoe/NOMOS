package app

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

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
	case "verify-sigstore":
		return attestVerifySigstore(args[1:], stdout, stderr)
	case "sign-sigstore":
		return attestSignSigstore(args[1:], stdout, stderr)
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
	fmt.Fprintln(w, "  nomos attest verify-sigstore --bundle <bundle.sigstore.json> --trusted-root <trusted_root.json> (--artifact <file> | --artifact-digest <alg:hex>) --identity <san> --issuer <iss> [--identity-regex <re>] [--issuer-regex <re>] [--verifier <cmd>] [--out <record.json>]")
	fmt.Fprintln(w, "      Verify a SUPPLIED Sigstore bundle OFFLINE through the external verifier (tools/sigstore-verifier). Never signs, never issues, never publishes.")
	fmt.Fprintln(w, "  nomos attest sign-sigstore --artifact <file> --fulcio-url <url> --rekor-url <url> --id-token-file <f> --trusted-root <f> --identity <san> --issuer <iss> --out-bundle <f> [--allow-host <h>]... [--verifier <cmd>] [--out <record.json>]")
	fmt.Fprintln(w, "      Keyless issuance against INJECTED, NON-PRODUCTION Fulcio/Rekor endpoints (#645), then independent verification. Sigstore public instances are refused; there is no flag to allow them (#638).")
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

// attestVerifySigstore is `nomos attest verify-sigstore` (#637): verify a
// supplied Sigstore bundle offline through the versioned external verifier.
// No verifier → non-zero exit and no verdict. A refusal writes no record.
func attestVerifySigstore(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("attest verify-sigstore", flag.ContinueOnError)
	flags.SetOutput(stderr)
	bundlePath := flags.String("bundle", "", "Sigstore bundle JSON (required)")
	trustedRoot := flags.String("trusted-root", "", "trusted root JSON the bundle is verified against (required; supplied, never fetched)")
	artifact := flags.String("artifact", "", "artifact file whose sha256 the bundle must cover")
	artifactDigest := flags.String("artifact-digest", "", "artifact digest the bundle must cover, as sha256|sha384|sha512:<hex> (alternative to --artifact)")
	identity := flags.String("identity", "", "required signer identity (certificate SAN), exact")
	identityRegex := flags.String("identity-regex", "", "required signer identity as a regular expression")
	issuer := flags.String("issuer", "", "required OIDC issuer of the signing certificate, exact")
	issuerRegex := flags.String("issuer-regex", "", "required issuer as a regular expression")
	tlogN := flags.Int("require-tlog-entries", 1, "minimum transparency-log entries the verifier must verify")
	sctN := flags.Int("require-sct", 1, "minimum signed certificate timestamps (0 only for an injected environment that runs no CT log; recorded in the record)")
	verifierCmd := flags.String("verifier", "", "external verifier command (default: $"+attestation.SigstoreVerifierEnv+", then "+attestation.SigstoreVerifierBinary+" on PATH)")
	out := flags.String("out", "", "write the verification record here (default: stdout)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *bundlePath == "" || *trustedRoot == "" {
		fmt.Fprintln(stderr, "attest verify-sigstore: --bundle and --trusted-root are required")
		return 2
	}
	if (*artifact == "") == (*artifactDigest == "") {
		fmt.Fprintln(stderr, "attest verify-sigstore: exactly one of --artifact or --artifact-digest is required")
		return 2
	}
	if *identity == "" && *identityRegex == "" {
		fmt.Fprintln(stderr, "attest verify-sigstore: --identity or --identity-regex is required — an unnamed signer is not a verification")
		return 2
	}
	if *issuer == "" && *issuerRegex == "" {
		fmt.Fprintln(stderr, "attest verify-sigstore: --issuer or --issuer-regex is required")
		return 2
	}
	command, err := attestation.ResolveSigstoreVerifier(*verifierCmd)
	if err != nil {
		fmt.Fprintf(stderr, "attest verify-sigstore: %v\n", err)
		return 1
	}
	req := attestation.SigstoreRequest{
		BundlePath: *bundlePath, TrustedRootPath: *trustedRoot, ArtifactPath: *artifact, ArtifactDigest: *artifactDigest,
		CertificateIdentity: attestation.SigstoreIdentity{SAN: *identity, SANRegex: *identityRegex, Issuer: *issuer, IssuerRegex: *issuerRegex},
		Require:             attestation.SigstoreRequire{TlogEntries: *tlogN, SignedCertificateTimestamps: *sctN, ObserverTimestamps: 1, NoCTLog: *sctN == 0},
	}
	record, _, err := attestation.VerifySigstoreBundle(attestation.ExternalSigstoreVerifier{Command: command}, req, time.Now().UTC())
	if err != nil {
		fmt.Fprintf(stderr, "attest verify-sigstore: REFUSED, no record written — %v\n", err)
		return 1
	}
	write := func(w io.Writer) error {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(record)
	}
	if *out == "" {
		if err := write(stdout); err != nil {
			fmt.Fprintf(stderr, "attest verify-sigstore: %v\n", err)
			return 1
		}
		return 0
	}
	if err := writeFile(*out, write); err != nil {
		fmt.Fprintf(stderr, "attest verify-sigstore: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "attest verify-sigstore: OK — %s signed by %s (issuer %s), %d tlog entr(ies), verifier %s %s via %s %s → %s\n",
		record.ArtifactDigest, record.SignerSAN, record.SignerIssuer, record.TlogEntries,
		record.Verifier, record.VerifierVersion, record.Library, record.LibraryVersion, *out)
	return 0
}

// attestSignSigstore is `nomos attest sign-sigstore` (#645): keyless issuance
// against injected, non-production endpoints through the external process,
// then NOMOS's own independent verification of the produced bundle.
func attestSignSigstore(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("attest sign-sigstore", flag.ContinueOnError)
	flags.SetOutput(stderr)
	artifact := flags.String("artifact", "", "artifact file to sign (required)")
	fulcioURL := flags.String("fulcio-url", "", "INJECTED Fulcio endpoint (required; production instances are refused)")
	rekorURL := flags.String("rekor-url", "", "INJECTED Rekor v1 endpoint (required; production instances are refused)")
	tokenFile := flags.String("id-token-file", "", "file holding the fixture/injected OIDC id token (required; NOMOS never obtains one)")
	trustedRoot := flags.String("trusted-root", "", "trust material of the injected environment, used for the independent verification (required)")
	identity := flags.String("identity", "", "signer identity the certificate must carry (SAN), exact")
	identityRegex := flags.String("identity-regex", "", "signer identity as a regular expression")
	issuer := flags.String("issuer", "", "OIDC issuer the certificate must carry, exact")
	issuerRegex := flags.String("issuer-regex", "", "issuer as a regular expression")
	outBundle := flags.String("out-bundle", "", "where to write the issued bundle (required)")
	verifierCmd := flags.String("verifier", "", "external verifier/issuer command (default: $"+attestation.SigstoreVerifierEnv+", then "+attestation.SigstoreVerifierBinary+" on PATH)")
	out := flags.String("out", "", "write the issuance record here (default: stdout)")
	var allowHosts multiFlag
	flags.Var(&allowHosts, "allow-host", "additional controlled hostname to accept (repeatable); never a Sigstore public instance")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *artifact == "" || *fulcioURL == "" || *rekorURL == "" || *tokenFile == "" || *trustedRoot == "" || *outBundle == "" {
		fmt.Fprintln(stderr, "attest sign-sigstore: --artifact, --fulcio-url, --rekor-url, --id-token-file, --trusted-root and --out-bundle are required")
		return 2
	}
	if (*identity == "" && *identityRegex == "") || (*issuer == "" && *issuerRegex == "") {
		fmt.Fprintln(stderr, "attest sign-sigstore: --identity/--identity-regex and --issuer/--issuer-regex are required")
		return 2
	}
	// Policy first, before any token is read or any process runs.
	for _, u := range []string{*fulcioURL, *rekorURL} {
		if err := attestation.CheckEndpointPolicy(u, allowHosts); err != nil {
			fmt.Fprintf(stderr, "attest sign-sigstore: REFUSED before any request — %v\n", err)
			return 1
		}
	}
	token, err := os.ReadFile(*tokenFile)
	if err != nil {
		fmt.Fprintf(stderr, "attest sign-sigstore: id token: %v\n", err)
		return 1
	}
	command, err := attestation.ResolveSigstoreVerifier(*verifierCmd)
	if err != nil {
		fmt.Fprintf(stderr, "attest sign-sigstore: %v\n", err)
		return 1
	}
	req := attestation.SigstoreIssueRequest{
		ArtifactPath: *artifact, OutBundlePath: *outBundle, FulcioURL: *fulcioURL, RekorURL: *rekorURL,
		IDToken: strings.TrimSpace(string(token)), TrustedRootPath: *trustedRoot,
		Identity:   attestation.SigstoreIdentity{SAN: *identity, SANRegex: *identityRegex, Issuer: *issuer, IssuerRegex: *issuerRegex},
		AllowHosts: allowHosts,
	}
	record, err := attestation.IssueSigstoreBundle(
		attestation.ExternalSigstoreIssuer{Command: command},
		attestation.ExternalSigstoreVerifier{Command: command},
		req, time.Now().UTC())
	if err != nil {
		fmt.Fprintf(stderr, "attest sign-sigstore: REFUSED, no bundle kept, no record written — %v\n", err)
		return 1
	}
	write := func(w io.Writer) error {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(record)
	}
	if *out == "" {
		if err := write(stdout); err != nil {
			return 1
		}
		return 0
	}
	if err := writeFile(*out, write); err != nil {
		fmt.Fprintf(stderr, "attest sign-sigstore: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "attest sign-sigstore: OK (NON-PRODUCTION) — %s signed by %s (issuer %s) via injected fulcio=%s rekor=%s; bundle %s; independently verified → %s\n",
		record.ArtifactDigest, record.SignerSAN, record.SignerIssuer, record.Endpoints["fulcio"], record.Endpoints["rekor"], record.BundlePath, *out)
	return 0
}
