// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd033-sha512sum R1.1–R1.4
package main

import (
	"crypto/sha512"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the program name used in error messages.
const progName = "sha512sum"

func main() {
	// R1.4/R4.3: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	// Use ContinueOnError for GNU-compatible error handling.
	flag.CommandLine.Init(os.Args[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)

	flagVersion := flag.Bool("version", false, "output version information and exit")
	flagBinary := flag.Bool("b", false, "read in binary mode")
	flag.BoolVar(flagBinary, "binary", false, "read in binary mode")
	flagText := flag.Bool("t", false, "read in text mode (default)")
	flag.BoolVar(flagText, "text", false, "read in text mode (default)")
	flagTag := flag.Bool("tag", false, "create a BSD-style checksum")

	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			printUsage()
			return
		}
		fmt.Fprintf(os.Stderr, "%s: %v\nTry '%s --help' for more information.\n", progName, err, progName)
		os.Exit(1)
	}

	if *flagVersion {
		fmt.Printf("%s (go-unix-utils) 1.0\n", progName)
		return
	}

	// R1.1: default is text mode. -b overrides to binary unless -t is also set.
	binaryMode := *flagBinary && !*flagText

	args := flag.Args()
	exitCode := 0

	if len(args) == 0 {
		// R1.2: no file arguments — read from stdin.
		if err := hashReader(os.Stdin, "-", binaryMode, *flagTag); err != nil {
			fmt.Fprintf(os.Stderr, "%s: -: %v\n", progName, err)
			exitCode = 1
		}
	} else {
		for _, name := range args {
			if name == "-" {
				// R1.2: "-" means stdin.
				if err := hashReader(os.Stdin, "-", binaryMode, *flagTag); err != nil {
					fmt.Fprintf(os.Stderr, "%s: -: %v\n", progName, err)
					exitCode = 1
				}
				continue
			}
			f, err := os.Open(name)
			if err != nil {
				// R1.4: print error to stderr, continue processing remaining files.
				fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, unwrapPathError(err))
				exitCode = 1
				continue
			}
			if err := hashReader(f, name, binaryMode, *flagTag); err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, err)
				exitCode = 1
			}
			f.Close() // best-effort cleanup, error ignored
		}
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// hashReader computes the SHA-512 digest of r and prints the result line to stdout.
// R1.1: GNU format "HASH  FILENAME" (text) or "HASH *FILENAME" (binary).
// R1.3: BSD tag format "SHA512 (FILENAME) = HASH" when tag is true.
func hashReader(r io.Reader, name string, binaryMode, tag bool) error {
	h := sha512.New()
	if _, err := io.Copy(h, r); err != nil {
		return err
	}
	digest := fmt.Sprintf("%x", h.Sum(nil))

	if tag {
		// R1.3: BSD-style output.
		fmt.Printf("SHA512 (%s) = %s\n", name, digest)
	} else if binaryMode {
		// R1.1: binary mode — "HASH *FILENAME".
		fmt.Printf("%s *%s\n", digest, name)
	} else {
		// R1.1: text mode (default) — "HASH  FILENAME" (two spaces).
		fmt.Printf("%s  %s\n", digest, name)
	}
	return nil
}

// unwrapPathError extracts the inner error from an os.PathError for cleaner
// error messages matching GNU sha512sum format.
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}

// printUsage writes GNU-compatible usage information to stdout.
func printUsage() {
	fmt.Printf("Usage: %s [OPTION]... [FILE]...\n", progName)
	fmt.Printf("Print or check SHA512 (512-bit) checksums.\n\n")
	fmt.Printf("With no FILE, or when FILE is -, read standard input.\n\n")
	fmt.Printf("  -b, --binary         read in binary mode\n")
	fmt.Printf("      --tag            create a BSD-style checksum\n")
	fmt.Printf("  -t, --text           read in text mode (default)\n")
	fmt.Printf("\n      --help     display this help and exit\n")
	fmt.Printf("      --version  output version information and exit\n")
}
