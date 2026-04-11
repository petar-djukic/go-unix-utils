// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/cksum: print CRC checksum and byte counts.
// Implements srd077-cksum R1.1-R1.4 (POSIX CRC-32 default), R2.1-R2.2
// (algorithm selection, output format), R3.1-R3.3 (exit codes, SIGPIPE).
package main

import (
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/blake2b"

	"github.com/petar-djukic/go-unix-utils/pkg/hashutil"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "cksum"

// Algorithm name constants for --algorithm flag values.
const (
	algCRC     = "crc"
	algSHA1    = "sha1"
	algSHA224  = "sha224"
	algSHA256  = "sha256"
	algSHA384  = "sha384"
	algSHA512  = "sha512"
	algBlake2b = "blake2b"
)

// usageText is the --help output printed to stdout.
// R1.4: --help prints usage to stdout and exits 0.
const usageText = `Usage: cksum [OPTION]... [FILE]...
  or:  cksum [OPTION]
Print CRC checksum and byte counts of each FILE.

With no FILE, or when FILE is -, read standard input.

      --algorithm=TYPE  select the digest algorithm (default: crc)
                         Supported: crc, blake2b, sha1, sha224, sha256, sha384, sha512
      --check          read checksums from the FILEs and check them
      --tag            create a BSD-style checksum
      --untagged       create a reversed style checksum, without digest type
      --warn           warn about improperly formatted checksum lines
      --quiet          don't print OK for each successfully verified file
      --status         don't output anything, status code shows success
      --help           display this help and exit
      --version        output version information and exit
`

// versionText is the --version output printed to stdout.
// R1.4: --version prints version info to stdout and exits 0.
const versionText = "cksum (go-unix-utils) 0.1.0\n"

// crcPoly is the POSIX CRC-32 generator polynomial (ISO 3309).
const crcPoly = 0x04C11DB7

// crcTable is the POSIX CRC-32 lookup table built from crcPoly.
// Initialized once at startup; read-only thereafter.
var crcTable [256]uint32

func init() {
	// Package-level read-only lookup table that cannot fail.
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

// algorithmEntry holds the tag name and hash factory for a modern algorithm.
// R2.1: each non-CRC algorithm maps to a hashutil-compatible configuration.
type algorithmEntry struct {
	tagName   string
	newHash   func() hash.Hash
	digestLen int // expected hex character count
}

// modernAlgorithms maps --algorithm flag values to their configurations.
// R2.1: supported non-CRC algorithms.
var modernAlgorithms = map[string]algorithmEntry{
	algSHA1:    {tagName: "SHA1", newHash: sha1.New, digestLen: 40},
	algSHA224:  {tagName: "SHA224", newHash: sha256.New224, digestLen: 56},
	algSHA256:  {tagName: "SHA256", newHash: sha256.New, digestLen: 64},
	algSHA384:  {tagName: "SHA384", newHash: sha512.New384, digestLen: 96},
	algSHA512:  {tagName: "SHA512", newHash: sha512.New, digestLen: 128},
	algBlake2b: {tagName: "BLAKE2b", newHash: newBlake2b512, digestLen: 128},
}

// newBlake2b512 returns a BLAKE2b-512 hash.Hash instance.
func newBlake2b512() hash.Hash {
	h, _ := blake2b.New(64, nil) // nil key for unkeyed hash; cannot fail for size 64
	return h
}

// config holds parsed command-line options for cksum.
type config struct {
	help      bool
	version   bool
	algorithm string // --algorithm=ALG; "" or "crc" means default CRC-32
	untagged  bool   // --untagged: GNU two-space format for non-CRC
	tag       bool   // --tag: BSD tag format
	check     bool   // --check: verify checksums from file
	warn      bool   // --warn: warn about malformed lines in --check
	quiet     bool   // --quiet: suppress OK lines in --check
	status    bool   // --status: suppress all output in --check
	files     []string
}

// isCRC returns true when the configured algorithm is the default POSIX CRC-32.
func (c config) isCRC() bool {
	return c.algorithm == "" || c.algorithm == algCRC
}

// R1.1: main entry with SIGPIPE handler and flag parsing.
func main() {
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		os.Exit(1)
	}

	os.Exit(run(cfg))
}

// run executes cksum logic and returns the exit code.
// R1.4: --help and --version print to stdout and exit 0.
func run(cfg config) int {
	if cfg.help {
		fmt.Fprint(os.Stdout, usageText)
		return 0
	}
	if cfg.version {
		fmt.Fprint(os.Stdout, versionText)
		return 0
	}
	if cfg.isCRC() {
		return runCRC(cfg)
	}
	return runModern(cfg)
}

// runCRC processes files using the POSIX CRC-32 algorithm.
// R1.1: default CRC-32 mode with "CHECKSUM BYTES FILENAME" output.
// R3.2: reports file errors to stderr and sets exit code 1.
func runCRC(cfg config) int {
	files := cfg.files
	if len(files) == 0 {
		files = []string{"-"}
	}
	exitCode := 0
	for _, name := range files {
		if err := processFileCRC(name); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", programName, formatFileError(err))
			exitCode = 1
		}
	}
	return exitCode
}

// formatFileError produces a GNU-style diagnostic from a file operation error.
// R3.2: strips Go's "open" wrapper from *os.PathError for "NAME: error" format.
func formatFileError(err error) string {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return fmt.Sprintf("%s: %s", pathErr.Path, pathErr.Err)
	}
	return err.Error()
}

// runModern processes files using a non-CRC hash algorithm via hashutil.
// R2.1: dispatches to hashutil for digest computation and formatting.
func runModern(cfg config) int {
	hcfg, err := buildHashConfig(cfg.algorithm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		return 1
	}
	if cfg.check {
		return runCheck(cfg, hcfg)
	}
	// R2.1: non-CRC defaults to tagged output; --untagged switches to GNU format.
	tag := !cfg.untagged || cfg.tag
	return hashutil.DigestFiles(cfg.files, hcfg, false, tag, os.Stdout, os.Stderr)
}

// buildHashConfig constructs a HashConfig for the named algorithm.
func buildHashConfig(algorithm string) (hashutil.HashConfig, error) {
	entry, ok := modernAlgorithms[algorithm]
	if !ok {
		return hashutil.HashConfig{}, fmt.Errorf("unrecognized algorithm '%s'", algorithm)
	}
	return hashutil.HashConfig{
		Algorithm: entry.tagName,
		NewHash:   entry.newHash,
		DigestLen: entry.digestLen,
	}, nil
}

// runCheck verifies checksums from each file argument.
// R2.3: --check mode with --warn, --quiet, --status modifiers.
func runCheck(cfg config, hcfg hashutil.HashConfig) int {
	opts := hashutil.CheckOptions{
		Warn:   cfg.warn,
		Quiet:  cfg.quiet,
		Status: cfg.status,
	}
	allOK := true
	for _, f := range cfg.files {
		ok, err := hashutil.VerifyChecksums(f, hcfg, opts, os.Stdout, os.Stderr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
			allOK = false
			continue
		}
		if !ok {
			allOK = false
		}
	}
	if !allOK {
		return 1
	}
	return 0
}

// processFileCRC computes and prints the CRC for a single file or stdin.
// R1.1: output format "CHECKSUM BYTES FILENAME".
func processFileCRC(name string) error {
	crc, size, err := computeCRC(name)
	if err != nil {
		return err
	}
	printCRCResult(crc, size, name)
	return nil
}

// computeCRC opens a file (or stdin for "-") and computes its POSIX CRC-32.
// R1.2: reads from stdin when the name is "-".
func computeCRC(name string) (uint32, int64, error) {
	if name == "-" {
		return computeCRCReader(os.Stdin)
	}
	f, err := os.Open(name)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close() // best-effort close on read-only file
	return computeCRCReader(f)
}

// computeCRCReader computes the POSIX CRC-32 checksum from a reader.
// The algorithm processes all data bytes, then folds in the byte count,
// and finally complements the result.
func computeCRCReader(r io.Reader) (uint32, int64, error) {
	var crc uint32
	var size int64
	buf := make([]byte, 32*1024)

	for {
		n, err := r.Read(buf)
		for i := range n {
			crc = (crc << 8) ^ crcTable[byte(crc>>24)^buf[i]]
		}
		size += int64(n)
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, fmt.Errorf("reading input: %w", err)
		}
	}

	crc = foldLength(crc, size)
	crc = ^crc
	return crc, size, nil
}

// foldLength folds the file byte count into the CRC, one byte at a time
// from least-significant to most-significant.
func foldLength(crc uint32, length int64) uint32 {
	for length > 0 {
		crc = (crc << 8) ^ crcTable[byte(crc>>24)^byte(length&0xFF)]
		length >>= 8
	}
	return crc
}

// printCRCResult writes the CRC output line.
// R1.1: format is "CHECKSUM BYTES FILENAME".
// R1.2: stdin omits the filename.
func printCRCResult(crc uint32, size int64, name string) {
	if name == "-" {
		fmt.Printf("%d %d\n", crc, size)
	} else {
		fmt.Printf("%d %d %s\n", crc, size, name)
	}
}

// parseArgs parses command-line arguments into config.
// R1.4: supports --help and --version.
func parseArgs(args []string) (config, error) {
	cfg := config{}
	flagsDone := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || !strings.HasPrefix(arg, "-") || arg == "-" {
			cfg.files = append(cfg.files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		skip, err := parseLongFlag(&cfg, args, i)
		if err != nil {
			return config{}, err
		}
		i += skip
	}
	if cfg.help || cfg.version {
		return cfg, nil
	}
	return cfg, validateConfig(cfg)
}

// parseLongFlag handles --name and --name=value flags.
func parseLongFlag(cfg *config, args []string, idx int) (int, error) {
	arg := args[idx]
	if arg == "--algorithm" || strings.HasPrefix(arg, "--algorithm=") {
		return parseAlgorithmFlag(cfg, arg, args, idx)
	}
	return parseSimpleFlag(cfg, arg)
}

// parseSimpleFlag handles boolean --name flags.
func parseSimpleFlag(cfg *config, arg string) (int, error) {
	switch arg {
	case "--untagged":
		cfg.untagged = true
	case "--tag":
		cfg.tag = true
	case "--check":
		cfg.check = true
	case "--warn":
		cfg.warn = true
	case "--quiet":
		cfg.quiet = true
	case "--status":
		cfg.status = true
	case "--help":
		cfg.help = true
	case "--version":
		cfg.version = true
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", arg)
	}
	return 0, nil
}

// parseAlgorithmFlag parses --algorithm=ALG or --algorithm ALG.
// R2.1: normalizes the algorithm name to lowercase.
func parseAlgorithmFlag(cfg *config, arg string, args []string, idx int) (int, error) {
	var val string
	var skip int
	if strings.Contains(arg, "=") {
		val = arg[strings.Index(arg, "=")+1:]
	} else {
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("option '--algorithm' requires an argument")
		}
		val = args[idx+1]
		skip = 1
	}
	cfg.algorithm = strings.ToLower(val)
	return skip, nil
}

// validateConfig checks for invalid flag combinations.
// R3.2: rejects invalid combinations with exit code 1.
func validateConfig(cfg config) error {
	if cfg.check && cfg.isCRC() {
		return fmt.Errorf("--check is not supported with the default CRC algorithm")
	}
	if cfg.check && len(cfg.files) == 0 {
		return fmt.Errorf("--check requires a file argument")
	}
	if cfg.check && cfg.tag {
		return fmt.Errorf("the --tag and --check options are mutually exclusive")
	}
	if cfg.untagged && cfg.isCRC() {
		return fmt.Errorf("--untagged is not supported with the default CRC algorithm")
	}
	return nil
}
