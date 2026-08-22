package main

import (
	"fmt"
	"os"

	"github.com/shevanio/shevanio-ai/v2/internal/app"
	"github.com/shevanio/shevanio-ai/v2/internal/envcompat"
)

// version is set by GoReleaser via ldflags at build time.
var version = "dev"

func main() {
	if err := envcompat.ApplyLegacyFallbacks(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	app.Version = app.ResolveVersion(version)

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
