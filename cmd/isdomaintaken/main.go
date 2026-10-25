package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/berkaycubuk/isdomaintaken/internal/cli"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		code := cli.CodeError
		var ce *cli.Error
		if errors.As(err, &ce) {
			code = ce.Code
		}
		// An empty message is a silent exit carrying only a status code.
		if msg := err.Error(); msg != "" {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", msg)
		}
		os.Exit(int(code))
	}
}
