// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const blockSize = 8192

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	if handleSpecialFlag(args) {
		return
	}

	line := buildOutputLine(args)
	block := fillBlock(line)
	writeLoop(block)
}

func handleSpecialFlag(args []string) bool {
	if len(args) != 1 {
		return false
	}
	switch args[0] {
	case "--help":
		printHelp()
		return true
	case "--version":
		printVersion()
		return true
	}
	return false
}

func printHelp() {
	fmt.Print("Usage: yes [STRING]...\n" +
		"  or:  yes OPTION\n" +
		"Repeatedly output a line with all specified STRING(s), or 'y'.\n" +
		"\n" +
		"      --help     display this help and exit\n" +
		"      --version  output version information and exit\n")
}

func printVersion() {
	fmt.Println("yes (go-unix-utils)")
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

func writeLoop(block []byte) {
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

func isEPIPE(err error) bool {
	return errors.Is(err, syscall.EPIPE)
}
