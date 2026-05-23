// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

var crcTable [256]uint32

func init() {
	const poly = 0x04C11DB7
	for i := range 256 {
		c := uint32(i) << 24
		for range 8 {
			if c&0x80000000 != 0 {
				c = (c << 1) ^ poly
			} else {
				c <<= 1
			}
		}
		crcTable[i] = c
	}
}

func main() {
	sys.InstallSIGPIPEHandler()
	files := os.Args[1:]
	exitCode := 0
	if len(files) == 0 {
		if err := processStdin(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "cksum: %v\n", err)
			exitCode = 1
		}
	} else {
		for _, file := range files {
			if err := processFile(file, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "cksum: %v\n", err)
				exitCode = 1
			}
		}
	}
	os.Exit(exitCode)
}

func processStdin(stdout io.Writer) error {
	crc, size, err := computeCRC(os.Stdin)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%d %d\n", crc, size)
	return nil
}

func processFile(file string, stdout io.Writer) error {
	r, err := openInput(file)
	if err != nil {
		return err
	}
	defer r.Close()
	crc, size, err := computeCRC(r)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%d %d %s\n", crc, size, file)
	return nil
}

func computeCRC(r io.Reader) (uint32, int64, error) {
	var crc uint32
	var size int64
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		for _, b := range buf[:n] {
			crc = (crc << 8) ^ crcTable[(crc>>24)^uint32(b)]
		}
		size += int64(n)
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, err
		}
	}
	for n := size; n > 0; n >>= 8 {
		crc = (crc << 8) ^ crcTable[(crc>>24)^uint32(n&0xFF)]
	}
	crc = ^crc
	return crc, size, nil
}

func openInput(file string) (io.ReadCloser, error) {
	if file == "-" {
		return io.NopCloser(os.Stdin), nil
	}
	return os.Open(file)
}
