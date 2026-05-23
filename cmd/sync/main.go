// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd085-sync R1.1-R1.4, R2.1-R2.3.
package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unicode"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

type options struct {
	data       bool
	fileSystem bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	opts, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "sync: %s\n", err)
		fmt.Fprintf(os.Stderr, "Try 'sync --help' for more information.\n")
		os.Exit(1)
	}
	os.Exit(run(opts, files))
}

func run(opts options, files []string) int {
	if opts.data && opts.fileSystem {
		fmt.Fprintf(os.Stderr, "sync: cannot specify both --data and --file-system\n")
		return 1
	}
	if opts.data && len(files) == 0 {
		fmt.Fprintf(os.Stderr, "sync: --data needs at least one argument\n")
		return 1
	}
	if opts.fileSystem || len(files) == 0 {
		syscall.Sync()
		return 0
	}
	exitCode := 0
	for _, path := range files {
		if err := syncFile(path, opts); err != nil {
			fmt.Fprintf(os.Stderr, "sync: %s\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

func syncFile(path string, _ options) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("error opening '%s': %s", path, sysMsg(err))
	}
	defer f.Close()
	if err := syscall.Fsync(int(f.Fd())); err != nil {
		return fmt.Errorf("error syncing '%s': %s", path, err)
	}
	return nil
}

func sysMsg(err error) string {
	s := err.Error()
	if pe, ok := err.(*os.PathError); ok {
		s = pe.Err.Error()
	}
	if len(s) > 0 && unicode.IsLower(rune(s[0])) {
		s = string(unicode.ToUpper(rune(s[0]))) + s[1:]
	}
	return s
}

func parseArgs(args []string) (options, []string, error) {
	var opts options
	var files []string
	endFlags := false
	for i := range len(args) {
		arg := args[i]
		if endFlags || !strings.HasPrefix(arg, "-") {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			endFlags = true
			continue
		}
		if strings.HasPrefix(arg, "--") {
			if err := parseLongFlag(arg, &opts); err != nil {
				return options{}, nil, err
			}
			continue
		}
		if err := parseShortFlags(arg[1:], &opts); err != nil {
			return options{}, nil, err
		}
	}
	return opts, files, nil
}

func parseLongFlag(flag string, opts *options) error {
	switch flag {
	case "--data":
		opts.data = true
	case "--file-system":
		opts.fileSystem = true
	default:
		return fmt.Errorf("unrecognized option '%s'", flag)
	}
	return nil
}

func parseShortFlags(flags string, opts *options) error {
	for _, ch := range flags {
		switch ch {
		case 'd':
			opts.data = true
		case 'f':
			opts.fileSystem = true
		default:
			return fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return nil
}
