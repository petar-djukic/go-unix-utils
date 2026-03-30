// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/basenc encodes and decodes data using multiple encoding schemes.
//
// Implements prd081-basenc: R1.1 (--base64), R1.2 (--base64url),
// R1.3 (--base32), R1.4 (--base32hex).
package main

import (
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/petar-djukic/go-unix-utils/pkg/encutil"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	progName       = "basenc"
	defaultWrapCol = 76
)

type encodingScheme struct {
	encode func([]byte) string
	decode func(string) ([]byte, error)
}

type config struct {
	scheme        *encodingScheme
	decode        bool
	ignoreGarbage bool
	wrapCol       int
	file          string
}

// schemeBase64 returns the RFC 4648 Base64 encoding scheme.
// R1.1: --base64 uses standard Base64 alphabet.
func schemeBase64() *encodingScheme {
	return &encodingScheme{
		encode: base64.StdEncoding.EncodeToString,
		decode: base64.StdEncoding.DecodeString,
	}
}

// schemeBase64URL returns the RFC 4648 Base64 URL-safe encoding scheme.
// R1.2: --base64url replaces + and / with - and _.
func schemeBase64URL() *encodingScheme {
	return &encodingScheme{
		encode: base64.URLEncoding.EncodeToString,
		decode: base64.URLEncoding.DecodeString,
	}
}

// schemeBase32 returns the RFC 4648 Base32 encoding scheme.
// R1.3: --base32 uses standard Base32 alphabet.
func schemeBase32() *encodingScheme {
	return &encodingScheme{
		encode: base32.StdEncoding.EncodeToString,
		decode: base32.StdEncoding.DecodeString,
	}
}

// schemeBase32Hex returns the RFC 4648 Base32 extended hex encoding scheme.
// R1.4: --base32hex uses extended hex alphabet (0-9, A-V).
func schemeBase32Hex() *encodingScheme {
	return &encodingScheme{
		encode: base32.HexEncoding.EncodeToString,
		decode: base32.HexEncoding.DecodeString,
	}
}

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

// parseArgs parses command-line arguments into a config struct.
func parseArgs(args []string) (config, error) {
	c := config{wrapCol: defaultWrapCol}
	for i := 0; i < len(args); i++ {
		if err := parseOneArg(args, &i, &c); err != nil {
			return c, err
		}
	}
	if c.scheme == nil {
		return c, fmt.Errorf("missing encoding type")
	}
	return c, nil
}

// parseOneArg handles a single argument during parsing.
func parseOneArg(args []string, i *int, c *config) error {
	arg := args[*i]
	switch {
	case arg == "--base64":
		c.scheme = schemeBase64()
	case arg == "--base64url":
		c.scheme = schemeBase64URL()
	case arg == "--base32":
		c.scheme = schemeBase32()
	case arg == "--base32hex":
		c.scheme = schemeBase32Hex()
	case arg == "-d" || arg == "--decode":
		c.decode = true
	case arg == "-i" || arg == "--ignore-garbage":
		c.ignoreGarbage = true
	case arg == "-w":
		return parseWrapArg(args, i, c)
	case hasPrefix(arg, "--wrap="):
		return parseWrapValue(arg[len("--wrap="):], c)
	case hasPrefix(arg, "-w"):
		return parseWrapValue(arg[2:], c)
	case arg == "--":
		if *i+1 < len(args) {
			c.file = args[*i+1]
		}
		*i = len(args) // stop parsing
	case arg == "--help":
		printUsage()
		os.Exit(0)
	case arg == "--version":
		printVersion()
		os.Exit(0)
	case hasPrefix(arg, "-") && arg != "-":
		return fmt.Errorf("invalid option -- '%s'", arg)
	default:
		c.file = arg
	}
	return nil
}

// parseWrapArg handles -w with a separate argument.
func parseWrapArg(args []string, i *int, c *config) error {
	*i++
	if *i >= len(args) {
		return fmt.Errorf("option requires an argument -- 'w'")
	}
	return parseWrapValue(args[*i], c)
}

// parseWrapValue parses a wrap column value string into the config.
func parseWrapValue(s string, c *config) error {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return fmt.Errorf("invalid wrap size: '%s'", s)
	}
	c.wrapCol = n
	return nil
}

// run executes the encode or decode operation.
func run(c config) error {
	rc, err := encutil.OpenInput(c.file)
	if err != nil {
		return fmt.Errorf("%s: No such file or directory", c.file)
	}
	defer rc.Close() // best-effort close

	if c.decode {
		return runDecode(rc, c)
	}
	return runEncode(rc, c)
}

// runEncode encodes input using the selected scheme with wrap control.
func runEncode(r io.Reader, c config) error {
	if c.wrapCol == 0 {
		return encodeNoWrap(r, c.scheme.encode)
	}
	return encutil.Encode(r, os.Stdout, encutil.EncoderConfig{
		Encode:  c.scheme.encode,
		WrapCol: c.wrapCol,
	})
}

// encodeNoWrap encodes input with no wrapping and no trailing newline,
// matching GNU coreutils basenc -w 0 behavior.
func encodeNoWrap(r io.Reader, encode func([]byte) string) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	encoded := encode(data)
	_, err = io.WriteString(os.Stdout, encoded)
	return err
}

// runDecode decodes input using the selected scheme.
func runDecode(r io.Reader, c config) error {
	err := encutil.Decode(r, os.Stdout, encutil.DecoderConfig{
		Decode:        c.scheme.decode,
		IgnoreGarbage: c.ignoreGarbage,
	})
	if err != nil {
		return fmt.Errorf("invalid input")
	}
	return nil
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func printUsage() {
	fmt.Fprintf(os.Stdout, "Usage: %s [OPTION]... [FILE]\n", progName)
	fmt.Fprintln(os.Stdout, "Encode or decode FILE, or standard input, to standard output.")
}

// printVersion prints version information to stdout.
func printVersion() {
	fmt.Fprintf(os.Stdout, "%s (go-unix-utils)\n", progName)
}
