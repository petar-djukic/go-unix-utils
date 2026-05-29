// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd105-stty R1.1, R2.1, R3.1, R3.2, R4.1, R5.1, R6.1, R6.2, R7.1, R7.2, R7.3.
package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: stty [-F DEVICE | --file=DEVICE] [SETTING]...
  or:  stty [-F DEVICE | --file=DEVICE] [-a|--all]
  or:  stty [-F DEVICE | --file=DEVICE] [-g|--save]
Print or change terminal line settings.

      --help     display this help and exit
      --version  output version information and exit
`

const versionText = `stty (go-unix-utils) dev
`

type options struct {
	showAll  bool
	save     bool
	device   string
	settings []string
}

func main() {
	sys.InstallSIGPIPEHandler()
	opts := parseArgs(os.Args[1:])
	fd, closer := openFD(opts.device)
	if closer != nil {
		defer closer()
	}
	t, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		exitTermiosErr(opts.device, err)
	}
	if err := run(fd, t, opts); err != nil {
		fmt.Fprintf(os.Stderr, "stty: %v\n", err)
		os.Exit(1)
	}
}

func run(fd int, t *unix.Termios, opts options) error {
	if len(opts.settings) > 0 {
		return applySettings(fd, t, opts.settings)
	}
	rows, cols := getWinSize(fd)
	if opts.save {
		return printSaveFormat(os.Stdout, t)
	}
	if opts.showAll {
		return printAllDisplay(os.Stdout, t, rows, cols)
	}
	return printDefaultDisplay(os.Stdout, t, rows, cols)
}

func parseArgs(args []string) options {
	var opts options
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "--help":
			fmt.Fprint(os.Stdout, helpText)
			os.Exit(0)
		case arg == "--version":
			fmt.Fprint(os.Stdout, versionText)
			os.Exit(0)
		case arg == "-a" || arg == "--all":
			opts.showAll = true
		case arg == "-g" || arg == "--save":
			opts.save = true
		case arg == "-F" || arg == "--file":
			i++
			if i >= len(args) {
				fmt.Fprintf(os.Stderr, "stty: option requires an argument -- 'F'\n")
				os.Exit(1)
			}
			opts.device = args[i]
		case strings.HasPrefix(arg, "--file="):
			opts.device = arg[len("--file="):]
		default:
			opts.settings = append(opts.settings, arg)
		}
	}
	return opts
}

func openFD(device string) (int, func()) {
	if device == "" {
		return int(os.Stdin.Fd()), nil
	}
	fd, err := unix.Open(device, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "stty: %s: %s\n", device, sysErr(err))
		os.Exit(1)
	}
	return fd, func() { unix.Close(fd) }
}

func exitTermiosErr(device string, err error) {
	dev := "'standard input'"
	if device != "" {
		dev = device
	}
	fmt.Fprintf(os.Stderr, "stty: %s: %s\n", dev, sysErr(err))
	os.Exit(1)
}

func sysErr(err error) string {
	s := err.Error()
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
