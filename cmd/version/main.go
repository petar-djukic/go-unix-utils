// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the cmd/version binary.
// Prints the build-time version tag and exits 0.
//
// Implements prd011-magefiles R1.
//
// Build with version injection:
//
//	go build -ldflags "-X main.version=$(git describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null || echo dev)" -o bin/version ./cmd/version/
package main

import (
	"fmt"
	"os"
)

// version is the build-time version string set via -ldflags "-X main.version=<tag>".
// Defaults to "dev" when the linker flag is absent (R1.2).
var version = "dev" //nolint:gochecknoglobals

// Version is the exported build-time version string.
// Other cmd/ packages may import this package and reference Version to embed the
// same build tag without duplicating the ldflags mechanism (R1.5).
var Version = version //nolint:gochecknoglobals

// usage is the message written to stderr when an unrecognised argument is supplied.
const usage = "Usage: version [--version | -v]\n"

func main() {
	args := os.Args[1:]
	if len(args) == 0 || (len(args) == 1 && (args[0] == "--version" || args[0] == "-v")) {
		fmt.Println(Version)
		return
	}
	fmt.Fprint(os.Stderr, usage)
	os.Exit(2)
}
