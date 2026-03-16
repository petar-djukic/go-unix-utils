// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd052-tty R1.1-R1.3, R2.1-R2.2:
// cmd/tty prints the file name of the terminal connected to stdin.
// Supports -s (--silent, --quiet) for exit-code-only operation.
// Exits 0 when stdin is a terminal, 1 when not, 2 on usage error.
package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

// progName is the name used in diagnostic output.
const progName = "tty"

func main() {
	// D1: install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	silent := false

	for i, arg := range args {
		if arg == "--" {
			// Everything after -- is an extra operand.
			if i+1 < len(args) {
				// R2.1: extra operand → error, exit 2.
				fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", progName, args[i+1])     //nolint:errcheck // best-effort diagnostic
				fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck // best-effort diagnostic
				os.Exit(2)
			}
			break
		}
		if strings.HasPrefix(arg, "--") {
			switch arg {
			case "--help":
				fmt.Fprintf(os.Stdout, //nolint:errcheck // best-effort output
					"Usage: %s [OPTION]...\n"+
						"Print the file name of the terminal connected to standard input.\n\n"+
						"  -s, --silent, --quiet  print nothing, only return an exit status\n"+
						"      --help     display this help and exit\n"+
						"      --version  output version information and exit\n",
					progName,
				)
				os.Exit(0)
			case "--version":
				fmt.Fprintf(os.Stdout, "%s (%s) %s\n", //nolint:errcheck // best-effort output
					progName, "go-unix-utils", version.Version,
				)
				os.Exit(0)
			case "--silent", "--quiet":
				// R1.3: silent mode.
				silent = true
			default:
				// R2.2: unknown long flag → exit 2.
				fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", progName, arg)     //nolint:errcheck // best-effort diagnostic
				fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck // best-effort diagnostic
				os.Exit(2)
			}
		} else if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			// Short flags — process each character.
			for _, c := range arg[1:] {
				switch c {
				case 's':
					// R1.3: silent mode.
					silent = true
				default:
					// R2.2: unknown short flag → exit 2.
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", progName, c)         //nolint:errcheck // best-effort diagnostic
					fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck // best-effort diagnostic
					os.Exit(2)
				}
			}
		} else {
			// R2.1: extra operand → error, exit 2.
			fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", progName, arg)       //nolint:errcheck // best-effort diagnostic
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck // best-effort diagnostic
			os.Exit(2)
		}
	}

	// R1.1/R1.2: check if stdin is a terminal.
	if !sys.IsTerminal(os.Stdin.Fd()) {
		// R1.2: not a terminal.
		if !silent {
			fmt.Fprintln(os.Stdout, "not a tty") //nolint:errcheck // best-effort output
		}
		os.Exit(1)
	}

	// R1.1: stdin is a terminal — print the device name.
	if !silent {
		name, err := ttyName(int(os.Stdin.Fd()))
		if err != nil {
			fmt.Fprintln(os.Stdout, "not a tty") //nolint:errcheck // best-effort output
			os.Exit(1)
		}
		fmt.Fprintln(os.Stdout, name) //nolint:errcheck // best-effort output
	}
	os.Exit(0)
}

// ttyName returns the device path for the given file descriptor.
// Tries /proc/self/fd/N (Linux) first, then falls back to scanning /dev
// for a character device with a matching rdev (macOS/BSD).
func ttyName(fd int) (string, error) {
	// Linux: /proc/self/fd/N is a symlink to the terminal device.
	procPath := fmt.Sprintf("/proc/self/fd/%d", fd)
	if name, err := os.Readlink(procPath); err == nil {
		return name, nil
	}

	// macOS/BSD: stat the fd and search /dev for a matching device.
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		return "", fmt.Errorf("fstat: %w", err)
	}

	// Search common terminal device directories.
	for _, dir := range []string{"/dev"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.Type()&os.ModeCharDevice == 0 {
				continue
			}
			path := dir + "/" + e.Name()
			var est unix.Stat_t
			if unix.Stat(path, &est) != nil {
				continue
			}
			if est.Rdev == st.Rdev {
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("unable to determine tty name for fd %d", fd)
}
