// Package fixtureservices runs a CONTROLLED, NON-PRODUCTION Fulcio + Rekor v1
// pair on localhost for the keyless-issuance slice (#645). It is a fixture:
// its CA is generated at start, its OIDC "verification" only decodes an
// UNSIGNED fixture token, and its log holds the entries of one process. It
// exists so that issuance can be exercised and verified end to end with no
// production service, no real identity and no public write.
package fixtureservices

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sigstore/rekor/pkg/generated/models"
	"github.com/sigstore/rekor/pkg/types"
	_ "github.com/sigstore/rekor/pkg/types/dsse/v0.0.1"         // register dsse
	_ "github.com/sigstore/rekor/pkg/types/hashedrekord/v0.0.1" // register hashedrekord
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/testing/ca"
	"github.com/sigstore/sigstore-go/pkg/tlog"
)

// FixtureIssuer is the OIDC issuer the fixture Fulcio accepts. It is not a
// real issuer: tokens are unsigned JSON and only their claims are read.
const FixtureIssuer = "https://oidc.fixture.invalid"

// Services is a running fixture pair.
type Services struct {
	FulcioURL string
	RekorURL  string
	// TrustedRootJSON is the trust material that verifies what this fixture issues.
	TrustedRootJSON []byte

	server    *http.Server
	listener  net.Listener
	vs        *ca.VirtualSigstore
	rootCert  *x509.Certificate
	interCert *x509.Certificate
	interKey  *ecdsa.PrivateKey
	rekorPub  crypto.PublicKey
	mu        sync.Mutex
	logIndex  int64
	entries   map[string]models.LogEntryAnon
	served    []string
}

// Start listens on addr ("127.0.0.1:0" for an ephemeral port).
func Start(addr string) (*Services, error) {
	vs, err := ca.NewVirtualSigstore()
	if err != nil {
		return nil, err
	}
	rootCert, rootKey, err := ca.GenerateRootCa()
	if err != nil {
		return nil, err
	}
	interCert, interKey, err := ca.GenerateFulcioIntermediate(rootCert, rootKey)
	if err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	base := "http://" + ln.Addr().String()
	s := &Services{
		FulcioURL: base, RekorURL: base, listener: ln, vs: vs,
		rootCert: rootCert, interCert: interCert, interKey: interKey,
		entries: map[string]models.LogEntryAnon{},
	}
	for _, l := range vs.RekorLogs() {
		s.rekorPub = l.PublicKey
	}
	fulcioCA := &root.FulcioCertificateAuthority{
		Root: rootCert, Intermediates: []*x509.Certificate{interCert}, URI: base,
		ValidityPeriodStart: time.Now().Add(-time.Hour), ValidityPeriodEnd: time.Now().Add(24 * time.Hour),
	}
	// The virtual CA keys its log maps by the hex log ID and stores that hex
	// STRING as the ID bytes — fine in memory, wrong once serialised: a trusted
	// root carries the raw 32-byte key ID (hex-decoded), which is what a bundle's
	// tlog entry names. Re-key both maps on raw bytes before serialising.
	tr, err := root.NewTrustedRoot(root.TrustedRootMediaType01, []root.CertificateAuthority{fulcioCA}, rawKeyIDs(vs.CTLogs()), nil, rawKeyIDs(vs.RekorLogs()))
	if err != nil {
		return nil, err
	}
	s.TrustedRootJSON, err = tr.MarshalJSON()
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/signingCert", s.handleSigningCert)
	mux.HandleFunc("/api/v1/log/entries", s.handleCreateEntry)
	mux.HandleFunc("/api/v1/log/entries/", s.handleGetEntry)
	s.server = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = s.server.Serve(ln) }()
	return s, nil
}

// Close stops the servers.
func (s *Services) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

// Served lists what the fixture did, for the record.
func (s *Services) Served() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.served...)
}

// WriteMaterial writes trusted_root.json, a fixture id token and services.json into dir.
func (s *Services) WriteMaterial(dir, subject string) (map[string]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	token := FixtureIDToken(subject, time.Now().Add(time.Hour))
	files := map[string]string{
		"trusted_root.json": filepath.Join(dir, "trusted_root.json"),
		"id_token":          filepath.Join(dir, "id_token"),
		"services.json":     filepath.Join(dir, "services.json"),
	}
	if err := os.WriteFile(files["trusted_root.json"], s.TrustedRootJSON, 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(files["id_token"], []byte(token), 0o600); err != nil {
		return nil, err
	}
	meta, _ := json.MarshalIndent(map[string]any{
		"schema_version": "nomos-sigstore-fixture-services-v1",
		"fulcio_url":     s.FulcioURL,
		"rekor_url":      s.RekorURL,
		"oidc_issuer":    FixtureIssuer,
		"subject":        subject,
		"trusted_root":   files["trusted_root.json"],
		"id_token":       files["id_token"],
		"non_production": true,
		"claim_boundary": "A localhost fixture: generated CA, unsigned fixture OIDC token, single-process log. It proves the issuance protocol, not any production trust.",
	}, "", "  ")
	if err := os.WriteFile(files["services.json"], append(meta, '\n'), 0o644); err != nil {
		return nil, err
	}
	return files, nil
}

// FixtureIDToken builds an UNSIGNED JWT (alg none) with the fixture issuer.
// The fixture Fulcio reads its claims and nothing else — it verifies no OIDC.
func FixtureIDToken(subject string, exp time.Time) string {
	enc := func(v any) string {
		b, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	header := enc(map[string]any{"alg": "none", "typ": "JWT"})
	payload := enc(map[string]any{
		"iss": FixtureIssuer, "sub": subject, "email": subject, "email_verified": true,
		"aud": "sigstore", "exp": exp.Unix(), "iat": time.Now().Unix(), "nomos_fixture": true,
	})
	return header + "." + payload + "."
}

func decodeFixtureToken(bearer string) (issuer, subject string, err error) {
	parts := strings.Split(strings.TrimSpace(bearer), ".")
	if len(parts) < 2 {
		return "", "", errors.New("token is not a JWT shape")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", fmt.Errorf("token payload: %w", err)
	}
	var claims struct {
		Iss   string `json:"iss"`
		Sub   string `json:"sub"`
		Email string `json:"email"`
		Exp   int64  `json:"exp"`
	}
	if err := json.Unmarshal(raw, &claims); err != nil {
		return "", "", fmt.Errorf("token claims: %w", err)
	}
	if claims.Iss != FixtureIssuer {
		return "", "", fmt.Errorf("issuer %q is not the fixture issuer", claims.Iss)
	}
	if claims.Exp != 0 && time.Now().Unix() > claims.Exp {
		return "", "", errors.New("token expired")
	}
	subject = claims.Email
	if subject == "" {
		subject = claims.Sub
	}
	if subject == "" {
		return "", "", errors.New("token has no subject")
	}
	return claims.Iss, subject, nil
}

// ---- Fulcio ----------------------------------------------------------------

func (s *Services) handleSigningCert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	bearer := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	issuer, subject, err := decodeFixtureToken(bearer)
	if err != nil {
		http.Error(w, "fixture fulcio: "+err.Error(), http.StatusUnauthorized)
		return
	}
	var req struct {
		PublicKeyRequest struct {
			PublicKey struct {
				Algorithm string `json:"algorithm"`
				Content   string `json:"content"`
			} `json:"publicKey"`
			ProofOfPossession string `json:"proofOfPossession"`
		} `json:"publicKeyRequest"`
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "fixture fulcio: request: "+err.Error(), http.StatusBadRequest)
		return
	}
	block, _ := pem.Decode([]byte(req.PublicKeyRequest.PublicKey.Content))
	if block == nil {
		http.Error(w, "fixture fulcio: public key is not PEM", http.StatusBadRequest)
		return
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		http.Error(w, "fixture fulcio: public key: "+err.Error(), http.StatusBadRequest)
		return
	}
	// Proof of possession: the client signed the token subject with its key.
	pop, err := base64.StdEncoding.DecodeString(req.PublicKeyRequest.ProofOfPossession)
	if err != nil {
		http.Error(w, "fixture fulcio: proof of possession is not base64", http.StatusBadRequest)
		return
	}
	ecPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		http.Error(w, "fixture fulcio: only ECDSA keys are issued by the fixture", http.StatusBadRequest)
		return
	}
	digest := sha256.Sum256([]byte(subject))
	if !ecdsa.VerifyASN1(ecPub, digest[:], pop) {
		http.Error(w, "fixture fulcio: proof of possession does not verify", http.StatusBadRequest)
		return
	}
	leaf, err := s.issueLeaf(subject, issuer, pub)
	if err != nil {
		http.Error(w, "fixture fulcio: "+err.Error(), http.StatusInternalServerError)
		return
	}
	chain := []string{pemCert(leaf), pemCert(s.interCert), pemCert(s.rootCert)}
	s.mu.Lock()
	s.served = append(s.served, "fulcio:signingCert:"+subject)
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		// Detached SCT: the fixture runs no CT log, so the certificate carries no SCT.
		"signedCertificateDetachedSct": map[string]any{"chain": map[string]any{"certificates": chain}},
	})
}

func (s *Services) issueLeaf(subject, issuer string, pub crypto.PublicKey) (*x509.Certificate, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 100))
	if err != nil {
		return nil, err
	}
	issuerDER, err := asn1.MarshalWithParams(issuer, "utf8")
	if err != nil {
		return nil, err
	}
	now := time.Now().Add(-time.Minute)
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		NotBefore:    now,
		NotAfter:     now.Add(10 * time.Minute),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		IsCA:         false,
		ExtraExtensions: []pkix.Extension{
			{Id: asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 1}, Value: []byte(issuer)}, // OIDC issuer (v1)
			{Id: asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 8}, Value: issuerDER},      // OIDC issuer (v2, DER UTF8String)
		},
	}
	if strings.Contains(subject, "@") {
		tmpl.EmailAddresses = []string{subject}
	} else if u, err := url.Parse(subject); err == nil && u.Scheme != "" {
		tmpl.URIs = []*url.URL{u}
	} else {
		return nil, fmt.Errorf("subject %q is neither an email nor a URI", subject)
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, s.interCert, pub, s.interKey)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(der)
}

func pemCert(c *x509.Certificate) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Raw}))
}

// ---- Rekor v1 --------------------------------------------------------------

func (s *Services) handleCreateEntry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	proposed, err := models.UnmarshalProposedEntry(strings.NewReader(string(body)), jsonConsumer{})
	if err != nil {
		http.Error(w, "fixture rekor: proposed entry: "+err.Error(), http.StatusBadRequest)
		return
	}
	impl, err := types.CreateVersionedEntry(proposed)
	if err != nil {
		http.Error(w, "fixture rekor: entry: "+err.Error(), http.StatusBadRequest)
		return
	}
	canonical, err := types.CanonicalizeEntry(r.Context(), impl)
	if err != nil {
		http.Error(w, "fixture rekor: canonicalize: "+err.Error(), http.StatusBadRequest)
		return
	}
	leafHash := sha256.Sum256(append([]byte{0}, canonical...))
	uuid := hex.EncodeToString(leafHash[:])
	logID, err := logIDOf(s.rekorPub)
	if err != nil {
		http.Error(w, "fixture rekor: log id: "+err.Error(), http.StatusInternalServerError)
		return
	}
	integrated := time.Now().Unix()
	bodyB64 := base64.StdEncoding.EncodeToString(canonical)
	// The virtual log signs a checkpoint over a ONE-LEAF tree: every entry is
	// index 0 of its own tree. The SET, the entry and the proof must agree on
	// that index or the promise does not verify. A fixture, not a log.
	proof, err := s.vs.GetInclusionProof(canonical)
	if err != nil {
		http.Error(w, "fixture rekor: inclusion proof: "+err.Error(), http.StatusInternalServerError)
		return
	}
	index := *proof.LogIndex
	set, err := s.vs.RekorSignPayload(tlog.RekorPayload{Body: bodyB64, IntegratedTime: integrated, LogIndex: index, LogID: logID})
	if err != nil {
		http.Error(w, "fixture rekor: SET: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.mu.Lock()
	s.logIndex++
	s.mu.Unlock()
	entry := models.LogEntryAnon{
		Body:           bodyB64,
		IntegratedTime: &integrated,
		LogID:          &logID,
		LogIndex:       &index,
		Verification: &models.LogEntryAnonVerification{
			SignedEntryTimestamp: set,
			InclusionProof:       proof,
		},
	}
	s.mu.Lock()
	s.entries[uuid] = entry
	s.served = append(s.served, "rekor:createLogEntry:"+uuid[:12])
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("ETag", uuid)
	w.Header().Set("Location", s.RekorURL+"/api/v1/log/entries/"+uuid)
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(models.LogEntry{uuid: entry})
}

func (s *Services) handleGetEntry(w http.ResponseWriter, r *http.Request) {
	uuid := strings.TrimPrefix(r.URL.Path, "/api/v1/log/entries/")
	s.mu.Lock()
	entry, ok := s.entries[uuid]
	s.mu.Unlock()
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(models.LogEntry{uuid: entry})
}

type jsonConsumer struct{}

func (jsonConsumer) Consume(r io.Reader, v any) error { return json.NewDecoder(r).Decode(v) }

func logIDOf(pub crypto.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:]), nil
}

var _ = elliptic.P256 // keep crypto/elliptic referenced for readers of the CA shape

// rawKeyIDs re-keys a virtual log map on hex-decoded key IDs (see Start).
func rawKeyIDs(logs map[string]*root.TransparencyLog) map[string]*root.TransparencyLog {
	out := map[string]*root.TransparencyLog{}
	for hexID, l := range logs {
		raw, err := hex.DecodeString(hexID)
		if err != nil {
			raw = []byte(hexID)
		}
		copyLog := *l
		copyLog.ID = raw
		out[hexID] = &copyLog
	}
	return out
}
