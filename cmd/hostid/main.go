// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd048-hostid R1.1-R1.2, R2.1-R2.2:
// cmd/hostid prints the 32-bit host identifier as an 8-digit lowercase
// hexadecimal number. Supports --help and --version flags. Rejects unknown
// flags and extra operands. Installs SIGPIPE handler per ARCHITECTURE.yaml.
package main

/*
#include <unistd.h>
*/
import "C"

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

	// R1.1-R1.2: print the 32-bit host identifier as 8-digit lowercase hex.
	hostid := C.gethostid()
	fmt.Printf("%08x\n", uint32(hostid))
}

// parseArgs validates command-line arguments. hostid accepts only --help and
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
					"Print the numeric identifier (in hexadecimal) for the current host.\n\n"+
					"      --help     display this help and exit\n"+
					"      --version  output version information and exit\n",
				os.Args[0],
			)
			os.Exit(0)
		}

		// R2.2: --version prints version info to stdout and exits 0.
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
