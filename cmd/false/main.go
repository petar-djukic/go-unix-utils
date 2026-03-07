// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements GNU false: exits 1 unconditionally.
// Implements prd014-false R1-R4.
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
			fmt.Println("Usage: false [ignored command line arguments]")
			fmt.Println("Exit with a status code indicating failure.")
			os.Exit(0)
		case "--version":
			fmt.Println("false (go-unix-utils) dev")
			os.Exit(0)
		}
	}

	os.Exit(1)
}
