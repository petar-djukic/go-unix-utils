// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd077-cksum R1.1-R1.4, R2.1-R2.2: Compute POSIX CRC-32
// checksums and byte counts for files or stdin, with --algorithm selection
// for modern hash algorithms (md5, sha1, sha224, sha256, sha384, sha512,
// blake2b) and --untagged/--check output control.
package main

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/blake2b"

	"github.com/petar-djukic/go-unix-utils/pkg/hashutil"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// crcPoly is the POSIX CRC-32 polynomial in normal (unreflected) form.
const crcPoly = 0x04C11DB7

// crcTable is the POSIX CRC-32 lookup table.
// R1.1: read-only table initialized once; cannot fail.
var crcTable [256]uint32

func init() {
	for i := range 256 {
		crc := uint32(i) << 24
		for range 8 {
			if crc&0x80000000 != 0 {
				crc = (crc << 1) ^ crcPoly
			} else {
				crc <<= 1
			}
		}
		crcTable[i] = crc
	}
}

// cksumOptions holds the parsed flag state for a cksum invocation.
type cksumOptions struct {
	algorithm string // --algorithm: digest algorithm (default "crc")
	untagged  bool   // --untagged: GNU-style output for non-CRC (R2.2)
	check     bool   // --check: verify checksums (non-CRC only)
	warn      bool   // --warn: warn on malformed lines
	quiet     bool   // --quiet: suppress OK lines
	status    bool   // --status: suppress all output
	strict    bool   // --strict: exit non-zero on malformed lines
}

func main() {
	// R3.3: Install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	opts, files := parseFlags(os.Args[1:])
	os.Exit(run(opts, files))
}

// run dispatches to CRC or hash mode based on the selected algorithm.
func run(opts cksumOptions, files []string) int {
	if opts.algorithm == "crc" {
		return runCRC(opts, files)
	}
	return runHash(opts, files)
}

// runCRC processes files in POSIX CRC-32 mode.
// R1.2: reads stdin when no file arguments are given.
// R1.3: processes multiple files in argument order.
func runCRC(opts cksumOptions, files []string) int {
	if opts.check {
		fmt.Fprintf(os.Stderr, "cksum: --check is not supported with --algorithm=crc\n")
		return 1
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	exitCode := 0
	for _, f := range files {
		if err := crcOneFile(f); err != nil {
			fmt.Fprintf(os.Stderr, "cksum: %v\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

// crcOneFile computes and prints the CRC for a single file or stdin.
// R1.1: output format is "CHECKSUM BYTES FILENAME".
// R1.2: stdin output omits the filename.
func crcOneFile(filename string) error {
	r, displayName, err := openInput(filename)
	if err != nil {
		return err
	}
	defer r.Close() // best-effort close on read-only file

	crc, length, err := posixCRC(r)
	if err != nil {
		return err
	}
	formatCRCOutput(crc, length, displayName)
	return nil
}

// formatCRCOutput prints a single CRC result line.
// R1.1: "CHECKSUM BYTES FILENAME" for files, "CHECKSUM BYTES" for stdin.
func formatCRCOutput(crc uint32, length int64, filename string) {
	if filename == "" {
		fmt.Printf("%d %d\n", crc, length)
	} else {
		fmt.Printf("%d %d %s\n", crc, length, filename)
	}
}

// openInput opens a file for reading, or returns stdin for "-".
// R1.2/R1.3: dash argument maps to stdin with empty display name.
func openInput(filename string) (io.ReadCloser, string, error) {
	if filename == "-" {
		return io.NopCloser(os.Stdin), "", nil
	}
	f, err := os.Open(filename)
	if err != nil {
		return nil, "", err
	}
	return f, filename, nil
}

// posixCRC computes the POSIX CRC-32 checksum and byte count from a reader.
// R1.1: uses the POSIX cksum algorithm with length appended to the CRC.
func posixCRC(r io.Reader) (uint32, int64, error) {
	var crc uint32
	var length int64
	buf := make([]byte, 8192)
	for {
		n, err := r.Read(buf)
		for _, b := range buf[:n] {
			crc = (crc << 8) ^ crcTable[(crc>>24)^uint32(b)]
		}
		length += int64(n)
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, fmt.Errorf("reading input: %w", err)
		}
	}
	crc = appendLengthToCRC(crc, length)
	crc = ^crc
	return crc, length, nil
}

// appendLengthToCRC feeds the file length bytes (LSB first) into the CRC.
func appendLengthToCRC(crc uint32, length int64) uint32 {
	for l := length; l > 0; l >>= 8 {
		crc = (crc << 8) ^ crcTable[(crc>>24)^uint32(l&0xff)]
	}
	return crc
}

// runHash processes files using a non-CRC hash algorithm.
// R2.1: delegates to hashutil for digest computation and formatting.
func runHash(opts cksumOptions, files []string) int {
	cfg, err := buildHashConfig(opts.algorithm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cksum: %v\n", err)
		return 1
	}
	if opts.check {
		return runHashCheck(opts, files, cfg)
	}
	tag := !opts.untagged
	return hashutil.DigestFiles(files, cfg, false, tag, os.Stdout, os.Stderr)
}

// runHashCheck verifies checksums from the given files.
// R2.2 (non-CRC): reads checksum file, prints OK/FAILED per entry.
func runHashCheck(opts cksumOptions, files []string, cfg hashutil.HashConfig) int {
	if len(files) == 0 {
		files = []string{"-"}
	}
	checkOpts := hashutil.CheckOptions{
		Warn:   opts.warn || opts.strict,
		Quiet:  opts.quiet,
		Status: opts.status,
	}
	exitCode := 0
	for _, f := range files {
		ok, err := hashutil.VerifyChecksums(
			f, cfg, checkOpts, os.Stdout, os.Stderr,
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cksum: %v\n", err)
			exitCode = 1
		} else if !ok {
			exitCode = 1
		}
	}
	return exitCode
}

// buildHashConfig returns a HashConfig for the named algorithm.
// R2.1: supported algorithms: md5, sha1, sha224, sha256, sha384, sha512, blake2b.
func buildHashConfig(algo string) (hashutil.HashConfig, error) {
	switch algo {
	case "md5":
		return hashutil.HashConfig{Algorithm: "MD5", NewHash: md5.New, DigestLen: 32}, nil
	case "sha1":
		return hashutil.HashConfig{Algorithm: "SHA1", NewHash: sha1.New, DigestLen: 40}, nil
	case "sha224":
		return hashutil.HashConfig{Algorithm: "SHA224", NewHash: sha256.New224, DigestLen: 56}, nil
	case "sha256":
		return hashutil.HashConfig{Algorithm: "SHA256", NewHash: sha256.New, DigestLen: 64}, nil
	case "sha384":
		return hashutil.HashConfig{Algorithm: "SHA384", NewHash: sha512.New384, DigestLen: 96}, nil
	case "sha512":
		return hashutil.HashConfig{Algorithm: "SHA512", NewHash: sha512.New, DigestLen: 128}, nil
	case "blake2b":
		return hashutil.HashConfig{
			Algorithm: "BLAKE2b",
			NewHash:   newBlake2bHash,
			DigestLen: 128,
		}, nil
	default:
		return hashutil.HashConfig{}, fmt.Errorf("unrecognized algorithm: %s", algo)
	}
}

// newBlake2bHash returns a new BLAKE2b-512 hash instance.
func newBlake2bHash() hash.Hash {
	h, _ := blake2b.New512(nil) // nil key; error only for invalid sizes
	return h
}

// parseFlags parses GNU cksum-compatible flags from the argument list.
// D5: uses manual parsing for long flag forms.
func parseFlags(args []string) (cksumOptions, []string) {
	opts := cksumOptions{algorithm: "crc"}
	var files []string
	endOfFlags := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if endOfFlags {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		consumed := handleLongFlag(arg, args, i, &opts)
		if consumed > 0 {
			i += consumed - 1
			continue
		}
		files = append(files, arg)
	}
	return opts, files
}

// handleLongFlag processes a long-form flag and returns the number
// of args consumed (0 if not handled).
func handleLongFlag(arg string, args []string, idx int, opts *cksumOptions) int {
	if !strings.HasPrefix(arg, "--") {
		return 0
	}
	if strings.HasPrefix(arg, "--algorithm") {
		return parseAlgorithmFlag(arg, args, idx, opts)
	}
	return handleSimpleLongFlag(arg, opts)
}

// handleSimpleLongFlag processes long flags that take no value.
func handleSimpleLongFlag(arg string, opts *cksumOptions) int {
	switch arg {
	case "--untagged":
		opts.untagged = true
	case "--check":
		opts.check = true
	case "--warn":
		opts.warn = true
	case "--quiet":
		opts.quiet = true
	case "--status":
		opts.status = true
	case "--strict":
		opts.strict = true
	case "--version":
		fmt.Printf("cksum (go-unix-utils) %s\n", version)
		os.Exit(0)
	case "--help":
		printHelp()
		os.Exit(0)
	default:
		return 0
	}
	return 1
}

// parseAlgorithmFlag parses --algorithm=ALG or --algorithm ALG.
// R2.1: validates at usage time in buildHashConfig.
func parseAlgorithmFlag(arg string, args []string, idx int, opts *cksumOptions) int {
	var val string
	consumed := 1
	if strings.Contains(arg, "=") {
		val = arg[strings.Index(arg, "=")+1:]
	} else if idx+1 < len(args) {
		val = args[idx+1]
		consumed = 2
	} else {
		fmt.Fprintf(os.Stderr, "cksum: option '--algorithm' requires an argument\n")
		os.Exit(1)
	}
	opts.algorithm = val
	return consumed
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Print(`Usage: cksum [OPTION]... [FILE]...
Print CRC checksum and byte counts of each FILE.

With no FILE, or when FILE is -, read standard input.

      --algorithm=ALG  select the digest algorithm (crc, md5, sha1, sha224,
                         sha256, sha384, sha512, blake2b)
      --check          read checksums from the FILEs and check them
      --untagged       create a reversed style checksum, without digest type
      --warn           warn about improperly formatted checksum lines
      --quiet          don't print OK for each successfully verified file
      --status         don't output anything, status code shows success
      --strict         exit non-zero for improperly formatted checksum lines
      --help           display this help and exit
      --version        output version information and exit
`)
}
