// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// digest.go implements prd086 R1.2, R1.3, R1.4, R3.1, R3.2:
// hash digest computation, output formatting, and file processing.

package hashutil

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// FormatGNU formats a digest line in GNU format: "HASH  FILENAME" for text
// mode or "HASH *FILENAME" for binary mode.
//
// R1.2: GNU format output.
func FormatGNU(digest, filename string, binary bool) string {
	sep := "  "
	if binary {
		sep = " *"
	}
	return digest + sep + filename
}

// FormatBSDTag formats a digest line in BSD tag format:
// "ALGORITHM (FILENAME) = HASH".
//
// R1.3: BSD tag format output.
func FormatBSDTag(algorithm, filename, digest string) string {
	return fmt.Sprintf("%s (%s) = %s", algorithm, filename, digest)
}

// ComputeDigest reads all bytes from r, computes the hash using cfg, and
// returns the lowercase hex-encoded digest string.
//
// R1.4: Digest computation.
func ComputeDigest(r io.Reader, cfg HashConfig) (string, error) {
	h := cfg.NewHash()
	if _, err := io.Copy(h, r); err != nil {
		return "", fmt.Errorf("computing digest: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// DigestFiles computes and prints digests for each file (or stdin when files
// is empty or contains "-"), returning the exit code (0 for success, 1 if any
// file failed).
//
// R3.1, R3.2: File processing with error continuation.
func DigestFiles(files []string, cfg HashConfig, binary, tag bool, stdout, stderr io.Writer) int {
	if len(files) == 0 {
		files = []string{"-"}
	}
	exitCode := 0
	for _, name := range files {
		if err := digestOneFile(name, cfg, binary, tag, stdout); err != nil {
			fmt.Fprintf(stderr, "%s: %s\n", programName(name), err)
			exitCode = 1
		}
	}
	return exitCode
}

// digestOneFile computes and prints the digest for a single file or stdin.
func digestOneFile(name string, cfg HashConfig, binary, tag bool, stdout io.Writer) error {
	r, displayName, err := openFileOrStdin(name)
	if err != nil {
		return err
	}
	if r != os.Stdin {
		defer r.Close() // best-effort close for opened files
	}
	digest, err := ComputeDigest(r, cfg)
	if err != nil {
		return err
	}
	line := formatDigestLine(cfg, digest, displayName, binary, tag)
	fmt.Fprintln(stdout, line)
	return nil
}

// openFileOrStdin opens the named file, or returns stdin for "-".
func openFileOrStdin(name string) (io.ReadCloser, string, error) {
	if name == "-" {
		return os.Stdin, "-", nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, name, err
	}
	return f, name, nil
}

// formatDigestLine formats a digest line using tag or GNU format.
func formatDigestLine(cfg HashConfig, digest, name string, binary, tag bool) string {
	if tag {
		return FormatBSDTag(cfg.Algorithm, name, digest)
	}
	return FormatGNU(digest, name, binary)
}

// programName returns the display name for error messages.
func programName(name string) string {
	if name == "-" {
		return "-"
	}
	return name
}
