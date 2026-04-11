// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/cksum: print CRC checksum and byte counts.
// Implements srd077-cksum R1.1 (POSIX CRC-32 default), R1.2 (stdin),
// R1.3 (multiple files), R1.4 (error handling and --help/--version).
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "cksum"

// usageText is the --help output printed to stdout.
// R1.4: --help prints usage to stdout and exits 0.
const usageText = `Usage: cksum [FILE]...
  or:  cksum [OPTION]
Print CRC checksum and byte counts of each FILE.

      --help     display this help and exit
      --version  output version information and exit
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

// config holds parsed command-line options for cksum.
type config struct {
	help    bool
	version bool
	files   []string
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

	files := cfg.files
	if len(files) == 0 {
		files = []string{"-"}
	}

	exitCode := 0
	for _, name := range files {
		if err := processFile(name); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
			exitCode = 1
		}
	}
	return exitCode
}

// processFile computes and prints the CRC for a single file or stdin.
// R1.1: output format "CHECKSUM BYTES FILENAME".
func processFile(name string) error {
	crc, size, err := computeCRC(name)
	if err != nil {
		return err
	}
	printResult(crc, size, name)
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

// printResult writes the CRC output line.
// R1.1: format is "CHECKSUM BYTES FILENAME".
// R1.2: stdin omits the filename.
func printResult(crc uint32, size int64, name string) {
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

	for _, arg := range args {
		if flagsDone || !strings.HasPrefix(arg, "-") || arg == "-" {
			cfg.files = append(cfg.files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if err := parseLongFlag(&cfg, arg); err != nil {
			return config{}, err
		}
	}
	return cfg, nil
}

// parseLongFlag handles --name flags.
func parseLongFlag(cfg *config, arg string) error {
	switch arg {
	case "--help":
		cfg.help = true
	case "--version":
		cfg.version = true
	default:
		return fmt.Errorf("unrecognized option '%s'", arg)
	}
	return nil
}
