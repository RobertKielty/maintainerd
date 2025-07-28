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
		addr          = flag.String("addr", ":8080", "Address to listen on (e.g. :8080)")
	)
	flag.Parse()

	if *webhookSecret == "" {
		*webhookSecret = os.Getenv("GITHUB_WEBHOOK_SECRET")
	}
	if *webhookSecret == "" {
		log.Fatal("must provide --webhook-secret or set GITHUB_WEBHOOK_SECRET")
	}

	// instantiate and initialize listener
	listener := &onboarding.EventListener{
		Secret: []byte(*webhookSecret),
	}
	if err := listener.Init(*dbPath, *fossaEnvVar); err != nil {
		log.Fatalf("failed to init EventListener: %v", err)
	}

	log.Printf("Starting onboarding server on %s…", *addr)
	if err := listener.Run(*addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
