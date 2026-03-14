// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd034-mkdir R1.1–R1.4, R2.1–R2.3, R3.1–R3.4, R4.1–R4.3
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error and --help output.
const programName = "mkdir"

func main() {
	// D2: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	var operands []string
	// R2.1: -p / --parents flag for recursive directory creation.
	parents := false
	// R3.1: -m / --mode flag for explicit permission setting.
	modeStr := ""
	hasMode := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--help":
			printHelp()
			return
		case arg == "--version":
			printVersion()
			return
		case arg == "--parents":
			parents = true
		case arg == "--mode" || strings.HasPrefix(arg, "--mode="):
			if strings.Contains(arg, "=") {
				modeStr = arg[len("--mode="):]
			} else {
				i++
				if i >= len(args) {
					fmt.Fprintf(os.Stderr, "%s: option '--mode' requires an argument\n", programName)
					fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
					os.Exit(1)
				}
				modeStr = args[i]
			}
			hasMode = true
		case arg == "--":
			// End of flags; remaining args are operands.
			operands = append(operands, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(arg, "--"):
			// Unrecognized long option.
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", programName, arg)
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
			os.Exit(1)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			// Parse bundled short options (e.g., -p, -pm 0755).
			flags := arg[1:]
			for j := 0; j < len(flags); j++ {
				switch flags[j] {
				case 'p':
					parents = true
				case 'm':
					// R3.1: -m takes an argument: rest of this flag group or next arg.
					rest := flags[j+1:]
					if len(rest) > 0 {
						modeStr = rest
					} else {
						i++
						if i >= len(args) {
							fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'm'\n", programName)
							fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
							os.Exit(1)
						}
						modeStr = args[i]
					}
					hasMode = true
					j = len(flags) // consume rest of flag group
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", programName, flags[j])
					fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
					os.Exit(1)
				}
			}
		default:
			operands = append(operands, arg)
		}
	}

	// R1.1, R1.2: at least one operand is required.
	if len(operands) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		os.Exit(1)
	}

	// R3.1, R3.2: parse mode string if -m was given.
	var mode os.FileMode
	if hasMode {
		var err error
		mode, err = parseMode(modeStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: invalid mode '%s'\n", programName, modeStr)
			os.Exit(1)
		}
	}

	exitCode := 0
	for _, dir := range operands {
		if parents {
			// R2.1: create intermediate parent directories as needed.
			// R2.2: no error when target already exists with -p.
			// R2.3: no error when intermediate directories already exist with -p.
			if err := os.MkdirAll(dir, 0o777); err != nil {
				fmt.Fprintf(os.Stderr, "%s: cannot create directory '%s': %s\n", programName, dir, errMessage(err))
				exitCode = 1
				continue
			}
			// R3.3: apply mode only to the final target directory.
			if hasMode {
				if err := os.Chmod(dir, mode); err != nil {
					fmt.Fprintf(os.Stderr, "%s: cannot set permissions for '%s': %s\n", programName, dir, errMessage(err))
					exitCode = 1
				}
			}
		} else {
			// R1.1: create directory with default permissions (0777 modified by umask).
			// R1.2: process each operand independently.
			// R1.3: os.Mkdir returns an error when the parent does not exist.
			// R1.3 (task R1.2): os.Mkdir returns an error when the target already exists.
			if err := os.Mkdir(dir, 0o777); err != nil {
				fmt.Fprintf(os.Stderr, "%s: cannot create directory '%s': %s\n", programName, dir, errMessage(err))
				exitCode = 1
				continue
			}
			// R3.1, R3.3: apply the specified mode, overriding the umask-derived default.
			if hasMode {
				if err := os.Chmod(dir, mode); err != nil {
					fmt.Fprintf(os.Stderr, "%s: cannot set permissions for '%s': %s\n", programName, dir, errMessage(err))
					exitCode = 1
				}
			}
		}
	}

	os.Exit(exitCode)
}

// parseMode parses a mode string as either an octal value or a symbolic mode
// string. Returns the os.FileMode to apply via os.Chmod.
//
// R3.2: supports both octal (e.g., 0755, 755) and symbolic (e.g., u=rwx,go=rx).
func parseMode(modeStr string) (os.FileMode, error) {
	if len(modeStr) == 0 {
		return 0, fmt.Errorf("empty mode string")
	}
	// Try octal: string starts with a digit 0-7.
	if modeStr[0] >= '0' && modeStr[0] <= '7' {
		n, err := strconv.ParseUint(modeStr, 8, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid octal mode %q: %w", modeStr, err)
		}
		return unixModeToFileMode(uint32(n)), nil
	}
	return parseSymbolicMode(modeStr)
}

// unixModeToFileMode converts a Unix permission integer (lower 12 bits: 0o7777)
// to an os.FileMode with setuid/setgid/sticky mapped to Go's higher bits.
func unixModeToFileMode(m uint32) os.FileMode {
	mode := os.FileMode(m & 0o777)
	if m&0o4000 != 0 {
		mode |= os.ModeSetuid
	}
	if m&0o2000 != 0 {
		mode |= os.ModeSetgid
	}
	if m&0o1000 != 0 {
		mode |= os.ModeSticky
	}
	return mode
}

// parseSymbolicMode parses a symbolic mode string (e.g., u=rwx,go=rx) and
// returns the computed os.FileMode. Each clause has the form [ugoa...][=+-][rwxXst...].
// Multiple clauses are separated by commas. When no who is specified, the umask
// is applied to limit the affected bits, matching GNU coreutils behavior.
//
// R3.2: symbolic mode parsing.
func parseSymbolicMode(modeStr string) (os.FileMode, error) {
	// Get current umask for the no-who case.
	umask := syscall.Umask(0)
	syscall.Umask(umask)

	// GNU mkdir computes symbolic modes starting from the default directory
	// creation mode (0777), not from 0. This ensures that unmentioned who
	// categories retain their default bits.
	var mode os.FileMode = 0o777
	for _, clause := range strings.Split(modeStr, ",") {
		if len(clause) == 0 {
			return 0, fmt.Errorf("invalid mode %q", modeStr)
		}

		// Parse who: [ugoa]*
		i := 0
		var whoMask uint32
		hasWho := false
		for i < len(clause) && strings.IndexByte("ugoa", clause[i]) >= 0 {
			switch clause[i] {
			case 'u':
				whoMask |= 0o700
			case 'g':
				whoMask |= 0o070
			case 'o':
				whoMask |= 0o007
			case 'a':
				whoMask |= 0o777
			}
			hasWho = true
			i++
		}
		if !hasWho {
			whoMask = 0o777
		}

		if i >= len(clause) {
			return 0, fmt.Errorf("invalid mode %q", modeStr)
		}

		op := clause[i]
		if op != '+' && op != '-' && op != '=' {
			return 0, fmt.Errorf("invalid mode %q", modeStr)
		}
		i++

		// Parse permissions: [rwxXst]*
		var permBits uint32 // rwx as 3-bit value (r=4, w=2, x=1)
		var specialBits os.FileMode
		for i < len(clause) {
			switch clause[i] {
			case 'r':
				permBits |= 4
			case 'w':
				permBits |= 2
			case 'x', 'X':
				// X is execute-if-directory; mkdir always creates directories.
				permBits |= 1
			case 's':
				if whoMask&0o700 != 0 {
					specialBits |= os.ModeSetuid
				}
				if whoMask&0o070 != 0 {
					specialBits |= os.ModeSetgid
				}
			case 't':
				specialBits |= os.ModeSticky
			default:
				return 0, fmt.Errorf("invalid mode %q", modeStr)
			}
			i++
		}

		// Expand permBits into the who positions.
		var effectiveBits uint32
		if whoMask&0o700 != 0 {
			effectiveBits |= permBits << 6
		}
		if whoMask&0o070 != 0 {
			effectiveBits |= permBits << 3
		}
		if whoMask&0o007 != 0 {
			effectiveBits |= permBits
		}

		// Apply umask when no who is specified.
		if !hasWho {
			effectiveBits &^= uint32(umask)
		}

		switch op {
		case '=':
			// Clear who bits, then set effective bits.
			mode &^= os.FileMode(whoMask)
			mode |= os.FileMode(effectiveBits) | specialBits
		case '+':
			mode |= os.FileMode(effectiveBits) | specialBits
		case '-':
			mode &^= os.FileMode(effectiveBits) | specialBits
		}
	}
	return mode, nil
}

// errMessage extracts the underlying error message from a *os.PathError,
// stripping the op and path prefix that Go adds, to match GNU coreutils style.
func errMessage(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// printHelp writes usage information to stdout and exits 0.
//
// R1.4: --help prints usage to stdout and exits 0.
func printHelp() {
	fmt.Print(`Usage: mkdir [OPTION]... DIRECTORY...
Create the DIRECTORY(ies), if they do not already exist.

  -m, --mode=MODE  set file mode (as in chmod), not a=rwx - umask
  -p, --parents    no error if existing, make parent directories as needed
      --help       display this help and exit
      --version    output version information and exit
`)
}

// printVersion writes version information to stdout and exits 0.
//
// R1.4: --version prints version info to stdout and exits 0.
func printVersion() {
	fmt.Println("mkdir (go-unix-utils) 0.1")
}
