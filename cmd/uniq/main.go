// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/uniq implements the uniq (report or filter adjacent duplicate lines) command.
// Implements: prd028-uniq R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// config holds all parsed command-line options.
type config struct {
	count       bool // -c: prefix lines with count
	repeated    bool // -d: only print duplicate lines (one per run)
	allRepeated bool // -D: print all duplicate lines (every copy)
	unique      bool // -u: only print unique lines
	inFile      string
	outFile     string
}

func main() {
	// R4.4: Install SIGPIPE handler per ARCHITECTURE.yaml shared protocol.
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "uniq: %v\n", err)
		os.Exit(1)
	}

	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "uniq: %v\n", err)
		os.Exit(1)
	}
}

// parseArgs parses command-line arguments into a config.
func parseArgs(args []string) (*config, error) {
	cfg := &config{}
	var positional []string
	endOfFlags := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if endOfFlags {
			positional = append(positional, arg)
			continue
		}

		if arg == "--" {
			endOfFlags = true
			continue
		}

		if arg == "-" {
			positional = append(positional, arg)
			continue
		}

		// Long options.
		if strings.HasPrefix(arg, "--") {
			switch arg {
			case "--count":
				cfg.count = true
			case "--repeated":
				cfg.repeated = true
			case "--all-repeated":
				cfg.allRepeated = true
			case "--unique":
				cfg.unique = true
			default:
				return nil, fmt.Errorf("unrecognized option '%s'", arg)
			}
			continue
		}

		// Short flags.
		if strings.HasPrefix(arg, "-") {
			rest := arg[1:]
			for _, ch := range rest {
				switch ch {
				case 'c':
					cfg.count = true
				case 'd':
					cfg.repeated = true
				case 'D':
					cfg.allRepeated = true
				case 'u':
					cfg.unique = true
				default:
					return nil, fmt.Errorf("invalid option -- '%c'", ch)
				}
			}
			continue
		}

		positional = append(positional, arg)
	}

	// R1.3: Optional input file as first positional, optional output file as second.
	if len(positional) > 2 {
		return nil, fmt.Errorf("extra operand '%s'", positional[2])
	}
	if len(positional) >= 1 {
		cfg.inFile = positional[0]
	}
	if len(positional) >= 2 {
		cfg.outFile = positional[1]
	}

	return cfg, nil
}

// run executes the uniq logic with the given configuration.
func run(cfg *config) error {
	// Open input.
	var reader io.Reader
	if cfg.inFile == "" || cfg.inFile == "-" {
		reader = os.Stdin
	} else {
		f, err := os.Open(cfg.inFile)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }() // best-effort cleanup, error ignored
		reader = f
	}

	// Open output.
	var w *bufio.Writer
	if cfg.outFile == "" {
		w = bufio.NewWriter(os.Stdout)
	} else {
		f, err := os.Create(cfg.outFile)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }() // best-effort cleanup, error ignored
		w = bufio.NewWriter(f)
	}

	if err := processUniq(reader, w, cfg); err != nil {
		return err
	}

	// R4.3: Flush buffered output; report write error.
	if err := w.Flush(); err != nil {
		return fmt.Errorf("write error: %w", err)
	}

	return nil
}

// processUniq reads lines from reader, groups adjacent duplicates, and writes
// output to w according to the flags in cfg.
//
// R1.1: Suppress all but the first occurrence of any run of identical adjacent lines.
// R1.2: Non-adjacent duplicates are unaffected.
// R1.4: Comparison is case-sensitive; each line includes its newline terminator.
// R2.2: -D emits every copy of each duplicate run inline.
func processUniq(reader io.Reader, w *bufio.Writer, cfg *config) error {
	if cfg.allRepeated {
		return processAllRepeated(reader, w, cfg)
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var prevLine string
	var count int
	first := true

	for scanner.Scan() {
		line := scanner.Text()

		if first {
			prevLine = line
			count = 1
			first = false
			continue
		}

		// R1.4: Case-sensitive comparison of the full line.
		if line == prevLine {
			count++
			continue
		}

		// End of a run; emit the previous group.
		if err := emitGroup(w, prevLine, count, cfg); err != nil {
			return err
		}

		prevLine = line
		count = 1
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read error: %w", err)
	}

	// Emit the last group.
	if !first {
		if err := emitGroup(w, prevLine, count, cfg); err != nil {
			return err
		}
	}

	return nil
}

// processAllRepeated handles -D mode, which emits every copy of each duplicate
// run. Unlike the default mode, lines are emitted inline as the run grows rather
// than once at the end.
//
// R2.2: -D prints all lines of each run that appears more than once.
func processAllRepeated(reader io.Reader, w *bufio.Writer, _ *config) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var prevLine string
	var count int
	first := true

	for scanner.Scan() {
		line := scanner.Text()

		if first {
			prevLine = line
			count = 1
			first = false
			continue
		}

		if line == prevLine {
			// R2.2: Duplicate found; emit the deferred first copy if this is
			// the second occurrence, then emit the current copy.
			if count == 1 {
				if err := writeLine(w, prevLine); err != nil {
					return err
				}
			}
			if err := writeLine(w, line); err != nil {
				return err
			}
			count++
			continue
		}

		// New run starts; reset.
		prevLine = line
		count = 1
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read error: %w", err)
	}

	return nil
}

// writeLine writes a single line followed by a newline to w.
func writeLine(w *bufio.Writer, line string) error {
	if _, err := w.WriteString(line); err != nil {
		return err
	}
	return w.WriteByte('\n')
}

// emitGroup writes a single group (line repeated count times) to w, respecting
// the -c, -d, and -u flags.
//
// R1.3: -c prefixes output with the count, right-justified in a 7-wide field.
// R1.4: -d prints only groups with count > 1; -u prints only groups with count == 1.
func emitGroup(w *bufio.Writer, line string, count int, cfg *config) error {
	// R2.1: -d suppresses lines that appear only once.
	if cfg.repeated && count == 1 {
		return nil
	}
	// R2.3: -u suppresses lines that appear more than once.
	if cfg.unique && count > 1 {
		return nil
	}

	// R2.4: -c prefixes each line with the occurrence count.
	// R2.4: -c prefixes each line with the occurrence count.
	if cfg.count {
		if _, err := fmt.Fprintf(w, "%7d %s\n", count, line); err != nil {
			return err
		}
		return nil
	}

	return writeLine(w, line)
}
