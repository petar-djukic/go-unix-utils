// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

type patternKind int

const (
	kindRegexp patternKind = iota
	kindSkip
	kindInteger
)

type pattern struct {
	kind    patternKind
	regex   *regexp.Regexp
	lineNum int
	raw     string
}

type splitter struct {
	lines   []string
	pos     int
	fileNum int
	prefix  string
	digits  int
	created []string
}

func main() {
	sys.InstallSIGPIPEHandler()
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 2 {
		if len(args) == 0 {
			return fmt.Errorf("csplit: missing operand")
		}
		return fmt.Errorf("csplit: missing operand after '%s'", args[0])
	}
	pats, err := parsePatterns(args[1:])
	if err != nil {
		return err
	}
	data, err := readInput(args[0])
	if err != nil {
		return err
	}
	s := &splitter{
		lines:  splitToLines(data),
		prefix: "xx",
		digits: 2,
	}
	return s.process(pats)
}

func readInput(filename string) ([]byte, error) {
	if filename == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("csplit: %v", err)
		}
		return data, nil
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("csplit: %v", err)
	}
	return data, nil
}

func splitToLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	var lines []string
	for len(data) > 0 {
		idx := bytes.IndexByte(data, '\n')
		if idx < 0 {
			lines = append(lines, string(data))
			break
		}
		lines = append(lines, string(data[:idx+1]))
		data = data[idx+1:]
	}
	return lines
}

func parsePatterns(args []string) ([]pattern, error) {
	pats := make([]pattern, 0, len(args))
	for _, arg := range args {
		p, err := parsePattern(arg)
		if err != nil {
			return nil, err
		}
		pats = append(pats, p)
	}
	return pats, nil
}

func parsePattern(arg string) (pattern, error) {
	if expr, ok := parseDelimited(arg, '/'); ok {
		re, err := regexp.Compile(expr)
		if err != nil {
			return pattern{}, fmt.Errorf("csplit: '%s': %v", arg, err)
		}
		return pattern{kind: kindRegexp, regex: re, raw: arg}, nil
	}
	if expr, ok := parseDelimited(arg, '%'); ok {
		re, err := regexp.Compile(expr)
		if err != nil {
			return pattern{}, fmt.Errorf("csplit: '%s': %v", arg, err)
		}
		return pattern{kind: kindSkip, regex: re, raw: arg}, nil
	}
	n, err := strconv.Atoi(arg)
	if err == nil && n >= 0 {
		return pattern{kind: kindInteger, lineNum: n, raw: arg}, nil
	}
	return pattern{}, fmt.Errorf("csplit: '%s': invalid pattern", arg)
}

func parseDelimited(arg string, delim byte) (string, bool) {
	if len(arg) < 2 || arg[0] != delim || arg[len(arg)-1] != delim {
		return "", false
	}
	return arg[1 : len(arg)-1], true
}

func (s *splitter) process(pats []pattern) error {
	for _, pat := range pats {
		if err := s.apply(pat); err != nil {
			removeFiles(s.created)
			return err
		}
	}
	if err := s.emitPiece(len(s.lines)); err != nil {
		removeFiles(s.created)
		return err
	}
	return nil
}

func (s *splitter) apply(pat pattern) error {
	switch pat.kind {
	case kindRegexp:
		idx := matchRegexp(s.lines, s.pos, pat.regex)
		if idx < 0 {
			s.emitPiece(len(s.lines))
			return fmt.Errorf("csplit: '%s': match not found", pat.raw)
		}
		return s.emitPiece(idx)
	case kindSkip:
		idx := matchRegexp(s.lines, s.pos, pat.regex)
		if idx < 0 {
			return fmt.Errorf("csplit: '%s': match not found", pat.raw)
		}
		s.pos = idx
		return nil
	default:
		return s.applyInteger(pat)
	}
}

func (s *splitter) applyInteger(pat pattern) error {
	target := pat.lineNum - 1
	if target < s.pos || target > len(s.lines) {
		return fmt.Errorf("csplit: '%d': line number out of range", pat.lineNum)
	}
	return s.emitPiece(target)
}

func (s *splitter) emitPiece(to int) error {
	piece := s.lines[s.pos:to]
	name := fmt.Sprintf("%s%0*d", s.prefix, s.digits, s.fileNum)
	s.created = append(s.created, name)
	content := strings.Join(piece, "")
	if err := os.WriteFile(name, []byte(content), 0666); err != nil {
		return fmt.Errorf("csplit: %v", err)
	}
	s.fileNum++
	s.pos = to
	fmt.Println(len(content))
	return nil
}

func matchRegexp(lines []string, start int, re *regexp.Regexp) int {
	for i := start; i < len(lines); i++ {
		line := strings.TrimSuffix(lines[i], "\n")
		if re.MatchString(line) {
			return i
		}
	}
	return -1
}

func removeFiles(files []string) {
	for _, f := range files {
		os.Remove(f)
	}
}
