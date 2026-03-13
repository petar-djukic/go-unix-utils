// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd052-tty R1.1, R1.2, R1.3, R2.1, R2.2
package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error and help messages.
const programName = "tty"

func main() {
	// D1: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	silent := false
	var operands []string

	// D4: Parse flags manually to support -s, --silent, --quiet, --help, --version.
	for _, arg := range args {
		switch arg {
		case "-s", "--silent", "--quiet":
			silent = true
		case "--help":
			fmt.Print(helpText)
			return
		case "--version":
			fmt.Println("tty (go-unix-utils) 0.1")
			return
		default:
			if strings.HasPrefix(arg, "-") {
				// R2.2: Unknown flags produce an error and exit 2.
				fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\nTry '%s --help' for more information.\n", programName, arg, programName)
				os.Exit(2)
			}
			operands = append(operands, arg)
		}
	}

	// R2.1: Extra operands produce an error and exit 2.
	if len(operands) > 0 {
		fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\nTry '%s --help' for more information.\n", programName, operands[0], programName)
		os.Exit(2)
	}

	// D2: Use pkg/sys.IsTerminal to detect whether stdin is a terminal.
	isTTY := sys.IsTerminal(os.Stdin.Fd())

	if isTTY {
		// R1.1: stdin is a terminal — print device name and exit 0.
		if !silent {
			name, err := ttyName(int(os.Stdin.Fd()))
			if err != nil {
				// Fallback: we know it's a tty but can't determine the name.
				fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
				os.Exit(1)
			}
			fmt.Println(name)
		}
		// R1.3: silent mode — no output, exit 0.
		os.Exit(0)
	}

	// R1.2: stdin is not a terminal — print "not a tty" and exit 1.
	// R1.3: silent mode — no output, exit 1.
	if !silent {
		fmt.Println("not a tty")
	}
	os.Exit(1)
}

// ttyName returns the terminal device path for the given file descriptor.
// On Linux it reads the /proc/self/fd/N symlink. On Darwin it fstats the fd
// and scans /dev/ for a character device with matching rdev.
func ttyName(fd int) (string, error) {
	// Linux: /proc/self/fd/N is a symlink to the tty device path.
	if target, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", fd)); err == nil {
		return target, nil
	}

	// Darwin/fallback: fstat the fd and scan /dev/ for matching character device.
	var fdStat unix.Stat_t
	if err := unix.Fstat(fd, &fdStat); err != nil {
		return "", fmt.Errorf("fstat: %w", err)
	}

	entries, err := os.ReadDir("/dev")
	if err != nil {
		return "", fmt.Errorf("reading /dev: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := "/dev/" + entry.Name()
		var st unix.Stat_t
		if err := unix.Stat(path, &st); err != nil {
			continue
		}
		if st.Rdev == fdStat.Rdev && st.Mode&unix.S_IFMT == unix.S_IFCHR {
			return path, nil
		}
	}

	return "", fmt.Errorf("cannot determine terminal name for fd %d", fd)
}

// helpText is the usage message printed by --help.
const helpText = `Usage: tty [OPTION]...
Print the file name of the terminal connected to standard input.

  -s, --silent, --quiet   print nothing, only return an exit status
      --help     display this help and exit
      --version  output version information and exit
`
