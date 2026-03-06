// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/uniq — report or filter adjacent duplicate lines.
// Implements prd028-uniq R1-R4.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const version = "uniq (go-unix-utils) 0.1"

type config struct {
	count, repeated, unique, ignoreCase, zeroTerm bool
	allRepeated                                   string
	skipFields, skipChars, checkChars             int
}

// preprocessArgs normalizes -D (optional value) so flag.Parse can handle it.
func preprocessArgs(args []string) []string {
	methods := map[string]bool{"none": true, "prepend": true, "separate": true}
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-D" || a == "--all-repeated":
			if i+1 < len(args) && methods[args[i+1]] {
				out = append(out, "-D="+args[i+1])
				i++
			} else {
				out = append(out, "-D=none")
			}
		case strings.HasPrefix(a, "--all-repeated="):
			out = append(out, "-D="+a[len("--all-repeated="):])
		default:
			out = append(out, a)
		}
	}
	return out
}

func main() {
	sys.InstallSIGPIPEHandler()
	var cfg config
	var showVersion bool
	fs := flag.NewFlagSet("uniq", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.BoolVar(&cfg.count, "c", false, "")
	fs.BoolVar(&cfg.repeated, "d", false, "")
	fs.StringVar(&cfg.allRepeated, "D", "", "")
	fs.BoolVar(&cfg.unique, "u", false, "")
	fs.BoolVar(&cfg.ignoreCase, "i", false, "")
	fs.IntVar(&cfg.skipFields, "f", 0, "")
	fs.IntVar(&cfg.skipChars, "s", 0, "")
	fs.IntVar(&cfg.checkChars, "w", 0, "")
	fs.BoolVar(&cfg.zeroTerm, "z", false, "")
	fs.BoolVar(&showVersion, "version", false, "")
	fs.Usage = func() { printUsage(os.Stdout) }
	if err := fs.Parse(preprocessArgs(os.Args[1:])); err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		}
		os.Exit(1)
	}
	if showVersion {
		fmt.Println(version)
		os.Exit(0)
	}
	if cfg.count && cfg.allRepeated != "" {
		fmt.Fprintf(os.Stderr, "uniq: printing all duplicated lines and repeat counts is meaningless\nTry 'uniq --help' for more information.\n")
		os.Exit(1)
	}
	var input io.Reader = os.Stdin
	var output io.Writer = os.Stdout
	args := fs.Args()
	if len(args) > 0 && args[0] != "-" {
		f, err := os.Open(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "uniq: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		input = f
	}
	if len(args) > 1 {
		f, err := os.Create(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "uniq: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		output = f
	}
	if err := run(cfg, input, output); err != nil {
		fmt.Fprintf(os.Stderr, "uniq: write error: %v\n", err)
		os.Exit(1)
	}
}

func compareKey(line string, cfg config) string {
	s := line
	for n := 0; n < cfg.skipFields && len(s) > 0; n++ {
		for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
			s = s[1:]
		}
		for len(s) > 0 && s[0] != ' ' && s[0] != '\t' {
			s = s[1:]
		}
	}
	if cfg.skipChars > 0 {
		if cfg.skipChars >= len(s) {
			s = ""
		} else {
			s = s[cfg.skipChars:]
		}
	}
	if cfg.checkChars > 0 && cfg.checkChars < len(s) {
		s = s[:cfg.checkChars]
	}
	if cfg.ignoreCase {
		s = strings.Map(unicode.ToLower, s)
	}
	return s
}

func run(cfg config, input io.Reader, output io.Writer) error {
	delim := byte('\n')
	if cfg.zeroTerm {
		delim = 0
	}
	w := bufio.NewWriter(output)
	defer w.Flush()
	reader := bufio.NewReader(input)
	var prevLine, prevKey string
	count, firstGroup := 0, true
	var groupLines []string

	writeLine := func(line string) error {
		_, err := fmt.Fprintf(w, "%s%c", line, delim)
		return err
	}
	flush := func() error {
		if count == 0 {
			return nil
		}
		if cfg.allRepeated != "" {
			if count >= 2 {
				if cfg.allRepeated == "prepend" || (cfg.allRepeated == "separate" && !firstGroup) {
					if err := w.WriteByte(delim); err != nil {
						return err
					}
				}
				firstGroup = false
				for _, l := range groupLines {
					if err := writeLine(l); err != nil {
						return err
					}
				}
			}
			groupLines = groupLines[:0]
			return nil
		}
		show := (!cfg.repeated || count >= 2) && (!cfg.unique || count == 1)
		if cfg.repeated && cfg.unique {
			show = false
		}
		if !show {
			return nil
		}
		if cfg.count {
			_, err := fmt.Fprintf(w, "%7d %s%c", count, prevLine, delim)
			return err
		}
		return writeLine(prevLine)
	}

	for {
		line, err := readLine(reader, delim)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		key := compareKey(line, cfg)
		if count == 0 || key != prevKey {
			if count > 0 {
				if err := flush(); err != nil {
					return err
				}
			}
			prevLine, prevKey, count = line, key, 1
			if cfg.allRepeated != "" {
				groupLines = append(groupLines[:0], line)
			}
		} else {
			count++
			if cfg.allRepeated != "" {
				groupLines = append(groupLines, line)
			}
		}
	}
	return flush()
}

func readLine(r *bufio.Reader, delim byte) (string, error) {
	var buf []byte
	for {
		chunk, err := r.ReadBytes(delim)
		if len(chunk) > 0 {
			if chunk[len(chunk)-1] == delim {
				buf = append(buf, chunk[:len(chunk)-1]...)
				return string(buf), nil
			}
			buf = append(buf, chunk...)
		}
		if err != nil {
			if len(buf) > 0 {
				return string(buf), nil
			}
			return "", err
		}
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, "Usage: uniq [OPTION]... [INPUT [OUTPUT]]\n"+
		"Filter adjacent matching lines from INPUT (or standard input),\n"+
		"writing to OUTPUT (or standard output).\n\n"+
		"  -c             prefix lines by the number of occurrences\n"+
		"  -d             only print duplicate lines, one for each group\n"+
		"  -D[METHOD]     print all duplicate lines (none, prepend, separate)\n"+
		"  -f N           avoid comparing the first N fields\n"+
		"  -i             ignore differences in case when comparing\n"+
		"  -s N           avoid comparing the first N characters\n"+
		"  -u             only print unique lines\n"+
		"  -w N           compare no more than N characters in lines\n"+
		"  -z             line delimiter is NUL, not newline\n"+
		"      --help     display this help and exit\n"+
		"      --version  output version information and exit\n")
}
