// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd062-touch R1.1, R1.2, R1.3, R1.4.
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	noCreate, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "touch: %s\n", err)
		fmt.Fprintf(os.Stderr, "Try 'touch --help' for more information.\n")
		os.Exit(1)
	}

	os.Exit(run(noCreate, files))
}

func run(noCreate bool, files []string) int {
	exitCode := 0
	now := time.Now()
	for _, path := range files {
		if err := touchFile(path, noCreate, now); err != nil {
			fmt.Fprintf(os.Stderr, "touch: %s\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

func touchFile(path string, noCreate bool, t time.Time) error {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		if noCreate {
			return nil
		}
		f, createErr := os.Create(path)
		if createErr != nil {
			return fmt.Errorf("cannot touch '%s': %s", path, sysMsg(createErr))
		}
		f.Close()
	}
	if err := os.Chtimes(path, t, t); err != nil {
		return fmt.Errorf("cannot touch '%s': %s", path, sysMsg(err))
	}
	return nil
}

func sysMsg(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

func parseArgs(args []string) (bool, []string, error) {
	noCreate := false
	var files []string
	endFlags := false
	i := 0
	for i < len(args) {
		arg := args[i]
		if endFlags || arg == "-" || !strings.HasPrefix(arg, "-") {
			files = append(files, arg)
			i++
			continue
		}
		if arg == "--" {
			endFlags = true
			i++
			continue
		}
		if strings.HasPrefix(arg, "--") {
			if err := parseLongFlag(arg, &noCreate); err != nil {
				return false, nil, err
			}
			i++
			continue
		}
		if err := parseShortFlags(arg[1:], &noCreate); err != nil {
			return false, nil, err
		}
		i++
	}
	if len(files) == 0 {
		return false, nil, fmt.Errorf("missing file operand")
	}
	return noCreate, files, nil
}

func parseLongFlag(flag string, noCreate *bool) error {
	switch flag {
	case "--no-create":
		*noCreate = true
	default:
		return fmt.Errorf("unrecognized option '%s'", flag)
	}
	return nil
}

func parseShortFlags(flags string, noCreate *bool) error {
	for _, ch := range flags {
		switch ch {
		case 'c':
			*noCreate = true
		default:
			return fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return nil
}
