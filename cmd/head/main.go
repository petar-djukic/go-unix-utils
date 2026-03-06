// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the head utility (prd018-head R1-R4).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()
	count, negative, byteMode, quiet, verbose, files := int64(10), false, false, false, false, []string(nil)
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			files = append(files, arg)
			continue
		}
		switch {
		case strings.HasPrefix(arg, "--lines"):
			count, negative = parseNum(mustLongVal(args, &i, "--lines"), "lines", false)
			byteMode = false
		case strings.HasPrefix(arg, "--bytes"):
			count, negative = parseNum(mustLongVal(args, &i, "--bytes"), "bytes", true)
			byteMode = true
		case arg == "--quiet", arg == "--silent":
			quiet, verbose = true, false
		case arg == "--verbose":
			verbose, quiet = true, false
		case len(arg) >= 2 && strings.TrimLeft(arg[1:], "0123456789") == "":
			count, _ = strconv.ParseInt(arg[1:], 10, 64)
			negative, byteMode = false, false
		default:
			for j := 1; j < len(arg); j++ {
				switch arg[j] {
				case 'n':
					count, negative = parseNum(shortVal(args, &i, arg, j), "lines", false)
					byteMode, j = false, len(arg)-1
				case 'c':
					count, negative = parseNum(shortVal(args, &i, arg, j), "bytes", true)
					byteMode, j = true, len(arg)-1
				case 'q':
					quiet, verbose = true, false
				case 'v':
					verbose, quiet = true, false
				default:
					die(fmt.Sprintf("invalid option -- '%c'", arg[j]))
				}
			}
		}
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	showHeaders := (len(files) > 1 && !quiet) || (verbose && !quiet)
	exitCode := 0
	for idx, file := range files {
		if showHeaders {
			if idx > 0 {
				fmt.Println()
			}
			name := file
			if file == "-" {
				name = "standard input"
			}
			fmt.Printf("==> %s <==\n", name)
		}
		r, closer, err := openInput(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "head: cannot open '%s' for reading: %s\n", file, capitalizeErr(err))
			exitCode = 1
			continue
		}
		switch {
		case !byteMode && !negative:
			err = headLines(r, count)
		case !byteMode && negative:
			err = headLinesNeg(r, count)
		case byteMode && !negative:
			err = headBytes(r, count)
		default:
			err = headBytesNeg(r, count)
		}
		if closer != nil {
			closer.Close()
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "head: error reading '%s': %s\n", file, err)
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

func headLines(r io.Reader, n int64) error {
	br := bufio.NewReader(r)
	for range n {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			os.Stdout.Write(line)
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
	return nil
}
func headLinesNeg(r io.Reader, n int64) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	lines := splitLines(data)
	for i := range max(int64(len(lines))-n, 0) {
		os.Stdout.Write(lines[i])
	}
	return nil
}
func headBytes(r io.Reader, n int64) error {
	_, err := io.CopyN(os.Stdout, r, n)
	if err == io.EOF {
		return nil
	}
	return err
}
func headBytesNeg(r io.Reader, n int64) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if end := int64(len(data)) - n; end > 0 {
		_, err = os.Stdout.Write(data[:end])
		return err
	}
	return nil
}
func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, data[start:i+1])
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}
func openInput(file string) (io.Reader, io.Closer, error) {
	if file == "-" {
		return os.Stdin, nil, nil
	}
	f, err := os.Open(file)
	if err != nil {
		return nil, nil, err
	}
	return f, f, nil
}
func mustLongVal(args []string, i *int, name string) string {
	if _, val, ok := strings.Cut(args[*i], "="); ok {
		return val
	}
	*i++
	if *i >= len(args) {
		die(fmt.Sprintf("option '%s' requires an argument", name))
	}
	return args[*i]
}
func shortVal(args []string, i *int, arg string, j int) string {
	if j+1 < len(arg) {
		return arg[j+1:]
	}
	*i++
	if *i >= len(args) {
		die(fmt.Sprintf("option requires an argument -- '%c'", arg[j]))
	}
	return args[*i]
}
func parseNum(s, label string, allowSuffix bool) (int64, bool) {
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	val, err := strconv.ParseInt(s[:i], 10, 64)
	if err != nil {
		die(fmt.Sprintf("invalid number of %s: '%s'", label, s))
	}
	suffix := s[i:]
	if !allowSuffix && suffix != "" {
		die(fmt.Sprintf("invalid number of %s: '%s'", label, s))
	}
	mult := int64(1)
	switch suffix {
	case "":
	case "b":
		mult = 512
	case "K", "KiB":
		mult = 1024
	case "M", "MiB":
		mult = 1024 * 1024
	case "G", "GiB":
		mult = 1024 * 1024 * 1024
	default:
		die(fmt.Sprintf("invalid number of %s: '%s'", label, s))
	}
	return val * mult, neg
}

func capitalizeErr(err error) string {
	msg := err.Error()
	if idx := strings.LastIndex(msg, ": "); idx >= 0 {
		msg = msg[idx+2:]
	}
	runes := []rune(msg)
	if len(runes) > 0 {
		runes[0] = unicode.ToUpper(runes[0])
	}
	return string(runes)
}

func die(msg string) {
	fmt.Fprintf(os.Stderr, "head: %s\n", msg)
	os.Exit(1)
}
