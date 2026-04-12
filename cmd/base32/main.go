// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/base32 encodes and decodes data in Base32 format.
// Implements: srd079 R1.1-R1.4 (encoding), R2.1-R2.4 (decoding),
// R3.1-R3.3 (exit codes, error reporting, and SIGPIPE).
package main

import (
	"bytes"
	"encoding/base32"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/encutil"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	progName       = "base32"
	defaultWrapCol = 76
	versionStr     = "base32 (go-unix-utils) 0.1"
)

func main() {
	sys.InstallSIGPIPEHandler()
	if code := runMain(); code != 0 {
		os.Exit(code)
	}
}

// runMain parses flags and dispatches to encode or decode.
// R3.1: exits 0 on success. R3.2: exits 1 on any error.
func runMain() int {
	if handleSpecialFlags() {
		return 0
	}

	fs := flag.NewFlagSet(progName, flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	wrap, decode, ignoreGarbage := defineFlags(fs)

	if err := fs.Parse(os.Args[1:]); err != nil {
		return handleFlagError(err)
	}

	filename := "-"
	if fs.NArg() > 0 {
		filename = fs.Arg(0)
	}

	if err := run(filename, *decode, *ignoreGarbage, *wrap); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 1
	}
	return 0
}

// handleSpecialFlags checks for --help and --version before flag parsing.
// R3.1: GNU base32 prints help/version to stdout and exits 0.
func handleSpecialFlags() bool {
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--help":
			printHelp(os.Stdout)
			return true
		case "--version":
			fmt.Println(versionStr)
			return true
		case "--":
			return false
		}
	}
	return false
}

// defineFlags registers all command-line flags on fs.
func defineFlags(fs *flag.FlagSet) (wrap *int, decode, ignoreGarbage *bool) {
	wrap = fs.Int("w", defaultWrapCol, "")
	fs.IntVar(wrap, "wrap", defaultWrapCol, "")
	decode = fs.Bool("d", false, "")
	fs.BoolVar(decode, "decode", false, "")
	ignoreGarbage = fs.Bool("i", false, "")
	fs.BoolVar(ignoreGarbage, "ignore-garbage", false, "")
	return wrap, decode, ignoreGarbage
}

// handleFlagError prints an error for invalid flags and returns exit code.
// R3.2: exits 1 on invalid option.
func handleFlagError(err error) int {
	if errors.Is(err, flag.ErrHelp) {
		printHelp(os.Stdout)
		return 0
	}
	fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
	return 1
}

// printHelp writes usage information to w.
func printHelp(w io.Writer) {
	fmt.Fprintf(w,
		"Usage: %s [OPTION]... [FILE]\n"+
			"Base32 encode or decode FILE, or standard input, to standard output.\n\n"+
			"With no FILE, or when FILE is -, read standard input.\n\n"+
			"  -d, --decode          decode data\n"+
			"  -i, --ignore-garbage  when decoding, ignore non-alphabet characters\n"+
			"  -w, --wrap=COLS       wrap encoded lines after COLS character (default 76).\n"+
			"                          Use 0 to disable line wrapping\n\n"+
			"      --help        display this help and exit\n"+
			"      --version     output version information and exit\n",
		progName)
}

// run dispatches to encode or decode based on the decode flag.
func run(filename string, decode, ignoreGarbage bool, wrapCol int) error {
	if decode {
		return decodeInput(filename, ignoreGarbage)
	}
	return encode(filename, wrapCol)
}

// encode reads from filename (or stdin for "-"), Base32-encodes the data,
// and writes wrapped output to stdout.
// R1.1-R1.4: encode pipeline.
func encode(filename string, wrapCol int) error {
	rc, err := encutil.OpenInput(filename)
	if err != nil {
		return fileError(filename, err)
	}
	defer rc.Close()

	var buf bytes.Buffer
	cfg := encutil.EncoderConfig{
		Encode:  base32Encode,
		WrapCol: wrapCol,
	}
	if err := encutil.Encode(rc, &buf, cfg); err != nil {
		return err
	}

	out := adjustOutput(buf.Bytes(), wrapCol)
	if len(out) == 0 {
		return nil
	}
	_, err = os.Stdout.Write(out)
	return err
}

// adjustOutput trims trailing newlines for GNU compatibility.
func adjustOutput(out []byte, wrapCol int) []byte {
	// GNU base32 produces no output for empty input.
	if len(out) == 1 && out[0] == '\n' {
		return nil
	}
	// GNU base32 -w 0 omits the trailing newline.
	if wrapCol == 0 && len(out) > 0 && out[len(out)-1] == '\n' {
		return out[:len(out)-1]
	}
	return out
}

// decodeInput reads, decodes Base32, and writes binary to stdout.
// R2.1-R2.4: decode pipeline with error reporting.
func decodeInput(filename string, ignoreGarbage bool) error {
	rc, err := encutil.OpenInput(filename)
	if err != nil {
		return fileError(filename, err)
	}
	defer rc.Close()

	cfg := encutil.DecoderConfig{
		Decode:        base32Decode,
		IgnoreGarbage: ignoreGarbage,
	}
	if err := encutil.Decode(rc, os.Stdout, cfg); err != nil {
		return fmt.Errorf("invalid input")
	}
	return nil
}

// base32Encode encodes data using RFC 4648 Base32 standard alphabet.
func base32Encode(data []byte) string {
	return base32.StdEncoding.EncodeToString(data)
}

// base32Decode decodes a Base32-encoded string, stripping whitespace.
// R2.2: spaces and tabs are ignored during decoding.
func base32Decode(s string) ([]byte, error) {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\t", "")
	return base32.StdEncoding.DecodeString(s)
}

// fileError unwraps os.PathError for GNU-compatible error messages.
// R3.2: produces "filename: reason" format.
func fileError(filename string, err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return fmt.Errorf("%s: %v", filename, pe.Err)
	}
	return err
}
