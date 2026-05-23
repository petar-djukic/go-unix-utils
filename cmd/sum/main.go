// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()
	files := os.Args[1:]
	exitCode := 0
	if len(files) == 0 {
		checksum, totalBytes, err := bsdChecksum(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sum: stdin: %v\n", err)
			os.Exit(1)
		}
		blocks := (totalBytes + 1023) / 1024
		fmt.Fprintf(os.Stdout, "%05d %5d\n", checksum, blocks)
	} else {
		for _, file := range files {
			f, err := os.Open(file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "sum: %v\n", err)
				exitCode = 1
				continue
			}
			checksum, totalBytes, err := bsdChecksum(f)
			f.Close()
			if err != nil {
				fmt.Fprintf(os.Stderr, "sum: %v\n", err)
				exitCode = 1
				continue
			}
			blocks := (totalBytes + 1023) / 1024
			fmt.Fprintf(os.Stdout, "%05d %5d %s\n", checksum, blocks, file)
		}
	}
	os.Exit(exitCode)
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
