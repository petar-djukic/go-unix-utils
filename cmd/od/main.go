// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/od implements GNU od: octal and other format dump.
//
// Implements prd072-od R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4.
package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sizeparse"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const defaultWidth = 16

// typeSpec describes one output format: a format letter and byte size.
type typeSpec struct {
	letter byte
	size   int
}

// odConfig holds parsed command-line options.
type odConfig struct {
	specs     []typeSpec
	files     []string
	addrRadix byte  // R2.1: 'o', 'd', 'x', 'n'
	skipBytes int64 // R2.2: bytes to skip before formatting
	readBytes int64 // R2.3: max bytes to format (-1 = unlimited)
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses flags, opens input, and performs the dump.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "od: %s\n", err)
		return 1
	}
	if len(cfg.specs) == 0 {
		// R1.1: default is octal 2-byte words.
		cfg.specs = []typeSpec{{'o', 2}}
	}
	if len(cfg.files) == 0 {
		// R1.4: read stdin when no file given.
		cfg.files = []string{"-"}
	}
	widths := resolveWidths(cfg.specs)
	reader, code := openInputs(cfg.files, stdin, stderr)
	r, startOff := applySkipAndLimit(reader, cfg)
	if dumpErr := dump(r, stdout, cfg.specs, widths, cfg.addrRadix, startOff); dumpErr != nil {
		return 1
	}
	return code
}

// applySkipAndLimit skips input bytes and applies a read limit.
// R2.2: skip bytes. R2.3: limit bytes.
func applySkipAndLimit(r io.Reader, cfg odConfig) (io.Reader, int) {
	if cfg.skipBytes > 0 {
		io.CopyN(io.Discard, r, cfg.skipBytes) //nolint:errcheck // short input is not an error
	}
	if cfg.readBytes >= 0 {
		r = io.LimitReader(r, cfg.readBytes)
	}
	return r, int(cfg.skipBytes)
}

// parseArgs separates flags from file arguments and returns configuration.
func parseArgs(args []string) (odConfig, error) {
	cfg := odConfig{addrRadix: 'o', readBytes: -1}
	done := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if done || len(a) == 0 || (a != "-" && a[0] != '-') {
			cfg.files = append(cfg.files, a)
			continue
		}
		if a == "--" {
			done = true
			continue
		}
		if a == "-" {
			cfg.files = append(cfg.files, a)
			continue
		}
		adv, err := parseFlag(&cfg, a, args, i)
		if err != nil {
			return odConfig{}, err
		}
		i += adv
	}
	return cfg, nil
}

// parseFlag dispatches a single flag argument to the appropriate handler.
func parseFlag(cfg *odConfig, arg string, args []string, idx int) (int, error) {
	// R2.1: -A / --address-radix
	if matchFlag(arg, 'A', "--address-radix") {
		val, adv, err := extractFlagValue(arg, args, idx, 'A', "--address-radix")
		if err != nil {
			return 0, err
		}
		return adv, setAddrRadix(cfg, val)
	}
	// R2.2: -j / --skip-bytes
	if matchFlag(arg, 'j', "--skip-bytes") {
		return parseBytesFlag(arg, args, idx, 'j', "--skip-bytes", &cfg.skipBytes)
	}
	// R2.3: -N / --read-bytes
	if matchFlag(arg, 'N', "--read-bytes") {
		return parseBytesFlag(arg, args, idx, 'N', "--read-bytes", &cfg.readBytes)
	}
	// R1.2: -t / --format (type specifiers)
	s, advance, err := extractTypeArg(arg, args, idx)
	if err != nil {
		return 0, err
	}
	cfg.specs = append(cfg.specs, s...)
	return advance, nil
}

// parseBytesFlag extracts and parses a byte-count flag value via sizeparse.
func parseBytesFlag(
	arg string, args []string, idx int, short byte, long string, dest *int64,
) (int, error) {
	val, adv, err := extractFlagValue(arg, args, idx, short, long)
	if err != nil {
		return 0, err
	}
	n, perr := sizeparse.Parse(val)
	if perr != nil {
		return 0, fmt.Errorf("invalid %s value: %q", long, val)
	}
	*dest = n
	return adv, nil
}

// matchFlag reports whether arg matches short flag -X or long flag --name[=].
func matchFlag(arg string, short byte, long string) bool {
	if arg == long || strings.HasPrefix(arg, long+"=") {
		return true
	}
	return len(arg) >= 2 && arg[0] == '-' && arg[1] == short
}

// extractFlagValue extracts the value for a short (-X) or long (--name) flag.
func extractFlagValue(
	arg string, args []string, idx int, short byte, long string,
) (string, int, error) {
	if strings.HasPrefix(arg, long+"=") {
		return arg[len(long)+1:], 0, nil
	}
	if arg == long {
		if idx+1 >= len(args) {
			return "", 0, fmt.Errorf("option '%s' requires an argument", long)
		}
		return args[idx+1], 1, nil
	}
	// Short form: -Xval or -X val
	if len(arg) > 2 {
		return arg[2:], 0, nil
	}
	if idx+1 >= len(args) {
		return "", 0, fmt.Errorf("option requires an argument -- '%c'", short)
	}
	return args[idx+1], 1, nil
}

// setAddrRadix validates and sets the address radix.
// R2.1: valid radix values are d, o, x, n.
func setAddrRadix(cfg *odConfig, val string) error {
	switch val {
	case "d", "o", "x", "n":
		cfg.addrRadix = val[0]
		return nil
	}
	return fmt.Errorf(
		"invalid output address radix '%s'; it must be one character from [doxn]", val,
	)
}

// extractTypeArg handles -t, -tTYPE, --format=TYPE arguments.
// Returns parsed specs, number of extra args consumed, or error.
func extractTypeArg(arg string, args []string, idx int) ([]typeSpec, int, error) {
	if strings.HasPrefix(arg, "--format=") {
		s, err := parseTypeStr(arg[len("--format="):])
		return s, 0, err
	}
	if arg == "-t" || arg == "--format" {
		if idx+1 >= len(args) {
			return nil, 0, fmt.Errorf("option requires an argument -- 't'")
		}
		s, err := parseTypeStr(args[idx+1])
		return s, 1, err
	}
	if strings.HasPrefix(arg, "-t") {
		s, err := parseTypeStr(arg[2:])
		return s, 0, err
	}
	return nil, 0, fmt.Errorf("unrecognized option '%s'", arg)
}

// parseTypeStr parses a TYPE string into one or more typeSpecs.
// R1.2: TYPE is a letter (a,c,d,f,o,u,x) optionally followed by a size.
func parseTypeStr(s string) ([]typeSpec, error) {
	var specs []typeSpec
	i := 0
	for i < len(s) {
		ts, n, err := specFromLetter(s[i], s[i+1:])
		if err != nil {
			return nil, err
		}
		specs = append(specs, ts)
		i += 1 + n
	}
	if len(specs) == 0 {
		return nil, fmt.Errorf("invalid type string ''")
	}
	return specs, nil
}

// specFromLetter creates a typeSpec from a format letter and optional size.
func specFromLetter(letter byte, rest string) (typeSpec, int, error) {
	switch letter {
	case 'a', 'c':
		return typeSpec{letter: letter, size: 1}, 0, nil
	case 'd', 'o', 'u', 'x':
		size, n := parseIntSize(rest)
		if size == 0 {
			size = 4
		}
		return typeSpec{letter: letter, size: size}, n, nil
	case 'f':
		size, n := parseFloatSize(rest)
		if size == 0 {
			size = 8
		}
		return typeSpec{letter: letter, size: size}, n, nil
	}
	return typeSpec{}, 0, fmt.Errorf("invalid character '%c' in type string", letter)
}

// parseIntSize reads an optional size for integer types (d, o, u, x).
func parseIntSize(s string) (int, int) {
	if len(s) == 0 {
		return 0, 0
	}
	sizes := map[byte]int{
		'1': 1, '2': 2, '4': 4, '8': 8,
		'C': 1, 'S': 2, 'I': 4, 'L': 8,
	}
	if sz, ok := sizes[s[0]]; ok {
		return sz, 1
	}
	return 0, 0
}

// parseFloatSize reads an optional size for float type.
func parseFloatSize(s string) (int, int) {
	if len(s) == 0 {
		return 0, 0
	}
	sizes := map[byte]int{'4': 4, 'F': 4, '8': 8, 'D': 8}
	if sz, ok := sizes[s[0]]; ok {
		return sz, 1
	}
	return 0, 0
}

// naturalWidth returns the total per-value character width for a type spec,
// including the leading space separator.
func naturalWidth(spec typeSpec) int {
	switch spec.letter {
	case 'a', 'c':
		return 4
	case 'o':
		return 1 + octalWidth(spec.size)
	case 'x':
		return 1 + spec.size*2
	case 'u':
		return 1 + unsignedWidth(spec.size)
	case 'd':
		return 1 + signedWidth(spec.size)
	case 'f':
		return floatTotalWidth(spec.size)
	}
	return 0
}

// resolveWidths computes the per-value display width for each spec.
// When multiple specs share a byte size, the max width is used for alignment.
func resolveWidths(specs []typeSpec) []int {
	maxBySize := make(map[int]int)
	for _, s := range specs {
		w := naturalWidth(s)
		if w > maxBySize[s.size] {
			maxBySize[s.size] = w
		}
	}
	widths := make([]int, len(specs))
	for i, s := range specs {
		widths[i] = maxBySize[s.size]
	}
	return widths
}

// openInputs opens file arguments and returns a concatenated reader.
// R1.4: reads "-" as stdin; concatenates multiple files in order.
func openInputs(files []string, stdin io.Reader, stderr io.Writer) (io.Reader, int) {
	var readers []io.Reader
	code := 0
	for _, name := range files {
		if name == "-" {
			readers = append(readers, stdin)
			continue
		}
		f, err := os.Open(name)
		if err != nil {
			fmt.Fprintf(stderr, "od: %s\n", err)
			code = 1
			continue
		}
		readers = append(readers, f)
	}
	return io.MultiReader(readers...), code
}

// addrWidth returns the character width of the address column for a radix.
// R2.1: different radixes have different display widths.
func addrWidth(radix byte) int {
	switch radix {
	case 'o':
		return 7
	case 'd':
		return 7
	case 'x':
		return 6
	}
	return 0 // 'n': no address column
}

// formatAddr formats an offset as an address string in the given radix.
// R2.1: supports octal, decimal, hex, and none.
func formatAddr(offset int, radix byte) string {
	switch radix {
	case 'o':
		return fmt.Sprintf("%07o", offset)
	case 'd':
		return fmt.Sprintf("%07d", offset)
	case 'x':
		return fmt.Sprintf("%06x", offset)
	}
	return ""
}

// dump reads input and writes formatted output.
func dump(
	r io.Reader, w io.Writer, specs []typeSpec, widths []int,
	radix byte, startOffset int,
) error {
	bw := bufio.NewWriter(w)
	aw := addrWidth(radix)
	buf := make([]byte, defaultWidth)
	offset := startOffset
	for {
		n, err := io.ReadFull(r, buf)
		if n > 0 {
			if wErr := writeBlock(bw, buf[:n], specs, widths, offset, radix, aw); wErr != nil {
				return wErr
			}
			offset += n
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return err
		}
	}
	// R2.4: final line with address of byte past the last byte read.
	if radix != 'n' {
		fmt.Fprintf(bw, "%s\n", formatAddr(offset, radix))
	}
	return bw.Flush()
}

// writeBlock writes all type spec lines for one input block.
// R1.3: each -t option produces an additional output line per block.
func writeBlock(
	bw *bufio.Writer, block []byte, specs []typeSpec, widths []int,
	offset int, radix byte, aw int,
) error {
	for i, spec := range specs {
		if i == 0 && radix != 'n' {
			fmt.Fprintf(bw, "%s", formatAddr(offset, radix))
		} else if i > 0 {
			writeSpaces(bw, aw)
		}
		writeTypeLine(bw, block, spec, widths[i])
		bw.WriteByte('\n') //nolint:errcheck
	}
	return nil
}

// writeSpaces writes n space characters to the writer.
func writeSpaces(bw *bufio.Writer, n int) {
	for i := 0; i < n; i++ {
		bw.WriteByte(' ') //nolint:errcheck
	}
}

// writeTypeLine writes formatted values for one type spec across the block.
// Each value is right-justified in a field of the given width.
func writeTypeLine(bw *bufio.Writer, block []byte, spec typeSpec, width int) {
	for pos := 0; pos < len(block); pos += spec.size {
		chunk := paddedChunk(block, pos, spec.size)
		formatted := formatValue(chunk, spec)
		fmt.Fprintf(bw, "%*s", width, formatted)
	}
}

// paddedChunk returns spec.size bytes from block at pos, zero-padding if short.
func paddedChunk(block []byte, pos, size int) []byte {
	end := pos + size
	if end <= len(block) {
		return block[pos:end]
	}
	padded := make([]byte, size)
	copy(padded, block[pos:])
	return padded
}

// formatValue returns the formatted string for a single value (no padding).
func formatValue(data []byte, spec typeSpec) string {
	switch spec.letter {
	case 'a':
		return formatNamedChar(data[0])
	case 'c':
		return formatCChar(data[0])
	case 'o':
		return fmt.Sprintf("%0*o", octalWidth(spec.size), readUint(data, spec.size))
	case 'x':
		return fmt.Sprintf("%0*x", spec.size*2, readUint(data, spec.size))
	case 'u':
		return fmt.Sprintf("%d", readUint(data, spec.size))
	case 'd':
		return fmt.Sprintf("%d", toSigned(readUint(data, spec.size), spec.size))
	case 'f':
		return formatFloat(data, spec.size)
	}
	return ""
}

// formatFloat formats a floating-point value using %g notation matching GNU od.
func formatFloat(data []byte, size int) string {
	if size == 4 {
		bits := binary.LittleEndian.Uint32(data)
		return fmt.Sprintf("%g", math.Float32frombits(bits))
	}
	bits := binary.LittleEndian.Uint64(data)
	return fmt.Sprintf("%.15g", math.Float64frombits(bits))
}

// readUint reads a little-endian unsigned integer of the given byte size.
func readUint(data []byte, size int) uint64 {
	switch size {
	case 1:
		return uint64(data[0])
	case 2:
		return uint64(binary.LittleEndian.Uint16(data))
	case 4:
		return uint64(binary.LittleEndian.Uint32(data))
	case 8:
		return binary.LittleEndian.Uint64(data)
	}
	return 0
}

// toSigned interprets a uint64 as a signed integer of the given byte size.
func toSigned(val uint64, size int) int64 {
	switch size {
	case 1:
		return int64(int8(val))
	case 2:
		return int64(int16(val))
	case 4:
		return int64(int32(val))
	case 8:
		return int64(val)
	}
	return 0
}

// octalWidth returns the zero-padded field width for octal values.
func octalWidth(size int) int {
	w := [9]int{0, 3, 6, 0, 11, 0, 0, 0, 22}
	return w[size]
}

// unsignedWidth returns the field width for unsigned decimal values.
func unsignedWidth(size int) int {
	w := [9]int{0, 3, 5, 0, 10, 0, 0, 0, 20}
	return w[size]
}

// signedWidth returns the field width for signed decimal values.
func signedWidth(size int) int {
	w := [9]int{0, 4, 6, 0, 11, 0, 0, 0, 20}
	return w[size]
}

// floatTotalWidth returns the total per-value width (including space) for floats.
func floatTotalWidth(size int) int {
	if size == 4 {
		return 16
	}
	return 25
}

// controlNames maps ASCII control characters (0-32) to named strings.
var controlNames = [33]string{
	"nul", "soh", "stx", "etx", "eot", "enq", "ack", "bel",
	"bs", "ht", "nl", "vt", "ff", "cr", "so", "si",
	"dle", "dc1", "dc2", "dc3", "dc4", "nak", "syn", "etb",
	"can", "em", "sub", "esc", "fs", "gs", "rs", "us",
	"sp",
}

// formatNamedChar returns the named representation of a byte for type 'a'.
// High bit is stripped per GNU od behavior.
func formatNamedChar(b byte) string {
	b &= 0x7F
	if int(b) < len(controlNames) {
		return controlNames[b]
	}
	if b == 0x7F {
		return "del"
	}
	return string(rune(b))
}

// cEscapes maps bytes to C-style escape strings for type 'c'.
var cEscapes = map[byte]string{
	0x00: `\0`, 0x07: `\a`, 0x08: `\b`, 0x09: `\t`,
	0x0a: `\n`, 0x0b: `\v`, 0x0c: `\f`, 0x0d: `\r`,
}

// formatCChar returns the C-style representation of a byte for type 'c'.
func formatCChar(b byte) string {
	if esc, ok := cEscapes[b]; ok {
		return esc
	}
	if b >= 0x20 && b <= 0x7E {
		return string(rune(b))
	}
	return fmt.Sprintf("%03o", b)
}
