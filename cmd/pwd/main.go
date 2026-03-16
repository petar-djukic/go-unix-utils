// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd051-pwd R1.1-R1.4, R2.1-R2.2:
// cmd/pwd prints the current working directory. Supports -L (logical, prints
// PWD env if valid) and -P (physical, resolves symlinks). Default is -P.
// When both flags are given, the last one takes precedence.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

// progName is the name used in diagnostic output.
const progName = "pwd"

// mode selects logical (-L) vs physical (-P) working directory resolution.
type mode int

const (
	modePhysical mode = iota // R1.1: default is physical.
	modeLogical              // R1.2: logical uses PWD env.
)

func main() {
	// D1: install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R1.1: default mode is physical.
	m := modePhysical

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			// Everything after -- is a positional operand.
			if i+1 < len(args) {
				// R2.1: positional operands are an error.
				fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", progName, args[i+1]) //nolint:errcheck // best-effort diagnostic
				fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck // best-effort diagnostic
				os.Exit(1)
			}
			break
		}
		if strings.HasPrefix(arg, "--") {
			switch arg {
			case "--help":
				// R4.2: print usage to stdout and exit 0.
				fmt.Fprintf(os.Stdout, //nolint:errcheck // best-effort output
					"Usage: %s [OPTION]...\n"+
						"Print the full filename of the current working directory.\n\n"+
						"  -L, --logical   use PWD from environment, even if it contains symlinks\n"+
						"  -P, --physical  avoid all symlinks\n"+
						"      --help     display this help and exit\n"+
						"      --version  output version information and exit\n",
					progName,
				)
				os.Exit(0)
			case "--version":
				// R4.1: print version to stdout and exit 0.
				fmt.Fprintf(os.Stdout, "%s (%s) %s\n", //nolint:errcheck // best-effort output
					progName, "go-unix-utils", version.Version,
				)
				os.Exit(0)
			case "--logical":
				// R1.2: logical mode.
				m = modeLogical
			case "--physical":
				// R1.3: physical mode.
				m = modePhysical
			default:
				// R2.2: unknown long flag.
				fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", progName, arg)     //nolint:errcheck // best-effort diagnostic
				fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck // best-effort diagnostic
				os.Exit(1)
			}
		} else if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			// Short flags — process each character.
			// R1.4: last flag wins, so process left to right.
			for _, c := range arg[1:] {
				switch c {
				case 'L':
					// R1.2: logical mode.
					m = modeLogical
				case 'P':
					// R1.3: physical mode.
					m = modePhysical
				default:
					// R2.2: unknown short flag.
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", progName, c)         //nolint:errcheck // best-effort diagnostic
					fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck // best-effort diagnostic
					os.Exit(1)
				}
			}
		} else {
			// R2.1: positional operands are an error.
			fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", progName, arg)           //nolint:errcheck // best-effort diagnostic
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck // best-effort diagnostic
			os.Exit(1)
		}
	}

	dir, err := getWorkingDir(m)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err) //nolint:errcheck // best-effort diagnostic
		os.Exit(1)
	}

	fmt.Fprintln(os.Stdout, dir) //nolint:errcheck // best-effort output
}

// getWorkingDir returns the current working directory using the selected mode.
// R1.2: In logical mode, returns PWD if it is absolute, contains no . or ..
// components, and refers to the same directory as the physical path (same
// device and inode). Falls back to physical mode if PWD is invalid.
// R1.3: In physical mode, returns os.Getwd() directly.
func getWorkingDir(m mode) (string, error) {
	if m == modeLogical {
		pwd := os.Getenv("PWD")
		if pwd != "" && filepath.IsAbs(pwd) && !containsDotComponents(pwd) {
			// Validate that PWD refers to the same directory as the physical cwd.
			pwdInfo, err := sys.Stat(pwd)
			if err == nil {
				physDir, err := os.Getwd()
				if err == nil {
					physInfo, err := sys.Stat(physDir)
					if err == nil && pwdInfo.Dev == physInfo.Dev && pwdInfo.Ino == physInfo.Ino {
						return pwd, nil
					}
				}
			}
		}
	}
	// R1.3: physical mode or logical fallback. Resolve symlinks in the path
	// returned by os.Getwd(), which on some platforms preserves the symlink
	// components used in the original chdir.
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(dir)
}

// containsDotComponents reports whether path contains . or .. components.
func containsDotComponents(path string) bool {
	for _, component := range strings.Split(path, "/") {
		if component == "." || component == ".." {
			return true
		}
	}
	return false
}
