// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/base32 encodes data to Base32 format.
// Implements: srd079 R1.1 (encoding from file/stdin), R1.2 (default 76-col wrap),
// R1.3 (--wrap flag), R1.4 (file open error), R3.3 (SIGPIPE).
package main

import (
	"bytes"
	"encoding/base32"
	"errors"
	"flag"
	"fmt"
	"os"

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

	flag.Usage = usage
	flag.Parse()

	filename := "-"
	if flag.NArg() > 0 {
		filename = flag.Arg(0)
	}

	if err := encode(filename, *wrap); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		os.Exit(1)
	}
}

// usage prints the help message to stderr.
func usage() {
	fmt.Fprintf(os.Stderr,
		"Usage: %s [OPTION]... [FILE]\n"+
			"Base32 encode FILE, or standard input, to standard output.\n\n",
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

// base32Encode encodes data using RFC 4648 Base32 standard alphabet.
// R1.1: delegates to encoding/base32.StdEncoding.
func base32Encode(data []byte) string {
	return base32.StdEncoding.EncodeToString(data)
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
