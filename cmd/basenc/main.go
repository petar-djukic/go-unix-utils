// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/basenc encodes and decodes data using selectable encoding schemes.
// Implements: srd081 R1.1-R1.4 (encoding schemes: base64, base64url,
// base32, base32hex, base16, z85), R2.3-R2.4 (decode, wrap control),
// R3.1-R3.4 (ignore-garbage, input handling, error cases, SIGPIPE).
package main

import (
	"bytes"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
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
	progName       = "basenc"
	defaultWrapCol = 76
	versionStr     = "basenc (go-unix-utils) 0.1"
)

// z85Chars is the Z85 encoding alphabet per ZeroMQ RFC 32.
const z85Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ.-:+=^!/*?&<>()[]{}@%$#"

func main() {
	sys.InstallSIGPIPEHandler()
	if code := runMain(); code != 0 {
		os.Exit(code)
	}
}

// runMain parses flags and dispatches to encode or decode.
// R3.3, R3.4: exits 0 on success, 1 on any error.
func runMain() int {
	if handleSpecialFlags() {
		return 0
	}

	fs := flag.NewFlagSet(progName, flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	opts := defineFlags(fs)

	if err := fs.Parse(os.Args[1:]); err != nil {
		return handleFlagError(err)
	}

	enc, err := selectEncoding(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 1
	}

	filename := "-"
	if fs.NArg() > 0 {
		filename = fs.Arg(0)
	}

	if err := run(filename, enc, opts); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 1
	}
	return 0
}

// flagOpts holds parsed command-line flag values.
type flagOpts struct {
	wrap          *int
	decode        *bool
	ignoreGarbage *bool
	useBase64     *bool
	useBase64URL  *bool
	useBase32     *bool
	useBase32Hex  *bool
	useBase16     *bool
	useZ85        *bool
}

// defineFlags registers all command-line flags on fs.
func defineFlags(fs *flag.FlagSet) flagOpts {
	var o flagOpts
	o.wrap = fs.Int("w", defaultWrapCol, "")
	fs.IntVar(o.wrap, "wrap", defaultWrapCol, "")
	o.decode = fs.Bool("d", false, "")
	fs.BoolVar(o.decode, "decode", false, "")
	o.ignoreGarbage = fs.Bool("i", false, "")
	fs.BoolVar(o.ignoreGarbage, "ignore-garbage", false, "")
	o.useBase64 = fs.Bool("base64", false, "")
	o.useBase64URL = fs.Bool("base64url", false, "")
	o.useBase32 = fs.Bool("base32", false, "")
	o.useBase32Hex = fs.Bool("base32hex", false, "")
	o.useBase16 = fs.Bool("base16", false, "")
	o.useZ85 = fs.Bool("z85", false, "")
	return o
}

// encoding holds the encode and decode functions for a selected scheme.
type encoding struct {
	encode func([]byte) string
	decode func(string) ([]byte, error)
}

// selectEncoding returns the encoding selected by flags.
// R3.3: exits 1 when no encoding scheme is specified.
func selectEncoding(o flagOpts) (encoding, error) {
	count := boolCount(*o.useBase64, *o.useBase64URL,
		*o.useBase32, *o.useBase32Hex, *o.useBase16, *o.useZ85)
	if count == 0 {
		return encoding{}, fmt.Errorf("missing encoding type")
	}
	if count > 1 {
		return encoding{}, fmt.Errorf("extra encoding type")
	}
	return resolveEncoding(o), nil
}

// resolveEncoding returns the encoding for the single selected flag.
func resolveEncoding(o flagOpts) encoding {
	switch {
	case *o.useBase64:
		return encoding{encode: b64StdEncode, decode: b64StdDecode}
	case *o.useBase64URL:
		return encoding{encode: b64URLEncode, decode: b64URLDecode}
	case *o.useBase32:
		return encoding{encode: b32StdEncode, decode: b32StdDecode}
	case *o.useBase32Hex:
		return encoding{encode: b32HexEncode, decode: b32HexDecode}
	case *o.useBase16:
		return encoding{encode: b16Encode, decode: b16Decode}
	default:
		return encoding{encode: z85Encode, decode: z85Decode}
	}
}

// boolCount returns the number of true values.
func boolCount(vals ...bool) int {
	n := 0
	for _, v := range vals {
		if v {
			n++
		}
	}
	return n
}

// handleSpecialFlags checks for --help and --version before flag parsing.
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

// handleFlagError prints an error for invalid flags and returns exit code.
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
			"Encode or decode FILE, or standard input, to standard output.\n\n"+
			"With no FILE, or when FILE is -, read standard input.\n\n"+
			"      --base64          same as 'base64' program\n"+
			"      --base64url       file- and url-safe base64\n"+
			"      --base32          same as 'base32' program\n"+
			"      --base32hex       extended hex alphabet base32\n"+
			"      --base16          hex encoding\n"+
			"      --z85             ascii85-like encoding\n\n"+
			"  -d, --decode          decode data\n"+
			"  -i, --ignore-garbage  when decoding, ignore non-alphabet characters\n"+
			"  -w, --wrap=COLS       wrap encoded lines after COLS character (default 76).\n"+
			"                          Use 0 to disable line wrapping\n\n"+
			"      --help        display this help and exit\n"+
			"      --version     output version information and exit\n",
		progName)
}

// run dispatches to encode or decode based on the decode flag.
func run(filename string, enc encoding, o flagOpts) error {
	if *o.decode {
		return decodeInput(filename, enc, *o.ignoreGarbage)
	}
	return encodeInput(filename, enc, *o.wrap)
}

// encodeInput reads from filename and encodes to stdout.
// R1.1-R1.4: encode using selected alphabet with wrap control.
func encodeInput(filename string, enc encoding, wrapCol int) error {
	rc, err := encutil.OpenInput(filename)
	if err != nil {
		return fileError(filename, err)
	}
	defer rc.Close()

	var buf bytes.Buffer
	cfg := encutil.EncoderConfig{
		Encode:  enc.encode,
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
	if len(out) == 1 && out[0] == '\n' {
		return nil
	}
	if wrapCol == 0 && len(out) > 0 && out[len(out)-1] == '\n' {
		return out[:len(out)-1]
	}
	return out
}

// decodeInput reads, decodes, and writes binary to stdout.
// R2.3: decode using selected encoding. R3.1: ignore-garbage support.
func decodeInput(filename string, enc encoding, ignoreGarbage bool) error {
	rc, err := encutil.OpenInput(filename)
	if err != nil {
		return fileError(filename, err)
	}
	defer rc.Close()

	cfg := encutil.DecoderConfig{
		Decode:        enc.decode,
		IgnoreGarbage: ignoreGarbage,
	}
	if err := encutil.Decode(rc, os.Stdout, cfg); err != nil {
		return fmt.Errorf("invalid input")
	}
	return nil
}

// fileError unwraps os.PathError for GNU-compatible error messages.
func fileError(filename string, err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return fmt.Errorf("%s: %v", filename, pe.Err)
	}
	return err
}

// --- Encoding functions ---

// R1.1: RFC 4648 Base64 standard alphabet.
func b64StdEncode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func b64StdDecode(s string) ([]byte, error) {
	s = stripSpaceTabs(s)
	return base64.StdEncoding.DecodeString(s)
}

// R1.2: RFC 4648 Base64 URL-safe alphabet.
func b64URLEncode(data []byte) string {
	return base64.URLEncoding.EncodeToString(data)
}

func b64URLDecode(s string) ([]byte, error) {
	s = stripSpaceTabs(s)
	return base64.URLEncoding.DecodeString(s)
}

// R1.3: RFC 4648 Base32 standard alphabet.
func b32StdEncode(data []byte) string {
	return base32.StdEncoding.EncodeToString(data)
}

func b32StdDecode(s string) ([]byte, error) {
	s = stripSpaceTabs(s)
	return base32.StdEncoding.DecodeString(s)
}

// R1.4: RFC 4648 Base32 extended hex alphabet.
func b32HexEncode(data []byte) string {
	return base32.HexEncoding.EncodeToString(data)
}

func b32HexDecode(s string) ([]byte, error) {
	s = stripSpaceTabs(s)
	return base32.HexEncoding.DecodeString(s)
}

// R2.1: Base16 (hexadecimal), uppercase output.
func b16Encode(data []byte) string {
	return strings.ToUpper(hex.EncodeToString(data))
}

func b16Decode(s string) ([]byte, error) {
	s = stripSpaceTabs(s)
	return hex.DecodeString(s)
}

// R2.2: ZeroMQ Z85 encoding.
func z85Encode(data []byte) string {
	// Z85 requires input length to be a multiple of 4.
	if len(data)%4 != 0 {
		// Pad to multiple of 4 with zero bytes for encoding.
		padded := make([]byte, len(data)+4-len(data)%4)
		copy(padded, data)
		data = padded
	}
	var buf strings.Builder
	buf.Grow(len(data) * 5 / 4)
	for i := 0; i < len(data); i += 4 {
		val := uint32(data[i])<<24 |
			uint32(data[i+1])<<16 |
			uint32(data[i+2])<<8 |
			uint32(data[i+3])
		z85EncodeWord(&buf, val)
	}
	return buf.String()
}

// z85EncodeWord encodes a 32-bit value as 5 Z85 characters.
func z85EncodeWord(buf *strings.Builder, val uint32) {
	var chars [5]byte
	for j := 4; j >= 0; j-- {
		chars[j] = z85Chars[val%85]
		val /= 85
	}
	buf.Write(chars[:])
}

func z85Decode(s string) ([]byte, error) {
	s = stripSpaceTabs(s)
	if len(s)%5 != 0 {
		return nil, fmt.Errorf("invalid z85 input length")
	}
	out := make([]byte, 0, len(s)*4/5)
	for i := 0; i < len(s); i += 5 {
		val, err := z85DecodeWord(s[i : i+5])
		if err != nil {
			return nil, err
		}
		out = append(out,
			byte(val>>24), byte(val>>16), byte(val>>8), byte(val))
	}
	return out, nil
}

// z85DecodeWord decodes 5 Z85 characters into a 32-bit value.
func z85DecodeWord(chunk string) (uint32, error) {
	var val uint32
	for j := 0; j < 5; j++ {
		idx := strings.IndexByte(z85Chars, chunk[j])
		if idx < 0 {
			return 0, fmt.Errorf("invalid z85 character: %c", chunk[j])
		}
		val = val*85 + uint32(idx)
	}
	return val, nil
}

// stripSpaceTabs removes spaces and tabs from s.
func stripSpaceTabs(s string) string {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\t", "")
	return s
}
