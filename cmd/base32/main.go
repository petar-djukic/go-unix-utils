// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/base32 encodes and decodes data in Base32 format.
// Implements: srd079 R1.1 (encoding from file/stdin), R1.2 (default 76-col wrap),
// R1.3 (--wrap flag), R1.4 (file open error), R2.1 (decode mode),
// R2.2 (whitespace tolerance), R2.3 (--ignore-garbage), R2.4 (invalid input error),
// R3.1-R3.3 (exit codes and SIGPIPE).
package main

import (
	"bytes"
	"encoding/base32"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/encutil"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	progName       = "base32"
	defaultWrapCol = 76
)

func main() {
	sys.InstallSIGPIPEHandler()

	wrap := flag.Int("w", defaultWrapCol, "wrap encoded lines after COLS characters (0 to disable)")
	flag.IntVar(wrap, "wrap", defaultWrapCol, "wrap encoded lines after COLS characters (0 to disable)")
	decode := flag.Bool("d", false, "decode data")
	flag.BoolVar(decode, "decode", false, "decode data")
	ignoreGarbage := flag.Bool("i", false, "when decoding, ignore non-alphabet characters")
	flag.BoolVar(ignoreGarbage, "ignore-garbage", false, "when decoding, ignore non-alphabet characters")

	flag.Usage = usage
	flag.Parse()

	filename := "-"
	if flag.NArg() > 0 {
		filename = flag.Arg(0)
	}

	if err := run(filename, *decode, *ignoreGarbage, *wrap); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		os.Exit(1)
	}
}

// run dispatches to encode or decode based on the decode flag.
func run(filename string, decode, ignoreGarbage bool, wrapCol int) error {
	if decode {
		return decodeInput(filename, ignoreGarbage)
	}
	return encode(filename, wrapCol)
}

// usage prints the help message to stderr.
func usage() {
	fmt.Fprintf(os.Stderr,
		"Usage: %s [OPTION]... [FILE]\n"+
			"Base32 encode or decode FILE, or standard input, to standard output.\n\n",
		progName)
	flag.PrintDefaults()
}

// encode reads from filename (or stdin for "-"), Base32-encodes the data,
// and writes wrapped output to stdout.
// R1.1: read from file or stdin; R1.2/R1.3: wrap control; R1.4: file errors.
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

// adjustOutput trims the trailing newline that encutil.Encode always appends
// in cases where GNU base32 omits it: empty input and -w 0 mode.
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

// decodeInput reads from filename (or stdin for "-"), decodes Base32 data,
// and writes the binary result to stdout.
// R2.1: decode mode; R2.2: whitespace tolerance; R2.3: ignore-garbage;
// R2.4: invalid input error.
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
// R1.1: delegates to encoding/base32.StdEncoding.
func base32Encode(data []byte) string {
	return base32.StdEncoding.EncodeToString(data)
}

// base32Decode decodes a Base32-encoded string, stripping spaces first.
// R2.2: spaces are ignored during decoding (in addition to \n and \r
// already stripped by encutil.Decode).
func base32Decode(s string) ([]byte, error) {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\t", "")
	return base32.StdEncoding.DecodeString(s)
}

// fileError unwraps os.PathError to produce GNU-compatible error messages
// of the form "filename: reason".
func fileError(filename string, err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return fmt.Errorf("%s: %v", filename, pe.Err)
	}
	return err
}
