// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/sum computes BSD or System V checksums and block counts for files or stdin.
//
// Implements prd078-sum: R1.1 (BSD checksum), R1.2 (stdin), R1.3 (multiple files),
// R1.4 (error handling), R2.1 (-r BSD), R2.2 (-s System V), R3.1 (exit 0),
// R3.2 (exit 1 on error), R3.3 (SIGPIPE handling).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// blockSizeBSD is the block size for BSD checksum output (1024 bytes).
const blockSizeBSD = 1024

// blockSizeSysV is the block size for System V checksum output (512 bytes).
const blockSizeSysV = 512

func main() {
	// R3.3: SIGPIPE handling.
	sys.InstallSIGPIPEHandler()

	sysv, files := parseArgs(os.Args[1:])

	if sysv {
		os.Exit(runSysV(files))
	}
	os.Exit(runBSD(files))
}

// parseArgs parses -r (BSD, default) and -s (System V) flags.
//
// R2.1: -r selects BSD. R2.2: -s selects System V.
func parseArgs(args []string) (sysv bool, files []string) {
	for _, arg := range args {
		switch arg {
		case "-s":
			sysv = true
		case "-r":
			sysv = false
		default:
			files = append(files, arg)
		}
	}
	return sysv, files
}

// runBSD executes BSD checksum mode.
//
// R1.1-R1.4: BSD checksum processing.
func runBSD(files []string) int {
	if len(files) == 0 {
		return processBSDStdin()
	}
	return processBSDFiles(files)
}

// processBSDStdin computes and prints the BSD checksum for stdin.
//
// R1.2: Stdin output omits the filename.
func processBSDStdin() int {
	cksum, size, err := computeBSD(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sum: -: %s\n", err)
		return 1
	}
	blocks := ceilDiv(size, blockSizeBSD)
	fmt.Fprintf(os.Stdout, "%05d %5d\n", cksum, blocks)
	return 0
}

// processBSDFiles computes and prints the BSD checksum for each named file.
//
// R1.3: One line per file in argument order.
// R1.4: Exit 1 if any file fails, continue processing remaining.
func processBSDFiles(files []string) int {
	exitCode := 0
	w := bufio.NewWriter(os.Stdout)
	for _, name := range files {
		if err := processBSDOneFile(w, name); err != nil {
			fmt.Fprintf(os.Stderr, "sum: %s: %s\n", name, err)
			exitCode = 1
		}
	}
	w.Flush() // best-effort flush
	return exitCode
}

// processBSDOneFile computes and prints the BSD checksum line for a single file.
//
// R1.1: Output format "CHECKSUM BLOCKS FILENAME".
func processBSDOneFile(w *bufio.Writer, name string) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()

	cksum, size, err := computeBSD(f)
	if err != nil {
		return err
	}
	blocks := ceilDiv(size, blockSizeBSD)
	fmt.Fprintf(w, "%05d %5d %s\n", cksum, blocks, name)
	return nil
}

// computeBSD reads all bytes from r and returns the BSD 16-bit rotating checksum
// and the byte count.
//
// R1.1: BSD rotating checksum algorithm.
func computeBSD(r io.Reader) (uint16, int64, error) {
	var cksum uint16
	var size int64
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		for i := range n {
			// Rotate right 1 bit, then add byte.
			cksum = (cksum >> 1) + ((cksum & 1) << 15)
			cksum += uint16(buf[i])
		}
		size += int64(n)
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, err
		}
	}
	return cksum, size, nil
}

// runSysV executes System V checksum mode.
//
// R2.2: System V checksum processing.
func runSysV(files []string) int {
	if len(files) == 0 {
		return processSysVStdin()
	}
	return processSysVFiles(files)
}

// processSysVStdin computes and prints the System V checksum for stdin.
//
// R1.2: Stdin output omits the filename.
func processSysVStdin() int {
	cksum, size, err := computeSysV(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sum: -: %s\n", err)
		return 1
	}
	blocks := ceilDiv(size, blockSizeSysV)
	fmt.Fprintf(os.Stdout, "%d %d\n", cksum, blocks)
	return 0
}

// processSysVFiles computes and prints the System V checksum for each named file.
//
// R1.3: One line per file in argument order.
// R1.4: Exit 1 if any file fails, continue processing remaining.
func processSysVFiles(files []string) int {
	exitCode := 0
	w := bufio.NewWriter(os.Stdout)
	for _, name := range files {
		if err := processSysVOneFile(w, name); err != nil {
			fmt.Fprintf(os.Stderr, "sum: %s: %s\n", name, err)
			exitCode = 1
		}
	}
	w.Flush() // best-effort flush
	return exitCode
}

// processSysVOneFile computes and prints the System V checksum line for a single file.
//
// R2.2: Output format "CHECKSUM BLOCKS FILENAME".
func processSysVOneFile(w *bufio.Writer, name string) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()

	cksum, size, err := computeSysV(f)
	if err != nil {
		return err
	}
	blocks := ceilDiv(size, blockSizeSysV)
	fmt.Fprintf(w, "%d %d %s\n", cksum, blocks, name)
	return nil
}

// computeSysV reads all bytes from r and returns the System V 16-bit checksum
// and the byte count.
//
// R2.2: System V checksum: sum all bytes, fold to 16 bits.
func computeSysV(r io.Reader) (uint16, int64, error) {
	var sum uint32
	var size int64
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		for i := range n {
			sum += uint32(buf[i])
		}
		size += int64(n)
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, err
		}
	}
	// Fold 32-bit sum into 16 bits.
	folded := (sum & 0xFFFF) + ((sum >> 16) & 0xFFFF)
	folded = (folded & 0xFFFF) + ((folded >> 16) & 0xFFFF)
	return uint16(folded), size, nil
}

// ceilDiv returns the ceiling division of n by d.
func ceilDiv(n int64, d int64) int64 {
	if n == 0 {
		return 0
	}
	return (n + d - 1) / d
}
