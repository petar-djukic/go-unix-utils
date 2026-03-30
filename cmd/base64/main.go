// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/base64 encodes and decodes Base64 data per RFC 4648.
//
// Implements prd080-base64: R1.1 (encode stdin/file), R1.2 (default wrap at 76),
// R1.3 (-w COLS wrap control), R1.4 (exit 1 on file open error),
// R2.1 (-d decode), R2.2 (ignore whitespace), R2.3 (-i ignore garbage),
// R2.4 (exit 1 on invalid input), R3.1-R3.3 (exit codes and SIGPIPE).
package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/petar-djukic/go-unix-utils/pkg/encutil"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	progName       = "base64"
	defaultWrapCol = 76
)

func main() {
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		os.Exit(1)
	}

	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		os.Exit(1)
	}
}

type config struct {
	decode        bool
	ignoreGarbage bool
	wrapCol       int
	file          string
}

// parseArgs parses command-line arguments into a config struct.
// R1.3: -w COLS / --wrap=COLS sets wrap column.
func parseArgs(args []string) (config, error) {
	c := config{wrapCol: defaultWrapCol}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-d" || arg == "--decode":
			c.decode = true
		case arg == "-i" || arg == "--ignore-garbage":
			c.ignoreGarbage = true
		case arg == "-w":
			v, err := nextArg(args, &i)
			if err != nil {
				return c, err
			}
			c.wrapCol, err = parseWrapCol(v)
			if err != nil {
				return c, err
			}
		case hasPrefix(arg, "--wrap="):
			var err error
			c.wrapCol, err = parseWrapCol(arg[len("--wrap="):])
			if err != nil {
				return c, err
			}
		case hasPrefix(arg, "-w"):
			var err error
			c.wrapCol, err = parseWrapCol(arg[2:])
			if err != nil {
				return c, err
			}
		case arg == "--":
			if i+1 < len(args) {
				c.file = args[i+1]
			}
			return c, nil
		case arg == "--help":
			printUsage()
			os.Exit(0)
		case arg == "--version":
			printVersion()
			os.Exit(0)
		case hasPrefix(arg, "-") && arg != "-":
			return c, fmt.Errorf("invalid option -- '%s'", arg)
		default:
			c.file = arg
		}
	}
	return c, nil
}

// run executes the encode or decode operation.
func run(c config) error {
	rc, err := encutil.OpenInput(c.file)
	if err != nil {
		// R1.4: exit 1 when a named file cannot be opened.
		return fmt.Errorf("%s: No such file or directory", c.file)
	}
	defer rc.Close() // best-effort close

	if c.decode {
		return runDecode(rc, c)
	}
	return runEncode(rc, c)
}

// runEncode encodes input to Base64 with wrap control.
// R1.1: encode from file or stdin. R1.2: default wrap at 76.
// R1.3: -w 0 disables wrapping and omits trailing newline (GNU compat).
func runEncode(r io.Reader, c config) error {
	if c.wrapCol == 0 {
		return encodeNoWrap(r)
	}
	return encutil.Encode(r, os.Stdout, encutil.EncoderConfig{
		Encode:  base64.StdEncoding.EncodeToString,
		WrapCol: c.wrapCol,
	})
}

// encodeNoWrap encodes input with no wrapping and no trailing newline,
// matching GNU coreutils base64 -w 0 behavior.
func encodeNoWrap(r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	_, err = io.WriteString(os.Stdout, encoded)
	return err
}

// runDecode decodes Base64 input with optional garbage skipping.
// R2.1: decode mode. R2.2: whitespace ignored. R2.3: -i ignores garbage.
// R2.4: returns "invalid input" on decode error to match GNU format.
func runDecode(r io.Reader, c config) error {
	err := encutil.Decode(r, os.Stdout, encutil.DecoderConfig{
		Decode:        base64.StdEncoding.DecodeString,
		IgnoreGarbage: c.ignoreGarbage,
	})
	if err != nil {
		return fmt.Errorf("invalid input")
	}
	return nil
}

func nextArg(args []string, i *int) (string, error) {
	*i++
	if *i >= len(args) {
		return "", fmt.Errorf("option requires an argument -- 'w'")
	}
	return args[*i], nil
}

func parseWrapCol(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid wrap size: '%s'", s)
	}
	return n, nil
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func printUsage() {
	fmt.Fprintf(os.Stdout, "Usage: %s [OPTION]... [FILE]\n", progName)
	fmt.Fprintln(os.Stdout, "Base64 encode or decode FILE, or standard input, to standard output.")
}

// printVersion prints version information to stdout.
func printVersion() {
	fmt.Fprintf(os.Stdout, "%s (go-unix-utils)\n", progName)
}
