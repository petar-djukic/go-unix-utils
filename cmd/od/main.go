// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

type typeSpec struct {
	format byte
	size   int
}

type options struct {
	types       []typeSpec
	addrRadix   byte
	outputWidth int
	verbose     bool
	skipBytes   int64
	readBytes   int64
	readLimit   bool
}

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

func main() {
	sys.InstallSIGPIPEHandler()
	opts, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "od: %s\n", err)
		os.Exit(1)
	}
	if len(opts.types) == 0 {
		opts.types = []typeSpec{{format: 'o', size: 2}}
	}
	if opts.outputWidth == 0 {
		opts.outputWidth = 16
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	exitCode := run(opts, files)
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func run(opts options, files []string) int {
	reader, exitCode := openInputs(files)
	if exitCode != 0 {
		return exitCode
	}
	if closer, ok := reader.(io.Closer); ok {
		defer closer.Close()
	}
	var r io.Reader = reader
	if opts.skipBytes > 0 {
		if _, err := io.CopyN(io.Discard, r, opts.skipBytes); err != nil {
			return 0
		}
	}
	if opts.readLimit {
		r = io.LimitReader(r, opts.readBytes)
	}
	return dump(opts, r)
}

func dump(opts options, r io.Reader) int {
	buf := make([]byte, opts.outputWidth)
	var prevLine []byte
	duplicating := false
	offset := opts.skipBytes
	layouts := computeLayouts(opts.types)
	for {
		n, err := io.ReadFull(r, buf)
		if n == 0 {
			break
		}
		line := buf[:n]
		if !opts.verbose && prevLine != nil && n == len(buf) && equalBytes(prevLine, line) {
			if !duplicating {
				fmt.Println("*")
				duplicating = true
			}
			offset += int64(n)
			continue
		}
		duplicating = false
		printBlock(opts, offset, line, layouts)
		offset += int64(n)
		if prevLine == nil {
			prevLine = make([]byte, len(buf))
		}
		copy(prevLine, line)
		if err != nil {
			break
		}
	}
	if opts.addrRadix != 'n' {
		printAddress(opts.addrRadix, offset)
		fmt.Println()
	}
	return 0
}

func printBlock(opts options, offset int64, data []byte, layouts [][]int) {
	for i, ts := range opts.types {
		if i == 0 {
			printAddress(opts.addrRadix, offset)
		} else {
			printAddressPadding(opts.addrRadix)
		}
		formatLine(ts, data, layouts[i])
		fmt.Println()
	}
}

func printAddress(radix byte, offset int64) {
	switch radix {
	case 'o':
		fmt.Printf("%07o", offset)
	case 'x':
		fmt.Printf("%06x", offset)
	case 'd':
		fmt.Printf("%07d", offset)
	}
}

func printAddressPadding(radix byte) {
	switch radix {
	case 'o', 'd':
		fmt.Print("       ")
	case 'x':
		fmt.Print("      ")
	}
}

func computeLayouts(types []typeSpec) [][]int {
	bpc := computeBytesPerCol(types)
	colWidth := computeColWidth(types, bpc)
	layouts := make([][]int, len(types))
	for i, ts := range types {
		layouts[i] = allocFieldWidths(colWidth, bpc/ts.size)
	}
	return layouts
}

func computeBytesPerCol(types []typeSpec) int {
	result := 1
	for _, ts := range types {
		result = lcm(result, ts.size)
	}
	return result
}

func computeColWidth(types []typeSpec, bpc int) int {
	maxW := 0
	for _, ts := range types {
		w := (bpc / ts.size) * nativeFieldWidth(ts)
		if w > maxW {
			maxW = w
		}
	}
	return maxW
}

func allocFieldWidths(colWidth, numFields int) []int {
	widths := make([]int, numFields)
	base := colWidth / numFields
	extra := colWidth % numFields
	for i := range widths {
		widths[i] = base
		if i < extra {
			widths[i]++
		}
	}
	return widths
}

func nativeFieldWidth(ts typeSpec) int {
	switch ts.format {
	case 'a', 'c':
		return 4
	case 'o':
		return [9]int{0, 4, 7, 0, 12, 0, 0, 0, 23}[ts.size]
	case 'x':
		return ts.size*2 + 1
	case 'u':
		return [9]int{0, 4, 6, 0, 11, 0, 0, 0, 21}[ts.size]
	case 'd':
		return [9]int{0, 5, 7, 0, 12, 0, 0, 0, 21}[ts.size]
	case 'f':
		if ts.size == 4 {
			return 16
		}
		return 25
	}
	return 4
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func lcm(a, b int) int { return a / gcd(a, b) * b }

func formatLine(ts typeSpec, data []byte, fieldWidths []int) {
	fieldsPerCol := len(fieldWidths)
	fieldIdx := 0
	for i := 0; i < len(data); i += ts.size {
		end := i + ts.size
		if end > len(data) {
			padded := make([]byte, ts.size)
			copy(padded, data[i:])
			fmt.Printf("%*s", fieldWidths[fieldIdx], formatValue(ts, padded))
		} else {
			fmt.Printf("%*s", fieldWidths[fieldIdx], formatValue(ts, data[i:end]))
		}
		fieldIdx++
		if fieldIdx >= fieldsPerCol {
			fieldIdx = 0
		}
	}
}

func formatValue(ts typeSpec, data []byte) string {
	switch ts.format {
	case 'a':
		return namedChars[data[0]&0x7f]
	case 'c':
		return cCharRepr(data[0])
	case 'o':
		return fmt.Sprintf("%0*o", octalDigits(ts.size), readUint(data))
	case 'x':
		return fmt.Sprintf("%0*x", ts.size*2, readUint(data))
	case 'u':
		return fmt.Sprintf("%d", readUint(data))
	case 'd':
		return fmt.Sprintf("%d", signExtend(readUint(data), ts.size))
	case 'f':
		return formatFloat(data, ts.size)
	}
	return ""
}

func octalDigits(size int) int {
	return [9]int{0, 3, 6, 0, 11, 0, 0, 0, 22}[size]
}

func cCharRepr(b byte) string {
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
	case 0x5c:
		return "\\"
	}
	if b >= 0x20 && b < 0x7f {
		return string(rune(b))
	}
	return fmt.Sprintf("%03o", b)
}

func signExtend(v uint64, size int) int64 {
	switch size {
	case 1:
		return int64(int8(v))
	case 2:
		return int64(int16(v))
	case 4:
		return int64(int32(v))
	}
	return int64(v)
}

func formatFloat(data []byte, size int) string {
	switch size {
	case 4:
		f := math.Float32frombits(binary.LittleEndian.Uint32(data[:4]))
		return strconv.FormatFloat(float64(f), 'g', -1, 32)
	case 8:
		f := math.Float64frombits(binary.LittleEndian.Uint64(data[:8]))
		return strconv.FormatFloat(f, 'g', -1, 64)
	}
	return "0"
}

func readUint(b []byte) uint64 {
	switch len(b) {
	case 1:
		return uint64(b[0])
	case 2:
		return uint64(binary.LittleEndian.Uint16(b))
	case 4:
		return uint64(binary.LittleEndian.Uint32(b))
	case 8:
		return binary.LittleEndian.Uint64(b)
	}
	return 0
}

func equalBytes(a, b []byte) bool {
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

func openInputs(files []string) (io.Reader, int) {
	if len(files) == 1 {
		return openSingle(files[0])
	}
	readers := make([]io.Reader, 0, len(files))
	for _, name := range files {
		if name == "-" {
			readers = append(readers, os.Stdin)
			continue
		}
		f, err := os.Open(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "od: %s: No such file or directory\n", name)
			return nil, 1
		}
		readers = append(readers, f)
	}
	return io.MultiReader(readers...), 0
}

func openSingle(name string) (io.Reader, int) {
	if name == "-" {
		return os.Stdin, 0
	}
	f, err := os.Open(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "od: %s: No such file or directory\n", name)
		return nil, 1
	}
	return f, 0
}
