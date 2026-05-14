// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/cat implements srd006-cat: file concatenation with line numbering,
// blank squeezing, and non-printing display.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

var (
	flagN bool
	flagB bool
	flagS bool
	flagV bool
	flagE bool
	flagT bool
)

func main() {
	sys.InstallSIGPIPEHandler()

	args := parseFlags(os.Args[1:])
	if len(args) == 0 {
		args = []string{"-"}
	}

	exitCode := 0
	lineNum := 1
	prevBlank := false
	for _, name := range args {
		var err error
		lineNum, prevBlank, err = catFile(name, lineNum, prevBlank)
		if err != nil {
			if errors.Is(err, syscall.EPIPE) {
				os.Exit(0)
			}
			fmt.Fprintf(os.Stderr, "cat: %s\n", formatErr(err))
			exitCode = 1
		}
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func parseFlags(args []string) []string {
	var files []string
	endOfFlags := false
	for _, arg := range args {
		if endOfFlags || arg == "" || arg == "-" || arg[0] != '-' {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		for _, c := range arg[1:] {
			applyFlag(c)
		}
	}
	return files
}

func applyFlag(c rune) {
	switch c {
	case 'n':
		flagN = true
	case 'b':
		flagB = true
	case 's':
		flagS = true
	case 'v':
		flagV = true
	case 'E':
		flagE = true
	case 'T':
		flagT = true
	case 'A':
		flagV = true
		flagE = true
		flagT = true
	case 'e':
		flagV = true
		flagE = true
	case 't':
		flagV = true
		flagT = true
	case 'u':
		// R4.8: accepted but ignored
	default:
		fmt.Fprintf(os.Stderr, "cat: invalid option -- '%c'\n", c)
		os.Exit(1)
	}
}

// R1.1, R1.2: open a named file or stdin and write to stdout.
func catFile(name string, lineNum int, prevBlank bool) (int, bool, error) {
	var r io.Reader
	if name == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(name)
		if err != nil {
			return lineNum, prevBlank, err
		}
		defer f.Close()
		r = f
	}
	if !flagN && !flagB && !flagS && !flagV && !flagE && !flagT {
		_, err := io.Copy(os.Stdout, r)
		return lineNum, prevBlank, err
	}
	return catTransform(r, lineNum, prevBlank)
}

// R4.9: squeeze → transform (-v/-T) → end marker (-E) → number (-n/-b).
// R5.3: always flush to detect stdout write errors.
func catTransform(r io.Reader, lineNum int, prevBlank bool) (int, bool, error) {
	br := bufio.NewReader(r)
	w := bufio.NewWriter(os.Stdout)
	var readErr error
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			lineNum, prevBlank = writeLine(w, line, lineNum, prevBlank)
		}
		if err != nil {
			if err != io.EOF {
				readErr = err
			}
			break
		}
	}
	if ferr := w.Flush(); ferr != nil {
		return lineNum, prevBlank, ferr
	}
	return lineNum, prevBlank, readErr
}

// R2.1, R2.2, R2.3, R3.1, R4.9: squeeze, then transform, then number.
func writeLine(w *bufio.Writer, line []byte, lineNum int, prevBlank bool) (int, bool) {
	isBlank := len(line) == 1 && line[0] == '\n'
	if flagS && isBlank && prevBlank {
		return lineNum, true
	}
	if (flagN || flagB) && !(flagB && isBlank) {
		fmt.Fprintf(w, "%6d\t", lineNum)
		lineNum++
	}
	if flagV || flagE || flagT {
		writeTransformed(w, line)
	} else {
		w.Write(line)
	}
	return lineNum, isBlank
}

// R4.1, R4.2, R4.3, R4.4: byte-level display transformation.
func writeTransformed(w *bufio.Writer, line []byte) {
	for _, b := range line {
		if b == '\n' {
			if flagE {
				w.WriteByte('$')
			}
			w.WriteByte('\n')
			continue
		}
		if b == '\t' {
			if flagT {
				w.WriteString("^I")
			} else {
				w.WriteByte('\t')
			}
			continue
		}
		if flagV {
			writeVisible(w, b)
			continue
		}
		w.WriteByte(b)
	}
}

// R4.1: caret and M- notation for non-printing characters.
func writeVisible(w *bufio.Writer, b byte) {
	if b >= 128 {
		w.WriteString("M-")
		b -= 128
	}
	if b < 32 {
		w.WriteByte('^')
		w.WriteByte(b + 64)
	} else if b == 127 {
		w.WriteByte('^')
		w.WriteByte('?')
	} else {
		w.WriteByte(b)
	}
}

// R5.2: format os.PathError for stderr output.
func formatErr(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return fmt.Sprintf("%s: %s", pe.Path, pe.Err)
	}
	return err.Error()
}
