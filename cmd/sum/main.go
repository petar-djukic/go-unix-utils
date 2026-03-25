// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd078-sum R1.1-R1.4, R2.1-R2.2, R3.1-R3.3: Compute BSD or
// System V checksums and block counts for files or stdin, with SIGPIPE
// handling and error reporting matching GNU sum.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// bsdBlockSize is the block size for the BSD algorithm (R1.1).
const bsdBlockSize = 1024

// sysvBlockSize is the block size for the System V algorithm (R2.2).
const sysvBlockSize = 512

// bufSize is the read buffer size for checksum computation.
const bufSize = 8192

// progName is the binary name used in error messages.
const progName = "sum"

func main() {
	// R3.3: Install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	sysv, files := parseFlags(os.Args[1:])
	os.Exit(run(sysv, files))
}

// run processes all files and returns the exit code.
// R1.2: reads stdin when no files are given.
// R1.3: processes multiple files in argument order.
// R3.1/R3.2: exits 0 on success, 1 on any error.
func run(sysv bool, files []string) int {
	if len(files) == 0 {
		return processOne("", sysv)
	}
	exitCode := 0
	for _, f := range files {
		if code := processOne(f, sysv); code != 0 {
			exitCode = code
		}
	}
	return exitCode
}

// processOne computes and prints the checksum for one file.
// An empty filename means implicit stdin (no filename in output).
func processOne(filename string, sysv bool) int {
	r, displayName, err := openInput(filename)
	if err != nil {
		printError(filename, err)
		return 1
	}
	defer r.Close() // best-effort close on read-only file

	if sysv {
		return printSysV(r, displayName)
	}
	return printBSD(r, displayName)
}

// printBSD computes and prints the BSD checksum for a reader.
// R1.1: 16-bit rotating checksum, 1024-byte blocks.
func printBSD(r io.Reader, displayName string) int {
	cksum, length, err := bsdChecksum(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 1
	}
	blocks := (length + bsdBlockSize - 1) / bsdBlockSize
	formatBSD(cksum, blocks, displayName)
	return 0
}

// formatBSD outputs one BSD checksum line.
// R1.1: GNU sum BSD format: 5-digit zero-padded checksum, 5-char blocks.
func formatBSD(cksum uint16, blocks int64, displayName string) {
	if displayName == "" {
		fmt.Printf("%05d %5d\n", cksum, blocks)
	} else {
		fmt.Printf("%05d %5d %s\n", cksum, blocks, displayName)
	}
}

// printSysV computes and prints the System V checksum for a reader.
// R2.2: 16-bit byte sum, 512-byte blocks.
func printSysV(r io.Reader, displayName string) int {
	cksum, length, err := sysvChecksum(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 1
	}
	blocks := (length + sysvBlockSize - 1) / sysvBlockSize
	formatSysV(cksum, blocks, displayName)
	return 0
}

// formatSysV outputs one System V checksum line.
// R2.2: plain numeric format for checksum and blocks.
func formatSysV(cksum uint16, blocks int64, displayName string) {
	if displayName == "" {
		fmt.Printf("%d %d\n", cksum, blocks)
	} else {
		fmt.Printf("%d %d %s\n", cksum, blocks, displayName)
	}
}

// openInput opens a file for reading, or returns stdin for "-" or empty.
// R1.2: empty string means implicit stdin; "-" means explicit stdin.
func openInput(filename string) (io.ReadCloser, string, error) {
	if filename == "" || filename == "-" {
		displayName := filename // "" for implicit, "-" for explicit
		return io.NopCloser(os.Stdin), displayName, nil
	}
	f, err := os.Open(filename)
	if err != nil {
		return nil, "", err
	}
	return f, filename, nil
}

// printError formats a file-open error matching GNU sum's style.
// R1.4: "sum: FILENAME: error message".
func printError(filename string, err error) {
	// Extract just the OS error message from Go's os.Open error.
	msg := strings.TrimPrefix(err.Error(), "open "+filename+": ")
	// Capitalize first letter to match GNU coreutils error format.
	if len(msg) > 0 && msg[0] >= 'a' && msg[0] <= 'z' {
		msg = string(msg[0]-32) + msg[1:]
	}
	fmt.Fprintf(os.Stderr, "%s: %s: %s\n", progName, filename, msg)
}

// bsdChecksum computes the BSD 16-bit rotating checksum.
// R1.1: for each byte, rotate right 1 bit then add the byte value.
func bsdChecksum(r io.Reader) (uint16, int64, error) {
	var cksum uint32
	var length int64
	buf := make([]byte, bufSize)
	for {
		n, err := r.Read(buf)
		for _, b := range buf[:n] {
			cksum = (cksum >> 1) + ((cksum & 1) << 15)
			cksum += uint32(b)
			cksum &= 0xFFFF
		}
		length += int64(n)
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, fmt.Errorf("reading input: %w", err)
		}
	}
	return uint16(cksum), length, nil
}

// sysvChecksum computes the System V 16-bit sum.
// R2.2: sum all bytes, then fold 32-bit result into 16 bits.
func sysvChecksum(r io.Reader) (uint16, int64, error) {
	var sum uint32
	var length int64
	buf := make([]byte, bufSize)
	for {
		n, err := r.Read(buf)
		for _, b := range buf[:n] {
			sum += uint32(b)
		}
		length += int64(n)
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, fmt.Errorf("reading input: %w", err)
		}
	}
	// Fold 32-bit sum into 16 bits.
	folded := (sum & 0xFFFF) + (sum >> 16)
	folded = (folded & 0xFFFF) + (folded >> 16)
	return uint16(folded), length, nil
}

// parseFlags extracts the -r and -s flags and returns remaining file args.
// R2.1: -r selects BSD (default). R2.2: -s selects System V.
func parseFlags(args []string) (sysv bool, files []string) {
	endOfFlags := false
	for _, arg := range args {
		if endOfFlags {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if arg == "-s" || arg == "--sysv" {
			sysv = true
			continue
		}
		if arg == "-r" {
			sysv = false
			continue
		}
		files = append(files, arg)
	}
	return sysv, files
}
