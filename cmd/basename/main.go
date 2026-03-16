// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd015-basename R1.1-R1.5, R2.1-R2.3, R3.1-R3.4:
// cmd/basename strips directory components from pathnames and optionally
// removes a suffix. Supports multi-argument mode (-a), suffix option (-s),
// NUL-delimited output (-z), and --help/--version flags. Installs SIGPIPE
// handler for clean exit on broken pipe.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

// progName is the name used in diagnostic output.
const progName = "basename"

func main() {
	// R1.4 (prd): install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	var (
		multipleMode bool
		suffix       string
		zeroTerminate bool
	)

	// Parse flags manually to match GNU basename behavior.
	var operands []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			operands = append(operands, args[i+1:]...)
			break
		}
		if arg == "--help" {
			// R2.3 (task R4): print usage to stdout and exit 0.
			fmt.Fprintf(os.Stdout, //nolint:errcheck // best-effort output
				"Usage: %s NAME [SUFFIX]\n  or:  %s OPTION... NAME...\nPrint NAME with any leading directory components removed.\nIf specified, also remove a trailing SUFFIX.\n\n"+
					"  -a, --multiple       support multiple arguments and treat each as a NAME\n"+
					"  -s, --suffix=SUFFIX  remove a trailing SUFFIX; implies -a\n"+
					"  -z, --zero           end each output line with NUL, not newline\n"+
					"      --help     display this help and exit\n"+
					"      --version  output version information and exit\n",
				progName, progName,
			)
			os.Exit(0)
		}
		if arg == "--version" {
			// R2.3 (task R4): print version to stdout and exit 0.
			fmt.Fprintf(os.Stdout, "%s (%s) %s\n", //nolint:errcheck // best-effort output
				progName, "go-unix-utils", version.Version,
			)
			os.Exit(0)
		}
		if arg == "--multiple" {
			multipleMode = true
			continue
		}
		if arg == "--zero" {
			zeroTerminate = true
			continue
		}
		if strings.HasPrefix(arg, "--suffix=") {
			suffix = arg[len("--suffix="):]
			multipleMode = true
			continue
		}
		if strings.HasPrefix(arg, "--suffix") && arg == "--suffix" {
			// --suffix SUFFIX (space-separated)
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s: option '--suffix' requires an argument\n", progName) //nolint:errcheck // best-effort diagnostic
				os.Exit(1)
			}
			i++
			suffix = args[i]
			multipleMode = true
			continue
		}
		// Handle short flags, possibly combined (e.g., -az, -as .txt)
		if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
			j := 1
			for j < len(arg) {
				switch arg[j] {
				case 'a':
					multipleMode = true
					j++
				case 'z':
					zeroTerminate = true
					j++
				case 's':
					multipleMode = true
					// Rest of this arg or next arg is the suffix.
					if j+1 < len(arg) {
						suffix = arg[j+1:]
					} else if i+1 < len(args) {
						i++
						suffix = args[i]
					} else {
						fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 's'\n", progName) //nolint:errcheck // best-effort diagnostic
						os.Exit(1)
					}
					j = len(arg) // consumed rest
				default:
					// Unknown short flag — treat as operand.
					operands = append(operands, arg)
					j = len(arg)
				}
			}
			continue
		}
		operands = append(operands, arg)
	}

	terminator := "\n"
	if zeroTerminate {
		terminator = "\x00"
	}

	if multipleMode {
		// R2.1: multiple-argument mode. Each operand is a NAME.
		if len(operands) == 0 {
			fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName) //nolint:errcheck // best-effort diagnostic
			os.Exit(1)
		}
		for _, name := range operands {
			result := basename(name)
			// R2.2: apply suffix removal if -s was given.
			if suffix != "" && result != suffix && strings.HasSuffix(result, suffix) {
				result = result[:len(result)-len(suffix)]
			}
			fmt.Fprintf(os.Stdout, "%s%s", result, terminator) //nolint:errcheck // best-effort output
		}
	} else {
		// Single-argument mode.
		// R3.3, R3.4: exit 1 on incorrect argument count.
		if len(operands) == 0 {
			fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName) //nolint:errcheck // best-effort diagnostic
			os.Exit(1)
		}
		if len(operands) > 2 {
			fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", progName, operands[2]) //nolint:errcheck // best-effort diagnostic
			os.Exit(1)
		}

		name := operands[0]
		result := basename(name)

		// R1.2: suffix removal when second operand is provided.
		if len(operands) == 2 {
			sfx := operands[1]
			// Do not remove suffix if the result equals the suffix (GNU behavior).
			if sfx != "" && result != sfx && strings.HasSuffix(result, sfx) {
				result = result[:len(result)-len(sfx)]
			}
		}

		// R1.1, R3.1: print result followed by terminator.
		fmt.Fprintf(os.Stdout, "%s%s", result, terminator) //nolint:errcheck // best-effort output
	}
}

// basename strips the directory prefix from name, matching GNU basename behavior.
// R1.1: strips longest prefix ending in '/'.
// R1.3 (PRD): strips trailing slashes before processing.
// R1.4 (PRD): all-slash input returns "/".
// R1.5 (PRD): empty input returns "".
func basename(name string) string {
	// R1.5: empty string produces empty result.
	if name == "" {
		return ""
	}

	// R1.3 (PRD): strip trailing slashes.
	for len(name) > 1 && name[len(name)-1] == '/' {
		name = name[:len(name)-1]
	}

	// R1.4 (PRD): if name is just "/", return "/".
	if name == "/" {
		return "/"
	}

	// R1.1: strip longest prefix ending in '/'.
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}

	return name
}
