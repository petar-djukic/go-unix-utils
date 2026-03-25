// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd079-base32 R1.1–R1.4, R2.1–R2.2: Base32 encode from file or
// stdin with configurable line wrapping using pkg/encutil for shared encode
// logic and encoding/base32 from the Go standard library.
package main

import (
	"bytes"
	"encoding/base32"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/petar-djukic/go-unix-utils/pkg/encutil"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// defaultWrapCol is the default wrap column for encoded output.
// R1.2: wrap at 76 characters per line by default.
const defaultWrapCol = 76

func main() {
	sys.InstallSIGPIPEHandler()

	wrapCol := defaultWrapCol
	var filename string

	args := os.Args[1:]
	exitCode := parseArgs(args, &wrapCol, &filename)
	if exitCode >= 0 {
		os.Exit(exitCode)
	}

	if err := run(filename, wrapCol); err != nil {
		fmt.Fprintf(os.Stderr, "base32: %s\n", formatError(err))
		os.Exit(1)
	}
}

// parseArgs processes command-line arguments, setting wrapCol and filename.
// Returns -1 if parsing succeeds, or an exit code >= 0 to exit immediately.
func parseArgs(args []string, wrapCol *int, filename *string) int {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			if i+1 < len(args) {
				*filename = args[i+1]
			}
			return -1
		case arg == "-w" || arg == "--wrap":
			i++
			if i >= len(args) {
				fmt.Fprintf(os.Stderr, "base32: option '%s' requires an argument\n", arg)
				return 1
			}
			if code := parseWrapArg(args[i], wrapCol); code >= 0 {
				return code
			}
		case len(arg) > 2 && arg[:2] == "-w":
			if code := parseWrapArg(arg[2:], wrapCol); code >= 0 {
				return code
			}
		case len(arg) > 7 && arg[:7] == "--wrap=":
			if code := parseWrapArg(arg[7:], wrapCol); code >= 0 {
				return code
			}
		case arg == "-":
			*filename = arg
		case len(arg) > 0 && arg[0] == '-':
			fmt.Fprintf(os.Stderr, "base32: unrecognized option '%s'\n", arg)
			return 1
		default:
			*filename = arg
		}
	}
	return -1
}

// parseWrapArg parses a wrap column value. Returns -1 on success, or an exit
// code >= 0 on failure.
func parseWrapArg(s string, wrapCol *int) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		fmt.Fprintf(os.Stderr, "base32: invalid wrap size: '%s'\n", s)
		return 1
	}
	*wrapCol = n
	return -1
}

// run performs the base32 encoding operation.
// R1.1: reads from file or stdin, encodes to base32.
// R2.2: uses pkg/encutil.Encode with EncoderConfig.
func run(filename string, wrapCol int) error {
	input, err := encutil.OpenInput(filename)
	if err != nil {
		return err
	}
	defer input.Close()

	cfg := encutil.EncoderConfig{
		Encode:  base32.StdEncoding.EncodeToString,
		WrapCol: wrapCol,
	}

	var buf bytes.Buffer
	if err := encutil.Encode(input, &buf, cfg); err != nil {
		return err
	}

	output := buf.Bytes()
	output = adjustOutput(output, wrapCol)
	if len(output) == 0 {
		return nil
	}
	_, err = os.Stdout.Write(output)
	return err
}

// formatError converts Go os.PathError into GNU-style error messages.
// GNU format: "filename: Error message" (no "open" prefix, capitalized).
func formatError(err error) string {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return fmt.Sprintf("%s: %s", pathErr.Path, sentenceCase(pathErr.Err.Error()))
	}
	return err.Error()
}

// sentenceCase capitalizes the first letter of s.
func sentenceCase(s string) string {
	if len(s) == 0 {
		return s
	}
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}
	return s
}

// adjustOutput trims encutil output to match GNU base32 behavior.
// GNU base32: empty input produces no output; -w0 omits trailing newline.
func adjustOutput(output []byte, wrapCol int) []byte {
	// R1.2: GNU base32 produces no output for empty input.
	if bytes.Equal(output, []byte("\n")) {
		return nil
	}
	// GNU base32 -w0: no trailing newline.
	if wrapCol == 0 {
		return bytes.TrimSuffix(output, []byte("\n"))
	}
	return output
}
