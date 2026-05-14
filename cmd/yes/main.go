// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"os"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const blockSize = 8192

func main() {
	sys.InstallSIGPIPEHandler()

	line := buildOutputLine(os.Args[1:])
	block := fillBlock(line)
	for {
		_, err := os.Stdout.Write(block)
		if err != nil {
			if isEPIPE(err) {
				os.Exit(0)
			}
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

// R2.1: pre-fill a block with repeated copies of the line for bulk writes.
func fillBlock(line string) []byte {
	buf := make([]byte, 0, blockSize+len(line))
	for len(buf)+len(line) <= blockSize {
		buf = append(buf, line...)
	}
	if len(buf) == 0 {
		buf = append(buf, line...)
	}
	return buf
}

func isEPIPE(err error) bool {
	return errors.Is(err, syscall.EPIPE)
}
