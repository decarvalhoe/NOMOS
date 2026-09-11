// Adversarial fixture for scripts/security_process_gate.py (NRT-025 #678).
// It pins golang.org/x/text v0.3.6 and CALLS language.ParseAcceptLanguage,
// the symbol behind GO-2021-0113 (CVE-2021-38561). govulncheck must report it
// as a called vulnerability and the gate must turn red. Never build this into
// a product artifact.
module example.invalid/nomos/security-fixture

go 1.22

require golang.org/x/text v0.3.6
