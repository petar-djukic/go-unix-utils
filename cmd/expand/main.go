// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd024-expand.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()
	w := bufio.NewWriter(os.Stdout)
	exitCode := 0
	if len(os.Args) <= 1 {
		expand(os.Stdin, w)
	} else {
		for _, name := range os.Args[1:] {
			if name == "-" {
				expand(os.Stdin, w)
				continue
			}
			f, err := os.Open(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "expand: %v\n", err)
				exitCode = 1
				continue
			}
			expand(f, w)
			f.Close()
		}
	}
	if err := w.Flush(); err != nil {
		os.Exit(1)
	}
	os.Exit(exitCode)
}

func expand(r io.Reader, w *bufio.Writer) {
	br := bufio.NewReader(r)
	col := 1
	for {
		b, err := br.ReadByte()
		if err != nil {
			return
		}
		switch b {
		case '\t':
			spaces := 8 - (col-1)%8
			for range spaces {
				w.WriteByte(' ')
			}
			col += spaces
		case '\n':
			w.WriteByte('\n')
			col = 1
		default:
			w.WriteByte(b)
			col++
		}
	}
}
