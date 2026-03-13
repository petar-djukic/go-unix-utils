// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd015-basename R1.1–R1.5, R2.1–R2.3, R3.1–R3.4, R4.1–R4.3
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error and --help output.
const programName = "basename"

func main() {
	// D1: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// Parse flags: -a/--multiple, -s/--suffix=SUFFIX, -z/--zero.
	var multiple bool
	var suffix string
	var hasSuffix bool
	var zero bool
	var operands []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--help":
			printHelp()
			return
		case arg == "--version":
			printVersion()
			return
		case arg == "--":
			// End of flags; remaining args are operands.
			operands = append(operands, args[i+1:]...)
			i = len(args)
		case arg == "-a" || arg == "--multiple":
			// R2.1: enable multiple operand mode.
			multiple = true
		case arg == "-z" || arg == "--zero":
			// R2.3/R3.1: NUL-terminated output.
			zero = true
		case arg == "-s" || arg == "--suffix":
			// R2.2: -s SUFFIX (next arg is the suffix value).
			hasSuffix = true
			multiple = true // -s implies -a.
			i++
			if i < len(args) {
				suffix = args[i]
			}
		case strings.HasPrefix(arg, "--suffix="):
			// R2.2: --suffix=SUFFIX form.
			hasSuffix = true
			multiple = true
			suffix = arg[len("--suffix="):]
		case strings.HasPrefix(arg, "-") && len(arg) > 1 && arg[1] != '-':
			// Short flag cluster: e.g. -az, -as .txt
			cluster := arg[1:]
			for j := 0; j < len(cluster); j++ {
				switch cluster[j] {
				case 'a':
					multiple = true
				case 'z':
					zero = true
				case 's':
					// R2.2: -s consumes the rest of the cluster or the next arg as SUFFIX.
					hasSuffix = true
					multiple = true
					rest := cluster[j+1:]
					if rest != "" {
						suffix = rest
					} else {
						i++
						if i < len(args) {
							suffix = args[i]
						}
					}
					j = len(cluster) // break out of cluster loop
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", programName, cluster[j])
					fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
					os.Exit(1)
				}
			}
		case strings.HasPrefix(arg, "--"):
			// R4.1: unrecognized long option.
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", programName, arg)
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
			os.Exit(1)
		default:
			operands = append(operands, arg)
		}
	}

	// R3.1: choose line terminator.
	terminator := "\n"
	if zero {
		terminator = "\x00"
	}

	if multiple {
		// R2.1: multi-argument mode — process each operand independently.
		if len(operands) == 0 {
			fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
			os.Exit(1)
		}
		for _, name := range operands {
			result := basename(name)
			// R2.2: strip suffix if -s was given.
			if hasSuffix {
				result = stripSuffix(result, suffix)
			}
			fmt.Print(result + terminator)
		}
	} else {
		// Single-argument mode.
		// R3.3, R3.4: validate argument count.
		if len(operands) == 0 {
			fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
			os.Exit(1)
		}
		if len(operands) > 2 {
			fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", programName, operands[2])
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
			os.Exit(1)
		}

		name := operands[0]
		result := basename(name)

		// R1.2: strip SUFFIX if second operand provided.
		if len(operands) == 2 {
			result = stripSuffix(result, operands[1])
		}

		// R3.1: print result followed by terminator. R3.2: exit 0 on success.
		fmt.Print(result + terminator)
	}
}

// stripSuffix removes suffix from result if result ends with suffix and the
// result would be non-empty after removal.
//
// R1.2, R2.2: SUFFIX is a literal string match. The result must remain non-empty.
func stripSuffix(result, suffix string) string {
	if suffix != "" && result != suffix && strings.HasSuffix(result, suffix) {
		result = result[:len(result)-len(suffix)]
	}
	return result
}

// basename strips the directory component from name, implementing the POSIX
// basename algorithm.
//
// R1.1: Strip the longest prefix ending in '/'.
// R1.3: Strip trailing slashes before removing the directory component.
// R1.4: If name consists entirely of slashes, return "/".
// R1.5: If name is empty, return "".
func basename(name string) string {
	// R1.5: empty string returns empty.
	if name == "" {
		return ""
	}

	// R1.3: strip trailing slashes.
	stripped := strings.TrimRight(name, "/")

	// R1.4: all slashes → "/".
	if stripped == "" {
		return "/"
	}

	// R1.1: find the last '/' and take everything after it.
	if i := strings.LastIndex(stripped, "/"); i >= 0 {
		return stripped[i+1:]
	}
	return stripped
}

// printHelp writes usage information to stdout and exits 0.
func printHelp() {
	fmt.Print(`Usage: basename NAME [SUFFIX]
  or:  basename OPTION... NAME...
Print NAME with any leading directory components removed.
If specified, also remove a trailing SUFFIX.

  -a, --multiple       support multiple arguments and treat each as a NAME
  -s, --suffix=SUFFIX  remove a trailing SUFFIX; implies -a
  -z, --zero           end each output line with NUL, not newline
      --help     display this help and exit
      --version  output version information and exit
`)
}

// printVersion writes version information to stdout and exits 0.
func printVersion() {
	fmt.Println("basename (go-unix-utils) 0.1")
}
