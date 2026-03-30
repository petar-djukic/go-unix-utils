// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/basenc encodes and decodes data using multiple encoding schemes.
//
// Implements prd081-basenc: R1.1 (--base64), R1.2 (--base64url),
// R1.3 (--base32), R1.4 (--base32hex), R2.1 (--base16), R2.2 (--z85),
// R2.3 (-d/--decode), R2.4 (-w/--wrap).
package main

import (
	"bytes"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/encutil"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	progName       = "basenc"
	defaultWrapCol = 76
	// z85Alphabet is the ZeroMQ Z85 encoding alphabet (85 printable characters).
	z85Alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ.-:+=^!/*?&<>()[]{}@%$#"
)

// z85DecodeTable maps each byte to its Z85 alphabet index, or -1 if invalid.
var z85DecodeTable = buildZ85DecodeTable()

func buildZ85DecodeTable() [256]int8 {
	var table [256]int8
	for i := range len(table) {
		table[i] = -1
	}
	for i := range len(z85Alphabet) {
		table[z85Alphabet[i]] = int8(i)
	}
	return table
}

type encodingScheme struct {
	encode         func([]byte) string
	decode         func(string) ([]byte, error)
	validateEncode func([]byte) error // optional pre-encode validation
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

// schemeBase16 returns the hexadecimal (Base16) encoding scheme.
// R2.1: --base16 encodes with uppercase hex characters.
func schemeBase16() *encodingScheme {
	return &encodingScheme{
		encode: func(data []byte) string {
			return strings.ToUpper(hex.EncodeToString(data))
		},
		decode: func(s string) ([]byte, error) {
			return hex.DecodeString(s)
		},
	}
}

// schemeZ85 returns the ZeroMQ Z85 encoding scheme.
// R2.2: --z85 encodes 4-byte groups into 5-character Z85 strings.
func schemeZ85() *encodingScheme {
	return &encodingScheme{
		encode: z85Encode,
		decode: z85Decode,
		validateEncode: func(data []byte) error {
			if len(data)%4 != 0 {
				return fmt.Errorf("invalid input (length must be a multiple of 4)")
			}
			return nil
		},
	}
}

// z85Encode encodes data as Z85. Input length must be a multiple of 4.
// R2.2: ZeroMQ Z85 encoding algorithm.
func z85Encode(data []byte) string {
	var buf strings.Builder
	buf.Grow(len(data) * 5 / 4)
	for i := 0; i < len(data); i += 4 {
		value := uint32(data[i])<<24 |
			uint32(data[i+1])<<16 |
			uint32(data[i+2])<<8 |
			uint32(data[i+3])
		var chars [5]byte
		for j := 4; j >= 0; j-- {
			chars[j] = z85Alphabet[value%85]
			value /= 85
		}
		buf.Write(chars[:])
	}
	return buf.String()
}

// z85Decode decodes a Z85 string. Input length must be a multiple of 5.
// R2.2: ZeroMQ Z85 decoding algorithm.
func z85Decode(s string) ([]byte, error) {
	if len(s)%5 != 0 {
		return nil, fmt.Errorf(
			"invalid z85 input length %d (must be multiple of 5)", len(s))
	}
	result := make([]byte, len(s)*4/5)
	for i := 0; i < len(s); i += 5 {
		value, err := z85DecodeGroup(s[i : i+5])
		if err != nil {
			return nil, err
		}
		pos := (i / 5) * 4
		result[pos] = byte(value >> 24)
		result[pos+1] = byte(value >> 16)
		result[pos+2] = byte(value >> 8)
		result[pos+3] = byte(value)
	}
	return result, nil
}

// z85DecodeGroup decodes a 5-character Z85 group into a uint32 value.
func z85DecodeGroup(s string) (uint32, error) {
	var value uint64
	for j := range 5 {
		idx := z85DecodeTable[s[j]]
		if idx < 0 {
			return 0, fmt.Errorf("invalid z85 character: %c", s[j])
		}
		value = value*85 + uint64(idx)
	}
	if value > 0xFFFFFFFF {
		return 0, fmt.Errorf("z85 group value overflow")
	}
	return uint32(value), nil
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
	case arg == "--base16":
		c.scheme = schemeBase16()
	case arg == "--z85":
		c.scheme = schemeZ85()
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

// runEncode reads all input, validates if needed, and encodes with wrap control.
// R2.3: encoding is the default mode. R2.4: wrapping at configurable column.
func runEncode(r io.Reader, c config) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if c.scheme.validateEncode != nil {
		if err := c.scheme.validateEncode(data); err != nil {
			return err
		}
	}
	if c.wrapCol == 0 {
		_, err = io.WriteString(os.Stdout, c.scheme.encode(data))
		return err
	}
	return encutil.Encode(bytes.NewReader(data), os.Stdout, encutil.EncoderConfig{
		Encode:  c.scheme.encode,
		WrapCol: c.wrapCol,
	})
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
