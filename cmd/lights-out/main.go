// Command lights-out runs one conductor command: a JSON request on stdin, a
// JSON result on stdout, or a JSON error on stderr with exit status 1.
package main

import (
	"os"

	"github.com/DereKk8/lights-out/internal/conductor"
)

func main() {
	ctx, cancel := conductor.Timeout()
	err := conductor.Run(ctx, os.Stdin, os.Stdout, conductor.Execute, conductor.Followed)
	cancel()
	if err != nil {
		if writeErr := conductor.WriteError(os.Stderr, err); writeErr != nil {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
