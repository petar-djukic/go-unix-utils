// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd006-cat R1.1–R1.5, R2.1–R2.4, R3.1–R3.3
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
	numberAll      bool // -n: number all output lines
	numberNonBlank bool // -b: number non-blank output lines only
	squeeze        bool // -s: squeeze runs of blank lines
}

// active reports whether any output-transforming flag is set.
func (o catOpts) active() bool {
	return o.numberAll || o.numberNonBlank || o.squeeze
}

// processor streams input to output applying cat flag transformations.
// Line counter and blank-run state are shared across all input files,
// matching GNU cat behavior.
type processor struct {
	opts     catOpts
	w        *bufio.Writer
	lineNum  int // R2.1, R2.2: shared line counter across all inputs
	blankRun int // R2.3: consecutive blank-line count for squeeze
}

func main() {
	// R1.5: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	flagN := flag.Bool("n", false, "number all output lines")
	flagB := flag.Bool("b", false, "number non-blank output lines only")
	flagS := flag.Bool("s", false, "squeeze multiple adjacent blank lines")
	flag.Parse()

	p := &processor{
		opts: catOpts{
			numberAll:      *flagN,
			numberNonBlank: *flagB,
			squeeze:        *flagS,
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

// processLines applies -n, -b, -s transformations one line at a time.
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

// writeLine applies squeeze and numbering transformations and writes line to p.w.
func (p *processor) writeLine(line string) error {
	isBlank := line == "\n"

	// R2.3: suppress extra blank lines when -s is set.
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

	if _, err := io.WriteString(p.w, line); err != nil {
		return fmt.Errorf("write error: %w", err)
	}
	return nil
}
