// Package main is a convenience entry point.
// The server binary lives in cmd/server/main.go.
// Run: go run ./cmd/server/
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "Run the server with: go run ./cmd/server/")
	os.Exit(1)
}

