// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements GNU true: exits 0 unconditionally.
// Implements prd013-true R1-R4.
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help":
			fmt.Println("Usage: true [ignored command line arguments]")
			fmt.Println("Exit with a status code indicating success.")
			os.Exit(0)
		case "--version":
			fmt.Println("true (go-unix-utils) dev")
			os.Exit(0)
		}
	}

	os.Exit(0)
}
