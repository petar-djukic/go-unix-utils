// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/expand converts tabs to spaces (prd024-expand R1).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const defaultTabStop = 8

func main() {
	sys.InstallSIGPIPEHandler()
	cfg := parseArgs(os.Args[1:])
	os.Exit(run(cfg))
}

type config struct {
	files []string
}

func parseArgs(args []string) config {
	var cfg config
	for i := range len(args) {
		a := args[i]
		if a == "--" {
			cfg.files = append(cfg.files, args[i+1:]...)
			break
		}
		if a == "-" || len(a) == 0 || a[0] != '-' {
			cfg.files = append(cfg.files, a)
			continue
		}
		die(fmt.Sprintf("invalid option -- '%s'", a[1:]))
	}
	if len(cfg.files) == 0 {
		cfg.files = []string{"-"}
	}
	return cfg
}

// run processes all files and returns the exit code.
// R1.3: multiple file arguments are processed in order.
func run(cfg config) int {
	out := bufio.NewWriter(os.Stdout)
	exitCode := 0
	for _, name := range cfg.files {
		if err := processFile(name, out); err != nil {
			fmt.Fprintf(os.Stderr, "expand: %v\n", err)
			exitCode = 1
		}
	}
	if err := out.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "expand: write error: %v\n", err)
		return 1
	}
	return exitCode
}

// processFile opens one input and expands its tabs.
func processFile(name string, out *bufio.Writer) error {
	r, err := openInput(name)
	if err != nil {
		if pe, ok := err.(*os.PathError); ok {
			return fmt.Errorf("%s: %s", name, pe.Err)
		}
		return err
	}
	if r != os.Stdin {
		defer r.Close()
	}
	return expandStream(bufio.NewReader(r), out)
}

func openInput(name string) (*os.File, error) {
	if name == "-" {
		return os.Stdin, nil
	}
	return os.Open(name)
}

// expandStream reads input byte by byte and replaces tabs with spaces.
// R1.1: tabs advance to the next multiple-of-8 column (1-indexed).
// R1.2: consecutive tabs each advance independently.
// R1.3: non-tab characters pass through unchanged.
// R1.4: newlines reset column to 0 (internally 0-based).
func expandStream(r *bufio.Reader, out *bufio.Writer) error {
	col := 0
	for {
		c, err := r.ReadByte()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if werr := writeByte(out, c, &col); werr != nil {
			return werr
		}
	}
}

// writeByte handles one input byte, expanding tabs to spaces.
func writeByte(out *bufio.Writer, c byte, col *int) error {
	switch c {
	case '\t':
		return expandTab(out, col)
	case '\n':
		*col = 0
		return out.WriteByte('\n')
	default:
		*col++
		return out.WriteByte(c)
	}
}

// expandTab writes spaces to advance to the next tab stop.
func expandTab(out *bufio.Writer, col *int) error {
	spaces := defaultTabStop - *col%defaultTabStop
	for range spaces {
		if err := out.WriteByte(' '); err != nil {
			return err
		}
	}
	*col += spaces
	return nil
}

func die(msg string) {
	fmt.Fprintf(os.Stderr, "expand: %s\n", msg)
	os.Exit(1)
}
