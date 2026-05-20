// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd035-rmdir R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R3.1, R3.2, R3.3, R3.4.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: rmdir [OPTION]... DIRECTORY...
Remove the DIRECTORY(ies), if they are empty.

      --ignore-fail-on-non-empty
                  ignore each failure that is solely because a directory
                    is non-empty
  -p, --parents   remove DIRECTORY and its ancestors; e.g., 'rmdir -p a/b/c' is
                    similar to 'rmdir a/b/c a/b a'
  -v, --verbose   output a diagnostic for every directory processed
      --help        display this help and exit
      --version     output version information and exit
`

const versionText = `rmdir (go-unix-utils) dev
`

type options struct {
	parents              bool
	verbose              bool
	ignoreFailOnNonEmpty bool
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, dirs, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "rmdir: %s\n", err)
		fmt.Fprintf(os.Stderr, "Try 'rmdir --help' for more information.\n")
		os.Exit(1)
	}

	os.Exit(run(opts, dirs))
}

func run(opts options, dirs []string) int {
	exitCode := 0
	for _, dir := range dirs {
		if err := syscall.Rmdir(dir); err != nil {
			if opts.ignoreFailOnNonEmpty && isNonEmptyError(err) {
				if opts.verbose {
					fmt.Fprintf(os.Stdout, "rmdir: removing directory, '%s'\n", dir)
				}
				continue
			}
			fmt.Fprintf(os.Stderr, "rmdir: failed to remove '%s': %s\n",
				dir, sysErrMsg(err))
			exitCode = 1
			continue
		}
		if opts.verbose {
			fmt.Fprintf(os.Stdout, "rmdir: removing directory, '%s'\n", dir)
		}
		if opts.parents {
			for parent := filepath.Dir(filepath.Clean(dir)); parent != "." && parent != "/"; parent = filepath.Dir(parent) {
				if err := syscall.Rmdir(parent); err != nil {
					if opts.ignoreFailOnNonEmpty && isNonEmptyError(err) {
						break
					}
					fmt.Fprintf(os.Stderr, "rmdir: failed to remove directory '%s': %s\n",
						parent, sysErrMsg(err))
					exitCode = 1
					break
				}
				if opts.verbose {
					fmt.Fprintf(os.Stdout, "rmdir: removing directory, '%s'\n", parent)
				}
			}
		}
	}
	return exitCode
}

func isNonEmptyError(err error) bool {
	errno, ok := err.(syscall.Errno)
	return ok && errno == syscall.ENOTEMPTY
}

func sysErrMsg(err error) string {
	errno, ok := err.(syscall.Errno)
	if !ok {
		return err.Error()
	}
	switch errno {
	case syscall.ENOTEMPTY:
		return "Directory not empty"
	case syscall.ENOENT:
		return "No such file or directory"
	case syscall.ENOTDIR:
		return "Not a directory"
	case syscall.EACCES:
		return "Permission denied"
	default:
		return errno.Error()
	}
}

func parseArgs(args []string) (options, []string, error) {
	var opts options
	var dirs []string
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			dirs = append(dirs, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			n, err := parseLongFlag(args, i, &opts)
			if err != nil {
				return opts, nil, err
			}
			i += n
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			n, err := parseShortFlags(args[i][1:], &opts)
			if err != nil {
				return opts, nil, err
			}
			i += n
			continue
		}
		dirs = append(dirs, arg)
		i++
	}
	if len(dirs) == 0 {
		return opts, nil, fmt.Errorf("missing operand")
	}
	return opts, dirs, nil
}

func parseLongFlag(args []string, idx int, opts *options) (int, error) {
	switch args[idx] {
	case "--help":
		fmt.Fprint(os.Stdout, helpText)
		os.Exit(0)
		return 0, nil
	case "--version":
		fmt.Fprint(os.Stdout, versionText)
		os.Exit(0)
		return 0, nil
	case "--parents":
		opts.parents = true
		return 1, nil
	case "--verbose":
		opts.verbose = true
		return 1, nil
	case "--ignore-fail-on-non-empty":
		opts.ignoreFailOnNonEmpty = true
		return 1, nil
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", args[idx])
	}
}

func parseShortFlags(flags string, opts *options) (int, error) {
	for _, c := range flags {
		switch c {
		case 'p':
			opts.parents = true
		case 'v':
			opts.verbose = true
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", c)
		}
	}
	return 1, nil
}
