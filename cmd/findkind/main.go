package main

import (
	"context"
	"log"
	"os"

	"github.com/a2y-d5l/findkind/internal/config"
	"github.com/a2y-d5l/findkind/internal/scan"
)

func main() {
	cfg, err := config.ParseFlags(os.Stdout)
	if err != nil {
		// User-supplied flags are invalid → exit 1
		log.Print(err)
		os.Exit(1)
	}

	ctx := context.Background()
	if err := scan.Run(ctx, os.Stdout, cfg); err != nil {
		// Internal failure → exit 2
		log.Print(err)
		os.Exit(2)
	}
}
