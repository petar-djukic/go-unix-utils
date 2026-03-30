// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/cksum computes POSIX CRC-32 checksums and byte counts, or hash digests
// using alternate algorithms, for files or stdin.
//
// Implements prd077-cksum: R1.1-R1.4 (CRC checksum), R2.1 (algorithm selection),
// R2.2 (--untagged output), R2.3 (--raw output), R3.1 (exit 0 on success),
// R3.2 (exit 1 on file error or invalid algorithm), R3.3 (SIGPIPE handling).
package main

import (
	"bufio"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/blake2b"

	"github.com/petar-djukic/go-unix-utils/pkg/hashutil"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// crcPolynomial is the POSIX CRC-32 generator polynomial.
const crcPolynomial = 0x04C11DB7

// crcTable is the precomputed 256-entry lookup table for POSIX CRC-32.
//
// R1.1: CRC table for the POSIX cksum algorithm.
var crcTable = buildCRCTable()

// options holds all parsed command-line flags.
type options struct {
	algorithm string
	untagged  bool
	raw       bool
	files     []string
}

func main() {
	// R3.3: SIGPIPE handling.
	sys.InstallSIGPIPEHandler()

	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "cksum: %s\n", err)
		os.Exit(1)
	}

	if opts.algorithm == "crc" {
		os.Exit(runCRC(opts.files))
	}
	os.Exit(runHashAlgorithm(opts))
}

// parseArgs parses GNU-compatible flags from the argument list.
//
// R2.1: --algorithm flag. R2.2: --untagged. R2.3: --raw.
func parseArgs(args []string) (options, error) {
	opts := options{algorithm: "crc"}
	i := 0
	for i < len(args) {
		arg := args[i]
		i++
		switch {
		case strings.HasPrefix(arg, "--algorithm="):
			opts.algorithm = strings.TrimPrefix(arg, "--algorithm=")
		case arg == "--algorithm":
			if i >= len(args) {
				return options{}, fmt.Errorf("option '--algorithm' requires an argument")
			}
			opts.algorithm = args[i]
			i++
		case arg == "--untagged":
			opts.untagged = true
		case arg == "--raw":
			opts.raw = true
		case arg == "--":
			opts.files = append(opts.files, args[i:]...)
			i = len(args)
		default:
			opts.files = append(opts.files, arg)
		}
	}
	return opts, validateAlgorithm(opts.algorithm)
}

// validateAlgorithm checks that the algorithm name is recognized.
//
// R3.2: Exit 1 on invalid algorithm.
func validateAlgorithm(algo string) error {
	switch algo {
	case "crc", "sm3", "blake2b", "sha1", "sha224", "sha256", "sha384", "sha512":
		return nil
	default:
		return fmt.Errorf("unrecognized algorithm '%s'", algo)
	}
}

// runCRC executes CRC-32 checksum mode.
//
// R1.1-R1.4, R3.1: CRC mode processing.
func runCRC(files []string) int {
	if len(files) == 0 {
		return processCRCStdin()
	}
	return processCRCFiles(files)
}

// runHashAlgorithm executes a non-CRC hash algorithm.
//
// R2.1: Algorithm selection. R2.2: --untagged. R2.3: --raw.
func runHashAlgorithm(opts options) int {
	cfg := buildHashConfig(opts.algorithm)
	if opts.raw {
		return runRawMode(opts.files, cfg)
	}
	tag := !opts.untagged
	return hashutil.DigestFiles(opts.files, cfg, false, tag, os.Stdout, os.Stderr)
}

// buildHashConfig returns the HashConfig for the named algorithm.
//
// R2.1: Maps algorithm names to hash constructors and digest lengths.
func buildHashConfig(algo string) hashutil.HashConfig {
	switch algo {
	case "sha1":
		return hashutil.HashConfig{Algorithm: "SHA1", NewHash: sha1.New, DigestLen: 40}
	case "sha224":
		return hashutil.HashConfig{Algorithm: "SHA224", NewHash: sha256.New224, DigestLen: 56}
	case "sha256":
		return hashutil.HashConfig{Algorithm: "SHA256", NewHash: sha256.New, DigestLen: 64}
	case "sha384":
		return hashutil.HashConfig{Algorithm: "SHA384", NewHash: sha512.New384, DigestLen: 96}
	case "sha512":
		return hashutil.HashConfig{Algorithm: "SHA512", NewHash: sha512.New, DigestLen: 128}
	case "blake2b":
		return hashutil.HashConfig{
			Algorithm: "BLAKE2b",
			NewHash:   newBlake2b512,
			DigestLen: 128,
		}
	case "sm3":
		return hashutil.HashConfig{Algorithm: "SM3", NewHash: newSM3, DigestLen: 64}
	default:
		return hashutil.HashConfig{}
	}
}

// newBlake2b512 returns a new BLAKE2b-512 hash.
func newBlake2b512() hash.Hash {
	h, _ := blake2b.New512(nil) // nil key cannot fail
	return h
}

// runRawMode outputs raw binary digests for each file.
//
// R2.3: --raw outputs raw binary digest bytes.
func runRawMode(files []string, cfg hashutil.HashConfig) int {
	if len(files) == 0 {
		files = []string{"-"}
	}
	exitCode := 0
	for _, name := range files {
		if err := outputRawDigest(name, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "cksum: %s\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

// outputRawDigest computes the digest and writes raw binary bytes to stdout.
func outputRawDigest(name string, cfg hashutil.HashConfig) error {
	r, err := openInput(name)
	if err != nil {
		return fmt.Errorf("%s: %s", name, err)
	}
	if r != os.Stdin {
		defer r.Close() // best-effort close for opened files
	}
	digest, err := hashutil.ComputeDigest(r, cfg)
	if err != nil {
		return fmt.Errorf("%s: %s", name, err)
	}
	raw, err := hex.DecodeString(digest)
	if err != nil {
		return fmt.Errorf("decoding digest: %w", err)
	}
	_, err = os.Stdout.Write(raw)
	return err
}

// openInput opens the named file or returns stdin for "-".
func openInput(name string) (io.ReadCloser, error) {
	if name == "-" {
		return os.Stdin, nil
	}
	return os.Open(name)
}

// --- CRC-32 implementation ---

// buildCRCTable generates the POSIX CRC-32 lookup table.
func buildCRCTable() [256]uint32 {
	var table [256]uint32
	for i := range 256 {
		crc := uint32(i) << 24
		for range 8 {
			if crc&0x80000000 != 0 {
				crc = (crc << 1) ^ crcPolynomial
			} else {
				crc <<= 1
			}
		}
		table[i] = crc
	}
	return table
}

// processCRCStdin computes and prints the CRC for stdin.
//
// R1.2: Stdin output omits the filename.
func processCRCStdin() int {
	crc, size, err := computeCRC(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cksum: -: %s\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "%d %d\n", crc, size)
	return 0
}

// processCRCFiles computes and prints the CRC for each named file.
//
// R1.3: One line per file in argument order.
// R1.4: Exit 1 if any file fails, continue processing remaining.
func processCRCFiles(files []string) int {
	exitCode := 0
	w := bufio.NewWriter(os.Stdout)
	for _, name := range files {
		if err := processCRCOneFile(w, name); err != nil {
			fmt.Fprintf(os.Stderr, "cksum: %s: %s\n", name, err)
			exitCode = 1
		}
	}
	w.Flush() // best-effort flush
	return exitCode
}

// processCRCOneFile computes and prints the CRC line for a single file.
//
// R1.1: Output format "CHECKSUM BYTES FILENAME".
func processCRCOneFile(w *bufio.Writer, name string) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()

	crc, size, err := computeCRC(f)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "%d %d %s\n", crc, size, name)
	return nil
}

// computeCRC reads all bytes from r and returns the POSIX CRC-32 checksum
// and the byte count.
//
// R1.1: POSIX CRC-32 algorithm.
func computeCRC(r io.Reader) (uint32, int64, error) {
	var crc uint32
	var size int64
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		for i := range n {
			crc = (crc << 8) ^ crcTable[(crc>>24)^uint32(buf[i])]
		}
		size += int64(n)
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, err
		}
	}
	crc = foldLength(crc, size)
	crc = ^crc
	return crc, size, nil
}

// foldLength feeds the byte count into the CRC, byte by byte from MSB.
//
// R1.1: Length incorporated into CRC before final inversion.
func foldLength(crc uint32, size int64) uint32 {
	for s := size; s > 0; s >>= 8 {
		crc = (crc << 8) ^ crcTable[(crc>>24)^uint32(s&0xFF)]
	}
	return crc
}
