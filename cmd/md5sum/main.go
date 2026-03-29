// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/md5sum computes MD5 message digests for files or stdin.
//
// Implements prd030-md5sum: R1.1 (file digest computation), R1.2 (stdin),
// R1.3 (binary/text mode flags), R1.4 (SIGPIPE handling).
package main

import (
	"crypto/md5"
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/hashutil"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// md5Config is the hashutil configuration for MD5 digests.
//
// R1.1: MD5 hash via crypto/md5 with 32-character hex digest.
var md5Config = hashutil.HashConfig{
	Algorithm: "MD5",
	NewHash:   md5.New,
	DigestLen: 32,
}

func main() {
	// R1.4: SIGPIPE handling.
	sys.InstallSIGPIPEHandler()

	binary, tag, files := parseArgs(os.Args[1:])
	exitCode := hashutil.DigestFiles(files, md5Config, binary, tag, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// parseArgs parses GNU-compatible flags from args and returns the binary mode
// flag, tag mode flag, and remaining file arguments.
//
// R1.3: -b/--binary sets binary mode; -t/--text sets text mode (default).
func parseArgs(args []string) (binary, tag bool, files []string) {
	for _, arg := range args {
		switch arg {
		case "-b", "--binary":
			binary = true
		case "-t", "--text":
			binary = false
		case "--tag":
			tag = true
		case "--":
			// remaining args are files
			continue
		case "--help":
			printUsage()
			os.Exit(0)
		case "--version":
			fmt.Fprintln(os.Stdout, "md5sum (go-unix-utils)")
			os.Exit(0)
		default:
			files = append(files, arg)
		}
	}
	return binary, tag, files
}

// printUsage prints the usage message to stdout.
func printUsage() {
	fmt.Fprintln(os.Stdout, "Usage: md5sum [OPTION]... [FILE]...")
	fmt.Fprintln(os.Stdout, "Print or check MD5 checksums.")
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "  -b, --binary  read in binary mode")
	fmt.Fprintln(os.Stdout, "  -t, --text    read in text mode (default)")
	fmt.Fprintln(os.Stdout, "      --tag     create a BSD-style checksum")
}
