// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd105-stty R1.1–R1.4, R2.1–R2.4, R3.1–R3.3:
// change and print terminal line settings.
package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// options holds the parsed display and device flags.
type options struct {
	device   string
	showAll  bool
	showSave bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "stty: %v\n", err)
		os.Exit(1)
	}
}

// run is the main entry point for stty logic.
func run(args []string) error {
	opts, settings, err := parseArgs(args)
	if err != nil {
		return err
	}
	fd, cleanup, err := openDevice(opts.device)
	if err != nil {
		return err
	}
	defer cleanup()
	t, err := unix.IoctlGetTermios(fd, reqGetTermios)
	if err != nil {
		return fmt.Errorf("unable to get terminal attributes: %w", err)
	}
	if len(settings) == 0 {
		return displaySettings(t, opts, fd)
	}
	if err := applyAll(t, settings); err != nil {
		return err
	}
	return unix.IoctlSetTermios(fd, reqSetTermios, t)
}

// parseArgs separates display/device flags from setting arguments.
func parseArgs(args []string) (options, []string, error) {
	var opts options
	var settings []string
	for i := 0; i < len(args); i++ {
		consumed, err := parseOneArg(args[i:], &opts, &settings)
		if err != nil {
			return opts, nil, err
		}
		i += consumed
	}
	return opts, settings, nil
}

// parseOneArg parses a single CLI argument into opts or settings.
// Returns the number of extra arguments consumed beyond args[0].
func parseOneArg(args []string, opts *options, settings *[]string) (int, error) {
	arg := args[0]
	switch {
	case arg == "-a" || arg == "--all":
		opts.showAll = true
	case arg == "-g" || arg == "--save":
		opts.showSave = true
	case arg == "-F":
		if len(args) < 2 {
			return 0, fmt.Errorf("option requires an argument -- 'F'")
		}
		opts.device = args[1]
		return 1, nil
	case strings.HasPrefix(arg, "--file="):
		opts.device = strings.TrimPrefix(arg, "--file=")
	default:
		*settings = append(*settings, arg)
	}
	return 0, nil
}

// openDevice opens the terminal device or uses stdin.
// R1.4: -F DEVICE operates on the specified device.
func openDevice(device string) (int, func(), error) {
	if device == "" {
		return int(os.Stdin.Fd()), func() {}, nil
	}
	f, err := os.OpenFile(device, os.O_RDWR, 0)
	if err != nil {
		return 0, nil, fmt.Errorf("%s: %w", device, err)
	}
	return int(f.Fd()), func() { f.Close() }, nil // best-effort close
}
