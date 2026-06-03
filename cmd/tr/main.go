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

type config struct {
	delete     bool
	squeeze    bool
	complement bool
	operands   []string
}

func main() {
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "tr: %s\n", err)
		os.Exit(1)
	}
	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "tr: %s\n", err)
		os.Exit(1)
	}
}

func parseArgs(args []string) (config, error) {
	var cfg config
	endOfFlags := false
	for _, arg := range args {
		if endOfFlags || arg == "-" {
			cfg.operands = append(cfg.operands, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if arg == "--delete" {
			cfg.delete = true
			continue
		}
		if arg == "--squeeze-repeats" {
			cfg.squeeze = true
			continue
		}
		if arg == "--complement" {
			cfg.complement = true
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			if arg[1] == '-' {
				return cfg, fmt.Errorf("unrecognized option '%s'", arg)
			}
			if err := parseShortFlags(&cfg, arg[1:]); err != nil {
				return cfg, err
			}
			continue
		}
		cfg.operands = append(cfg.operands, arg)
	}
	return cfg, nil
}

func parseShortFlags(cfg *config, flags string) error {
	for _, c := range flags {
		switch c {
		case 'd':
			cfg.delete = true
		case 's':
			cfg.squeeze = true
		case 'c', 'C':
			cfg.complement = true
		default:
			return fmt.Errorf("invalid option -- '%c'", c)
		}
	}
	return nil
}

func validateOperands(cfg config) error {
	n := len(cfg.operands)
	if n == 0 {
		return fmt.Errorf("missing operand")
	}
	if cfg.delete && !cfg.squeeze {
		if n > 1 {
			return fmt.Errorf("extra operand '%s'", cfg.operands[1])
		}
		return nil
	}
	if cfg.squeeze && !cfg.delete && n == 1 {
		return nil
	}
	if n < 2 {
		return fmt.Errorf("missing operand after '%s'", cfg.operands[0])
	}
	return nil
}

func run(cfg config) error {
	if err := validateOperands(cfg); err != nil {
		return err
	}
	set1, err := expandSet(cfg.operands[0])
	if err != nil {
		return err
	}
	if cfg.complement {
		set1 = complementSet(set1)
	}
	set1Members := memberSet(set1)
	if cfg.delete {
		if !cfg.squeeze {
			return process(nil, &set1Members, nil)
		}
		set2, err := expandSet(cfg.operands[1])
		if err != nil {
			return err
		}
		sqSet := memberSet(set2)
		return process(nil, &set1Members, &sqSet)
	}
	if cfg.squeeze && len(cfg.operands) == 1 {
		return process(nil, nil, &set1Members)
	}
	if hasEquivClass(cfg.operands[1]) {
		return fmt.Errorf("[=c=] expressions may not appear in string2 when translating")
	}
	set2, err := expandSet(cfg.operands[1])
	if err != nil {
		return err
	}
	if len(set2) == 0 {
		return fmt.Errorf("when not truncating set1, string2 must be non-empty")
	}
	table := buildTable(set1, set2)
	if cfg.squeeze {
		sqSet := memberSet(set2)
		return process(&table, nil, &sqSet)
	}
	return process(&table, nil, nil)
}

func complementSet(set []byte) []byte {
	var present [256]bool
	for _, b := range set {
		present[b] = true
	}
	var result []byte
	for i := range 256 {
		if !present[byte(i)] {
			result = append(result, byte(i))
		}
	}
	return result
}

func memberSet(set []byte) [256]bool {
	var m [256]bool
	for _, b := range set {
		m[b] = true
	}
	return m
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

func process(table *[256]byte, deleteSet, squeezeSet *[256]bool) error {
	r := bufio.NewReader(os.Stdin)
	w := bufio.NewWriter(os.Stdout)
	prev := -1
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		for i := range n {
			b := buf[i]
			if deleteSet != nil && deleteSet[b] {
				continue
			}
			if table != nil {
				b = table[b]
			}
			if squeezeSet != nil && squeezeSet[b] && int(b) == prev {
				continue
			}
			prev = int(b)
			if werr := w.WriteByte(b); werr != nil {
				return werr
			}
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

func hasEquivClass(spec string) bool {
	for i := 0; i < len(spec); i++ {
		if strings.HasPrefix(spec[i:], "[=") && strings.Contains(spec[i:], "=]") {
			return true
		}
	}
	return false
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
