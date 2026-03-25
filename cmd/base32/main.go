// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd079-base32 R1.1–R1.4, R2.1–R2.4, R3.1–R3.3: Base32 encode
// and decode from file or stdin with configurable line wrapping, decode mode,
// ignore-garbage support, --help, and --version using pkg/encutil for shared
// encode/decode logic and encoding/base32 from the Go standard library.
package main

import (
	"bytes"
	"encoding/base32"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/petar-djukic/go-unix-utils/pkg/encutil"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// defaultWrapCol is the default wrap column for encoded output.
// R1.2: wrap at 76 characters per line by default.
const defaultWrapCol = 76

// progName is the binary name used in error messages.
const progName = "base32"

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// options holds the parsed command-line options.
type options struct {
	wrapCol       int
	filename      string
	decode        bool
	ignoreGarbage bool
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts := options{wrapCol: defaultWrapCol}
	exitCode := parseArgs(os.Args[1:], &opts)
	if exitCode >= 0 {
		os.Exit(exitCode)
	}

	if err := run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, formatError(err))
		os.Exit(1)
	}
}

// parseArgs processes command-line arguments, populating opts.
// Returns -1 if parsing succeeds, or an exit code >= 0 to exit immediately.
func parseArgs(args []string, opts *options) int {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			if i+1 < len(args) {
				opts.filename = args[i+1]
			}
			return -1
		case arg == "--version":
			printVersion()
			return 0
		case arg == "--help":
			printHelp()
			return 0
		case arg == "-d" || arg == "--decode":
			opts.decode = true
		case arg == "-i" || arg == "--ignore-garbage":
			opts.ignoreGarbage = true
		case arg == "-w" || arg == "--wrap":
			i++
			if i >= len(args) {
				printOptionRequiresArg(arg)
				return 1
			}
			if code := parseWrapArg(args[i], &opts.wrapCol); code >= 0 {
				return code
			}
		case len(arg) > 2 && arg[:2] == "-w":
			if code := parseWrapArg(arg[2:], &opts.wrapCol); code >= 0 {
				return code
			}
		case len(arg) > 7 && arg[:7] == "--wrap=":
			if code := parseWrapArg(arg[7:], &opts.wrapCol); code >= 0 {
				return code
			}
		case arg == "-":
			opts.filename = arg
		case isDashArg(arg):
			if code := parseShortFlags(arg[1:], opts); code >= 0 {
				return code
			}
		default:
			opts.filename = arg
		}
	}
	return -1
}

// isDashArg returns true if the argument starts with "-" and has length > 1.
func isDashArg(arg string) bool {
	return len(arg) > 1 && arg[0] == '-'
}

// parseShortFlags handles combined short flags like "-di".
func parseShortFlags(flags string, opts *options) int {
	for _, ch := range flags {
		switch ch {
		case 'd':
			opts.decode = true
		case 'i':
			opts.ignoreGarbage = true
		default:
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '-%c'\n", progName, ch)
			return 1
		}
	}
	return -1
}

// printOptionRequiresArg prints error for missing option argument.
func printOptionRequiresArg(opt string) {
	fmt.Fprintf(os.Stderr, "%s: option '%s' requires an argument\n", progName, opt)
}

// parseWrapArg parses a wrap column value. Returns -1 on success, or an exit
// code >= 0 on failure.
func parseWrapArg(s string, wrapCol *int) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		fmt.Fprintf(os.Stderr, "%s: invalid wrap size: '%s'\n", progName, s)
		return 1
	}
	*wrapCol = n
	return -1
}

// run performs the base32 encoding or decoding operation.
func run(opts options) error {
	if opts.decode {
		return runDecode(opts)
	}
	return runEncode(opts)
}

// runEncode encodes file or stdin to base32.
// R1.1: reads from file or stdin, encodes to base32.
// R2.2: uses pkg/encutil.Encode with EncoderConfig.
func runEncode(opts options) error {
	input, err := encutil.OpenInput(opts.filename)
	if err != nil {
		return err
	}
	defer input.Close()

	cfg := encutil.EncoderConfig{
		Encode:  base32.StdEncoding.EncodeToString,
		WrapCol: opts.wrapCol,
	}

	var buf bytes.Buffer
	if err := encutil.Encode(input, &buf, cfg); err != nil {
		return err
	}

	output := buf.Bytes()
	output = adjustOutput(output, opts.wrapCol)
	if len(output) == 0 {
		return nil
	}
	_, err = os.Stdout.Write(output)
	return err
}

// runDecode decodes base32 input from file or stdin.
// R2.1: -d decodes Base32 input.
// R2.3: -i ignores non-alphabet characters during decode.
// R2.4/R3.2: invalid input produces error on stderr, exits 1.
func runDecode(opts options) error {
	input, err := encutil.OpenInput(opts.filename)
	if err != nil {
		return err
	}
	defer input.Close()

	cfg := encutil.DecoderConfig{
		Decode:        base32.StdEncoding.DecodeString,
		IgnoreGarbage: opts.ignoreGarbage,
	}

	if err := encutil.Decode(input, os.Stdout, cfg); err != nil {
		return fmt.Errorf("invalid input")
	}
	return nil
}

// formatError converts Go os.PathError into GNU-style error messages.
// GNU format: "filename: Error message" (no "open" prefix, capitalized).
func formatError(err error) string {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return fmt.Sprintf("%s: %s", pathErr.Path, sentenceCase(pathErr.Err.Error()))
	}
	return err.Error()
}

// sentenceCase capitalizes the first letter of s.
func sentenceCase(s string) string {
	if len(s) == 0 {
		return s
	}
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}
	return s
}

// adjustOutput trims encutil output to match GNU base32 behavior.
// GNU base32: empty input produces no output; -w0 omits trailing newline.
func adjustOutput(output []byte, wrapCol int) []byte {
	// R1.2: GNU base32 produces no output for empty input.
	if bytes.Equal(output, []byte("\n")) {
		return nil
	}
	// GNU base32 -w0: no trailing newline.
	if wrapCol == 0 {
		return bytes.TrimSuffix(output, []byte("\n"))
	}
	return output
}

// printVersion writes version information to stdout matching GNU format.
// R3.2: --version prints version info and exits 0.
func printVersion() {
	fmt.Printf("%s (go-unix-utils) %s\n", progName, version)
}

// printHelp writes usage information to stdout matching GNU format.
// R3.3: --help prints usage and exits 0.
func printHelp() {
	fmt.Printf(`Usage: %s [OPTION]... [FILE]
Base32 encode or decode FILE, or standard input, to standard output.

With no FILE, or when FILE is -, read standard input.

  -d, --decode          decode data
  -i, --ignore-garbage  when decoding, ignore non-alphabet characters
  -w, --wrap=COLS       wrap encoded lines after COLS character (default 76).
                          Use 0 to disable line wrapping

      --help     display this help and exit
      --version  output version information and exit
`, progName)
}
