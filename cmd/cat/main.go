// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd006-cat R1.1–R1.5, R2.1–R2.4, R3.1–R3.3, R4.1–R4.4
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// catOpts holds the parsed flag state for the cat command.
type catOpts struct {
	numberAll       bool // -n: number all output lines
	numberNonBlank  bool // -b: number non-blank output lines only
	squeeze         bool // -s: squeeze runs of blank lines
	showNonPrinting bool // -v: show non-printing characters using ^ and M- notation
	showEnds        bool // -E: append "$" before each newline
	showTabs        bool // -T: display tab characters as ^I
}

// active reports whether any output-transforming flag is set.
func (o catOpts) active() bool {
	return o.numberAll || o.numberNonBlank || o.squeeze ||
		o.showNonPrinting || o.showEnds || o.showTabs
}

// processor streams input to output applying cat flag transformations.
// Line counter and blank-run state are shared across all input files,
// matching GNU cat behavior.
type processor struct {
	opts     catOpts
	w        *bufio.Writer
	lineNum  int // R2.1, R2.2: shared line counter across all inputs
	blankRun int // R3.1: consecutive blank-line count for squeeze
}

func main() {
	// R1.5: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	flagN := flag.Bool("n", false, "number all output lines")
	flagB := flag.Bool("b", false, "number non-blank output lines only")
	flagS := flag.Bool("s", false, "squeeze multiple adjacent blank lines")
	flagV := flag.Bool("v", false, "use ^ and M- notation, except for LFD and TAB")
	flagE := flag.Bool("E", false, "display $ at end of each line")
	flagT := flag.Bool("T", false, "display TAB characters as ^I")
	flag.Parse()

	p := &processor{
		opts: catOpts{
			numberAll:       *flagN,
			numberNonBlank:  *flagB,
			squeeze:         *flagS,
			showNonPrinting: *flagV,
			showEnds:        *flagE,
			showTabs:        *flagT,
		},
		w: bufio.NewWriter(os.Stdout),
	}

	args := flag.Args()
	exitCode := 0

	if len(args) == 0 {
		// R1.2: no file arguments — read from stdin.
		if err := p.process(os.Stdin); err != nil {
			fmt.Fprintf(os.Stderr, "cat: write error: %v\n", err)
			p.w.Flush() // best-effort flush, error ignored
			os.Exit(1)
		}
	} else {
		// R1.1, R1.3, R1.4: process each argument left to right; "-" means stdin.
		for _, arg := range args {
			if err := p.processFile(arg); err != nil {
				fmt.Fprintf(os.Stderr, "cat: %v\n", err)
				exitCode = 1
			}
		}
	}

	if err := p.w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "cat: write error: %v\n", err)
		os.Exit(1)
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// processFile opens name and streams it through the processor.
// R1.3: "-" reads from stdin at this position in the sequence.
func (p *processor) processFile(name string) error {
	if name == "-" {
		return p.process(os.Stdin)
	}
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close() // best-effort cleanup, error ignored
	return p.process(f)
}

// process reads all content from r and writes it to p.w, applying flag transformations.
func (p *processor) process(r io.Reader) error {
	if !p.opts.active() {
		// Fast path: no transformations needed.
		if _, err := io.Copy(p.w, r); err != nil {
			return fmt.Errorf("write error: %w", err)
		}
		return nil
	}
	return p.processLines(r)
}

// processLines applies -n, -b, -s, -v, -E, -T transformations one line at a time.
func (p *processor) processLines(r io.Reader) error {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			if werr := p.writeLine(line); werr != nil {
				return werr
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read error: %w", err)
		}
	}
	return nil
}

// writeLine applies squeeze, display, and numbering transformations and writes line to p.w.
// R4.9: order of application is squeeze (-s) → display (-v/-T) → end marker (-E) → line number (-n/-b).
func (p *processor) writeLine(line string) error {
	isBlank := line == "\n"

	// R3.1: suppress extra blank lines when -s is set.
	if isBlank {
		p.blankRun++
		if p.opts.squeeze && p.blankRun > 1 {
			return nil
		}
	} else {
		p.blankRun = 0
	}

	// R2.2: -b takes precedence over -n; blank lines pass through without a number.
	// R2.1: -n numbers all lines including blank ones.
	if p.opts.numberNonBlank {
		if !isBlank {
			p.lineNum++
			if _, err := fmt.Fprintf(p.w, "%6d\t", p.lineNum); err != nil {
				return fmt.Errorf("write error: %w", err)
			}
		}
	} else if p.opts.numberAll {
		p.lineNum++
		if _, err := fmt.Fprintf(p.w, "%6d\t", p.lineNum); err != nil {
			return fmt.Errorf("write error: %w", err)
		}
	}

	// Apply display transformations if any are active.
	if p.opts.showNonPrinting || p.opts.showEnds || p.opts.showTabs {
		return p.writeTransformed(line)
	}
	if _, err := io.WriteString(p.w, line); err != nil {
		return fmt.Errorf("write error: %w", err)
	}
	return nil
}

// writeTransformed writes line with -v, -E, -T transformations applied.
// R4.9: non-printing display (-v/-T) is applied before end marker (-E).
func (p *processor) writeTransformed(line string) error {
	data := []byte(line)
	hasNewline := len(data) > 0 && data[len(data)-1] == '\n'
	content := data
	if hasNewline {
		content = data[:len(data)-1]
	}

	// R4.1, R4.2, R4.4: apply -v and -T transformations byte by byte.
	for _, b := range content {
		var s string
		switch {
		case p.opts.showTabs && b == '\t':
			// R4.4: -T displays tabs as ^I.
			s = "^I"
		case p.opts.showNonPrinting:
			// R4.1, R4.2: -v shows non-printing bytes; tabs and newlines are exempt.
			s = visibleByte(b)
		default:
			s = string([]byte{b})
		}
		if _, err := io.WriteString(p.w, s); err != nil {
			return fmt.Errorf("write error: %w", err)
		}
	}

	// R4.3: -E appends "$" before each newline; the newline itself follows.
	if hasNewline {
		if p.opts.showEnds {
			if _, err := p.w.WriteString("$"); err != nil {
				return fmt.Errorf("write error: %w", err)
			}
		}
		if err := p.w.WriteByte('\n'); err != nil {
			return fmt.Errorf("write error: %w", err)
		}
	}
	return nil
}

// visibleByte converts a single byte to its -v display representation.
// Tabs (0x09) and newlines (0x0A) are returned unchanged per R4.2.
//
// R4.1 encoding rules:
//   - 0x00–0x1F (except 0x09, 0x0A): caret notation ^X where X = b+64
//   - 0x7F (DEL): ^?
//   - 0x80–0x9F: M-^X where X = (b-0x80)+64
//   - 0xA0–0xFE: M-X where X = b-0x80
//   - 0xFF: M-^?
func visibleByte(b byte) string {
	switch {
	case b == '\t' || b == '\n':
		// R4.2: tab and newline are exempt from -v display.
		return string([]byte{b})
	case b < 0x20:
		// Control characters 0x00–0x1F (excluding tab 0x09 and newline 0x0A).
		return "^" + string([]byte{b + 64})
	case b == 0x7F:
		// DEL character.
		return "^?"
	case b >= 0x80 && b <= 0x9F:
		// High control characters: M-^X.
		return "M-^" + string([]byte{b - 0x80 + 64})
	case b >= 0xA0 && b <= 0xFE:
		// High non-printing: M-X.
		return "M-" + string([]byte{b - 0x80})
	case b == 0xFF:
		// 0xFF: M-^?
		return "M-^?"
	default:
		return string([]byte{b})
	}
}
