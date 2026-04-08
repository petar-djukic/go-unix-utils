// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/tac: concatenate and print files in reverse.
// Implements srd021-tac R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// config holds tac command-line options.
// R2.1: -s, R2.2: -b, R2.3: -r, R2.4: -s + -r.
type config struct {
	sep    string
	before bool
	regex  bool
	re     *regexp.Regexp
}

// parseFlags parses -s, -b, -r flags and returns config and file arguments.
func parseFlags() (config, []string) {
	var cfg config
	fs := flag.NewFlagSet("tac", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {}
	fs.StringVar(&cfg.sep, "s", "\n", "")
	fs.StringVar(&cfg.sep, "separator", "\n", "")
	fs.BoolVar(&cfg.before, "b", false, "")
	fs.BoolVar(&cfg.before, "before", false, "")
	fs.BoolVar(&cfg.regex, "r", false, "")
	fs.BoolVar(&cfg.regex, "regex", false, "")
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(1)
	}
	return cfg, fs.Args()
}

// openInput returns os.Stdin for "-", otherwise opens the named file.
// R1.3: stdin when filename is "-".
func openInput(name string) (*os.File, error) {
	if name == "-" {
		return os.Stdin, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, formatOpenError(name, err)
	}
	return f, nil
}

// formatOpenError extracts the underlying error from os.PathError to produce
// GNU-compatible error messages: "<name>: <reason>".
func formatOpenError(name string, err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return fmt.Errorf("%s: %s", name, pe.Err)
	}
	return fmt.Errorf("%s: %s", name, err)
}

// reverseRecords splits data into records and writes them in reverse to w.
func reverseRecords(data []byte, cfg config, w io.Writer) error {
	if len(data) == 0 {
		return nil
	}
	if cfg.regex {
		return reverseRegex(data, cfg, w)
	}
	return reverseString(data, cfg, w)
}

// reverseString handles literal string separator reversal.
// R2.1: custom string separator via -s.
func reverseString(data []byte, cfg config, w io.Writer) error {
	sep := []byte(cfg.sep)
	if cfg.before {
		return writeBeforeString(data, sep, w)
	}
	return writeAfterString(data, sep, w)
}

// writeAfterString writes records in reverse with separator after each.
// R1.1, R1.2, R2.1: split on separator, reverse, trailing sep preserved.
func writeAfterString(data, sep []byte, w io.Writer) error {
	hasSuffix := bytes.HasSuffix(data, sep)
	parts := bytes.Split(data, sep)
	if hasSuffix && len(parts) > 0 && len(parts[len(parts)-1]) == 0 {
		parts = parts[:len(parts)-1]
	}
	for i := len(parts) - 1; i >= 0; i-- {
		if _, err := w.Write(parts[i]); err != nil {
			return err
		}
		if i > 0 || hasSuffix {
			if _, err := w.Write(sep); err != nil {
				return err
			}
		}
	}
	return nil
}

// writeBeforeString writes records in reverse with separator before each.
// R2.2: -b places separator before each record instead of after.
func writeBeforeString(data, sep []byte, w io.Writer) error {
	hasPrefix := bytes.HasPrefix(data, sep)
	parts := bytes.Split(data, sep)
	if hasPrefix && len(parts) > 0 && len(parts[0]) == 0 {
		parts = parts[1:]
	}
	for i := len(parts) - 1; i >= 0; i-- {
		if i < len(parts)-1 || hasPrefix {
			if _, err := w.Write(sep); err != nil {
				return err
			}
		}
		if _, err := w.Write(parts[i]); err != nil {
			return err
		}
	}
	return nil
}

// reverseRegex handles regex separator reversal.
// R2.3, R2.4: -r interprets separator as regex.
func reverseRegex(data []byte, cfg config, w io.Writer) error {
	locs := cfg.re.FindAllIndex(data, -1)
	if len(locs) == 0 {
		_, err := w.Write(data)
		return err
	}
	records, seps := extractSegments(data, locs)
	if cfg.before {
		return writeBeforeRegex(records, seps, w)
	}
	return writeAfterRegex(records, seps, w)
}

// extractSegments splits data by regex match locations into records and seps.
// records[i] is text between consecutive matches; seps[i] is matched text.
// len(records) == len(seps) + 1.
func extractSegments(data []byte, locs [][]int) ([][]byte, [][]byte) {
	records := make([][]byte, 0, len(locs)+1)
	seps := make([][]byte, 0, len(locs))
	pos := 0
	for _, loc := range locs {
		records = append(records, data[pos:loc[0]])
		seps = append(seps, data[loc[0]:loc[1]])
		pos = loc[1]
	}
	records = append(records, data[pos:])
	return records, seps
}

// writeAfterRegex writes regex-split records in reverse with sep after each.
func writeAfterRegex(records, seps [][]byte, w io.Writer) error {
	hasSuffix := len(records[len(records)-1]) == 0
	recs := records
	if hasSuffix {
		recs = records[:len(records)-1]
	}
	k := len(recs)
	for j := 0; j < k; j++ {
		idx := k - 1 - j
		if _, err := w.Write(recs[idx]); err != nil {
			return err
		}
		if j < k-1 {
			if _, err := w.Write(seps[k-2-j]); err != nil {
				return err
			}
		} else if hasSuffix {
			if _, err := w.Write(seps[len(seps)-1]); err != nil {
				return err
			}
		}
	}
	return nil
}

// writeBeforeRegex writes regex-split records in reverse with sep before each.
func writeBeforeRegex(records, seps [][]byte, w io.Writer) error {
	hasPrefix := len(records[0]) == 0
	recs := records
	betweenSeps := seps
	var leadingSep []byte
	if hasPrefix {
		recs = records[1:]
		leadingSep = seps[0]
		betweenSeps = seps[1:]
	}
	k := len(recs)
	for j := 0; j < k; j++ {
		idx := k - 1 - j
		if j == 0 && hasPrefix {
			if _, err := w.Write(leadingSep); err != nil {
				return err
			}
		} else if j > 0 {
			if _, err := w.Write(betweenSeps[idx]); err != nil {
				return err
			}
		}
		if _, err := w.Write(recs[idx]); err != nil {
			return err
		}
	}
	return nil
}

// tacFile reads the named file and writes its records in reverse to w.
// R1.4: each file is processed independently.
func tacFile(name string, cfg config, w io.Writer) error {
	r, err := openInput(name)
	if err != nil {
		return err
	}
	if r != os.Stdin {
		defer r.Close()
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("%s: %s", name, err)
	}
	return reverseRecords(data, cfg, w)
}

func main() {
	sys.InstallSIGPIPEHandler()
	cfg, args := parseFlags()

	// R2.3: compile regex once at startup.
	if cfg.regex {
		re, err := regexp.Compile(cfg.sep)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tac: invalid separator regex\n")
			os.Exit(1)
		}
		cfg.re = re
	}

	// R1.3: no arguments means read stdin.
	if len(args) == 0 {
		args = []string{"-"}
	}

	exitCode := 0
	// R1.4: process each file independently in argument order.
	for _, name := range args {
		if err := tacFile(name, cfg, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "tac: %s\n", err)
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}
