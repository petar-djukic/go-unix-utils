// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/paste implements the paste (merge lines of files) command.
// Implements: prd027-paste R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3
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
	delimiters []string // R2.1: delimiter list (default ["\t"])
	serial     bool     // R3.1: -s serial mode
	files      []string
}

func main() {
	// R4.4: Install SIGPIPE handler per ARCHITECTURE.yaml shared protocol.
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "paste: %v\n", err)
		os.Exit(1)
	}

	w := bufio.NewWriter(os.Stdout)

	if err := run(cfg, w); err != nil {
		fmt.Fprintf(os.Stderr, "paste: %v\n", err)
		// R4.3: Flush before exit on error.
		_ = w.Flush() // best-effort flush, error ignored
		os.Exit(1)
	}

	// R4.3: Flush buffered output; exit 1 on write error.
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "paste: write error: %v\n", err)
		os.Exit(1)
	}
}

// parseArgs parses command-line arguments into a config.
func parseArgs(args []string) (*config, error) {
	cfg := &config{
		delimiters: []string{"\t"}, // R1.2: Default delimiter is TAB.
	}
	endOfFlags := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if endOfFlags || (!strings.HasPrefix(arg, "-") && arg != "-") {
			cfg.files = append(cfg.files, arg)
			continue
		}

		if arg == "-" {
			cfg.files = append(cfg.files, arg)
			continue
		}

		if arg == "--" {
			endOfFlags = true
			continue
		}

		// Long options
		if strings.HasPrefix(arg, "--") {
			switch {
			case arg == "--serial":
				cfg.serial = true
			case arg == "--delimiters" || strings.HasPrefix(arg, "--delimiters="):
				val, err := longOptValue(arg, "--delimiters", args, &i)
				if err != nil {
					return nil, err
				}
				cfg.delimiters = parseDelimiterEscapes(val)
			default:
				return nil, fmt.Errorf("unrecognized option '%s'", arg)
			}
			continue
		}

		// Short flags
		rest := arg[1:]
		for len(rest) > 0 {
			ch := rest[0]
			rest = rest[1:]
			switch ch {
			case 'd':
				val, err := shortOptValue(rest, args, &i)
				if err != nil {
					return nil, fmt.Errorf("option requires an argument -- 'd'")
				}
				cfg.delimiters = parseDelimiterEscapes(val)
				rest = ""
			case 's':
				cfg.serial = true
			default:
				return nil, fmt.Errorf("invalid option -- '%c'", ch)
			}
		}
	}

	// R1.4: When no files are given, read from stdin.
	if len(cfg.files) == 0 {
		cfg.files = []string{"-"}
	}

	return cfg, nil
}

// longOptValue extracts the value for a long option, either from --opt=val or --opt val.
func longOptValue(arg, name string, args []string, idx *int) (string, error) {
	if strings.Contains(arg, "=") {
		return arg[len(name)+1:], nil
	}
	if *idx+1 >= len(args) {
		return "", fmt.Errorf("option '%s' requires an argument", name)
	}
	*idx++
	return args[*idx], nil
}

// shortOptValue extracts the value for a short option: either remaining chars or next arg.
func shortOptValue(rest string, args []string, idx *int) (string, error) {
	if len(rest) > 0 {
		return rest, nil
	}
	if *idx+1 >= len(args) {
		return "", fmt.Errorf("missing argument")
	}
	*idx++
	return args[*idx], nil
}

// parseDelimiterEscapes processes escape sequences in a delimiter string and
// returns a slice where each element is a single delimiter. \0 produces an
// empty string entry so that cycling counts it as a position.
// R2.2: Recognizes \n (newline), \t (tab), \\ (backslash), \0 (empty string).
func parseDelimiterEscapes(s string) []string {
	var delims []string
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				delims = append(delims, "\n")
				i++
			case 't':
				delims = append(delims, "\t")
				i++
			case '\\':
				delims = append(delims, "\\")
				i++
			case '0':
				// R2.2: \0 means empty string (no delimiter), but counts as a position.
				delims = append(delims, "")
				i++
			default:
				delims = append(delims, string(s[i]))
			}
		} else {
			delims = append(delims, string(s[i]))
		}
	}
	return delims
}

// getDelimiter returns the delimiter for the given field index (0-based).
// R2.1, R2.3: Delimiters cycle through the list.
func getDelimiter(delimiters []string, index int) string {
	if len(delimiters) == 0 {
		return ""
	}
	return delimiters[index%len(delimiters)]
}

// run executes the paste operation.
func run(cfg *config, w *bufio.Writer) error {
	if cfg.serial {
		return runSerial(cfg, w)
	}
	return runParallel(cfg, w)
}

// runParallel implements the default parallel merge mode.
// R1.1: Reads one line from each file per output line.
// R1.2: Shorter files contribute empty fields.
// R1.3: "-" reads from stdin. Multiple "-" share a single reader.
func runParallel(cfg *config, w *bufio.Writer) error {
	readers := make([]*bufio.Reader, len(cfg.files))
	closers := make([]io.Closer, len(cfg.files))

	// R1.3: All "-" arguments share a single stdin reader.
	var stdinReader *bufio.Reader

	for i, name := range cfg.files {
		if name == "-" {
			if stdinReader == nil {
				stdinReader = bufio.NewReader(os.Stdin)
			}
			readers[i] = stdinReader
		} else {
			f, err := os.Open(name)
			if err != nil {
				// Close already-opened files.
				for j := 0; j < i; j++ {
					if closers[j] != nil {
						_ = closers[j].Close() // best-effort cleanup, error ignored
					}
				}
				return err
			}
			closers[i] = f
			readers[i] = bufio.NewReader(f)
		}
	}

	defer func() {
		for _, c := range closers {
			if c != nil {
				_ = c.Close() // best-effort cleanup, error ignored
			}
		}
	}()

	// R1.1, R1.2: Read lines until all files are exhausted.
	exhausted := make([]bool, len(readers))
	for {
		// Try to read one line from each file for this output line.
		lines := make([]string, len(readers))
		anyData := false

		for i, r := range readers {
			if exhausted[i] {
				continue
			}
			line, err := readLine(r)
			if err != nil {
				exhausted[i] = true
				continue
			}
			lines[i] = line
			anyData = true
		}

		if !anyData {
			break
		}

		// Write the merged output line.
		for i := range readers {
			if i > 0 {
				// R2.1, R2.3: Write delimiter between fields.
				if _, err := w.WriteString(getDelimiter(cfg.delimiters, i-1)); err != nil {
					return fmt.Errorf("write error: %w", err)
				}
			}
			if _, err := w.WriteString(lines[i]); err != nil {
				return fmt.Errorf("write error: %w", err)
			}
		}

		if err := w.WriteByte('\n'); err != nil {
			return fmt.Errorf("write error: %w", err)
		}
	}

	return nil
}

// runSerial implements the -s serial mode.
// R3.1: All lines of each file are joined with the delimiter on one output line.
// R3.2: Delimiter list cycles across fields within each output line.
func runSerial(cfg *config, w *bufio.Writer) error {
	for _, name := range cfg.files {
		var r *bufio.Reader
		var closer io.Closer

		if name == "-" {
			r = bufio.NewReader(os.Stdin)
		} else {
			f, err := os.Open(name)
			if err != nil {
				return err
			}
			closer = f
			r = bufio.NewReader(f)
		}

		first := true
		delimIdx := 0
		for {
			line, err := readLine(r)
			if err != nil {
				break
			}
			if !first {
				if _, werr := w.WriteString(getDelimiter(cfg.delimiters, delimIdx)); werr != nil {
					if closer != nil {
						_ = closer.Close() // best-effort cleanup, error ignored
					}
					return fmt.Errorf("write error: %w", werr)
				}
				delimIdx++
			}
			if _, werr := w.WriteString(line); werr != nil {
				if closer != nil {
					_ = closer.Close() // best-effort cleanup, error ignored
				}
				return fmt.Errorf("write error: %w", werr)
			}
			first = false
		}

		if err := w.WriteByte('\n'); err != nil {
			if closer != nil {
				_ = closer.Close() // best-effort cleanup, error ignored
			}
			return fmt.Errorf("write error: %w", err)
		}

		if closer != nil {
			_ = closer.Close() // best-effort cleanup, error ignored
		}
	}

	return nil
}

// readLine reads a single line from r, stripping the trailing newline.
// Returns io.EOF when no more data is available.
func readLine(r *bufio.Reader) (string, error) {
	var line strings.Builder
	for {
		part, isPrefix, err := r.ReadLine()
		if err != nil {
			if line.Len() > 0 {
				return line.String(), nil
			}
			return "", err
		}
		line.Write(part)
		if !isPrefix {
			return line.String(), nil
		}
	}
}
