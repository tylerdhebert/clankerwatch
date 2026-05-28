package main

import (
	"os"

	"clankerwatch/internal/clankerwatch"
)

func main() {
	os.Exit(clankerwatch.Main(os.Args[1:], os.Stdout, os.Stderr))
}
