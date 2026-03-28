// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd081-basenc R1.1–R1.4, R2.1–R2.4, R3.1–R3.4: multi-encoding
// utility supporting Base64, Base64URL, Base32, Base32hex, Base16, and Z85
// encoding schemes with configurable line wrapping, decode mode,
// ignore-garbage support, --version, --help, and diagnostic messages using
// pkg/encutil and pkg/sys.
//
// TODO: prd081-basenc non_goals: base2msbf and base2lsbf (Base2 binary encoding)
// are explicitly listed as non_goals. Task R2 references them but they are not
// implemented per E6 (non-goals enforcement).
package main

import (
	"bytes"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/encutil"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// defaultWrapCol is the default wrap column for encoded output.
// R2.4: wrap at 76 characters per line by default.
const defaultWrapCol = 76

// progName is the binary name used in error messages.
const progName = "basenc"

// z85Alphabet is the Z85 encoding alphabet per ZeroMQ spec:32/Z85.
const z85Alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ.-:+=^!/*?&<>()[]{}@%$#"

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// z85DecodeTable maps each byte to its Z85 alphabet index (0xFF = invalid).
var z85DecodeTable = buildZ85DecodeTable()

// options holds the parsed command-line options.
type options struct {
	encoding      string
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
	encodings := 0
	for i := 0; i < len(args); i++ {
		code := parseSingleArg(args, &i, opts, &encodings)
		if code >= 0 {
			return code
		}
	}
	return validateEncodings(encodings)
}

// parseSingleArg processes one command-line argument.
// Returns -1 to continue, or an exit code >= 0 to exit immediately.
func parseSingleArg(args []string, i *int, opts *options, enc *int) int {
	arg := args[*i]
	if e := encodingFromFlag(arg); e != "" {
		opts.encoding = e
		(*enc)++
		return -1
	}
	switch {
	case arg == "--":
		if *i+1 < len(args) {
			opts.filename = args[*i+1]
		}
		*i = len(args)
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
		(*i)++
		if *i >= len(args) {
			printOptionRequiresArg(arg)
			return 1
		}
		return parseWrapArg(args[*i], &opts.wrapCol)
	case len(arg) > 2 && arg[:2] == "-w":
		return parseWrapArg(arg[2:], &opts.wrapCol)
	case len(arg) > 7 && arg[:7] == "--wrap=":
		return parseWrapArg(arg[7:], &opts.wrapCol)
	case isDashArg(arg):
		return parseShortFlags(arg[1:], opts)
	default:
		opts.filename = arg
	}
	return -1
}

// encodingFromFlag returns the encoding name for a long flag, or "" if not
// an encoding flag. R1.1–R1.4, R2.1–R2.2.
func encodingFromFlag(arg string) string {
	switch arg {
	case "--base64":
		return "base64"
	case "--base64url":
		return "base64url"
	case "--base32":
		return "base32"
	case "--base32hex":
		return "base32hex"
	case "--base16":
		return "base16"
	case "--z85":
		return "z85"
	default:
		return ""
	}
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

// validateEncodings checks that exactly one encoding was specified.
// R1.3: exits 1 if none or more than one encoding flag is given.
func validateEncodings(count int) int {
	if count == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing encoding type\n", progName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		return 1
	}
	if count > 1 {
		fmt.Fprintf(os.Stderr, "%s: multiple encodings specified\n", progName)
		return 1
	}
	return -1
}

// run performs the basenc encoding or decoding operation.
func run(opts options) error {
	if opts.decode {
		return runDecode(opts)
	}
	return runEncode(opts)
}

// runEncode reads input and encodes using the selected encoding.
// R1.1: reads from file or stdin. R2.4: wraps at configured column width.
func runEncode(opts options) error {
	input, err := encutil.OpenInput(opts.filename)
	if err != nil {
		return err
	}
	defer input.Close()

	data, err := io.ReadAll(input)
	if err != nil {
		return err
	}

	if err := validateEncodeInput(opts.encoding, data); err != nil {
		return err
	}

	cfg := encutil.EncoderConfig{
		Encode:  getEncodeFn(opts.encoding),
		WrapCol: opts.wrapCol,
	}

	var buf bytes.Buffer
	if err := encutil.Encode(bytes.NewReader(data), &buf, cfg); err != nil {
		return err
	}

	output := adjustOutput(buf.Bytes(), opts.wrapCol)
	if len(output) == 0 {
		return nil
	}
	_, err = os.Stdout.Write(output)
	return err
}

// validateEncodeInput checks encoding-specific input constraints.
// R2.2: Z85 requires input length to be a multiple of 4 bytes.
func validateEncodeInput(encoding string, data []byte) error {
	if encoding == "z85" && len(data)%4 != 0 {
		return fmt.Errorf("invalid input (length must be multiple of 4)")
	}
	return nil
}

// runDecode decodes input using the selected encoding.
// R2.1: -d decodes input. R2.2: ignores newlines in decode input.
func runDecode(opts options) error {
	input, err := encutil.OpenInput(opts.filename)
	if err != nil {
		return err
	}
	defer input.Close()

	cfg := encutil.DecoderConfig{
		Decode:        getDecodeFn(opts.encoding),
		IgnoreGarbage: opts.ignoreGarbage,
	}

	if err := encutil.Decode(input, os.Stdout, cfg); err != nil {
		return fmt.Errorf("invalid input")
	}
	return nil
}

// getEncodeFn returns the encoding function for the selected encoding.
func getEncodeFn(encoding string) func([]byte) string {
	switch encoding {
	case "base64":
		return base64.StdEncoding.EncodeToString
	case "base64url":
		return base64.URLEncoding.EncodeToString
	case "base32":
		return base32.StdEncoding.EncodeToString
	case "base32hex":
		return base32.HexEncoding.EncodeToString
	case "base16":
		return hexEncode
	case "z85":
		return z85Encode
	default:
		return nil
	}
}

// getDecodeFn returns the decoding function for the selected encoding.
func getDecodeFn(encoding string) func(string) ([]byte, error) {
	switch encoding {
	case "base64":
		return base64.StdEncoding.DecodeString
	case "base64url":
		return base64.URLEncoding.DecodeString
	case "base32":
		return base32.StdEncoding.DecodeString
	case "base32hex":
		return base32.HexEncoding.DecodeString
	case "base16":
		return hex.DecodeString
	case "z85":
		return z85Decode
	default:
		return nil
	}
}

// hexEncode encodes data as uppercase hexadecimal.
// R2.1: Base16 output uses uppercase hex characters.
func hexEncode(data []byte) string {
	return strings.ToUpper(hex.EncodeToString(data))
}

// z85Encode encodes data using the ZeroMQ Z85 alphabet.
// R2.2: input length must be a multiple of 4 (validated by caller).
func z85Encode(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	var buf strings.Builder
	buf.Grow(len(data) * 5 / 4)
	for i := 0; i < len(data); i += 4 {
		val := binary.BigEndian.Uint32(data[i : i+4])
		var chars [5]byte
		for j := 4; j >= 0; j-- {
			chars[j] = z85Alphabet[val%85]
			val /= 85
		}
		buf.Write(chars[:])
	}
	return buf.String()
}

// z85Decode decodes a Z85-encoded string back to bytes.
func z85Decode(s string) ([]byte, error) {
	if len(s) == 0 {
		return nil, nil
	}
	if len(s)%5 != 0 {
		return nil, fmt.Errorf("invalid z85 input length")
	}
	result := make([]byte, len(s)*4/5)
	for i := 0; i < len(s); i += 5 {
		val, err := z85DecodeChunk(s[i : i+5])
		if err != nil {
			return nil, err
		}
		binary.BigEndian.PutUint32(result[i*4/5:], val)
	}
	return result, nil
}

// z85DecodeChunk decodes a single 5-character Z85 chunk to a uint32.
func z85DecodeChunk(chunk string) (uint32, error) {
	var val uint32
	for j := range 5 {
		idx := z85DecodeTable[chunk[j]]
		if idx == 0xFF {
			return 0, fmt.Errorf("invalid z85 character: %c", chunk[j])
		}
		val = val*85 + uint32(idx)
	}
	return val, nil
}

// buildZ85DecodeTable creates the reverse lookup table for Z85 decoding.
func buildZ85DecodeTable() [256]byte {
	var table [256]byte
	for i := range table {
		table[i] = 0xFF
	}
	for i, c := range []byte(z85Alphabet) {
		table[c] = byte(i)
	}
	return table
}

// formatError converts Go os.PathError into GNU-style error messages.
// GNU format: "filename: Error message" (no "open" prefix, capitalized).
func formatError(err error) string {
	if pathErr, ok := errors.AsType[*os.PathError](err); ok {
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

// adjustOutput trims encutil output to match GNU basenc behavior.
// Empty input produces no output; -w0 omits trailing newline.
func adjustOutput(output []byte, wrapCol int) []byte {
	if bytes.Equal(output, []byte("\n")) {
		return nil
	}
	if wrapCol == 0 {
		return bytes.TrimSuffix(output, []byte("\n"))
	}
	return output
}

// printVersion writes version information to stdout matching GNU format.
func printVersion() {
	fmt.Printf("%s (go-unix-utils) %s\n", progName, version)
}

// printHelp writes usage information to stdout matching GNU format.
func printHelp() {
	fmt.Printf(`Usage: %s [OPTION]... [FILE]
Encode or decode FILE, or standard input, to standard output.

With no FILE, or when FILE is -, read standard input.

Encoding selection:
      --base64          same as 'base64' program (RFC4648 section 4)
      --base64url       file- and url-safe base64 (RFC4648 section 5)
      --base32          same as 'base32' program (RFC4648 section 6)
      --base32hex       extended hex alphabet base32 (RFC4648 section 7)
      --base16          hex encoding (RFC4648 section 8)
      --z85             ascii85-like encoding (ZeroMQ spec:32/Z85)

  -d, --decode          decode data
  -i, --ignore-garbage  when decoding, ignore non-alphabet characters
  -w, --wrap=COLS       wrap encoded lines after COLS character (default 76).
                          Use 0 to disable line wrapping

      --help     display this help and exit
      --version  output version information and exit
`, progName)
}
