// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/sum: print checksum and block counts.
// Implements srd078-sum R1.1-R1.4 (BSD default checksum, stdin, multi-file,
// error handling), R2.1-R2.2 (algorithm selection), R3.1-R3.3 (exit codes,
// SIGPIPE).
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "sum"

// usageText is the --help output printed to stdout.
const usageText = `Usage: sum [OPTION]... [FILE]...
  or:  sum [OPTION]
Print checksum and block counts for each FILE.

With no FILE, or when FILE is -, read standard input.

  -r             use BSD sum algorithm (default)
  -s, --sysv     use System V sum algorithm
      --help     display this help and exit
      --version  output version information and exit
`

// versionText is the --version output printed to stdout.
const versionText = "sum (go-unix-utils) 0.1.0\n"

// Block sizes for each algorithm.
const (
	bsdBlockSize  = 1024
	sysvBlockSize = 512
)

// config holds parsed command-line options for sum.
type config struct {
	help    bool
	version bool
	sysv    bool // -s or --sysv selects System V algorithm
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

// run executes sum logic and returns the exit code.
func run(cfg config) int {
	if cfg.help {
		fmt.Fprint(os.Stdout, usageText)
		return 0
	}
	if cfg.version {
		fmt.Fprint(os.Stdout, versionText)
		return 0
	}
	return processFiles(cfg)
}

// processFiles iterates over file arguments and computes checksums.
// R1.3: processes multiple files in argument order.
// R1.4: continues on error, sets exit code 1.
func processFiles(cfg config) int {
	files := cfg.files
	if len(files) == 0 {
		files = []string{"-"}
	}
	exitCode := 0
	for _, name := range files {
		if err := processFile(name, cfg.sysv); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", programName, formatFileError(err))
			exitCode = 1
		}
	}
	return exitCode
}

// processFile opens a file (or stdin), computes the checksum, and prints it.
// R1.2: reads from stdin when name is "-".
func processFile(name string, sysv bool) error {
	r, closeFn, err := openInput(name)
	if err != nil {
		return err
	}
	defer closeFn()

	if sysv {
		return computeAndPrintSysV(r, name)
	}
	return computeAndPrintBSD(r, name)
}

// openInput opens a file for reading, or returns stdin for "-".
func openInput(name string) (io.Reader, func(), error) {
	if name == "-" {
		return os.Stdin, func() {}, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil // best-effort close on read-only file
}

// computeAndPrintBSD computes and prints the BSD checksum.
// R1.1, R2.1: 16-bit rotating checksum with 1024-byte blocks.
func computeAndPrintBSD(r io.Reader, name string) error {
	checksum, totalBytes, err := computeBSD(r)
	if err != nil {
		return err
	}
	blocks := (totalBytes + bsdBlockSize - 1) / bsdBlockSize
	printResult("%05d %5d", checksum, blocks, name)
	return nil
}

// computeBSD computes the BSD rotating checksum from a reader.
func computeBSD(r io.Reader) (uint16, int64, error) {
	var checksum uint32
	var totalBytes int64
	buf := make([]byte, 32*1024)

	for {
		n, err := r.Read(buf)
		for i := range n {
			// R2.1: rotate right 1 bit, then add byte
			if checksum&1 != 0 {
				checksum = (checksum >> 1) + 0x8000
			} else {
				checksum >>= 1
			}
			checksum = (checksum + uint32(buf[i])) & 0xFFFF
		}
		totalBytes += int64(n)
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, fmt.Errorf("reading input: %w", err)
		}
	}
	return uint16(checksum), totalBytes, nil
}

// computeAndPrintSysV computes and prints the System V checksum.
// R2.2: 16-bit sum with 512-byte blocks.
func computeAndPrintSysV(r io.Reader, name string) error {
	checksum, totalBytes, err := computeSysV(r)
	if err != nil {
		return err
	}
	blocks := (totalBytes + sysvBlockSize - 1) / sysvBlockSize
	printResult("%d %d", checksum, blocks, name)
	return nil
}

// computeSysV computes the System V checksum from a reader.
func computeSysV(r io.Reader) (uint16, int64, error) {
	var sum uint32
	var totalBytes int64
	buf := make([]byte, 32*1024)

	for {
		n, err := r.Read(buf)
		for i := range n {
			sum += uint32(buf[i])
		}
		totalBytes += int64(n)
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, fmt.Errorf("reading input: %w", err)
		}
	}
	// R2.2: fold 32-bit sum into 16-bit
	sum = (sum & 0xFFFF) + (sum >> 16)
	sum = (sum & 0xFFFF) + (sum >> 16)
	return uint16(sum), totalBytes, nil
}

// printResult formats and prints a checksum output line.
// Stdin ("-") omits the filename.
func printResult(format string, checksum uint16, blocks int64, name string) {
	if name == "-" {
		fmt.Printf(format+"\n", checksum, blocks)
	} else {
		fmt.Printf(format+" %s\n", checksum, blocks, name)
	}
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

// parseArgs parses command-line arguments into config.
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
		if err := parseFlag(&cfg, arg); err != nil {
			return config{}, err
		}
	}
	return cfg, nil
}

// parseFlag handles a single flag argument.
func parseFlag(cfg *config, arg string) error {
	switch arg {
	case "-r":
		// R2.1: -r selects BSD (already default), explicit no-op
	case "-s", "--sysv":
		cfg.sysv = true
	case "--help":
		cfg.help = true
	case "--version":
		cfg.version = true
	default:
		return fmt.Errorf("unrecognized option '%s'", arg)
	}
	return nil
}
