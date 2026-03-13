// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd012-yes R1.1–R1.4, R2.1–R2.2, R3.1–R3.3
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	// R3.1, D2: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	// R2.1, R2.2: only the first argument is checked for --help/--version.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help":
			// R2.2: print usage to stdout and exit 0.
			if _, err := fmt.Print(helpText); err != nil {
				os.Exit(1)
			}
			return
		case "--version":
			// R2.1: print version to stdout and exit 0.
			if _, err := fmt.Println("yes (go-unix-utils) 0.1"); err != nil {
				os.Exit(1)
			}
			return
		}
	}

	// R1.1: default output is "y".
	line := "y"

	// R1.2: join arguments with spaces when provided.
	// R1.3: consume "--" separator so flag-like args can be passed.
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) > 0 {
		line = strings.Join(args, " ")
	}

	// D2: buffer output for efficient repeated writes.
	w := bufio.NewWriter(os.Stdout)

	// R1.1, R1.2: write line repeatedly until write error or SIGPIPE.
	for {
		_, err := w.WriteString(line)
		if err != nil {
			// D4: exit 0 on broken pipe (SIGPIPE handled by signal handler).
			break
		}
		err = w.WriteByte('\n')
		if err != nil {
			break
		}
		// R2.2 (prd012): flush so partial buffer reaches pipe reader.
		if err := w.Flush(); err != nil {
			break
		}
	}
}

// helpText is the usage message printed by --help.
const helpText = `Usage: yes [STRING]...
  or:  yes OPTION
Repeatedly output a line with all specified STRING(s), or 'y'.

      --help     display this help and exit
      --version  output version information and exit
`
