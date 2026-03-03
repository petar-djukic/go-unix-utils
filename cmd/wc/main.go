// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/wc counts lines, words, and bytes in files or stdin, matching GNU wc
// output format under LC_ALL=C.
//
// Implements prd005-wc R1, R2, R3, R4, R5, R6.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "wc"

type config struct {
	lines      bool
	words      bool
	bytes      bool
	chars      bool
	maxLineLen bool
	totalMode  string
	files0From string
}

type fileCounts struct {
	lines  int64
	words  int64
	bytes  int64
	chars  int64
	maxLen int64
}

func main() {
	sys.InstallSIGPIPEHandler()

	cfg, files := parseArgs(os.Args[1:])

	// Default: no flags means lines + words + bytes (prd005-wc R1.1).
	if !cfg.lines && !cfg.words && !cfg.bytes && !cfg.chars && !cfg.maxLineLen {
		cfg.lines = true
		cfg.words = true
		cfg.bytes = true
	}

	// -m takes precedence over -c when both given (prd005-wc R2.3).
	if cfg.chars && cfg.bytes {
		cfg.bytes = false
	}

	// Handle --files0-from (prd005-wc R4.4).
	if cfg.files0From != "" {
		if len(files) > 0 {
			fmt.Fprintf(os.Stderr, "%s: file operands cannot be combined with --files0-from\n", progName)
			os.Exit(1)
		}
		var err error
		files, err = readFiles0From(cfg.files0From)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
			os.Exit(1)
		}
	}

	// No files → read stdin (prd005-wc R1.2).
	if len(files) == 0 {
		files = []string{""}
	}

	numWidth := computeNumberWidth(files)
	out := bufio.NewWriter(os.Stdout)
	showPerFile, showTotal := totalPolicy(cfg.totalMode, len(files))

	var total fileCounts
	exitCode := 0

	for _, file := range files {
		c, name, err := processFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
			exitCode = 1
			continue
		}

		total.lines += c.lines
		total.words += c.words
		total.bytes += c.bytes
		total.chars += c.chars
		if c.maxLen > total.maxLen {
			total.maxLen = c.maxLen
		}

		if showPerFile {
			printLine(out, cfg, c, name, numWidth)
		}
	}

	if showTotal {
		printLine(out, cfg, total, "total", numWidth)
	}

	if err := out.Flush(); err != nil {
		os.Exit(1) // prd005-wc R6.3
	}

	os.Exit(exitCode)
}

// parseArgs parses GNU-style flags including combined short flags (-lwc)
// and long flags with = or space-separated values.
func parseArgs(args []string) (config, []string) {
	var cfg config
	var files []string

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

		if strings.HasPrefix(arg, "--") {
			name, value, hasValue := strings.Cut(arg[2:], "=")
			switch name {
			case "lines":
				cfg.lines = true
			case "words":
				cfg.words = true
			case "bytes":
				cfg.bytes = true
			case "chars":
				cfg.chars = true
			case "max-line-length":
				cfg.maxLineLen = true
			case "total":
				if hasValue {
					cfg.totalMode = value
				} else if i+1 < len(args) {
					i++
					cfg.totalMode = args[i]
				}
			case "files0-from":
				if hasValue {
					cfg.files0From = value
				} else if i+1 < len(args) {
					i++
					cfg.files0From = args[i]
				}
			default:
				fmt.Fprintf(os.Stderr, "%s: unrecognized option '--%s'\n", progName, name)
				os.Exit(1)
			}
			continue
		}

		// Short flags may be combined: -lwc means -l -w -c.
		for _, ch := range arg[1:] {
			switch ch {
			case 'l':
				cfg.lines = true
			case 'w':
				cfg.words = true
			case 'c':
				cfg.bytes = true
			case 'm':
				cfg.chars = true
			case 'L':
				cfg.maxLineLen = true
			default:
				fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", progName, ch)
				os.Exit(1)
			}
		}
	}

	return cfg, files
}

// computeNumberWidth determines the minimum field width for count columns.
// When all files are regular and stat-able, the width is the digit count of
// their combined size. Otherwise the default width is 7, matching GNU wc.
func computeNumberWidth(files []string) int {
	var totalSize int64

	for _, f := range files {
		if f == "" || f == "-" {
			return 7
		}
		info, err := os.Stat(f)
		if err != nil || !info.Mode().IsRegular() {
			return 7
		}
		totalSize += info.Size()
	}

	width := 1
	for n := totalSize; n >= 10; n /= 10 {
		width++
	}
	return width
}

// totalPolicy decides whether per-file lines and a total line are printed
// based on --total mode and file count (prd005-wc R3.3).
func totalPolicy(mode string, nfiles int) (showPerFile, showTotal bool) {
	switch mode {
	case "always":
		return true, true
	case "only":
		return false, true
	case "never":
		return true, false
	default: // "auto" or ""
		return true, nfiles > 1
	}
}

// processFile opens and counts a single input. An empty path means implicit
// stdin (no filename in output); "-" means explicit stdin with "-" as label.
func processFile(path string) (fileCounts, string, error) {
	if path == "" {
		c, err := countInput(os.Stdin)
		return c, "", err
	}

	if path == "-" {
		c, err := countInput(os.Stdin)
		return c, "-", err
	}

	f, err := os.Open(path)
	if err != nil {
		if pe, ok := err.(*os.PathError); ok {
			return fileCounts{}, path, fmt.Errorf("%s: %v", path, pe.Err)
		}
		return fileCounts{}, path, err
	}
	defer f.Close()

	c, err := countInput(f)
	if err != nil {
		return c, path, fmt.Errorf("%s: %v", path, err)
	}

	return c, path, nil
}

// countInput reads all bytes from r and computes line, word, byte, char, and
// max-line-length counts. Word detection uses byte-oriented C-locale isspace
// semantics (design decision D3). Under LC_ALL=C chars equal bytes (R5.2).
// Max line length uses tab expansion to the next multiple of 8 (R2.5).
func countInput(r io.Reader) (fileCounts, error) {
	var c fileCounts
	buf := make([]byte, 32*1024)
	inWord := false
	var linePos int64

	for {
		n, err := r.Read(buf)
		for i := 0; i < n; i++ {
			b := buf[i]
			c.bytes++
			c.chars++ // LC_ALL=C: each byte is one character (R5.2)

			switch b {
			case '\n':
				c.lines++
				if linePos > c.maxLen {
					c.maxLen = linePos
				}
				linePos = 0
				if inWord {
					c.words++
					inWord = false
				}
			case '\r', '\f':
				if linePos > c.maxLen {
					c.maxLen = linePos
				}
				linePos = 0
				if inWord {
					c.words++
					inWord = false
				}
			case '\t':
				linePos += 8 - (linePos % 8)
				if inWord {
					c.words++
					inWord = false
				}
			case ' ':
				linePos++
				if inWord {
					c.words++
					inWord = false
				}
			case '\v':
				if inWord {
					c.words++
					inWord = false
				}
			default:
				// Printable ASCII (0x20-0x7E) advances display position.
				// 0x20 (space) is handled above; default sees 0x21-0x7E.
				// Non-printable and high bytes contribute to words but not
				// display width, matching GNU wc under LC_ALL=C.
				if b >= 0x20 && b <= 0x7E {
					linePos++
				}
				inWord = true
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return c, err
		}
	}

	// Final word with no trailing whitespace.
	if inWord {
		c.words++
	}

	// Final line segment with no trailing newline.
	if linePos > c.maxLen {
		c.maxLen = linePos
	}

	return c, nil
}

// printLine writes one output line: the requested counts right-justified in
// fixed-width columns followed by the filename (prd005-wc R2.6, R3.1).
func printLine(w *bufio.Writer, cfg config, c fileCounts, name string, width int) {
	first := true
	field := func(v int64) {
		if first {
			fmt.Fprintf(w, "%*d", width, v)
			first = false
		} else {
			fmt.Fprintf(w, " %*d", width, v)
		}
	}

	// Fixed output column order: lines, words, chars/bytes, max-line-length
	// (prd005-wc R2.6).
	if cfg.lines {
		field(c.lines)
	}
	if cfg.words {
		field(c.words)
	}
	if cfg.chars {
		field(c.chars)
	} else if cfg.bytes {
		field(c.bytes)
	}
	if cfg.maxLineLen {
		field(c.maxLen)
	}

	if name != "" {
		fmt.Fprintf(w, " %s", name)
	}
	fmt.Fprint(w, "\n")
}

// readFiles0From reads NUL-delimited filenames from path (or stdin when
// path is "-") for --files0-from support (prd005-wc R4.4).
func readFiles0From(path string) ([]string, error) {
	var r io.ReadCloser
	if path == "-" {
		r = io.NopCloser(os.Stdin)
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		r = f
	}
	defer r.Close()

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, name := range strings.Split(string(data), "\x00") {
		if name != "" {
			files = append(files, name)
		}
	}
	return files, nil
}
