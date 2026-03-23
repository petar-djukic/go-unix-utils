// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd086-hashutil R3.1–R3.2: DigestFiles for computing and
// printing hash digests of files or stdin.
package hashutil

import (
	"fmt"
	"io"
	"os"
)

// DigestFiles computes and prints digests for each file in files.
// When files is empty or contains "-", reads from stdin.
//
// R3.1: formats output using FormatBSDTag when tag is true, FormatGNU otherwise.
// R3.2: continues processing remaining files when one cannot be opened,
// printing an error to stderr for each failure. Returns 0 for success,
// 1 if any file failed.
func DigestFiles(files []string, cfg HashConfig, binary, tag bool, stdout, stderr io.Writer) int {
	if len(files) == 0 {
		files = []string{"-"}
	}
	exitCode := 0
	for _, f := range files {
		if err := digestOneFile(f, cfg, binary, tag, stdout); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", f, err)
			exitCode = 1
		}
	}
	return exitCode
}

// digestOneFile computes and prints the digest for a single file or stdin.
func digestOneFile(filename string, cfg HashConfig, binary, tag bool, stdout io.Writer) error {
	r, displayName, err := openInput(filename)
	if err != nil {
		return err
	}
	defer r.Close() // best-effort close on read-only file

	digest, err := ComputeDigest(r, cfg)
	if err != nil {
		return err
	}
	printDigest(displayName, digest, cfg, binary, tag, stdout)
	return nil
}

// openInput opens a file for reading, or returns stdin for "-".
func openInput(filename string) (io.ReadCloser, string, error) {
	if filename == "-" {
		return os.Stdin, "-", nil
	}
	f, err := os.Open(filename)
	if err != nil {
		return nil, "", err
	}
	return f, filename, nil
}

// printDigest formats and writes a single digest line to stdout.
func printDigest(filename, digest string, cfg HashConfig, binary, tag bool, stdout io.Writer) {
	if tag {
		fmt.Fprintln(stdout, FormatBSDTag(cfg.Algorithm, filename, digest))
	} else {
		fmt.Fprintln(stdout, FormatGNU(digest, filename, binary))
	}
}
