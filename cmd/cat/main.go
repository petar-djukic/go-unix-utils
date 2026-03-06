// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the cmd/cat binary.
// Concatenates files (or stdin) to stdout with optional line numbering,
// blank-line squeezing, and non-printing character display.
//
// Implements: prd006-cat R1-R5
// Architecture: docs/ARCHITECTURE.yaml § cmd/
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

// catConfig holds the resolved transformation settings for a single cat invocation.
// Alias flags (-A, -e, -t) are expanded into their component fields before construction.
type catConfig struct {
	number          bool // -n: number all output lines (R2.1)
	numberNonblank  bool // -b: number non-blank lines only; overrides -n (R2.2, R2.3)
	squeezeBlank    bool // -s: suppress repeated blank lines (R3.1)
	showNonprinting bool // -v: caret/M- notation for non-printing bytes (R4.1)
	showEnds        bool // -E: append $ before each newline (R4.3)
	showTabs        bool // -T: display tab as ^I (R4.4)
}

// catProcessor holds per-invocation mutable state shared across all file operands.
type catProcessor struct {
	cfg       catConfig
	lineNum   int  // 1-based line counter; resets only per invocation, not per file (R2.1)
	prevBlank bool // true when the last written line was blank; enables -s across file boundaries (R3.2)
}

// newCatProcessor creates a catProcessor with lineNum initialized to 1.
func newCatProcessor(cfg catConfig) *catProcessor {
	return &catProcessor{cfg: cfg, lineNum: 1}
}

// needsTransform reports whether any transformation flag is active.
// Returns false only when output is a verbatim byte-for-byte copy of input (R1.4).
func (p *catProcessor) needsTransform() bool {
	return p.cfg.number || p.cfg.numberNonblank || p.cfg.squeezeBlank ||
		p.cfg.showNonprinting || p.cfg.showEnds || p.cfg.showTabs
}

// processReader reads from r and writes to w, applying all active transformations.
// State (lineNum, prevBlank) persists across calls so successive file operands
// share line-numbering and blank-squeezing context (R2.1, R3.2).
func (p *catProcessor) processReader(r io.Reader, w *bufio.Writer) error {
	if !p.needsTransform() {
		// R1.4: fast path — byte-for-byte copy with no transformation.
		_, err := io.Copy(w, r)
		return err
	}

	br := bufio.NewReader(r)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			hasNewline := line[len(line)-1] == '\n'
			content := line
			if hasNewline {
				content = line[:len(line)-1]
			}
			if werr := p.writeLine(content, hasNewline, w); werr != nil {
				return werr
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// writeLine applies all active transformations to one logical line and writes it to w.
// Transformations are applied in the order specified by R4.9:
// squeeze → non-printing/tab display → end-of-line marker → line number prefix.
//
// content holds the line bytes without the trailing newline.
// hasNewline reports whether the original input line had a trailing newline.
func (p *catProcessor) writeLine(content []byte, hasNewline bool, w *bufio.Writer) error {
	isBlank := len(content) == 0 && hasNewline

	// R3.1: discard the second and subsequent consecutive blank lines.
	if p.cfg.squeezeBlank && isBlank && p.prevBlank {
		return nil
	}

	// R2.1, R2.2, R2.3: write line number prefix.
	// -b takes precedence over -n; blank lines are not numbered when -b is active.
	if p.cfg.numberNonblank {
		if !isBlank {
			if _, err := fmt.Fprintf(w, "%6d\t", p.lineNum); err != nil {
				return err
			}
			p.lineNum++
		}
	} else if p.cfg.number {
		if _, err := fmt.Fprintf(w, "%6d\t", p.lineNum); err != nil {
			return err
		}
		p.lineNum++
	}

	// Write content bytes with -v and -T transformations applied per byte.
	for _, b := range content {
		if err := p.writeTransformedByte(b, w); err != nil {
			return err
		}
	}

	// R4.3: append $ before the newline.
	if hasNewline && p.cfg.showEnds {
		if err := w.WriteByte('$'); err != nil {
			return err
		}
	}

	if hasNewline {
		if err := w.WriteByte('\n'); err != nil {
			return err
		}
	}

	p.prevBlank = isBlank
	return nil
}

// writeTransformedByte writes b to w, applying -T (R4.4) before -v (R4.1) so that
// tab handling is correct: with only -v active, tabs pass through unchanged (R4.2).
func (p *catProcessor) writeTransformedByte(b byte, w *bufio.Writer) error {
	// R4.4: display tab as two-character sequence ^I when -T is active.
	if b == '\t' && p.cfg.showTabs {
		_, err := w.WriteString("^I")
		return err
	}

	if p.cfg.showNonprinting {
		switch {
		case b == 0x7F:
			// DEL -> ^? (R4.1)
			_, err := w.WriteString("^?")
			return err
		case b < 0x20 && b != '\t':
			// Control character (not tab): ^X where X = b+64 (R4.1).
			// Newline (0x0A) is never present in content; only 0x00–0x1F
			// excluding tab reaches this branch.
			_, err := w.Write([]byte{'^', b + 64})
			return err
		case b >= 0x80 && b <= 0x9F:
			// High control: M-^X where X = (b−0x80)+64 (R4.1).
			_, err := w.Write([]byte{'M', '-', '^', b - 0x80 + 64})
			return err
		case b >= 0xA0 && b <= 0xFE:
			// High printable: M-X where X = b−0x80 (R4.1).
			_, err := w.Write([]byte{'M', '-', b - 0x80})
			return err
		case b == 0xFF:
			// 0xFF -> M-^? (R4.1)
			_, err := w.WriteString("M-^?")
			return err
		}
	}

	return w.WriteByte(b)
}

// processOperand processes one file operand. When operand is "-", stdin is read.
// On file open failure, the error is returned; the caller writes the diagnostic
// and continues with the next operand (R5.2).
func processOperand(proc *catProcessor, operand string, w *bufio.Writer) error {
	if operand == "-" {
		return proc.processReader(os.Stdin, w)
	}
	f, err := os.Open(operand)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close() // best-effort cleanup, error ignored
	}()
	return proc.processReader(f, w)
}

func main() {
	// R5.4: install SIGPIPE handler so downstream pipe close exits 0, not nonzero.
	// signal.Notify is used rather than signal.Ignore so that deferred functions
	// do not run on SIGPIPE, matching GNU cat behavior.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGPIPE)
	go func() {
		<-sigCh
		os.Exit(0)
	}()

	// Define flags per prd006-cat R2, R3, R4.
	var (
		flagNumber     bool // -n
		flagNonblank   bool // -b
		flagSqueeze    bool // -s
		flagShowVis    bool // -v
		flagShowEnds   bool // -E
		flagShowTabs   bool // -T
		flagShowAll    bool // -A = -v -E -T (R4.5)
		flagVisEnds    bool // -e = -v -E   (R4.6)
		flagVisTabs    bool // -t = -v -T   (R4.7)
		flagUnbuffered bool // -u: accepted, silently ignored (R4.8)
	)

	flag.BoolVar(&flagNumber, "n", false, "number all output lines")
	flag.BoolVar(&flagNonblank, "b", false, "number non-blank output lines")
	flag.BoolVar(&flagSqueeze, "s", false, "suppress repeated empty output lines")
	flag.BoolVar(&flagShowVis, "v", false, "use caret and M- notation for non-printing characters")
	flag.BoolVar(&flagShowEnds, "E", false, "display $ at end of each line")
	flag.BoolVar(&flagShowTabs, "T", false, "display TAB characters as ^I")
	flag.BoolVar(&flagShowAll, "A", false, "equivalent to -v -E -T")
	flag.BoolVar(&flagVisEnds, "e", false, "equivalent to -v -E")
	flag.BoolVar(&flagVisTabs, "t", false, "equivalent to -v -T")
	flag.BoolVar(&flagUnbuffered, "u", false, "(ignored)")

	flag.Parse()

	// Expand alias flags into their component fields (R4.5, R4.6, R4.7).
	cfg := catConfig{
		number:          flagNumber,
		numberNonblank:  flagNonblank,
		squeezeBlank:    flagSqueeze,
		showNonprinting: flagShowVis || flagShowAll || flagVisEnds || flagVisTabs,
		showEnds:        flagShowEnds || flagShowAll || flagVisEnds,
		showTabs:        flagShowTabs || flagShowAll || flagVisTabs,
	}

	out := bufio.NewWriter(os.Stdout)
	proc := newCatProcessor(cfg)
	exitCode := 0

	args := flag.Args()
	if len(args) == 0 {
		// R1.2: no operands — read from stdin.
		args = []string{"-"}
	}

	for _, arg := range args {
		if err := processOperand(proc, arg, out); err != nil {
			// R5.2: write error to stderr, continue with remaining operands.
			fmt.Fprintf(os.Stderr, "cat: %s: %v\n", arg, err)
			exitCode = 1
		}
	}

	// R5.3: detect and report stdout write errors.
	if err := out.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "cat: write error: %v\n", err)
		exitCode = 1
	}

	os.Exit(exitCode)
}
