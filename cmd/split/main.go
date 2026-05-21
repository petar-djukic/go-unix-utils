// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

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

type options struct {
	lines  int
	prefix string
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, file, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "split: %s\n", err)
		os.Exit(1)
	}

	r, closer, err := openInput(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "split: %s\n", err)
		os.Exit(1)
	}
	if closer != nil {
		defer closer.Close()
	}

	if err := splitLines(r, opts.prefix, opts.lines); err != nil {
		fmt.Fprintf(os.Stderr, "split: %s\n", err)
		os.Exit(1)
	}
}

func parseArgs(args []string) (options, string, error) {
	opts := options{lines: 1000, prefix: "x"}
	var positional []string

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			n, err := parseLongFlag(arg, args[i:], &opts)
			if err != nil {
				return opts, "", err
			}
			i += n
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			n, err := parseShortFlags(arg[1:], args[i+1:], &opts)
			if err != nil {
				return opts, "", err
			}
			i += 1 + n
			continue
		}
		positional = append(positional, arg)
		i++
	}

	return assignPositional(opts, positional)
}

func assignPositional(opts options, pos []string) (options, string, error) {
	file := "-"
	if len(pos) >= 1 {
		file = pos[0]
	}
	if len(pos) >= 2 {
		opts.prefix = pos[1]
	}
	if len(pos) > 2 {
		return opts, "", fmt.Errorf("extra operand '%s'", pos[2])
	}
	return opts, file, nil
}

func parseLongFlag(flag string, remaining []string, opts *options) (int, error) {
	switch {
	case flag == "--lines":
		if len(remaining) < 2 {
			return 0, fmt.Errorf("option '--lines' requires an argument")
		}
		n, err := parseLineCount(remaining[1])
		if err != nil {
			return 0, err
		}
		opts.lines = n
		return 2, nil
	case strings.HasPrefix(flag, "--lines="):
		n, err := parseLineCount(flag[len("--lines="):])
		if err != nil {
			return 0, err
		}
		opts.lines = n
		return 1, nil
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", flag)
	}
}

func parseShortFlags(flags string, remaining []string, opts *options) (int, error) {
	consumed := 0
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'l':
			val, extra, err := extractFlagValue(flags[j+1:], remaining, consumed, 'l')
			if err != nil {
				return 0, err
			}
			n, err := parseLineCount(val)
			if err != nil {
				return 0, err
			}
			opts.lines = n
			return consumed + extra, nil
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return consumed, nil
}

func extractFlagValue(rest string, remaining []string, consumed int, flag byte) (string, int, error) {
	if rest != "" {
		return rest, 0, nil
	}
	if len(remaining) > consumed {
		return remaining[consumed], 1, nil
	}
	return "", 0, fmt.Errorf("option requires an argument -- '%c'", flag)
}

func parseLineCount(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid number of lines: '%s'", s)
	}
	return n, nil
}

func openInput(name string) (io.Reader, io.Closer, error) {
	if name == "-" {
		return os.Stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, f, nil
}

func splitLines(r io.Reader, prefix string, linesPerPiece int) error {
	br := bufio.NewReader(r)
	lineCount := 0
	pieceIndex := 0
	var w *os.File

	for {
		line, readErr := br.ReadBytes('\n')
		if len(line) > 0 {
			if lineCount%linesPerPiece == 0 {
				if w != nil {
					w.Close()
				}
				var err error
				w, err = openPiece(prefix, pieceIndex)
				if err != nil {
					return err
				}
				pieceIndex++
			}
			if _, err := w.Write(line); err != nil {
				w.Close()
				return err
			}
			lineCount++
		}
		if readErr != nil {
			if w != nil {
				w.Close()
			}
			if readErr != io.EOF {
				return readErr
			}
			return nil
		}
	}
}

func openPiece(prefix string, index int) (*os.File, error) {
	suffix, err := alphabeticSuffix(index, 2)
	if err != nil {
		return nil, err
	}
	return os.Create(prefix + suffix)
}

func alphabeticSuffix(index, length int) (string, error) {
	maxPieces := 1
	for range length {
		maxPieces *= 26
	}
	if index >= maxPieces {
		return "", fmt.Errorf("output file suffixes exhausted")
	}
	buf := make([]byte, length)
	for i := length - 1; i >= 0; i-- {
		buf[i] = 'a' + byte(index%26)
		index /= 26
	}
	return string(buf), nil
}
