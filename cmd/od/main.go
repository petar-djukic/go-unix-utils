// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd072-od: Octal and Other Format Dump.
// Covers R1.1-R1.4 (default behavior and type specifiers),
// R2.1-R2.2 (address format and byte range).
package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sizeparse"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

const defaultWidth = 16

// typeSpec describes one output format column for od.
// R1.2: each -t flag adds a typeSpec to the output.
type typeSpec struct {
	size   int                // bytes per element
	format func([]byte) string // formats one element
}

// odOptions holds parsed command-line options.
type odOptions struct {
	types     []typeSpec
	addrRadix byte  // 'o', 'd', 'x', 'n'
	skipBytes int64
	readBytes int64 // 0 = unlimited
	width     int
	showDupes bool // -v
}

// namedChars maps byte values 0-127 to POSIX named representations.
// R1.2: type 'a' named-character format.
var namedChars = [128]string{
	"nul", "soh", "stx", "etx", "eot", "enq", "ack", "bel",
	"bs", "ht", "nl", "vt", "ff", "cr", "so", "si",
	"dle", "dc1", "dc2", "dc3", "dc4", "nak", "syn", "etb",
	"can", "em", "sub", "esc", "fs", "gs", "rs", "us",
	"sp", "!", "\"", "#", "$", "%", "&", "'",
	"(", ")", "*", "+", ",", "-", ".", "/",
	"0", "1", "2", "3", "4", "5", "6", "7",
	"8", "9", ":", ";", "<", "=", ">", "?",
	"@", "A", "B", "C", "D", "E", "F", "G",
	"H", "I", "J", "K", "L", "M", "N", "O",
	"P", "Q", "R", "S", "T", "U", "V", "W",
	"X", "Y", "Z", "[", "\\", "]", "^", "_",
	"`", "a", "b", "c", "d", "e", "f", "g",
	"h", "i", "j", "k", "l", "m", "n", "o",
	"p", "q", "r", "s", "t", "u", "v", "w",
	"x", "y", "z", "{", "|", "}", "~", "del",
}

// cEscapes maps byte values to C-style escape representations.
var cEscapes = map[byte]string{
	0: "\\0", 7: "\\a", 8: "\\b", 9: "\\t",
	10: "\\n", 11: "\\v", 12: "\\f", 13: "\\r",
}

func main() {
	// R1.4: Install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()
	opts, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "od: %s\n", err)
		os.Exit(1)
	}
	os.Exit(run(opts, files, os.Stdin, os.Stdout, os.Stderr))
}

// run opens inputs, skips bytes, dumps data, and returns exit code.
func run(opts odOptions, files []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(files) == 0 {
		files = []string{"-"}
	}
	r, err := openInputs(files, stdin, stderr)
	if err != nil {
		return 1
	}
	defer r.Close()
	if opts.skipBytes > 0 {
		if _, err := io.CopyN(io.Discard, r, opts.skipBytes); err != nil {
			fmt.Fprintf(stderr, "od: cannot skip past end of combined input\n")
		}
	}
	w := bufio.NewWriter(stdout)
	dumpData(r, opts, w)
	_ = w.Flush() // best-effort flush
	return 0
}

// parseArgs processes command-line arguments into options and file list.
// R1.1: parses all od flags; defaults to octal 2-byte words.
func parseArgs(args []string) (odOptions, []string, error) {
	opts := odOptions{addrRadix: 'o', width: defaultWidth}
	var files []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if arg == "--help" {
			printHelp()
			os.Exit(0)
		}
		if arg == "--version" {
			printVersion()
			os.Exit(0)
		}
		if strings.HasPrefix(arg, "--") {
			if err := parseLongFlag(arg, args, &i, &opts); err != nil {
				return opts, nil, err
			}
			continue
		}
		if len(arg) > 1 && arg[0] == '-' && arg != "-" {
			if err := parseShortFlags(arg[1:], args, &i, &opts); err != nil {
				return opts, nil, err
			}
			continue
		}
		files = append(files, arg)
	}
	if len(opts.types) == 0 {
		// R1.1: default format is octal 2-byte words.
		spec, _ := parseTypeString("o2")
		opts.types = append(opts.types, spec)
	}
	return opts, files, nil
}

// parseLongFlag handles a single --flag or --flag=value argument.
func parseLongFlag(arg string, args []string, idx *int, opts *odOptions) error {
	key, val, hasVal := strings.Cut(arg[2:], "=")
	shortMap := map[string]byte{
		"format": 't', "address-radix": 'A',
		"skip-bytes": 'j', "read-bytes": 'N',
	}
	if short, ok := shortMap[key]; ok {
		if !hasVal {
			*idx++
			if *idx >= len(args) {
				return fmt.Errorf("option '--%s' requires an argument", key)
			}
			val = args[*idx]
		}
		return applyValueFlag(short, val, opts)
	}
	switch key {
	case "width":
		if hasVal {
			return applyWidthValue(val, opts)
		}
		opts.width = 32
	case "output-duplicates":
		opts.showDupes = true
	default:
		return fmt.Errorf("unrecognized option '--%s'", key)
	}
	return nil
}

// parseShortFlags processes short option characters from a '-' argument.
func parseShortFlags(chars string, args []string, idx *int, opts *odOptions) error {
	for i := 0; i < len(chars); i++ {
		c := chars[i]
		if c == 't' || c == 'A' || c == 'j' || c == 'N' || c == 'w' {
			return handleValueFlag(c, chars[i+1:], args, idx, opts)
		}
		if err := applyBoolFlag(c, opts); err != nil {
			return err
		}
	}
	return nil
}

// handleValueFlag extracts the value for a flag that requires one.
func handleValueFlag(c byte, rest string, args []string, idx *int, opts *odOptions) error {
	val := rest
	if val == "" && c != 'w' {
		*idx++
		if *idx >= len(args) {
			return fmt.Errorf("option requires an argument -- '%c'", c)
		}
		val = args[*idx]
	}
	return applyValueFlag(c, val, opts)
}

// applyValueFlag applies a flag that takes a value argument.
func applyValueFlag(c byte, val string, opts *odOptions) error {
	switch c {
	case 't':
		spec, err := parseTypeString(val)
		if err != nil {
			return err
		}
		opts.types = append(opts.types, spec)
	case 'A':
		return parseAddrRadix(val, opts)
	case 'j':
		n, err := sizeparse.Parse(val)
		if err != nil {
			return fmt.Errorf("invalid number of bytes to skip: %q", val)
		}
		opts.skipBytes = n
	case 'N':
		n, err := sizeparse.Parse(val)
		if err != nil {
			return fmt.Errorf("invalid number of bytes to read: %q", val)
		}
		opts.readBytes = n
	case 'w':
		return applyWidthValue(val, opts)
	}
	return nil
}

// applyWidthValue parses and sets the output width from a string value.
func applyWidthValue(val string, opts *odOptions) error {
	if val == "" {
		opts.width = 32
		return nil
	}
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		return fmt.Errorf("invalid width: %q", val)
	}
	opts.width = n
	return nil
}

// applyBoolFlag handles short flags that take no value.
// R3.2: traditional short flags map to type specifiers.
func applyBoolFlag(c byte, opts *odOptions) error {
	tradMap := map[byte]string{
		'b': "o1", 'c': "c", 'd': "u2",
		'o': "o2", 's': "d2", 'x': "x2",
	}
	if ts, ok := tradMap[c]; ok {
		spec, _ := parseTypeString(ts)
		opts.types = append(opts.types, spec)
		return nil
	}
	if c == 'v' {
		opts.showDupes = true
		return nil
	}
	return fmt.Errorf("invalid option -- '%c'", c)
}

// parseAddrRadix sets the address radix from the -A value.
// R2.1: d, o, x, n are valid radix values.
func parseAddrRadix(val string, opts *odOptions) error {
	if len(val) != 1 || !strings.ContainsRune("doxn", rune(val[0])) {
		return fmt.Errorf("invalid address radix '%s'; must be one of [doxn]", val)
	}
	opts.addrRadix = val[0]
	return nil
}

// parseTypeString converts a format type string into a typeSpec.
// R1.2: format letter + optional size suffix.
func parseTypeString(s string) (typeSpec, error) {
	if len(s) == 0 {
		return typeSpec{}, fmt.Errorf("missing type string")
	}
	switch s[0] {
	case 'a':
		return typeSpec{size: 1, format: formatNamedChar}, nil
	case 'c':
		return typeSpec{size: 1, format: formatCChar}, nil
	case 'f':
		size, err := parseFloatSize(s[1:])
		if err != nil {
			return typeSpec{}, err
		}
		return typeSpec{size: size, format: makeFloatFmt(size)}, nil
	case 'd', 'o', 'u', 'x':
		size, err := parseIntSize(s[1:])
		if err != nil {
			return typeSpec{}, err
		}
		w := intFieldWidth(s[0], size)
		return typeSpec{size: size, format: makeIntFmt(s[0], size, w)}, nil
	default:
		return typeSpec{}, fmt.Errorf("invalid type string '%s'", s)
	}
}

// parseIntSize parses the size suffix for integer types.
// Default is sizeof(int) = 4.
func parseIntSize(suffix string) (int, error) {
	if suffix == "" {
		return 4, nil
	}
	switch suffix {
	case "C":
		return 1, nil
	case "S":
		return 2, nil
	case "I":
		return 4, nil
	case "L":
		return 8, nil
	}
	n, err := strconv.Atoi(suffix)
	if err != nil || (n != 1 && n != 2 && n != 4 && n != 8) {
		return 0, fmt.Errorf("invalid type size '%s'", suffix)
	}
	return n, nil
}

// parseFloatSize parses the size suffix for float types.
// Default is sizeof(double) = 8.
func parseFloatSize(suffix string) (int, error) {
	if suffix == "" {
		return 8, nil
	}
	switch suffix {
	case "F", "4":
		return 4, nil
	case "D", "L", "8":
		return 8, nil
	}
	return 0, fmt.Errorf("invalid float size '%s'", suffix)
}

// intFieldWidth returns the output field width for an integer type.
func intFieldWidth(typeLetter byte, size int) int {
	widths := map[byte][9]int{
		'o': {0, 3, 6, 0, 11, 0, 0, 0, 22},
		'x': {0, 2, 4, 0, 8, 0, 0, 0, 16},
		'u': {0, 3, 5, 0, 10, 0, 0, 0, 20},
		'd': {0, 4, 6, 0, 11, 0, 0, 0, 20},
	}
	if t, ok := widths[typeLetter]; ok {
		return t[size]
	}
	return 3
}

// makeIntFmt creates a formatter function for integer types.
func makeIntFmt(typeLetter byte, size, width int) func([]byte) string {
	switch typeLetter {
	case 'o':
		return func(data []byte) string {
			return fmt.Sprintf("%0*o", width, readUint(data, size))
		}
	case 'x':
		return func(data []byte) string {
			return fmt.Sprintf("%0*x", width, readUint(data, size))
		}
	case 'u':
		return func(data []byte) string {
			return fmt.Sprintf("%*d", width, readUint(data, size))
		}
	case 'd':
		return func(data []byte) string {
			return fmt.Sprintf("%*d", width, signExtend(readUint(data, size), size))
		}
	default:
		return func(data []byte) string {
			return fmt.Sprintf("%*d", width, readUint(data, size))
		}
	}
}

// makeFloatFmt creates a formatter function for floating-point types.
func makeFloatFmt(size int) func([]byte) string {
	if size == 4 {
		return func(data []byte) string {
			bits := uint32(readUint(data, 4))
			return fmt.Sprintf("%15.7e", float64(math.Float32frombits(bits)))
		}
	}
	return func(data []byte) string {
		return fmt.Sprintf("%24.17e", math.Float64frombits(readUint(data, 8)))
	}
}

// formatNamedChar formats a byte as a named character.
// R1.2: type 'a'; bytes >= 128 use b & 0x7F.
func formatNamedChar(data []byte) string {
	return fmt.Sprintf("%3s", namedChars[data[0]&0x7F])
}

// formatCChar formats a byte as a C-style character.
// R1.2: type 'c' with C escape sequences.
func formatCChar(data []byte) string {
	b := data[0]
	if esc, ok := cEscapes[b]; ok {
		return fmt.Sprintf("%3s", esc)
	}
	if b >= 0x20 && b < 0x7f {
		return fmt.Sprintf("%3s", string(rune(b)))
	}
	return fmt.Sprintf("%03o", b)
}

// readUint reads an unsigned integer from data in native byte order.
func readUint(data []byte, size int) uint64 {
	switch size {
	case 1:
		return uint64(data[0])
	case 2:
		return uint64(binary.NativeEndian.Uint16(data))
	case 4:
		return uint64(binary.NativeEndian.Uint32(data))
	case 8:
		return binary.NativeEndian.Uint64(data)
	default:
		return 0
	}
}

// signExtend converts an unsigned value to signed based on size.
func signExtend(val uint64, size int) int64 {
	switch size {
	case 1:
		return int64(int8(val))
	case 2:
		return int64(int16(val))
	case 4:
		return int64(int32(val))
	default:
		return int64(val)
	}
}

// dumpData reads input and writes formatted output.
// R1.3: multiple type specs produce multiple lines per block.
// R2.4: final address line after all data.
func dumpData(r io.Reader, opts odOptions, w *bufio.Writer) {
	buf := make([]byte, opts.width)
	offset := opts.skipBytes
	var bytesRead int64
	for {
		n, readErr := readBlock(r, buf, opts.readBytes, bytesRead)
		if n == 0 {
			break
		}
		writeBlock(w, buf[:n], offset, opts)
		offset += int64(n)
		bytesRead += int64(n)
		if readErr != nil {
			break
		}
	}
	writeEndAddress(w, offset, opts.addrRadix)
}

// readBlock reads up to len(buf) bytes, respecting readBytes limit.
func readBlock(r io.Reader, buf []byte, readBytes, bytesRead int64) (int, error) {
	limit := len(buf)
	if readBytes > 0 {
		remaining := readBytes - bytesRead
		if remaining <= 0 {
			return 0, io.EOF
		}
		if int64(limit) > remaining {
			limit = int(remaining)
		}
	}
	n, err := io.ReadFull(r, buf[:limit])
	if err == io.ErrUnexpectedEOF {
		err = io.EOF
	}
	return n, err
}

// writeBlock writes one block of data with all type format lines.
func writeBlock(w *bufio.Writer, data []byte, offset int64, opts odOptions) {
	addrW := 7
	if opts.addrRadix == 'x' {
		addrW = 6
	}
	for i, spec := range opts.types {
		if i == 0 && opts.addrRadix != 'n' {
			fmt.Fprint(w, formatAddress(offset, opts.addrRadix))
		} else if i > 0 && opts.addrRadix != 'n' {
			w.WriteString(strings.Repeat(" ", addrW))
		}
		writeValues(w, data, spec)
		w.WriteByte('\n')
	}
}

// writeValues formats and writes all elements from data for one type.
func writeValues(w *bufio.Writer, data []byte, spec typeSpec) {
	for i := 0; i < len(data); i += spec.size {
		end := i + spec.size
		var chunk []byte
		if end > len(data) {
			padded := make([]byte, spec.size)
			copy(padded, data[i:])
			chunk = padded
		} else {
			chunk = data[i:end]
		}
		w.WriteByte(' ')
		w.WriteString(spec.format(chunk))
	}
}

// writeEndAddress writes the final address offset after all data.
// R2.4: terminates output with the next-byte address.
func writeEndAddress(w *bufio.Writer, offset int64, radix byte) {
	if radix != 'n' {
		fmt.Fprintln(w, formatAddress(offset, radix))
	}
}

// formatAddress formats an offset in the specified radix.
// R2.1: supports d, o, x, n address radixes.
func formatAddress(offset int64, radix byte) string {
	switch radix {
	case 'd':
		return fmt.Sprintf("%07d", offset)
	case 'x':
		return fmt.Sprintf("%06x", offset)
	default:
		return fmt.Sprintf("%07o", offset)
	}
}

// openInputs opens all file arguments and concatenates them.
// R1.4: reads files in order; '-' means stdin.
func openInputs(files []string, stdin io.Reader, stderr io.Writer) (io.ReadCloser, error) {
	if len(files) == 1 {
		return openSingle(files[0], stdin)
	}
	readers := make([]io.Reader, 0, len(files))
	closers := make([]io.Closer, 0, len(files))
	for _, name := range files {
		rc, err := openSingle(name, stdin)
		if err != nil {
			fmt.Fprintf(stderr, "od: %s: %s\n", name, err)
			for _, c := range closers {
				c.Close() // best-effort cleanup
			}
			return nil, err
		}
		readers = append(readers, rc)
		closers = append(closers, rc)
	}
	return &multiRC{Reader: io.MultiReader(readers...), closers: closers}, nil
}

func openSingle(name string, stdin io.Reader) (io.ReadCloser, error) {
	if name == "-" {
		return io.NopCloser(stdin), nil
	}
	return os.Open(name)
}

// multiRC wraps a MultiReader with close-all semantics.
type multiRC struct {
	io.Reader
	closers []io.Closer
}

func (m *multiRC) Close() error {
	for _, c := range m.closers {
		c.Close() // best-effort
	}
	return nil
}

func printVersion() {
	fmt.Printf("od (go-unix-utils) %s\n", version)
}

func printHelp() {
	fmt.Print(`Usage: od [OPTION]... [FILE]...
Write an unambiguous representation of FILE in octal by default.
With no FILE, or when FILE is -, read standard input.

  -A RADIX  --address-radix=RADIX   output format for file offsets [doxn]
  -j BYTES  --skip-bytes=BYTES      skip BYTES input bytes first
  -N BYTES  --read-bytes=BYTES      limit dump to BYTES input bytes
  -t TYPE   --format=TYPE           select output format
  -v        --output-duplicates     do not use * to mark line suppression
  -w[BYTES] --width[=BYTES]         output BYTES bytes per output line
  -b -c -d -o -s -x                traditional format shortcuts
      --help     display this help and exit
      --version  output version information and exit
`)
}
