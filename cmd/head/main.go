// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements GNU head: print the first lines or bytes of files.
// Implements prd018-head R1-R4.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R4: Handle --help and --version before flag parsing.
	if len(args) > 0 {
		switch args[0] {
		case "--help":
			fmt.Println("Usage: head [OPTION]... [FILE]...")
			fmt.Println("Print the first 10 lines of each FILE to standard output.")
			fmt.Println("With more than one FILE, precede each with a header giving the file name.")
			fmt.Println()
			fmt.Println("With no FILE, or when FILE is -, read standard input.")
			fmt.Println()
			fmt.Println("Mandatory arguments to long options are mandatory for short options too.")
			fmt.Println("  -c, --bytes=[-]NUM       print the first NUM bytes of each file;")
			fmt.Println("                             with the leading '-', print all but the last")
			fmt.Println("                             NUM bytes of each file")
			fmt.Println("  -n, --lines=[-]NUM       print the first NUM lines instead of the first 10;")
			fmt.Println("                             with the leading '-', print all but the last")
			fmt.Println("                             NUM lines of each file")
			fmt.Println("  -q, --quiet, --silent    never print headers giving file names")
			fmt.Println("  -v, --verbose            always print headers giving file names")
			fmt.Println("      --help        display this help and exit")
			fmt.Println("      --version     output version information and exit")
			os.Exit(0)
		case "--version":
			fmt.Println("head (go-unix-utils) dev")
			os.Exit(0)
		}
	}

	// Defaults.
	mode := "lines"
	count := int64(10)
	negative := false
	quiet := false
	verbose := false
	var files []string

	// Parse flags manually.
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}

		// Long options with =.
		if strings.HasPrefix(arg, "--lines=") {
			mode = "lines"
			val := arg[len("--lines="):]
			n, neg, err := parseCount(val)
			if err != nil {
				fmt.Fprintf(os.Stderr, "head: invalid number of lines: %q\n", val)
				os.Exit(1)
			}
			count, negative = n, neg
			i++
			continue
		}
		if strings.HasPrefix(arg, "--bytes=") {
			mode = "bytes"
			val := arg[len("--bytes="):]
			n, neg, err := parseCount(val)
			if err != nil {
				fmt.Fprintf(os.Stderr, "head: invalid number of bytes: %q\n", val)
				os.Exit(1)
			}
			count, negative = n, neg
			i++
			continue
		}
		if arg == "--quiet" || arg == "--silent" {
			quiet = true
			verbose = false
			i++
			continue
		}
		if arg == "--verbose" {
			verbose = true
			quiet = false
			i++
			continue
		}
		if arg == "--lines" || arg == "--bytes" {
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "head: option '%s' requires an argument\n", arg)
				os.Exit(1)
			}
			if arg == "--lines" {
				mode = "lines"
			} else {
				mode = "bytes"
			}
			val := args[i+1]
			n, neg, err := parseCount(val)
			if err != nil {
				if mode == "lines" {
					fmt.Fprintf(os.Stderr, "head: invalid number of lines: %q\n", val)
				} else {
					fmt.Fprintf(os.Stderr, "head: invalid number of bytes: %q\n", val)
				}
				os.Exit(1)
			}
			count, negative = n, neg
			i += 2
			continue
		}

		// Short options.
		if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
			j := 1
			for j < len(arg) {
				ch := arg[j]
				switch ch {
				case 'n':
					mode = "lines"
					val := arg[j+1:]
					if val == "" {
						i++
						if i >= len(args) {
							fmt.Fprintf(os.Stderr, "head: option requires an argument -- 'n'\n")
							os.Exit(1)
						}
						val = args[i]
					}
					n, neg, err := parseCount(val)
					if err != nil {
						fmt.Fprintf(os.Stderr, "head: invalid number of lines: %q\n", val)
						os.Exit(1)
					}
					count, negative = n, neg
					j = len(arg)
				case 'c':
					mode = "bytes"
					val := arg[j+1:]
					if val == "" {
						i++
						if i >= len(args) {
							fmt.Fprintf(os.Stderr, "head: option requires an argument -- 'c'\n")
							os.Exit(1)
						}
						val = args[i]
					}
					n, neg, err := parseCount(val)
					if err != nil {
						fmt.Fprintf(os.Stderr, "head: invalid number of bytes: %q\n", val)
						os.Exit(1)
					}
					count, negative = n, neg
					j = len(arg)
				case 'q':
					quiet = true
					verbose = false
					j++
				case 'v':
					verbose = true
					quiet = false
					j++
				default:
					// Could be -NUM (legacy numeric option).
					if ch >= '0' && ch <= '9' {
						val := arg[j:]
						n, err := strconv.ParseInt(val, 10, 64)
						if err != nil {
							fmt.Fprintf(os.Stderr, "head: invalid number of lines: %q\n", arg)
							os.Exit(1)
						}
						mode = "lines"
						count = n
						negative = false
						j = len(arg)
					} else {
						fmt.Fprintf(os.Stderr, "head: invalid option -- '%c'\n", ch)
						os.Exit(1)
					}
				}
			}
			i++
			continue
		}

		// Not a flag; stop parsing.
		break
	}

	files = args[i:]

	// If no files given, read stdin.
	if len(files) == 0 {
		files = []string{"-"}
	}

	// Determine header mode.
	showHeaders := len(files) > 1
	if quiet {
		showHeaders = false
	}
	if verbose {
		showHeaders = true
	}

	exitCode := 0
	for idx, file := range files {
		if showHeaders {
			if idx > 0 {
				fmt.Println()
			}
			name := file
			if file == "-" {
				name = "standard input"
			}
			fmt.Printf("==> %s <==\n", name)
		}

		var r io.Reader
		if file == "-" {
			r = os.Stdin
		} else {
			f, err := os.Open(file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "head: cannot open '%s' for reading: %s\n", file, err.Error())
				exitCode = 1
				continue
			}
			r = f
			defer f.Close()
		}

		var err error
		if mode == "bytes" {
			if negative {
				err = outputAllButLastNBytes(r, count)
			} else {
				err = outputFirstNBytes(r, count)
			}
		} else {
			if negative {
				err = outputAllButLastNLines(r, count)
			} else {
				err = outputFirstNLines(r, count)
			}
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "head: error reading '%s': %s\n", file, err.Error())
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

// parseCount parses a count value that may be negative and may have a
// multiplier suffix (b, K, KiB, M, MiB, G, GiB). Returns the absolute
// value, whether it was negative, and any error.
func parseCount(s string) (int64, bool, error) {
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	}

	multiplier := int64(1)
	suffixes := []struct {
		suffix string
		mult   int64
	}{
		{"KiB", 1024},
		{"MiB", 1048576},
		{"GiB", 1073741824},
		{"K", 1024},
		{"M", 1048576},
		{"G", 1073741824},
		{"b", 512},
	}
	for _, sf := range suffixes {
		if strings.HasSuffix(s, sf.suffix) {
			multiplier = sf.mult
			s = s[:len(s)-len(sf.suffix)]
			break
		}
	}

	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false, err
	}

	return n * multiplier, neg, nil
}

// outputFirstNLines writes the first n lines from r to stdout.
// A line is terminated by '\n'; the last line may lack a trailing newline. R1.1, R1.2, R1.5.
func outputFirstNLines(r io.Reader, n int64) error {
	if n <= 0 {
		return nil
	}
	br := bufio.NewReader(r)
	var lineCount int64
	for lineCount < n {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			if _, werr := os.Stdout.Write(line); werr != nil {
				return werr
			}
			if line[len(line)-1] == '\n' {
				lineCount++
			} else {
				// Last line without trailing newline still counts. R1.5.
				lineCount++
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}
	return nil
}

// outputAllButLastNLines writes all lines except the last n from r to stdout. R1.3.
func outputAllButLastNLines(r io.Reader, n int64) error {
	if n <= 0 {
		_, err := io.Copy(os.Stdout, r)
		return err
	}
	// Read all lines into memory, then output all but the last n.
	br := bufio.NewReader(r)
	var lines [][]byte
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			lines = append(lines, line)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}
	outputCount := int64(len(lines)) - n
	if outputCount <= 0 {
		return nil
	}
	for i := int64(0); i < outputCount; i++ {
		if _, err := os.Stdout.Write(lines[i]); err != nil {
			return err
		}
	}
	return nil
}

// outputFirstNBytes writes the first n bytes from r to stdout. R2.1.
func outputFirstNBytes(r io.Reader, n int64) error {
	if n <= 0 {
		return nil
	}
	_, err := io.Copy(os.Stdout, io.LimitReader(r, n))
	return err
}

// outputAllButLastNBytes writes all bytes except the last n from r to stdout. R2.2.
func outputAllButLastNBytes(r io.Reader, n int64) error {
	if n <= 0 {
		_, err := io.Copy(os.Stdout, r)
		return err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if int64(len(data)) <= n {
		return nil
	}
	_, werr := os.Stdout.Write(data[:int64(len(data))-n])
	return werr
}
