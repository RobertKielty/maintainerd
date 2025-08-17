// main.go
package main

import (
	"flag"
	"log"
	"os"

	"maintainerd/onboarding"
)

func main() {
	// command‑line flags
	var (
		dbPath        = flag.String("db-path", "/data/onboarding.db", "Path to SQLite database file")
		fossaEnvVar   = flag.String("fossa-token-env", "FOSSA_API_TOKEN", "Name of the env var holding the FOSSA API token")
		webhookSecret = flag.String("webhook-secret", "", "GitHub webhook secret (raw string)")
		addr          = flag.String("addr", "2525", "Address to listen on (e.g. :2525)")
		ghRep         = flag.String("repo", "sandbox", "Name of the repository (e.g. sandbox)")
		ghOrg         = flag.String("org", "cncf", "Name of the GitHub org (e.g. cncf)")
		ghToken       = flag.String("gh-api", "", "GitHub API token (raw string)")
	)
	flag.Parse()

	if *webhookSecret == "" {
		*webhookSecret = os.Getenv("GITHUB_WEBHOOK_SECRET")
	}
	if *webhookSecret == "" {
		log.Fatal("must provide --webhook-secret or set GITHUB_WEBHOOK_SECRET")
	}
	if *ghToken == "" {
		*ghToken = os.Getenv("GITHUB_API_TOKEN")
	}

	// instantiate and initialize listener
	listener := &onboarding.EventListener{
		Secret: []byte(*webhookSecret),
	}
	if err := listener.Init(*dbPath, *fossaEnvVar, *ghToken, *ghOrg, *ghRep); err != nil {
		log.Fatalf("maintainerd: ERR, failed to init EventListener: %v", err)
	}

	log.Printf("maintainerd: DBG, Starting onboarding server on %s…", *addr)
	if err := listener.Run(*addr); err != nil {
		log.Fatalf("maintainerd: ERR, server error: %v", err)
	}
}
