// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd110-pr R1.1.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	bodySize   = 56
	footerSize = 5
)

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	var files []string
	for i, arg := range args {
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		files = append(files, arg)
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	w := bufio.NewWriter(os.Stdout)
	exitCode := 0
	for _, f := range files {
		if err := processFile(f, w); err != nil {
			if errors.Is(err, syscall.EPIPE) {
				os.Exit(0)
			}
			fmt.Fprintf(os.Stderr, "pr: %s\n", err)
			exitCode = 1
		}
	}
	if err := w.Flush(); err != nil {
		if errors.Is(err, syscall.EPIPE) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "pr: %s\n", err)
		exitCode = 1
	}
	return exitCode
}

func processFile(path string, w *bufio.Writer) error {
	var r io.Reader
	var filename string
	var modTime time.Time
	if path == "-" {
		r = os.Stdin
		filename = ""
		modTime = time.Now()
	} else {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return err
		}
		r = f
		filename = path
		modTime = info.ModTime()
	}
	return paginate(r, filename, modTime, w)
}

func paginate(r io.Reader, filename string, modTime time.Time, w *bufio.Writer) error {
	scanner := bufio.NewScanner(r)
	date := modTime.Format("2006-01-02 15:04")
	pageNum := 1
	for {
		lines, eof := readBody(scanner, bodySize)
		if len(lines) == 0 && eof {
			break
		}
		if err := writeHeader(w, date, filename, pageNum); err != nil {
			return err
		}
		if err := writeBody(w, lines); err != nil {
			return err
		}
		if err := writeFooter(w); err != nil {
			return err
		}
		pageNum++
		if eof {
			break
		}
	}
	return scanner.Err()
}

func readBody(scanner *bufio.Scanner, n int) ([]string, bool) {
	var lines []string
	for range n {
		if !scanner.Scan() {
			return lines, true
		}
		lines = append(lines, scanner.Text())
	}
	return lines, false
}

func writeHeader(w *bufio.Writer, date, filename string, pageNum int) error {
	if _, err := fmt.Fprint(w, "\n\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "%s  %s  Page %d\n", date, filename, pageNum); err != nil {
		return err
	}
	_, err := fmt.Fprint(w, "\n\n")
	return err
}

func writeBody(w *bufio.Writer, lines []string) error {
	for _, line := range lines {
		if _, err := fmt.Fprintf(w, "%s\n", line); err != nil {
			return err
		}
	}
	for range bodySize - len(lines) {
		if _, err := fmt.Fprint(w, "\n"); err != nil {
			return err
		}
	}
	return nil
}

func writeFooter(w *bufio.Writer) error {
	for range footerSize {
		if _, err := fmt.Fprint(w, "\n"); err != nil {
			return err
		}
	}
	return nil
}
