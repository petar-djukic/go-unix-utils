// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/tr implements srd054-tr: translate or delete characters.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	args := parseArgs(os.Args[1:])
	if err := run(args); err != nil {
		fmt.Fprintf(os.Stderr, "tr: %s\n", err)
		os.Exit(1)
	}
}

func parseArgs(args []string) []string {
	var operands []string
	endOfFlags := false
	for _, arg := range args {
		if !endOfFlags && arg == "--" {
			endOfFlags = true
			continue
		}
		operands = append(operands, arg)
	}
	return operands
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing operand")
	}
	if len(args) < 2 {
		return fmt.Errorf("missing operand after '%s'", args[0])
	}
	set1, err := expandSet(args[0])
	if err != nil {
		return err
	}
	set2, err := expandSet(args[1])
	if err != nil {
		return err
	}
	if len(set2) == 0 {
		return fmt.Errorf("when not truncating set1, string2 must be non-empty")
	}
	table := buildTable(set1, set2)
	return translate(table)
}

func buildTable(set1, set2 []byte) [256]byte {
	var table [256]byte
	for i := range table {
		table[i] = byte(i)
	}
	for i, b := range set1 {
		if i < len(set2) {
			table[b] = set2[i]
		} else {
			table[b] = set2[len(set2)-1]
		}
	}
	return table
}

func translate(table [256]byte) error {
	r := bufio.NewReader(os.Stdin)
	w := bufio.NewWriter(os.Stdout)
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		for i := range n {
			buf[i] = table[buf[i]]
		}
		if _, werr := w.Write(buf[:n]); werr != nil {
			return werr
		}
		if err != nil {
			if err != io.EOF {
				return err
			}
			break
		}
	}
	return w.Flush()
}

func expandSet(spec string) ([]byte, error) {
	var result []byte
	i := 0
	for i < len(spec) {
		if spec[i] == '[' {
			n, bytes, err := tryBracketExpr(spec, i)
			if err != nil {
				return nil, err
			}
			if n > 0 {
				result = append(result, bytes...)
				i += n
				continue
			}
		}
		ch, n := nextByte(spec, i)
		rn, rangeBytes, err := tryRange(spec, i, ch, n)
		if err != nil {
			return nil, err
		}
		if rn > 0 {
			result = append(result, rangeBytes...)
			i += rn
			continue
		}
		result = append(result, ch)
		i += n
	}
	return result, nil
}

func tryRange(spec string, i int, lo byte, loN int) (int, []byte, error) {
	dashPos := i + loN
	if dashPos >= len(spec) || spec[dashPos] != '-' {
		return 0, nil, nil
	}
	if dashPos+1 >= len(spec) {
		return 0, nil, nil
	}
	hi, hiN := nextByte(spec, dashPos+1)
	if lo > hi {
		return 0, nil, fmt.Errorf(
			"range-endpoints of '%c-%c' are in reverse collating sequence order",
			rune(lo), rune(hi))
	}
	return loN + 1 + hiN, byteRange(lo, hi), nil
}

func tryBracketExpr(spec string, i int) (int, []byte, error) {
	if strings.HasPrefix(spec[i:], "[:") {
		return parseCharClass(spec, i)
	}
	if strings.HasPrefix(spec[i:], "[=") {
		return parseEquivClass(spec, i)
	}
	return tryRepeat(spec, i)
}

func parseCharClass(spec string, i int) (int, []byte, error) {
	end := strings.Index(spec[i:], ":]")
	if end < 0 {
		return 0, nil, nil
	}
	className := spec[i+2 : i+end]
	bytes, err := expandCharClass(className)
	if err != nil {
		return 0, nil, err
	}
	return end + 2, bytes, nil
}

func expandCharClass(name string) ([]byte, error) {
	switch name {
	case "upper":
		return byteRange('A', 'Z'), nil
	case "lower":
		return byteRange('a', 'z'), nil
	case "digit":
		return byteRange('0', '9'), nil
	case "alpha":
		return append(byteRange('A', 'Z'), byteRange('a', 'z')...), nil
	case "alnum":
		r := byteRange('0', '9')
		r = append(r, byteRange('A', 'Z')...)
		r = append(r, byteRange('a', 'z')...)
		return r, nil
	case "blank":
		return []byte{'\t', ' '}, nil
	case "space":
		return []byte{'\t', '\n', '\v', '\f', '\r', ' '}, nil
	case "cntrl":
		return append(byteRange(0, 31), 127), nil
	case "print":
		return byteRange(32, 126), nil
	case "graph":
		return byteRange(33, 126), nil
	case "punct":
		return expandPunct(), nil
	case "xdigit":
		r := byteRange('0', '9')
		r = append(r, byteRange('A', 'F')...)
		r = append(r, byteRange('a', 'f')...)
		return r, nil
	default:
		return nil, fmt.Errorf("invalid character class '%s'", name)
	}
}

func expandPunct() []byte {
	var result []byte
	for b := byte(33); b <= 126; b++ {
		if !isAlnum(b) {
			result = append(result, b)
		}
	}
	return result
}

func isAlnum(b byte) bool {
	return (b >= '0' && b <= '9') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= 'a' && b <= 'z')
}

func parseEquivClass(spec string, i int) (int, []byte, error) {
	end := strings.Index(spec[i:], "=]")
	if end < 0 {
		return 0, nil, nil
	}
	ch := spec[i+2]
	return end + 2, []byte{ch}, nil
}

func tryRepeat(spec string, i int) (int, []byte, error) {
	end := strings.Index(spec[i:], "]")
	if end < 0 {
		return 0, nil, nil
	}
	inner := spec[i+1 : i+end]
	if len(inner) < 2 {
		return 0, nil, nil
	}
	ch, n := nextByte(inner, 0)
	if n >= len(inner) || inner[n] != '*' {
		return 0, nil, nil
	}
	countStr := inner[n+1:]
	bytes, err := makeRepeat(ch, countStr)
	if err != nil {
		return 0, nil, err
	}
	return end + 1, bytes, nil
}

func makeRepeat(ch byte, countStr string) ([]byte, error) {
	if countStr == "" {
		return []byte{ch}, nil
	}
	base := 10
	if strings.HasPrefix(countStr, "0") && len(countStr) > 1 {
		base = 8
	}
	count, err := strconv.ParseInt(countStr, base, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid repeat count '%s'", countStr)
	}
	result := make([]byte, count)
	for j := range result {
		result[j] = ch
	}
	return result, nil
}

func nextByte(spec string, i int) (byte, int) {
	if spec[i] != '\\' || i+1 >= len(spec) {
		return spec[i], 1
	}
	c := spec[i+1]
	if c >= '0' && c <= '7' {
		return parseOctal(spec, i)
	}
	if ch, ok := escapeChar(c); ok {
		return ch, 2
	}
	return c, 2
}

func escapeChar(c byte) (byte, bool) {
	switch c {
	case 'a':
		return '\a', true
	case 'b':
		return '\b', true
	case 'f':
		return '\f', true
	case 'n':
		return '\n', true
	case 'r':
		return '\r', true
	case 't':
		return '\t', true
	case 'v':
		return '\v', true
	case '\\':
		return '\\', true
	default:
		return 0, false
	}
}

func parseOctal(spec string, i int) (byte, int) {
	j := i + 1
	for j < len(spec) && j < i+4 && spec[j] >= '0' && spec[j] <= '7' {
		j++
	}
	val, _ := strconv.ParseUint(spec[i+1:j], 8, 16)
	return byte(val), j - i
}

func byteRange(lo, hi byte) []byte {
	if lo > hi {
		return nil
	}
	result := make([]byte, int(hi)-int(lo)+1)
	for i := range result {
		result[i] = lo + byte(i)
	}
	return result
}
