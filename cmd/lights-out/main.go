package main

import (
	"github.com/DereKk8/lights-out/internal/conductor"
	"os"
)

func main() {
	ctx, cancel := conductor.Timeout()
	defer cancel()
	if err := conductor.Run(ctx, os.Stdin, os.Stdout, conductor.Execute); err != nil {
		if writeErr := conductor.WriteError(os.Stderr, err); writeErr != nil {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
