// Package main is the entry point for dorikin.
package main

import (
	"os"

	"github.com/indrasvat/dorikin/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
