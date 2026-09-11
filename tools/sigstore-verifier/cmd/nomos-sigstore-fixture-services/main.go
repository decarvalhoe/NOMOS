// nomos-sigstore-fixture-services — a localhost, non-production Fulcio + Rekor
// pair for the keyless-issuance slice (#645). Writes trusted_root.json, a
// fixture id token and services.json into --out-dir, then serves until
// SIGINT/SIGTERM. Nothing here is a production service or a real identity.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/RBOKproject/Nomos/tools/sigstore-verifier/fixtureservices"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:0", "address to listen on (loopback only)")
	outDir := flag.String("out-dir", "", "directory for trusted_root.json, id_token, services.json (required)")
	subject := flag.String("subject", "fixture-signer@nomos.invalid", "fixture identity written into the id token")
	flag.Parse()
	if *outDir == "" {
		fmt.Fprintln(os.Stderr, "--out-dir is required")
		os.Exit(2)
	}
	svc, err := fixtureservices.Start(*listen)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fixture services: %v\n", err)
		os.Exit(1)
	}
	files, err := svc.WriteMaterial(*outDir, *subject)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fixture services: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("fixture services ready: fulcio=%s rekor=%s material=%s (NON-PRODUCTION)\n", svc.FulcioURL, svc.RekorURL, files["services.json"])
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	_ = svc.Close()
}
