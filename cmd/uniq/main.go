// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd028-uniq.
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
	inputFile, outputFile, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "uniq: %s\n", err)
		os.Exit(1)
	}
	exitCode := run(w, inputFile, outputFile)
	if err := w.Flush(); err != nil {
		os.Exit(1)
	}
	os.Exit(exitCode)
}

func parseArgs(args []string) (string, string, error) {
	var positional []string
	for i := range len(args) {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if len(a) > 0 && a[0] == '-' && a != "-" {
			return "", "", fmt.Errorf("invalid option -- '%s'", a[1:])
		}
		positional = append(positional, a)
	}
	inputFile := ""
	outputFile := ""
	if len(positional) > 0 {
		inputFile = positional[0]
	}
	if len(positional) > 1 {
		outputFile = positional[1]
	}
	if len(positional) > 2 {
		return "", "", fmt.Errorf("extra operand '%s'", positional[2])
	}
	return inputFile, outputFile, nil
}

func run(w *bufio.Writer, inputFile, outputFile string) int {
	r, closer, err := openInput(inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "uniq: %s\n", err)
		return 1
	}
	if closer != nil {
		defer closer.Close()
	}

	out, outCloser, err := openOutput(w, outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "uniq: %s\n", err)
		return 1
	}
	if outCloser != nil {
		defer outCloser.Close()
	}

	return deduplicate(r, out)
}

func openInput(name string) (io.Reader, io.Closer, error) {
	if name == "" || name == "-" {
		return os.Stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, f, nil
}

func openOutput(w *bufio.Writer, name string) (*bufio.Writer, io.Closer, error) {
	if name == "" {
		return w, nil, nil
	}
	f, err := os.Create(name)
	if err != nil {
		return nil, nil, err
	}
	return bufio.NewWriter(f), f, nil
}

func deduplicate(r io.Reader, w *bufio.Writer) int {
	scanner := bufio.NewScanner(r)
	first := true
	var prev string
	for scanner.Scan() {
		line := scanner.Text()
		if first || line != prev {
			if _, err := w.WriteString(line); err != nil {
				return 1
			}
			if err := w.WriteByte('\n'); err != nil {
				return 1
			}
			prev = line
			first = false
		}
	}
	if err := w.Flush(); err != nil {
		return 1
	}
	return 0
}
