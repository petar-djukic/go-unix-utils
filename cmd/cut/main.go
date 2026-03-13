// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/cut implements the cut (remove sections from lines) command.
// Implements: prd026-cut R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3, R4.1, R4.2, R4.3, R4.4
package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// cutMode represents the selection mode: bytes, characters, or fields.
type cutMode int

const (
	modeNone   cutMode = iota
	modeBytes          // -b
	modeChars          // -c
	modeFields         // -f
)

// rangeSpec represents a single range element in a LIST.
// Both low and high are 1-based inclusive. high == -1 means "to end of line".
type rangeSpec struct {
	low  int
	high int // -1 means open-ended (N-)
}

// config holds all parsed command-line options.
type config struct {
	mode            cutMode
	ranges          []rangeSpec
	delimiter       byte
	delimiterSet    bool // true when -d or --delimiter was explicitly specified
	outputDelimiter string
	outputDelimSet  bool
	suppress        bool
	complement      bool
	files           []string
}

func main() {
	// R4.4: Install SIGPIPE handler per ARCHITECTURE.yaml shared protocol.
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "cut: %v\n", err)
		os.Exit(1)
	}

	exitCode := 0
	w := bufio.NewWriter(os.Stdout)

	if len(cfg.files) == 0 {
		if err := cutReader(os.Stdin, w, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "cut: %v\n", err)
			os.Exit(1)
		}
	} else {
		for _, name := range cfg.files {
			if err := cutFile(name, w, cfg); err != nil {
				fmt.Fprintf(os.Stderr, "cut: %v\n", err)
				exitCode = 1
			}
		}
	}

	// R4.3: Flush buffered output; exit 1 on write error.
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "cut: write error: %v\n", err)
		os.Exit(1)
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// parseArgs parses command-line arguments into a config.
func parseArgs(args []string) (*config, error) {
	cfg := &config{
		delimiter: '\t', // R2.2: Default delimiter is TAB.
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
			case arg == "--bytes" || strings.HasPrefix(arg, "--bytes="):
				val, err := longOptValue(arg, "--bytes", args, &i)
				if err != nil {
					return nil, err
				}
				if cfg.mode != modeNone {
					return nil, fmt.Errorf("only one type of list may be specified")
				}
				cfg.mode = modeBytes
				ranges, err := parseRangeList(val)
				if err != nil {
					return nil, err
				}
				cfg.ranges = ranges
			case arg == "--characters" || strings.HasPrefix(arg, "--characters="):
				val, err := longOptValue(arg, "--characters", args, &i)
				if err != nil {
					return nil, err
				}
				if cfg.mode != modeNone {
					return nil, fmt.Errorf("only one type of list may be specified")
				}
				cfg.mode = modeChars
				ranges, err := parseRangeList(val)
				if err != nil {
					return nil, err
				}
				cfg.ranges = ranges
			case arg == "--fields" || strings.HasPrefix(arg, "--fields="):
				val, err := longOptValue(arg, "--fields", args, &i)
				if err != nil {
					return nil, err
				}
				if cfg.mode != modeNone {
					return nil, fmt.Errorf("only one type of list may be specified")
				}
				cfg.mode = modeFields
				ranges, err := parseRangeList(val)
				if err != nil {
					return nil, err
				}
				cfg.ranges = ranges
			case arg == "--delimiter" || strings.HasPrefix(arg, "--delimiter="):
				val, err := longOptValue(arg, "--delimiter", args, &i)
				if err != nil {
					return nil, err
				}
				if len(val) != 1 {
					return nil, fmt.Errorf("the delimiter must be a single character")
				}
				cfg.delimiter = val[0]
				cfg.delimiterSet = true
			case strings.HasPrefix(arg, "--output-delimiter="):
				cfg.outputDelimiter = arg[len("--output-delimiter="):]
				cfg.outputDelimSet = true
			case arg == "--output-delimiter":
				if i+1 >= len(args) {
					return nil, fmt.Errorf("option '--output-delimiter' requires an argument")
				}
				i++
				cfg.outputDelimiter = args[i]
				cfg.outputDelimSet = true
			case arg == "--only-delimited":
				cfg.suppress = true
			case arg == "--complement":
				cfg.complement = true
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
			case 'b':
				val, err := shortOptValue(rest, args, &i)
				if err != nil {
					return nil, fmt.Errorf("option requires an argument -- 'b'")
				}
				if cfg.mode != modeNone {
					return nil, fmt.Errorf("only one type of list may be specified")
				}
				cfg.mode = modeBytes
				ranges, err := parseRangeList(val)
				if err != nil {
					return nil, err
				}
				cfg.ranges = ranges
				rest = ""
			case 'c':
				val, err := shortOptValue(rest, args, &i)
				if err != nil {
					return nil, fmt.Errorf("option requires an argument -- 'c'")
				}
				if cfg.mode != modeNone {
					return nil, fmt.Errorf("only one type of list may be specified")
				}
				cfg.mode = modeChars
				ranges, err := parseRangeList(val)
				if err != nil {
					return nil, err
				}
				cfg.ranges = ranges
				rest = ""
			case 'f':
				val, err := shortOptValue(rest, args, &i)
				if err != nil {
					return nil, fmt.Errorf("option requires an argument -- 'f'")
				}
				if cfg.mode != modeNone {
					return nil, fmt.Errorf("only one type of list may be specified")
				}
				cfg.mode = modeFields
				ranges, err := parseRangeList(val)
				if err != nil {
					return nil, err
				}
				cfg.ranges = ranges
				rest = ""
			case 'd':
				val, err := shortOptValue(rest, args, &i)
				if err != nil {
					return nil, fmt.Errorf("option requires an argument -- 'd'")
				}
				if len(val) != 1 {
					return nil, fmt.Errorf("the delimiter must be a single character")
				}
				cfg.delimiter = val[0]
				cfg.delimiterSet = true
				rest = ""
			case 's':
				cfg.suppress = true
			default:
				return nil, fmt.Errorf("invalid option -- '%c'", ch)
			}
		}
	}

	// R1.1, D5: Exactly one of -b, -c, or -f must be specified.
	if cfg.mode == modeNone {
		return nil, fmt.Errorf("you must specify a list of bytes, characters, or fields")
	}

	// R2.2: -d is only valid with -f.
	if cfg.delimiterSet && cfg.mode != modeFields {
		return nil, fmt.Errorf("an input delimiter may be specified only when operating on fields")
	}

	// R2.3: -s is only valid with -f.
	if cfg.suppress && cfg.mode != modeFields {
		return nil, fmt.Errorf("suppressing non-delimited lines makes sense\n\tonly when operating on fields")
	}

	// R2.2: Default output delimiter matches input delimiter.
	if !cfg.outputDelimSet {
		cfg.outputDelimiter = string(cfg.delimiter)
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

// parseRangeList parses a comma-separated LIST of range specifications.
// R1.1, D4: Supports N, N-M, N-, -M. Positions are 1-based.
func parseRangeList(list string) ([]rangeSpec, error) {
	parts := strings.Split(list, ",")
	var ranges []rangeSpec

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		dashIdx := strings.Index(p, "-")
		if dashIdx < 0 {
			// Single number N
			n, err := strconv.Atoi(p)
			if err != nil || n <= 0 {
				return nil, fmt.Errorf("invalid byte, character or field list")
			}
			ranges = append(ranges, rangeSpec{low: n, high: n})
		} else if dashIdx == 0 {
			// -M (from 1 to M)
			m, err := strconv.Atoi(p[1:])
			if err != nil || m <= 0 {
				return nil, fmt.Errorf("invalid byte, character or field list")
			}
			ranges = append(ranges, rangeSpec{low: 1, high: m})
		} else if dashIdx == len(p)-1 {
			// N- (from N to end)
			n, err := strconv.Atoi(p[:dashIdx])
			if err != nil || n <= 0 {
				return nil, fmt.Errorf("invalid byte, character or field list")
			}
			ranges = append(ranges, rangeSpec{low: n, high: -1})
		} else {
			// N-M
			n, err := strconv.Atoi(p[:dashIdx])
			if err != nil || n <= 0 {
				return nil, fmt.Errorf("invalid byte, character or field list")
			}
			m, err := strconv.Atoi(p[dashIdx+1:])
			if err != nil || m <= 0 {
				return nil, fmt.Errorf("invalid byte, character or field list")
			}
			ranges = append(ranges, rangeSpec{low: n, high: m})
		}
	}

	if len(ranges) == 0 {
		return nil, fmt.Errorf("invalid byte, character or field list")
	}

	return ranges, nil
}

// isSelected returns true if position pos (1-based) is within any range in the list.
func isSelected(pos int, ranges []rangeSpec) bool {
	for _, r := range ranges {
		if r.high == -1 {
			if pos >= r.low {
				return true
			}
		} else {
			if pos >= r.low && pos <= r.high {
				return true
			}
		}
	}
	return false
}

// cutFile opens name and processes its contents. "-" reads from stdin.
func cutFile(name string, w *bufio.Writer, cfg *config) error {
	if name == "-" {
		return cutReader(os.Stdin, w, cfg)
	}
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }() // best-effort cleanup, error ignored
	return cutReader(f, w, cfg)
}

// cutReader processes input from r according to cfg, writing results to w.
func cutReader(r *os.File, w *bufio.Writer, cfg *config) error {
	scanner := bufio.NewScanner(r)
	// Increase scanner buffer for long lines.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		var err error
		switch cfg.mode {
		case modeBytes, modeChars:
			// R1.1, R1.2: Under LC_ALL=C, bytes and characters are equivalent.
			err = cutBytes(w, line, cfg)
		case modeFields:
			err = cutFields(w, line, cfg)
		}
		if err != nil {
			return fmt.Errorf("write error: %w", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read error: %w", err)
	}

	return nil
}

// cutBytes extracts selected byte/character positions from line and writes to w.
// R1.3: Newlines are not counted; output is terminated with a newline.
// R1.4: Out-of-range positions produce no output.
func cutBytes(w *bufio.Writer, line string, cfg *config) error {
	// Collect selected positions, considering --complement.
	n := len(line)
	first := true
	needOutputDelim := cfg.outputDelimSet

	// Determine max position needed for complement calculation.
	for pos := 1; pos <= n; pos++ {
		selected := isSelected(pos, cfg.ranges)
		if cfg.complement {
			selected = !selected
		}
		if selected {
			if needOutputDelim && !first {
				if _, err := w.WriteString(cfg.outputDelimiter); err != nil {
					return err
				}
			}
			if err := w.WriteByte(line[pos-1]); err != nil {
				return err
			}
			first = false
		}
	}

	if err := w.WriteByte('\n'); err != nil {
		return err
	}
	return nil
}

// cutFields extracts selected fields from a delimited line and writes to w.
// R2.1: Fields are delimited by cfg.delimiter.
// R2.3: Without -s, lines with no delimiter are printed unchanged.
// R2.4: Output fields are separated by cfg.outputDelimiter.
// R3.1, R3.3: --complement inverts the field selection.
func cutFields(w *bufio.Writer, line string, cfg *config) error {
	delim := string(cfg.delimiter)

	// R2.3: If line contains no delimiter...
	if !strings.Contains(line, delim) {
		if cfg.suppress {
			// -s: suppress the line entirely.
			return nil
		}
		// Print line unchanged.
		if _, err := w.WriteString(line); err != nil {
			return err
		}
		if err := w.WriteByte('\n'); err != nil {
			return err
		}
		return nil
	}

	fields := strings.Split(line, delim)
	nFields := len(fields)

	// Build ordered list of selected field indices (0-based).
	selected := make([]int, 0, nFields)
	for i := 0; i < nFields; i++ {
		sel := isSelected(i+1, cfg.ranges)
		if cfg.complement {
			sel = !sel
		}
		if sel {
			selected = append(selected, i)
		}
	}

	// Sort to ensure output order matches original field order.
	sort.Ints(selected)

	for j, idx := range selected {
		if j > 0 {
			if _, err := w.WriteString(cfg.outputDelimiter); err != nil {
				return err
			}
		}
		if _, err := w.WriteString(fields[idx]); err != nil {
			return err
		}
	}

	if err := w.WriteByte('\n'); err != nil {
		return err
	}
	return nil
}
