// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the basename utility for stripping directory and suffix
// from filenames.
//
// Implements prd015-basename: single-argument mode (R1), multi-argument mode (R2),
// output formatting and exit codes (R3).
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const version = "basename (go-unix-utils) 1.0"

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	var (
		multiple bool
		suffix   string
		zero     bool
		names    []string
	)

	// Parse flags manually, matching GNU basename behavior.
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if arg == "--help" {
			fmt.Fprintln(os.Stdout, "Usage: basename NAME [SUFFIX]\n  or:  basename OPTION... NAME...\nPrint NAME with any leading directory components removed.\nIf specified, also remove a trailing SUFFIX.\n\nMandatory arguments to long options are mandatory for short options too.\n  -a, --multiple       support multiple arguments and treat each as a NAME\n  -s, --suffix=SUFFIX  remove a trailing SUFFIX; implies -a\n  -z, --zero           end each output line with NUL, not newline\n      --help     display this help and exit\n      --version  output version information and exit")
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Fprintln(os.Stdout, version)
			os.Exit(0)
		}
		if arg == "--multiple" {
			multiple = true
			i++
			continue
		}
		if arg == "--zero" {
			zero = true
			i++
			continue
		}
		if strings.HasPrefix(arg, "--suffix=") {
			suffix = arg[len("--suffix="):]
			multiple = true
			i++
			continue
		}
		if strings.HasPrefix(arg, "--suffix") {
			// --suffix SUFFIX (space-separated)
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "basename: option '--suffix' requires an argument\n")
				os.Exit(1)
			}
			i++
			suffix = args[i]
			multiple = true
			i++
			continue
		}
		if len(arg) > 1 && arg[0] == '-' && arg != "-" {
			// Short flags: -a, -s, -z, or combinations.
			j := 1
			for j < len(arg) {
				switch arg[j] {
				case 'a':
					multiple = true
				case 'z':
					zero = true
				case 's':
					// -s takes the rest of the flag or the next argument as suffix.
					rest := arg[j+1:]
					if rest != "" {
						suffix = rest
					} else {
						if i+1 >= len(args) {
							fmt.Fprintf(os.Stderr, "basename: option requires an argument -- 's'\n")
							os.Exit(1)
						}
						i++
						suffix = args[i]
					}
					multiple = true
					j = len(arg) // consumed
					continue
				default:
					fmt.Fprintf(os.Stderr, "basename: invalid option -- '%c'\n", arg[j])
					os.Exit(1)
				}
				j++
			}
			i++
			continue
		}
		break
	}

	names = args[i:]

	// R3.3: Validate argument count.
	if len(names) == 0 {
		fmt.Fprintf(os.Stderr, "basename: missing operand\n")
		os.Exit(1)
	}

	if !multiple && len(names) > 2 {
		fmt.Fprintf(os.Stderr, "basename: extra operand '%s'\n", names[2])
		os.Exit(1)
	}

	terminator := byte('\n')
	if zero {
		terminator = 0
	}

	if multiple {
		// R2.1, R2.2: Multi-argument mode.
		for _, name := range names {
			result := stripDir(name)
			if suffix != "" {
				result = stripSuffix(result, suffix)
			}
			fmt.Fprint(os.Stdout, result)
			os.Stdout.Write([]byte{terminator})
		}
	} else {
		// R1.1, R1.2: Single-argument mode.
		result := stripDir(names[0])
		if len(names) == 2 {
			result = stripSuffix(result, names[1])
		}
		fmt.Fprint(os.Stdout, result)
		os.Stdout.Write([]byte{terminator})
	}
}

// stripDir removes the directory component from name, matching GNU basename behavior.
func stripDir(name string) string {
	// R1.5: Empty string produces empty string.
	if name == "" {
		return ""
	}

	// R1.3: Strip trailing slashes.
	for len(name) > 1 && name[len(name)-1] == '/' {
		name = name[:len(name)-1]
	}

	// R1.4: If name is just "/", return "/".
	if name == "/" {
		return "/"
	}

	// Strip directory prefix.
	idx := strings.LastIndex(name, "/")
	if idx >= 0 {
		name = name[idx+1:]
	}

	return name
}

// stripSuffix removes suffix from name if it matches and the result is non-empty.
func stripSuffix(name, suffix string) string {
	if suffix == "" {
		return name
	}
	// Only strip if suffix is not the entire name.
	if strings.HasSuffix(name, suffix) && name != suffix {
		return name[:len(name)-len(suffix)]
	}
	return name
}
