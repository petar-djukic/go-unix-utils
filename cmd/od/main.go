// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/od implements the od utility for dumping file contents in various formats.
// Implements srd072-od R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3, R3.4.
package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// typeSpec represents a parsed -t type specifier (R1.2).
type typeSpec struct {
	format byte // 'a', 'c', 'd', 'f', 'o', 'u', 'x'
	size   int  // bytes per element
}

// odConfig holds parsed command-line options.
type odConfig struct {
	types     []typeSpec
	addrRadix byte  // 'o', 'd', 'x', 'n' — R2.1
	width     int   // bytes per output line — R3.1
	skipBytes int64 // -j — R2.2
	readBytes int64 // -N, -1 means unlimited — R2.3
	showDupes bool  // -v — R3.4
}

func main() {
	sys.InstallSIGPIPEHandler()
	cfg, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "od: %v\n", err)
		os.Exit(1)
	}
	if err := run(cfg, files); err != nil {
		fmt.Fprintf(os.Stderr, "od: %v\n", err)
		os.Exit(1)
	}
}

// parseArgs parses command-line arguments into config, file list, and error.
func parseArgs(args []string) (*odConfig, []string, error) {
	cfg := &odConfig{addrRadix: 'o', width: 16, readBytes: -1}
	var files []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			files = append(files, args[i+1:]...)
			return finalizeCfg(cfg, files)
		case strings.HasPrefix(a, "--"):
			n, err := parseLongOpt(cfg, a, args[i+1:])
			if err != nil {
				return nil, nil, err
			}
			i += n
		case a == "-" || !strings.HasPrefix(a, "-"):
			files = append(files, a)
		default:
			n, err := parseShortOpts(cfg, a[1:], args[i+1:])
			if err != nil {
				return nil, nil, err
			}
			i += n
		}
	}
	return finalizeCfg(cfg, files)
}

// finalizeCfg applies default type spec if none were given (R1.1).
func finalizeCfg(cfg *odConfig, files []string) (*odConfig, []string, error) {
	if len(cfg.types) == 0 {
		cfg.types = []typeSpec{{format: 'o', size: 2}}
	}
	return cfg, files, nil
}

// parseLongOpt handles --option and --option=value forms.
func parseLongOpt(cfg *odConfig, arg string, rest []string) (int, error) {
	if idx := strings.IndexByte(arg, '='); idx > 0 {
		return 0, applyLongOpt(cfg, arg[:idx], arg[idx+1:])
	}
	switch arg {
	case "--output-duplicates":
		cfg.showDupes = true
		return 0, nil
	case "--width":
		cfg.width = 32
		return 0, nil
	case "--format", "--address-radix", "--skip-bytes", "--read-bytes":
		if len(rest) == 0 {
			return 0, fmt.Errorf("option %q requires an argument", arg)
		}
		return 1, applyLongOpt(cfg, arg, rest[0])
	default:
		return 0, fmt.Errorf("unrecognized option %q", arg)
	}
}

// applyLongOpt applies a long option with its value.
func applyLongOpt(cfg *odConfig, opt, val string) error {
	switch opt {
	case "--format":
		return applyArgOpt(cfg, 't', val)
	case "--address-radix":
		return applyArgOpt(cfg, 'A', val)
	case "--skip-bytes":
		return applyArgOpt(cfg, 'j', val)
	case "--read-bytes":
		return applyArgOpt(cfg, 'N', val)
	case "--width":
		w, err := strconv.Atoi(val)
		if err != nil || w <= 0 {
			return fmt.Errorf("invalid width: %q", val)
		}
		cfg.width = w
		return nil
	default:
		return fmt.Errorf("unrecognized option %q", opt)
	}
}

// parseShortOpts processes a short option group (e.g., "tx1" from "-tx1").
func parseShortOpts(cfg *odConfig, opts string, rest []string) (int, error) {
	consumed := 0
	for j := 0; j < len(opts); j++ {
		c := opts[j]
		remaining := opts[j+1:]
		switch {
		case c == 'v':
			cfg.showDupes = true
		case isTraditionalOpt(c):
			cfg.types = append(cfg.types, traditionalType(c))
		case c == 't' || c == 'A' || c == 'j' || c == 'N':
			val, n, err := optArgValue(remaining, rest, consumed)
			if err != nil {
				return 0, fmt.Errorf("option requires an argument -- '%c'", c)
			}
			consumed = n
			if err := applyArgOpt(cfg, c, val); err != nil {
				return 0, err
			}
			return consumed, nil
		case c == 'w':
			if remaining == "" {
				cfg.width = 32
			} else {
				w, err := strconv.Atoi(remaining)
				if err != nil || w <= 0 {
					return 0, fmt.Errorf("invalid width: %q", remaining)
				}
				cfg.width = w
			}
			return consumed, nil
		default:
			return 0, fmt.Errorf("unrecognized option '-%c'", c)
		}
	}
	return consumed, nil
}

func isTraditionalOpt(c byte) bool {
	return c == 'b' || c == 'c' || c == 'd' || c == 'o' || c == 's' || c == 'x'
}

// traditionalType maps short option aliases to type specifiers (R3.2).
func traditionalType(c byte) typeSpec {
	switch c {
	case 'b':
		return typeSpec{format: 'o', size: 1}
	case 'c':
		return typeSpec{format: 'c', size: 1}
	case 'd':
		return typeSpec{format: 'u', size: 2}
	case 'o':
		return typeSpec{format: 'o', size: 2}
	case 's':
		return typeSpec{format: 'd', size: 2}
	case 'x':
		return typeSpec{format: 'x', size: 2}
	}
	return typeSpec{}
}

// optArgValue gets the argument for an option from the remaining option
// group chars or the next command-line argument.
func optArgValue(remaining string, rest []string, consumed int) (string, int, error) {
	if remaining != "" {
		return remaining, consumed, nil
	}
	if consumed >= len(rest) {
		return "", consumed, fmt.Errorf("missing argument")
	}
	return rest[consumed], consumed + 1, nil
}

// applyArgOpt dispatches an option character with its value to the config.
func applyArgOpt(cfg *odConfig, opt byte, val string) error {
	switch opt {
	case 't':
		ts, err := parseTypeSpec(val)
		if err != nil {
			return err
		}
		cfg.types = append(cfg.types, ts)
	case 'A':
		return setAddrRadix(cfg, val)
	case 'j':
		n, err := parseByteCount(val)
		if err != nil {
			return fmt.Errorf("invalid --skip-bytes argument %q", val)
		}
		cfg.skipBytes = n
	case 'N':
		n, err := parseByteCount(val)
		if err != nil {
			return fmt.Errorf("invalid --read-bytes argument %q", val)
		}
		cfg.readBytes = n
	}
	return nil
}

// parseTypeSpec parses a type string like "x1", "o2", "c" (R1.2).
func parseTypeSpec(s string) (typeSpec, error) {
	if len(s) == 0 {
		return typeSpec{}, fmt.Errorf("empty type specification")
	}
	f := s[0]
	rest := s[1:]
	switch f {
	case 'a':
		return typeSpec{format: 'a', size: 1}, nil
	case 'c':
		return typeSpec{format: 'c', size: 1}, nil
	case 'd', 'o', 'u', 'x':
		sz, err := parseIntSize(rest)
		if err != nil {
			return typeSpec{}, fmt.Errorf("invalid type %q: %w", s, err)
		}
		return typeSpec{format: f, size: sz}, nil
	case 'f':
		sz, err := parseFloatSize(rest)
		if err != nil {
			return typeSpec{}, fmt.Errorf("invalid type %q: %w", s, err)
		}
		return typeSpec{format: f, size: sz}, nil
	default:
		return typeSpec{}, fmt.Errorf("invalid type character '%c'", f)
	}
}

func parseIntSize(s string) (int, error) {
	if s == "" {
		return 4, nil
	}
	switch s {
	case "C", "1":
		return 1, nil
	case "S", "2":
		return 2, nil
	case "I", "4":
		return 4, nil
	case "L", "8":
		return 8, nil
	default:
		return 0, fmt.Errorf("invalid size %q", s)
	}
}

func parseFloatSize(s string) (int, error) {
	if s == "" {
		return 8, nil
	}
	switch s {
	case "F", "4":
		return 4, nil
	case "D", "8":
		return 8, nil
	default:
		return 0, fmt.Errorf("invalid float size %q", s)
	}
}

// R2.1: set address offset format.
func setAddrRadix(cfg *odConfig, val string) error {
	if len(val) != 1 {
		return fmt.Errorf("invalid address radix %q", val)
	}
	switch val[0] {
	case 'd', 'o', 'x', 'n':
		cfg.addrRadix = val[0]
		return nil
	default:
		return fmt.Errorf("invalid address radix %q", val)
	}
}

// R2.2: parse byte count with optional multiplier suffix (b=512, k=1024, m=1048576).
func parseByteCount(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty byte count")
	}
	mul := int64(1)
	numStr := s
	last := s[len(s)-1]
	switch {
	case last == 'b':
		mul, numStr = 512, s[:len(s)-1]
	case last == 'k' || last == 'K':
		mul, numStr = 1024, s[:len(s)-1]
	case last == 'm' || last == 'M':
		mul, numStr = 1048576, s[:len(s)-1]
	}
	n, err := parseNumber(numStr)
	if err != nil {
		return 0, fmt.Errorf("invalid byte count %q: %w", s, err)
	}
	return n * mul, nil
}

// parseNumber parses an integer with 0x (hex) or 0 (octal) prefix support.
func parseNumber(s string) (int64, error) {
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return strconv.ParseInt(s[2:], 16, 64)
	}
	if len(s) > 1 && s[0] == '0' {
		return strconv.ParseInt(s, 8, 64)
	}
	return strconv.ParseInt(s, 10, 64)
}

// run opens inputs, applies skip/limit, and processes chunks.
func run(cfg *odConfig, files []string) error {
	reader, closer, err := openInputs(files)
	if err != nil {
		return err
	}
	defer closer()
	// R2.2: skip bytes before formatting.
	if cfg.skipBytes > 0 {
		skipped, skipErr := io.CopyN(io.Discard, reader, cfg.skipBytes)
		if skipErr != nil && skipped == 0 {
			return fmt.Errorf("cannot skip past end of combined input")
		}
	}
	// R2.3: limit read bytes.
	var r io.Reader = reader
	if cfg.readBytes >= 0 {
		r = io.LimitReader(reader, cfg.readBytes)
	}
	return processChunks(cfg, r)
}

// openInputs returns a combined reader for all input files/stdin (R1.4).
func openInputs(files []string) (io.Reader, func(), error) {
	if len(files) == 0 {
		return os.Stdin, func() {}, nil
	}
	var readers []io.Reader
	var closers []io.Closer
	for _, f := range files {
		if f == "-" {
			readers = append(readers, os.Stdin)
			continue
		}
		fh, err := os.Open(f)
		if err != nil {
			for _, c := range closers {
				c.Close() // best-effort cleanup
			}
			return nil, nil, err
		}
		readers = append(readers, fh)
		closers = append(closers, fh)
	}
	return io.MultiReader(readers...), func() {
		for _, c := range closers {
			c.Close() // best-effort cleanup
		}
	}, nil
}

// processChunks reads width-byte chunks and outputs formatted lines.
func processChunks(cfg *odConfig, r io.Reader) error {
	buf := make([]byte, cfg.width)
	offset := cfg.skipBytes
	cpb := charsPerByte(cfg.types)
	var prevChunk []byte
	suppressing := false
	for {
		n, readErr := io.ReadFull(r, buf)
		if n == 0 {
			break
		}
		chunk := make([]byte, n)
		copy(chunk, buf[:n])
		isDup := !cfg.showDupes && n == cfg.width && bytesEqual(chunk, prevChunk)
		if isDup {
			if !suppressing {
				fmt.Println("*")
				suppressing = true
			}
		} else {
			suppressing = false
			printChunk(cfg, offset, chunk, cpb)
		}
		prevChunk = chunk
		offset += int64(n)
		if readErr != nil {
			break
		}
	}
	// R2.4: final line with address past last byte.
	if cfg.addrRadix != 'n' {
		fmt.Println(formatAddress(offset, cfg.addrRadix))
	}
	return nil
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// printChunk outputs one or more formatted lines for a data chunk.
func printChunk(cfg *odConfig, offset int64, data []byte, cpb int) {
	aw := addrWidth(cfg.addrRadix)
	for i, ts := range cfg.types {
		var sb strings.Builder
		if cfg.addrRadix != 'n' {
			if i == 0 {
				sb.WriteString(formatAddress(offset, cfg.addrRadix))
			} else {
				writeSpaces(&sb, aw)
			}
		}
		elemW := elementWidth(ts, cpb, len(cfg.types) > 1)
		writeTypeLine(&sb, data, ts, elemW)
		fmt.Println(sb.String())
	}
}

func writeSpaces(sb *strings.Builder, n int) {
	for range n {
		sb.WriteByte(' ')
	}
}

// charsPerByte computes aligned display chars per input byte for multi-type.
func charsPerByte(types []typeSpec) int {
	if len(types) <= 1 {
		return 0
	}
	maxCPB := 0
	for _, ts := range types {
		nw := naturalWidth(ts)
		cpb := (nw + ts.size - 1) / ts.size
		if cpb > maxCPB {
			maxCPB = cpb
		}
	}
	return maxCPB
}

// naturalWidth returns the natural field width including leading space.
func naturalWidth(ts typeSpec) int {
	switch ts.format {
	case 'a', 'c':
		return 4
	case 'o':
		return 1 + octalDigits(ts.size)
	case 'x':
		return 1 + ts.size*2
	case 'u':
		return 1 + unsignedWidth(ts.size)
	case 'd':
		return 1 + signedWidth(ts.size)
	case 'f':
		return 1 + floatWidth(ts.size)
	}
	return 4
}

// elementWidth returns the display width for a single element.
func elementWidth(ts typeSpec, cpb int, multiType bool) int {
	if !multiType {
		return naturalWidth(ts)
	}
	return cpb * ts.size
}

// writeTypeLine writes formatted elements for one type spec into sb.
func writeTypeLine(sb *strings.Builder, data []byte, ts typeSpec, elemW int) {
	for i := 0; i < len(data); i += ts.size {
		end := i + ts.size
		if end > len(data) {
			end = len(data)
		}
		elem := make([]byte, ts.size)
		copy(elem, data[i:end])
		sb.WriteString(formatElement(elem, ts, elemW))
	}
}

// formatElement formats a single data element with the given field width.
func formatElement(data []byte, ts typeSpec, width int) string {
	valW := width - 1
	switch ts.format {
	case 'a':
		return fmt.Sprintf(" %*s", valW, namedChar(data[0]))
	case 'c':
		return fmt.Sprintf(" %*s", valW, cChar(data[0]))
	case 'o':
		d := octalDigits(ts.size)
		s := fmt.Sprintf("%0*o", d, readUint(data, ts.size))
		return fmt.Sprintf(" %*s", valW, s)
	case 'x':
		d := ts.size * 2
		s := fmt.Sprintf("%0*x", d, readUint(data, ts.size))
		return fmt.Sprintf(" %*s", valW, s)
	case 'u':
		s := fmt.Sprintf("%d", readUint(data, ts.size))
		return fmt.Sprintf(" %*s", valW, s)
	case 'd':
		s := fmt.Sprintf("%d", readSigned(data, ts.size))
		return fmt.Sprintf(" %*s", valW, s)
	case 'f':
		return formatFloatElem(data, ts.size, valW)
	}
	return ""
}

func formatFloatElem(data []byte, size, valW int) string {
	switch size {
	case 4:
		v := math.Float32frombits(binary.LittleEndian.Uint32(data))
		return fmt.Sprintf(" %*.7g", valW, float64(v))
	case 8:
		v := math.Float64frombits(binary.LittleEndian.Uint64(data))
		return fmt.Sprintf(" %*.17g", valW, v)
	}
	return ""
}

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

func readSigned(data []byte, size int) int64 {
	switch size {
	case 1:
		return int64(int8(data[0]))
	case 2:
		return int64(int16(binary.LittleEndian.Uint16(data)))
	case 4:
		return int64(int32(binary.LittleEndian.Uint32(data)))
	case 8:
		return int64(binary.LittleEndian.Uint64(data))
	}
	return 0
}

// R2.1: format address offset per radix.
func formatAddress(offset int64, radix byte) string {
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

func addrWidth(radix byte) int {
	switch radix {
	case 'o':
		return 7
	case 'd':
		return 7
	case 'x':
		return 6
	}
	return 0
}

func octalDigits(size int) int {
	switch size {
	case 1:
		return 3
	case 2:
		return 6
	case 4:
		return 11
	case 8:
		return 22
	}
	return 3
}

func unsignedWidth(size int) int {
	switch size {
	case 1:
		return 3
	case 2:
		return 5
	case 4:
		return 10
	case 8:
		return 20
	}
	return 3
}

func signedWidth(size int) int {
	switch size {
	case 1:
		return 4
	case 2:
		return 6
	case 4:
		return 11
	case 8:
		return 20
	}
	return 4
}

func floatWidth(size int) int {
	switch size {
	case 4:
		return 15
	case 8:
		return 25
	}
	return 15
}

// namedChars maps ASCII control characters to standard names (type 'a').
var namedChars = [33]string{
	"nul", "soh", "stx", "etx", "eot", "enq", "ack", "bel",
	"bs", "ht", "nl", "vt", "ff", "cr", "so", "si",
	"dle", "dc1", "dc2", "dc3", "dc4", "nak", "syn", "etb",
	"can", "em", "sub", "esc", "fs", "gs", "rs", "us",
	"sp",
}

func namedChar(b byte) string {
	b &= 0x7f
	if b < 33 {
		return namedChars[b]
	}
	if b == 127 {
		return "del"
	}
	return string(rune(b))
}

func cChar(b byte) string {
	switch b {
	case 0:
		return "\\0"
	case 7:
		return "\\a"
	case 8:
		return "\\b"
	case 9:
		return "\\t"
	case 10:
		return "\\n"
	case 11:
		return "\\v"
	case 12:
		return "\\f"
	case 13:
		return "\\r"
	default:
		if b >= 32 && b < 127 {
			return string(rune(b))
		}
		return fmt.Sprintf("%03o", b)
	}
}
