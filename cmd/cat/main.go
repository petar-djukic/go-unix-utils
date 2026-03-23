// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd006-cat: Concatenate and Display Files.
// Covers R1.1-R1.5 (basic concatenation), R2.1-R2.4 (line numbering),
// R3.1-R3.3 (blank squeezing), R4.1-R4.9 (non-printing display),
// R5.1-R5.4 (error handling and exit codes).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
// Defaults to "dev" when the linker variable is not set.
var version = "dev"

// catOptions holds the parsed flag state for a cat invocation.
// R1-R4: flags map directly to GNU cat flag semantics.
type catOptions struct {
	numberAll      bool // -n: number all output lines (R2.1)
	numberNonBlank bool // -b: number non-blank lines only (R2.2)
	squeezeBlank   bool // -s: squeeze consecutive blank lines (R3.1)
	showEnds       bool // -E: show $ at end of each line (R4.3)
	showTabs       bool // -T: show tabs as ^I (R4.4)
	showNonPrint   bool // -v: show non-printing chars (R4.1)
}

func main() {
	// R5.4: Install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	opts, files := parseFlags(os.Args[1:])
	exitCode := run(opts, files, os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// parseFlags parses GNU cat-compatible flags from the argument list.
// R1.1-R1.5, R2.1-R2.4, R3.1-R3.3, R4.1-R4.9: all flags defined.
// R4.8: -u is accepted but ignored.
// R5.1: --version prints version to stdout and exits 0.
// R5.2: --help prints usage to stdout and exits 0.
func parseFlags(args []string) (catOptions, []string) {
	var opts catOptions
	var files []string
	endOfFlags := false

	for _, arg := range args {
		if endOfFlags {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if arg == "--version" {
			printVersion()
			os.Exit(0)
		}
		if arg == "--help" {
			printHelp()
			os.Exit(0)
		}
		if len(arg) > 1 && arg[0] == '-' {
			if err := applyShortFlags(arg[1:], &opts); err != nil {
				fmt.Fprintf(os.Stderr, "cat: %s\n", err)
				os.Exit(1)
			}
		} else {
			files = append(files, arg)
		}
	}
	return opts, files
}

// printVersion writes the version string to stdout in GNU format.
// R5.1: "cat (go-unix-utils) VERSION" followed by a newline.
func printVersion() {
	fmt.Printf("cat (go-unix-utils) %s\n", version)
}

// printHelp writes usage information to stdout.
// R5.2: usage includes synopsis and flag descriptions.
func printHelp() {
	fmt.Print(`Usage: cat [OPTION]... [FILE]...
Concatenate FILE(s) to standard output.

With no FILE, or when FILE is -, read standard input.

  -A, --show-all           equivalent to -vET
  -b, --number-nonblank    number nonempty output lines, overrides -n
  -e                       equivalent to -vE
  -E, --show-ends          display $ at end of each line
  -n, --number             number all output lines
  -s, --squeeze-blank      suppress repeated empty output lines
  -t                       equivalent to -vT
  -T, --show-tabs          display TAB characters as ^I
  -u                       (ignored)
  -v, --show-nonprinting   use ^ and M- notation, except for LFD and TAB
      --help               display this help and exit
      --version            output version information and exit
`)
}

// applyShortFlags applies a sequence of short flag characters to opts.
// R4.5: -A is -vET. R4.6: -e is -vE. R4.7: -t is -vT.
func applyShortFlags(flags string, opts *catOptions) error {
	for _, c := range flags {
		switch c {
		case 'n':
			opts.numberAll = true
		case 'b':
			opts.numberNonBlank = true
		case 's':
			opts.squeezeBlank = true
		case 'E':
			opts.showEnds = true
		case 'T':
			opts.showTabs = true
		case 'v':
			opts.showNonPrint = true
		case 'A':
			opts.showNonPrint, opts.showEnds, opts.showTabs = true, true, true
		case 'e':
			opts.showNonPrint, opts.showEnds = true, true
		case 't':
			opts.showNonPrint, opts.showTabs = true, true
		case 'u':
			// R4.8: accepted but ignored
		default:
			return fmt.Errorf("invalid option -- '%c'", c)
		}
	}
	return nil
}

// run processes all files with the given options and returns the exit code.
// R1.1: reads each named file in argument order.
// R1.2: reads stdin when no files given or when "-" is a filename.
// R5.1: returns 0 on success.
// R5.2: returns 1 on any file open error, continues processing.
func run(opts catOptions, files []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if len(files) == 0 {
		files = []string{"-"}
	}
	state := newLineState()
	exitCode := 0
	for _, name := range files {
		r, err := openInput(name, stdin)
		if err != nil {
			reportError(stderr, name, err)
			exitCode = 1
			continue
		}
		err = processFile(r, stdout, opts, state)
		if name != "-" {
			r.Close()
		}
		if err != nil {
			exitCode = 1
		}
	}
	return exitCode
}

// processFile reads from r and writes transformed output to w.
// R1.3: content written in sequence with no separator.
// R1.4/R5.4: binary input not corrupted when no transform flags active.
// R4.9: applies transforms in order: squeeze, non-printing, ends, numbering.
func processFile(r io.Reader, w io.Writer, opts catOptions, state *lineState) error {
	if !needsTransform(opts) {
		_, err := io.Copy(w, r)
		return err
	}
	bw := bufio.NewWriterSize(w, 8192)
	buf := make([]byte, 4096)
	for {
		n, readErr := r.Read(buf)
		for i := range n {
			if err := processByte(bw, buf[i], opts, state); err != nil {
				return err
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return bw.Flush()
			}
			return readErr
		}
	}
}

// needsTransform reports whether any transformation flag is active.
func needsTransform(opts catOptions) bool {
	return opts.numberAll || opts.numberNonBlank || opts.squeezeBlank ||
		opts.showEnds || opts.showTabs || opts.showNonPrint
}

// lineState tracks state across files for numbering and squeezing.
// R2.1: line number resets to 1 per invocation, not per file.
// R3.2: blank squeezing applies across file boundaries.
type lineState struct {
	lineNumber     int  // current line number counter
	atLineStart    bool // whether we are at the start of a new line
	consecutiveNLs int  // count of consecutive blank lines for squeezing
}

// newLineState creates the initial line state for a cat invocation.
func newLineState() *lineState {
	return &lineState{atLineStart: true}
}

// processByte handles one input byte, applying squeeze/number/transform.
// R4.9: order is squeeze, then non-printing, then ends, then number.
func processByte(w *bufio.Writer, b byte, opts catOptions, state *lineState) error {
	if state.atLineStart {
		blank := b == '\n'
		if shouldSqueeze(opts, state, blank) {
			state.consecutiveNLs++
			return nil
		}
		if blank {
			state.consecutiveNLs++
		} else {
			state.consecutiveNLs = 0
		}
		if shouldNumber(opts, blank) {
			state.lineNumber++
			if _, err := w.WriteString(formatLineNumber(state.lineNumber)); err != nil {
				return err
			}
		}
		if !blank {
			state.atLineStart = false
		}
	}
	return writeOutputByte(w, b, opts, state)
}

// writeOutputByte writes a single byte to output with end/transform flags.
// R4.3: "$" before newline when -E active.
func writeOutputByte(w *bufio.Writer, b byte, opts catOptions, state *lineState) error {
	if b == '\n' {
		state.atLineStart = true
		if opts.showEnds {
			if err := w.WriteByte('$'); err != nil {
				return err
			}
		}
		return w.WriteByte('\n')
	}
	transformed := transformByte(b, opts)
	_, err := w.Write(transformed)
	return err
}

// transformByte converts a single byte according to active display flags.
// R4.1: caret notation and M- prefix for non-printing characters.
// R4.2: tabs and newlines exempted from -v display.
// R4.4: tabs shown as ^I when -T active.
func transformByte(b byte, opts catOptions) []byte {
	if b == '\t' {
		if opts.showTabs {
			return []byte{'^', 'I'}
		}
		return []byte{b}
	}
	if !opts.showNonPrint {
		return []byte{b}
	}
	return transformNonPrint(b)
}

// transformNonPrint converts a non-printing byte to its display form.
// R4.1: control -> ^X, 0x80-0x9F -> M-^X, 0xA0-0xFE -> M-X,
// 0x7F -> ^?, 0xFF -> M-^?.
func transformNonPrint(b byte) []byte {
	switch {
	case b < 32 && b != '\t' && b != '\n':
		return []byte{'^', b + 64}
	case b == 127:
		return []byte{'^', '?'}
	case b >= 128 && b < 160:
		return []byte{'M', '-', '^', b - 128 + 64}
	case b >= 160 && b < 255:
		return []byte{'M', '-', b - 128}
	case b == 255:
		return []byte{'M', '-', '^', '?'}
	default:
		return []byte{b}
	}
}

// formatLineNumber formats a line number in GNU cat format.
// R2.1: right-justified in width 6, followed by tab.
func formatLineNumber(n int) string {
	return fmt.Sprintf("%6d\t", n)
}

// shouldNumber reports whether the current line should be numbered.
// R2.2: -b skips blank lines.
// R2.3: -b takes precedence over -n.
func shouldNumber(opts catOptions, blank bool) bool {
	if opts.numberNonBlank {
		return !blank
	}
	return opts.numberAll
}

// shouldSqueeze reports whether a blank line should be suppressed.
// R3.1: only the first of consecutive blank lines is written.
func shouldSqueeze(opts catOptions, state *lineState, blank bool) bool {
	return opts.squeezeBlank && blank && state.consecutiveNLs > 0
}

// openInput opens a file for reading, or returns stdin for "-".
// R1.2: "-" means stdin.
// R5.2: returns error for files that cannot be opened.
func openInput(name string, stdin io.Reader) (io.ReadCloser, error) {
	if name == "-" {
		return io.NopCloser(stdin), nil
	}
	return os.Open(name)
}

// reportError writes a cat-style error message to stderr.
// R5.2: error message written to stderr.
func reportError(stderr io.Writer, filename string, err error) {
	_, _ = fmt.Fprintf(stderr, "cat: %s: %s\n", filename, err) // best-effort write
}
