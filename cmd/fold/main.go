// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd023-fold R1.1–R1.4, R2.1–R2.3.
// R1.1: read files or stdin, wrap to at most W columns (default 80).
// R1.2: lines at or under the width pass through unchanged.
// R1.3: lines exceeding the width are split at exactly W columns, repeatedly.
// R1.4: final segment preserves the original trailing newline (or lack thereof).
// R2.1: -w N sets width; N must be positive, else exit 1 with error.
// R2.2: default column mode; tabs expand to next tab stop (every 8 columns).
// R2.3: -b counts bytes instead of columns, disabling tab expansion.
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

const (
	defaultWidth = 80
	tabStop      = 8
)

// config holds parsed command-line options.
type config struct {
	width    int
	byteMode bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	cfg, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "fold: %v\n", err)
		os.Exit(1)
	}
	os.Exit(run(cfg, files))
}

// run processes all input sources and returns the exit code.
func run(cfg config, files []string) int {
	w := bufio.NewWriter(os.Stdout)
	if len(files) == 0 {
		foldReader(w, os.Stdin, cfg)
		w.Flush()
		return 0
	}
	exitCode := 0
	for _, name := range files {
		if err := processFile(w, name, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "fold: %v\n", err)
			exitCode = 1
		}
	}
	w.Flush()
	return exitCode
}

// processFile opens a file (or stdin for "-") and folds it.
func processFile(w *bufio.Writer, name string, cfg config) error {
	if name == "-" {
		foldReader(w, os.Stdin, cfg)
		return nil
	}
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	foldReader(w, f, cfg)
	return nil
}

// foldReader reads from r byte by byte and writes folded output to w.
func foldReader(w *bufio.Writer, r io.Reader, cfg config) {
	br := bufio.NewReader(r)
	col := 0
	for {
		b, err := br.ReadByte()
		if err != nil {
			return
		}
		col = processByte(w, b, col, cfg)
	}
}

// processByte handles one byte, returning the updated column position.
func processByte(w *bufio.Writer, b byte, col int, cfg config) int {
	if b == '\n' {
		w.WriteByte('\n')
		return 0
	}
	if cfg.byteMode {
		return processByteMode(w, b, col, cfg.width)
	}
	return processColumnMode(w, b, col, cfg.width)
}

// processByteMode wraps counting each byte as one unit. R2.3.
func processByteMode(w *bufio.Writer, b byte, col, width int) int {
	col++
	if col > width {
		w.WriteByte('\n')
		col = 1
	}
	w.WriteByte(b)
	return col
}

// processColumnMode wraps counting display columns with tab expansion. R2.2.
func processColumnMode(w *bufio.Writer, b byte, col, width int) int {
	switch b {
	case '\t':
		return processTab(w, col, width)
	case '\b':
		w.WriteByte(b)
		if col > 0 {
			col--
		}
		return col
	case '\r':
		w.WriteByte(b)
		return 0
	default:
		col++
		if col > width {
			w.WriteByte('\n')
			col = 1
		}
		w.WriteByte(b)
		return col
	}
}

// processTab handles a tab character with tab-stop column expansion. R2.2.
func processTab(w *bufio.Writer, col, width int) int {
	newCol := col + tabStop - col%tabStop
	if newCol > width {
		w.WriteByte('\n')
		newCol = tabStop
	}
	w.WriteByte('\t')
	return newCol
}

// parseArgs parses GNU-style command-line arguments into config and files.
func parseArgs(args []string) (config, []string, error) {
	cfg := config{width: defaultWidth}
	var files []string
	endOfFlags := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if endOfFlags || !strings.HasPrefix(arg, "-") || arg == "-" {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		var err error
		i, err = parseFlag(args, i, &cfg)
		if err != nil {
			return cfg, nil, err
		}
	}
	return cfg, files, nil
}

// parseFlag parses combined short flags from a single argument.
func parseFlag(args []string, i int, cfg *config) (int, error) {
	arg := args[i]
	for j := 1; j < len(arg); j++ {
		switch arg[j] {
		case 'b':
			cfg.byteMode = true
		case 'w':
			return parseWidthValue(args, i, j, cfg)
		default:
			return i, fmt.Errorf("invalid option -- '%c'", arg[j])
		}
	}
	return i, nil
}

// parseWidthValue extracts the width number from -wN or -w N form.
func parseWidthValue(args []string, i, j int, cfg *config) (int, error) {
	rest := args[i][j+1:]
	if rest != "" {
		return i, setWidth(cfg, rest)
	}
	i++
	if i >= len(args) {
		return i, fmt.Errorf("option requires an argument -- 'w'")
	}
	return i, setWidth(cfg, args[i])
}

// setWidth validates and sets the width in config. R2.1.
func setWidth(cfg *config, s string) error {
	w, err := strconv.Atoi(s)
	if err != nil || w <= 0 {
		return fmt.Errorf("invalid number of columns: '%s'", s)
	}
	cfg.width = w
	return nil
}
