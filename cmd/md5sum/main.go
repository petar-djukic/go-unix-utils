// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd030-md5sum R1.1–R1.4
package main

import (
	"crypto/md5"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the program name used in error messages.
const progName = "md5sum"

func main() {
	// D1: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	flagBinary := flag.Bool("b", false, "read in binary mode")
	flag.BoolVar(flagBinary, "binary", false, "read in binary mode")
	flagText := flag.Bool("t", false, "read in text mode (default)")
	flag.BoolVar(flagText, "text", false, "read in text mode (default)")
	flagTag := flag.Bool("tag", false, "create a BSD-style checksum")
	flag.Parse()

	// R1.1: default is text mode. -b overrides to binary unless -t is also set
	// (last flag wins in GNU, but Go flag package takes the last parse; match
	// GNU behavior: -b sets binary mode).
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

// hashReader computes the MD5 digest of r and prints the result line to stdout.
// R1.1: GNU format "HASH  FILENAME" (text) or "HASH *FILENAME" (binary).
// R1.3: BSD tag format "MD5 (FILENAME) = HASH" when tag is true.
func hashReader(r io.Reader, name string, binaryMode, tag bool) error {
	h := md5.New()
	if _, err := io.Copy(h, r); err != nil {
		return err
	}
	digest := fmt.Sprintf("%x", h.Sum(nil))

	if tag {
		// R1.3: BSD-style output.
		fmt.Printf("MD5 (%s) = %s\n", name, digest)
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
// error messages matching GNU md5sum format.
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
