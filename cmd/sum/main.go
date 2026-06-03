// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()
	flag.Bool("r", false, "")
	sysV := flag.Bool("s", false, "")
	flag.Parse()
	files := flag.Args()
	exitCode := 0
	if len(files) == 0 {
		checksum, totalBytes, err := compute(os.Stdin, *sysV)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sum: stdin: %v\n", err)
			os.Exit(1)
		}
		printLine(checksum, totalBytes, "", *sysV)
	} else {
		for _, file := range files {
			f, err := os.Open(file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "sum: %v\n", err)
				exitCode = 1
				continue
			}
			checksum, totalBytes, err := compute(f, *sysV)
			f.Close()
			if err != nil {
				fmt.Fprintf(os.Stderr, "sum: %v\n", err)
				exitCode = 1
				continue
			}
			printLine(checksum, totalBytes, file, *sysV)
		}
	}
	os.Exit(exitCode)
}

func compute(r io.Reader, sysV bool) (uint16, int64, error) {
	if sysV {
		return sysVChecksum(r)
	}
	return bsdChecksum(r)
}

func printLine(checksum uint16, totalBytes int64, name string, sysV bool) {
	var blocks int64
	if sysV {
		blocks = (totalBytes + 511) / 512
		fmt.Fprintf(os.Stdout, "%d %d", checksum, blocks)
	} else {
		blocks = (totalBytes + 1023) / 1024
		fmt.Fprintf(os.Stdout, "%05d %5d", checksum, blocks)
	}
	if name != "" {
		fmt.Fprintf(os.Stdout, " %s", name)
	}
	fmt.Fprintln(os.Stdout)
}

func bsdChecksum(r io.Reader) (uint16, int64, error) {
	var checksum uint16
	var totalBytes int64
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		for _, b := range buf[:n] {
			checksum = (checksum >> 1) + ((checksum & 1) << 15)
			checksum += uint16(b)
		}
		totalBytes += int64(n)
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, err
		}
	}
	return checksum, totalBytes, nil
}

func sysVChecksum(r io.Reader) (uint16, int64, error) {
	var sum uint32
	var totalBytes int64
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		for _, b := range buf[:n] {
			sum += uint32(b)
		}
		totalBytes += int64(n)
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, err
		}
	}
	fold := (sum & 0xffff) + ((sum >> 16) & 0xffff)
	fold = (fold & 0xffff) + (fold >> 16)
	return uint16(fold), totalBytes, nil
}
