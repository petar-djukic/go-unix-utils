// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/fold implements the fold (wrap lines) command.
// Implements: prd023-fold R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R3.1, R3.2, R3.3, R3.4, R4.1, R4.2, R4.3, R4.4
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// R1.1: Default wrap width is 80 columns.
const defaultWidth = 80

// tabStop is the interval for tab stop positions.
// R2.2: Tab stops every 8 columns.
const tabStop = 8

// foldConfig holds parsed options for the fold command.
type foldConfig struct {
	width     int      // R2.1: maximum line width
	byteMode  bool     // R2.3: count bytes instead of columns
	spaceMode bool     // R3.1: break at spaces
	files     []string // input files; empty means stdin
}

// parseArgs parses command-line arguments into a foldConfig.
func parseArgs(args []string) (foldConfig, error) {
	cfg := foldConfig{
		width: defaultWidth,
	}

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			cfg.files = append(cfg.files, args[i+1:]...)
			break
		}
		if arg == "-" || !isFlag(arg) {
			cfg.files = append(cfg.files, arg)
			i++
			continue
		}

		// Process flag characters in the argument.
		j := 1
		for j < len(arg) {
			ch := arg[j]
			switch ch {
			case 'b':
				// R2.3: byte mode
				cfg.byteMode = true
				j++
			case 's':
				// R3.1: space-break mode
				cfg.spaceMode = true
				j++
			case 'w':
				// R2.1: -w N
				rest := arg[j+1:]
				var val string
				if rest != "" {
					val = rest
				} else {
					i++
					if i >= len(args) {
						return cfg, fmt.Errorf("option requires an argument -- 'w'")
					}
					val = args[i]
				}
				w, err := strconv.Atoi(val)
				if err != nil || w <= 0 {
					return cfg, fmt.Errorf("fold: invalid number of columns: %q", val)
				}
				cfg.width = w
				j = len(arg) // consumed rest of arg
			default:
				return cfg, fmt.Errorf("fold: invalid option -- '%c'", ch)
			}
		}
		i++
	}

	return cfg, nil
}

// isFlag returns true if the argument looks like a flag (starts with '-' and has length > 1).
func isFlag(arg string) bool {
	return len(arg) > 1 && arg[0] == '-'
}

func main() {
	// R4.4: Install SIGPIPE handler per ARCHITECTURE.yaml shared protocol.
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	exitCode := 0
	w := bufio.NewWriter(os.Stdout)

	if len(cfg.files) == 0 {
		// R1.1: No file arguments — read from stdin.
		if err := foldReader(os.Stdin, w, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "fold: %v\n", err)
			os.Exit(1)
		}
	} else {
		for _, name := range cfg.files {
			if err := foldFile(name, w, cfg); err != nil {
				fmt.Fprintf(os.Stderr, "fold: %v\n", err)
				exitCode = 1
			}
		}
	}

	// R4.3: Flush buffered output; exit 1 on write error.
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "fold: write error: %v\n", err)
		os.Exit(1)
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// foldFile opens name and folds its lines to the writer.
// R1.1: "-" reads from stdin.
func foldFile(name string, w *bufio.Writer, cfg foldConfig) error {
	if name == "-" {
		return foldReader(os.Stdin, w, cfg)
	}
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }() // best-effort cleanup, error ignored
	return foldReader(f, w, cfg)
}

// foldReader reads from r and writes folded output to w.
//
// R1.1-R1.4: Wraps lines at the configured width.
// R2.2: In column mode, tabs advance to the next tab stop.
// R2.3: In byte mode, each byte counts as 1.
// R3.1-R3.4: In space mode, breaks at the last space before the wrap column.
func foldReader(r io.Reader, w *bufio.Writer, cfg foldConfig) error {
	br := bufio.NewReader(r)

	// buf accumulates bytes for the current line segment.
	var buf []byte
	col := 0       // current column position (column mode) or byte count (byte mode)
	lastSpace := -1 // index in buf of last space seen (for -s mode)

	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				// R1.4: Final segment without trailing newline.
				if len(buf) > 0 {
					if _, werr := w.Write(buf); werr != nil {
						return fmt.Errorf("write error: %w", werr)
					}
				}
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}

		if b == '\n' {
			// R1.2, R1.4: Output accumulated segment with newline, reset.
			buf = append(buf, '\n')
			if _, werr := w.Write(buf); werr != nil {
				return fmt.Errorf("write error: %w", werr)
			}
			buf = buf[:0]
			col = 0
			lastSpace = -1
			continue
		}

		// Calculate the new column after this byte.
		var newCol int
		if cfg.byteMode {
			// R2.3: Each byte counts as 1.
			newCol = col + 1
		} else {
			// R2.2: Column mode with tab expansion.
			switch b {
			case '\t':
				newCol = col + tabStop - col%tabStop
			case '\b':
				if col > 0 {
					newCol = col - 1
				} else {
					newCol = 0
				}
			case '\r':
				newCol = 0
			default:
				newCol = col + 1
			}
		}

		// R1.3: If adding this byte would exceed width, insert a wrap.
		if newCol > cfg.width {
			if cfg.spaceMode && lastSpace >= 0 {
				// R3.1, R3.3: Break after the last space.
				breakAt := lastSpace + 1 // include the space
				if _, werr := w.Write(buf[:breakAt]); werr != nil {
					return fmt.Errorf("write error: %w", werr)
				}
				if werr := w.WriteByte('\n'); werr != nil {
					return fmt.Errorf("write error: %w", werr)
				}
				// Carry over the remaining bytes after the space.
				remaining := make([]byte, len(buf[breakAt:]))
				copy(remaining, buf[breakAt:])
				buf = remaining
				// Recompute column for the carried-over bytes.
				col = recomputeCol(buf, cfg)
				lastSpace = -1
				// Re-check for spaces in the carried-over buffer.
				for idx, ch := range buf {
					if ch == ' ' {
						lastSpace = idx
					}
				}
				// Now add the current byte.
				newCol = advanceCol(col, b, cfg)
				if newCol > cfg.width {
					// The carried-over portion plus this byte still exceeds width.
					// Hard break the buffer if non-empty, then start fresh.
					if len(buf) > 0 {
						if err := hardWrap(w, &buf, &col, &lastSpace, cfg); err != nil {
							return err
						}
						newCol = advanceCol(col, b, cfg)
						if newCol > cfg.width && col == 0 {
							// Single byte exceeds width — output it anyway.
							buf = append(buf, b)
							if _, werr := w.Write(buf); werr != nil {
								return fmt.Errorf("write error: %w", werr)
							}
							if werr := w.WriteByte('\n'); werr != nil {
								return fmt.Errorf("write error: %w", werr)
							}
							buf = buf[:0]
							col = 0
							lastSpace = -1
							continue
						}
					}
				}
				buf = append(buf, b)
				col = newCol
				if b == ' ' {
					lastSpace = len(buf) - 1
				}
			} else {
				// R1.3, R3.2: Hard break at exactly W columns.
				if len(buf) > 0 {
					if _, werr := w.Write(buf); werr != nil {
						return fmt.Errorf("write error: %w", werr)
					}
				}
				if werr := w.WriteByte('\n'); werr != nil {
					return fmt.Errorf("write error: %w", werr)
				}
				buf = buf[:0]
				col = 0
				lastSpace = -1
				// Add current byte to fresh segment.
				buf = append(buf, b)
				col = advanceCol(0, b, cfg)
				if b == ' ' && cfg.spaceMode {
					lastSpace = 0
				}
			}
		} else {
			buf = append(buf, b)
			col = newCol
			if b == ' ' && cfg.spaceMode {
				lastSpace = len(buf) - 1
			}
		}
	}
}

// hardWrap writes buf contents with hard wraps at cfg.width, resetting state.
func hardWrap(w *bufio.Writer, buf *[]byte, col *int, lastSpace *int, _ foldConfig) error {
	if _, werr := w.Write(*buf); werr != nil {
		return fmt.Errorf("write error: %w", werr)
	}
	if werr := w.WriteByte('\n'); werr != nil {
		return fmt.Errorf("write error: %w", werr)
	}
	*buf = (*buf)[:0]
	*col = 0
	*lastSpace = -1
	return nil
}

// advanceCol computes the new column after adding byte b at column col.
func advanceCol(col int, b byte, cfg foldConfig) int {
	if cfg.byteMode {
		return col + 1
	}
	switch b {
	case '\t':
		return col + tabStop - col%tabStop
	case '\b':
		if col > 0 {
			return col - 1
		}
		return 0
	case '\r':
		return 0
	default:
		return col + 1
	}
}

// recomputeCol computes the column position for a buffer of bytes.
func recomputeCol(buf []byte, cfg foldConfig) int {
	col := 0
	for _, b := range buf {
		col = advanceCol(col, b, cfg)
	}
	return col
}
