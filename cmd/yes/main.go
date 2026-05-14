// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bufio"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	line := buildOutputLine(os.Args[1:])
	w := bufio.NewWriterSize(os.Stdout, 8192)
	for {
		_, err := w.WriteString(line)
		if err != nil {
			os.Exit(1)
		}
	}
}

func buildOutputLine(args []string) string {
	positional := stripDashDash(args)
	if len(positional) == 0 {
		return "y\n"
	}
	return strings.Join(positional, " ") + "\n"
}

func stripDashDash(args []string) []string {
	if len(args) > 0 && args[0] == "--" {
		return args[1:]
	}
	return args
}
