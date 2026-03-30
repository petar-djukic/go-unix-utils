// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/cksum computes POSIX CRC-32 checksums and byte counts for files or stdin.
//
// Implements prd077-cksum: R1.1 (CRC checksum computation and output format),
// R1.2 (stdin when no file arguments), R1.3 (multiple files in order),
// R1.4 (exit 1 on file error, continue processing).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// crcPolynomial is the POSIX CRC-32 generator polynomial.
const crcPolynomial = 0x04C11DB7

// crcTable is the precomputed 256-entry lookup table for POSIX CRC-32.
//
// R1.1: CRC table for the POSIX cksum algorithm.
var crcTable = buildCRCTable()

// buildCRCTable generates the POSIX CRC-32 lookup table.
func buildCRCTable() [256]uint32 {
	var table [256]uint32
	for i := range 256 {
		crc := uint32(i) << 24
		for range 8 {
			if crc&0x80000000 != 0 {
				crc = (crc << 1) ^ crcPolynomial
			} else {
				crc <<= 1
			}
		}
		table[i] = crc
	}
	return table
}

func main() {
	// R3.3: SIGPIPE handling.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	if len(args) == 0 {
		// R1.2: Read from stdin when no file arguments.
		exitCode := processStdin()
		os.Exit(exitCode)
	}

	// R1.3: Process multiple files in argument order.
	os.Exit(processFiles(args))
}

// processStdin computes and prints the CRC for stdin.
//
// R1.2: Stdin output omits the filename.
func processStdin() int {
	crc, size, err := computeCRC(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cksum: -: %s\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "%d %d\n", crc, size)
	return 0
}

// processFiles computes and prints the CRC for each named file.
//
// R1.3: One line per file in argument order.
// R1.4: Exit 1 if any file fails, continue processing remaining.
func processFiles(files []string) int {
	exitCode := 0
	w := bufio.NewWriter(os.Stdout)
	for _, name := range files {
		if err := processOneFile(w, name); err != nil {
			fmt.Fprintf(os.Stderr, "cksum: %s: %s\n", name, err)
			exitCode = 1
		}
	}
	w.Flush() // best-effort flush
	return exitCode
}

// processOneFile computes and prints the CRC line for a single file.
//
// R1.1: Output format "CHECKSUM BYTES FILENAME".
func processOneFile(w *bufio.Writer, name string) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()

	crc, size, err := computeCRC(f)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "%d %d %s\n", crc, size, name)
	return nil
}

// computeCRC reads all bytes from r and returns the POSIX CRC-32 checksum
// and the byte count.
//
// R1.1: POSIX CRC-32 algorithm: process each byte, then fold in the
// byte count, then invert.
func computeCRC(r io.Reader) (uint32, int64, error) {
	var crc uint32
	var size int64
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		for i := range n {
			crc = (crc << 8) ^ crcTable[(crc>>24)^uint32(buf[i])]
		}
		size += int64(n)
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, err
		}
	}
	crc = foldLength(crc, size)
	crc = ^crc
	return crc, size, nil
}

// foldLength feeds the byte count into the CRC, byte by byte from MSB.
//
// R1.1: After processing all data, the file length is incorporated
// into the CRC before final inversion.
func foldLength(crc uint32, size int64) uint32 {
	for s := size; s > 0; s >>= 8 {
		crc = (crc << 8) ^ crcTable[(crc>>24)^uint32(s&0xFF)]
	}
	return crc
}
