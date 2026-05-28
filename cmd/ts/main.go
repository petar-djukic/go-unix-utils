// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd004-ts R1.1, R1.2, R1.3, R1.4.
package main

import (
	"bufio"
	"fmt"
	"os"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run())
}

func run() int {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		ts := time.Now().Format("Jan 02 15:04:05")
		fmt.Fprintf(os.Stdout, "%s %s\n", ts, line)
	}
	return 0
}
