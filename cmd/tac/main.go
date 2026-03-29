// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/tac implements GNU tac: concatenate and print files in reverse.
//
// Implements prd021-tac: R1 (core reversal), R2 (separator options),
// R3 (exit codes and SIGPIPE).
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// R2.1: -s SEP uses SEP as the record separator instead of newline.
var separator = flag.String("s", "\n", "use STRING as the separator instead of newline")

// R2.2: -b places the separator before each record instead of after it.
var before = flag.Bool("b", false, "attach the separator before instead of after")

// R2.3: -r interprets the separator as a regular expression.
var regex = flag.Bool("r", false, "interpret the separator as a regular expression")

func main() {
	// R3.4: handle SIGPIPE gracefully.
	sys.InstallSIGPIPEHandler()

	flag.Parse()

	files := flag.Args()
	if len(files) == 0 {
		files = []string{"-"}
	}

	exitCode := processFiles(files)
	os.Exit(exitCode)
}

// processFiles processes each file and returns the exit code.
// R1.4: each file is processed independently in argument order.
// R3.1: returns 0 on success. R3.2: returns 1 on any error.
func processFiles(files []string) int {
	exitCode := 0
	for _, name := range files {
		if err := processFile(name); err != nil {
			fmt.Fprintf(os.Stderr, "tac: %v\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

// processFile reads one file (or stdin for "-") and writes its
// records in reverse order.
// R1.1: reads entire input, splits on separator, writes reversed.
// R1.3: reads stdin when filename is "-".
func processFile(name string) error {
	_, _ = readInput(name)
	// TODO: implement record splitting and reversal (R1.1, R1.2, R2.1-R2.4)
	return nil
}

// readInput opens the named file or stdin and reads all bytes.
func readInput(name string) ([]byte, error) {
	if name == "-" {
		return os.ReadFile("/dev/stdin")
	}
	return os.ReadFile(name)
}

// reverseRecords splits data into records and returns them reversed.
// R1.1: splits on separator, reverses order.
// R1.2: trailing separator terminates the last record.
func reverseRecords(data []byte) []byte {
	_ = *separator
	_ = *before
	_ = *regex
	// TODO: implement splitting and reversal logic
	return data
}

// writeOutput writes reversed records to stdout.
// R3.3: returns error on write failure.
func writeOutput(data []byte) error {
	_, err := os.Stdout.Write(data)
	return err
}
