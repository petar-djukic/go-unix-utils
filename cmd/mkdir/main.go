// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd034-mkdir R1.1, R1.2, R1.3, R1.4.
package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: mkdir [OPTION]... DIRECTORY...
Create the DIRECTORY(ies), if they do not already exist.

Mandatory arguments to long options are mandatory for short options too.
  -m, --mode=MODE   set file mode (as in chmod), not a=rwx - umask
  -p, --parents     no error if existing, make parent directories as needed,
                    with their file modes unaffected by any -m option
  -v, --verbose     print a message for each created directory
  -Z                set SELinux security context of each created directory
                    to the default type
      --context[=CTX]  like -Z, or if CTX is specified then set the
                    SELinux or SMACK security context to CTX
      --help        display this help and exit
      --version     output version information and exit
`

const versionText = `mkdir (go-unix-utils) dev
`

type options struct {
	verbose bool
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, dirs, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %s\n", err)
		fmt.Fprintf(os.Stderr, "Try 'mkdir --help' for more information.\n")
		os.Exit(1)
	}

	os.Exit(run(opts, dirs))
}

func run(opts options, dirs []string) int {
	exitCode := 0
	for _, dir := range dirs {
		if err := os.Mkdir(dir, 0o777); err != nil {
			fmt.Fprintf(os.Stderr, "mkdir: cannot create directory '%s': %s\n",
				dir, sysErrMsg(err))
			exitCode = 1
			continue
		}
		if opts.verbose {
			fmt.Fprintf(os.Stdout, "mkdir: created directory '%s'\n", dir)
		}
	}
	return exitCode
}

func sysErrMsg(err error) string {
	pe, ok := err.(*os.PathError)
	if !ok {
		return err.Error()
	}
	se, ok := pe.Err.(syscall.Errno)
	if !ok {
		return pe.Err.Error()
	}
	switch se {
	case syscall.EEXIST:
		return "File exists"
	case syscall.ENOENT:
		return "No such file or directory"
	case syscall.EACCES:
		return "Permission denied"
	case syscall.ENOTDIR:
		return "Not a directory"
	default:
		return se.Error()
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
			n, err := parseLongFlag(arg, &opts)
			if err != nil {
				return opts, nil, err
			}
			i += n
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			if err := parseShortFlags(arg[1:], &opts); err != nil {
				return opts, nil, err
			}
			i++
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

func parseLongFlag(flag string, opts *options) (int, error) {
	switch flag {
	case "--help":
		fmt.Fprint(os.Stdout, helpText)
		os.Exit(0)
		return 0, nil
	case "--version":
		fmt.Fprint(os.Stdout, versionText)
		os.Exit(0)
		return 0, nil
	case "--verbose":
		opts.verbose = true
		return 1, nil
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", flag)
	}
}

func parseShortFlags(flags string, opts *options) error {
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'v':
			opts.verbose = true
		default:
			return fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return nil
}
