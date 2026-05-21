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
	offset  int
	repeat  int // 0 = no repeat, >0 = repeat N times, -1 = repeat until exhausted
	raw     string
}

type splitter struct {
	lines   []string
	pos     int
	fileNum int
	prefix  string
	digits  int
	created []string
	atMatch bool
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
		if rep, ok := parseRepeat(arg); ok {
			if len(pats) == 0 {
				return nil, fmt.Errorf("csplit: '%s': no preceding pattern", arg)
			}
			pats[len(pats)-1].repeat = rep
			continue
		}
		p, err := parsePattern(arg)
		if err != nil {
			return nil, err
		}
		pats = append(pats, p)
	}
	return pats, nil
}

func parseRepeat(arg string) (int, bool) {
	if len(arg) < 3 || arg[0] != '{' || arg[len(arg)-1] != '}' {
		return 0, false
	}
	inner := arg[1 : len(arg)-1]
	if inner == "*" {
		return -1, true
	}
	n, err := strconv.Atoi(inner)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

func parsePattern(arg string) (pattern, error) {
	if expr, offset, ok := parseDelimitedWithOffset(arg, '/'); ok {
		re, err := regexp.Compile(expr)
		if err != nil {
			return pattern{}, fmt.Errorf("csplit: '%s': %v", arg, err)
		}
		return pattern{kind: kindRegexp, regex: re, offset: offset, raw: arg}, nil
	}
	if expr, offset, ok := parseDelimitedWithOffset(arg, '%'); ok {
		re, err := regexp.Compile(expr)
		if err != nil {
			return pattern{}, fmt.Errorf("csplit: '%s': %v", arg, err)
		}
		return pattern{kind: kindSkip, regex: re, offset: offset, raw: arg}, nil
	}
	n, err := strconv.Atoi(arg)
	if err == nil && n >= 0 {
		return pattern{kind: kindInteger, lineNum: n, raw: arg}, nil
	}
	return pattern{}, fmt.Errorf("csplit: '%s': invalid pattern", arg)
}

func parseDelimitedWithOffset(arg string, delim byte) (string, int, bool) {
	if len(arg) < 2 || arg[0] != delim {
		return "", 0, false
	}
	end := strings.LastIndexByte(arg[1:], delim)
	if end < 0 {
		return "", 0, false
	}
	end += 1
	expr := arg[1:end]
	rest := arg[end+1:]
	offset := 0
	if rest != "" {
		n, err := strconv.Atoi(rest)
		if err != nil {
			return "", 0, false
		}
		offset = n
	}
	return expr, offset, true
}

func (s *splitter) process(pats []pattern) error {
	for _, pat := range pats {
		if pat.repeat == 0 {
			if err := s.apply(pat, false); err != nil {
				removeFiles(s.created)
				return err
			}
		} else if pat.repeat == -1 {
			if err := s.applyNoEmitOnFail(pat, false); err != nil {
				break
			}
			for {
				if err := s.applyNoEmitOnFail(pat, true); err != nil {
					break
				}
			}
		} else {
			if err := s.apply(pat, false); err != nil {
				removeFiles(s.created)
				return fmt.Errorf("csplit: '%s': match not found on repetition 0", pat.raw)
			}
			for i := 1; i <= pat.repeat; i++ {
				if err := s.apply(pat, true); err != nil {
					removeFiles(s.created)
					return fmt.Errorf("csplit: '%s': match not found on repetition %d", pat.raw, i)
				}
			}
		}
	}
	if err := s.emitPiece(len(s.lines)); err != nil {
		removeFiles(s.created)
		return err
	}
	return nil
}

func (s *splitter) applyNoEmitOnFail(pat pattern, repeated bool) error {
	switch pat.kind {
	case kindRegexp:
		start := s.pos
		if s.atMatch {
			start = s.pos + 1
		}
		idx := matchRegexp(s.lines, start, pat.regex)
		if idx < 0 {
			return fmt.Errorf("csplit: '%s': match not found", pat.raw)
		}
		target := idx + pat.offset
		if target < s.pos {
			target = s.pos
		}
		if target > len(s.lines) {
			target = len(s.lines)
		}
		s.atMatch = true
		return s.emitPiece(target)
	case kindSkip:
		start := s.pos
		if s.atMatch {
			start = s.pos + 1
		}
		idx := matchRegexp(s.lines, start, pat.regex)
		if idx < 0 {
			return fmt.Errorf("csplit: '%s': match not found", pat.raw)
		}
		target := idx + pat.offset
		if target < s.pos {
			target = s.pos
		}
		if target > len(s.lines) {
			target = len(s.lines)
		}
		s.pos = target
		s.atMatch = true
		return nil
	default:
		var target int
		if repeated {
			target = s.pos + pat.lineNum
		} else {
			target = pat.lineNum - 1
		}
		if target > len(s.lines) || target < s.pos {
			return fmt.Errorf("csplit: '%d': line number out of range", pat.lineNum)
		}
		s.atMatch = false
		return s.emitPiece(target)
	}
}

func (s *splitter) apply(pat pattern, repeated bool) error {
	switch pat.kind {
	case kindRegexp:
		start := s.pos
		if s.atMatch {
			start = s.pos + 1
		}
		idx := matchRegexp(s.lines, start, pat.regex)
		if idx < 0 {
			s.emitPiece(len(s.lines))
			return fmt.Errorf("csplit: '%s': match not found", pat.raw)
		}
		target := idx + pat.offset
		if target < s.pos {
			target = s.pos
		}
		if target > len(s.lines) {
			target = len(s.lines)
		}
		s.atMatch = true
		return s.emitPiece(target)
	case kindSkip:
		start := s.pos
		if s.atMatch {
			start = s.pos + 1
		}
		idx := matchRegexp(s.lines, start, pat.regex)
		if idx < 0 {
			return fmt.Errorf("csplit: '%s': match not found", pat.raw)
		}
		target := idx + pat.offset
		if target < s.pos {
			target = s.pos
		}
		if target > len(s.lines) {
			target = len(s.lines)
		}
		s.pos = target
		s.atMatch = true
		return nil
	default:
		return s.applyInteger(pat, repeated)
	}
}

func (s *splitter) applyInteger(pat pattern, repeated bool) error {
	var target int
	if repeated {
		target = s.pos + pat.lineNum
	} else {
		target = pat.lineNum - 1
	}
	if target < s.pos || target > len(s.lines) {
		s.emitPiece(len(s.lines))
		return fmt.Errorf("csplit: '%d': line number out of range", pat.lineNum)
	}
	s.atMatch = false
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
