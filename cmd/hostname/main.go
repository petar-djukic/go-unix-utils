// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd047-hostname R1.1-R1.2, R2.1-R2.2:
// cmd/hostname prints the system hostname.
// Supports --help and --version flags. Rejects unknown flags and extra operands.
// Installs SIGPIPE handler per ARCHITECTURE.yaml.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

func main() {
	sys.InstallSIGPIPEHandler()

	if err := parseArgs(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", os.Args[0], err)             //nolint:errcheck // best-effort diagnostic
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", os.Args[0]) //nolint:errcheck // best-effort diagnostic
		os.Exit(1)
	}

	// R1.1: print the system hostname followed by a newline.
	hostname, err := os.Hostname()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot get hostname: %v\n", os.Args[0], err) //nolint:errcheck // best-effort diagnostic
		os.Exit(1)
	}

	fmt.Println(hostname)
}

// parseArgs validates command-line arguments. hostname accepts only --help and
// --version; any other flag or operand is an error.
func parseArgs(args []string) error {
	for _, arg := range args {
		if arg == "--" {
			continue
		}

		// R2.2: --help prints usage to stdout and exits 0.
		if arg == "--help" {
			fmt.Fprintf(os.Stdout, //nolint:errcheck // best-effort output
				"Usage: %s [OPTION]...\n"+
					"Print the system's hostname.\n\n"+
					"      --help     display this help and exit\n"+
					"      --version  output version information and exit\n",
				os.Args[0],
			)
			os.Exit(0)
		}

		// R2.1: --version prints version info to stdout and exits 0.
		if arg == "--version" {
			fmt.Fprintf(os.Stdout, "%s (%s) %s\n", //nolint:errcheck // best-effort output
				os.Args[0], "go-unix-utils", version.Version,
			)
			os.Exit(0)
		}

		// R2.2: unknown flags produce an error.
		if strings.HasPrefix(arg, "--") {
			return fmt.Errorf("unrecognized option '%s'", arg)
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			return fmt.Errorf("invalid option -- '%c'", arg[1])
		}

		// R2.1: extra operands produce an error.
		return fmt.Errorf("extra operand '%s'", arg)
	}

	return nil
}
