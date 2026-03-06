// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/basename: strip directory and suffix from filenames.
// Implements prd015-basename R1, R2, R3.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	binName = "basename"
	version = "0.1.0"
)

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	var (
		multiple bool
		suffix   string
		zero     bool
	)

	// Parse flags manually, matching cmd/yes and cmd/true convention.
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		switch arg {
		case "--help":
			printHelp()
			os.Exit(0)
		case "--version":
			if _, err := fmt.Printf("%s (go-unix-utils) %s\n", binName, version); err != nil {
				os.Exit(1)
			}
			os.Exit(0)
		case "--multiple":
			multiple = true
			continue
		case "--zero":
			zero = true
			continue
		}
		if strings.HasPrefix(arg, "--suffix=") {
			suffix = arg[len("--suffix="):]
			multiple = true
			continue
		}
		if arg == "--suffix" {
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s: option '--suffix' requires an argument\nTry '%s --help' for more information.\n", binName, binName)
				os.Exit(1)
			}
			i++
			suffix = args[i]
			multiple = true
			continue
		}
		// Short flags: -a, -s, -z, combinable.
		if strings.HasPrefix(arg, "-") && len(arg) > 1 && arg[1] != '-' {
			for j := 1; j < len(arg); j++ {
				switch arg[j] {
				case 'a':
					multiple = true
				case 'z':
					zero = true
				case 's':
					// R2.2: -s SUFFIX implies -a.
					multiple = true
					if j+1 < len(arg) {
						suffix = arg[j+1:]
					} else if i+1 < len(args) {
						i++
						suffix = args[i]
					} else {
						fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 's'\nTry '%s --help' for more information.\n", binName, binName)
						os.Exit(1)
					}
					j = len(arg) // consumed rest of arg
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\nTry '%s --help' for more information.\n", binName, arg[j], binName)
					os.Exit(1)
				}
			}
			continue
		}
		positional = append(positional, arg)
	}

	terminator := "\n"
	if zero {
		terminator = "\x00"
	}

	if multiple {
		// R2.1: multi-argument mode.
		if len(positional) == 0 {
			fmt.Fprintf(os.Stderr, "%s: missing operand\nTry '%s --help' for more information.\n", binName, binName)
			os.Exit(1)
		}
		for _, name := range positional {
			result := basenameStr(name)
			if suffix != "" {
				result = removeSuffix(result, suffix)
			}
			if _, err := fmt.Print(result + terminator); err != nil {
				os.Exit(1)
			}
		}
	} else {
		// R1: single-argument mode.
		switch len(positional) {
		case 0:
			fmt.Fprintf(os.Stderr, "%s: missing operand\nTry '%s --help' for more information.\n", binName, binName)
			os.Exit(1)
		case 1:
			result := basenameStr(positional[0])
			if _, err := fmt.Print(result + terminator); err != nil {
				os.Exit(1)
			}
		case 2:
			// R1.2: NAME SUFFIX legacy syntax.
			result := basenameStr(positional[0])
			result = removeSuffix(result, positional[1])
			if _, err := fmt.Print(result + terminator); err != nil {
				os.Exit(1)
			}
		default:
			// R3.3: too many arguments without -a.
			fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\nTry '%s --help' for more information.\n", binName, positional[2], binName)
			os.Exit(1)
		}
	}
}

// basenameStr strips the directory prefix from name, matching GNU basename behavior.
// R1.1: strip longest prefix ending in '/'.
// R1.3: strip trailing slashes first.
// R1.4: all-slash input returns "/".
// R1.5: empty string returns "".
func basenameStr(name string) string {
	if name == "" {
		return ""
	}
	// Strip trailing slashes.
	for len(name) > 1 && name[len(name)-1] == '/' {
		name = name[:len(name)-1]
	}
	if name == "/" {
		return "/"
	}
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	return name
}

// removeSuffix removes suffix from s if s != suffix and s ends with suffix.
// R1.2: SUFFIX is a literal string match.
func removeSuffix(s, suffix string) string {
	if s != suffix && strings.HasSuffix(s, suffix) {
		return s[:len(s)-len(suffix)]
	}
	return s
}

func printHelp() {
	_, err := fmt.Printf("Usage: %s NAME [SUFFIX]\n  or:  %s OPTION... NAME...\nPrint NAME with any leading directory components removed.\nIf specified, also remove a trailing SUFFIX.\n\nMandatory arguments to long options are mandatory for short options too.\n  -a, --multiple       support multiple arguments and treat each as a NAME\n  -s, --suffix=SUFFIX  remove a trailing SUFFIX; implies -a\n  -z, --zero           end each output line with NUL, not newline\n      --help        display this help and exit\n      --version     output version information and exit\n", binName, binName)
	if err != nil {
		os.Exit(1)
	}
}
