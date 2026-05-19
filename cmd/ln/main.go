// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: ln [OPTION]... TARGET LINK_NAME
  or:  ln [OPTION]... TARGET
  or:  ln [OPTION]... TARGET... DIRECTORY
In the 1st form, create a link to TARGET with the name LINK_NAME.
In the 2nd form, create a link to TARGET in the current directory.
In the 3rd form, create links to each TARGET in DIRECTORY.

  -s, --symbolic  make symbolic links instead of hard links
      --help      display this help and exit
      --version   output version information and exit
`

const versionText = `ln (go-unix-utils) dev
`

type options struct {
	symbolic bool
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, targets, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "ln: %s\n", err)
		fmt.Fprintf(os.Stderr, "Try 'ln --help' for more information.\n")
		os.Exit(1)
	}

	os.Exit(run(opts, targets))
}

func run(opts options, targets []string) int {
	exitCode := 0
	last := targets[len(targets)-1]

	switch {
	case len(targets) == 1:
		linkName := filepath.Base(targets[0])
		if err := createLink(targets[0], linkName, opts); err != nil {
			fmt.Fprintf(os.Stderr, "ln: %s\n", err)
			exitCode = 1
		}
	case isDirectory(last):
		for _, target := range targets[:len(targets)-1] {
			linkName := filepath.Join(last, filepath.Base(target))
			if err := createLink(target, linkName, opts); err != nil {
				fmt.Fprintf(os.Stderr, "ln: %s\n", err)
				exitCode = 1
			}
		}
	case len(targets) == 2:
		if err := createLink(targets[0], targets[1], opts); err != nil {
			fmt.Fprintf(os.Stderr, "ln: %s\n", err)
			exitCode = 1
		}
	default:
		fmt.Fprintf(os.Stderr, "ln: target '%s' is not a directory\n", last)
		exitCode = 1
	}

	return exitCode
}

func createLink(target, linkName string, opts options) error {
	if opts.symbolic {
		if err := os.Symlink(target, linkName); err != nil {
			return fmt.Errorf("failed to create symbolic link '%s': %s",
				linkName, sysError(err))
		}
		return nil
	}
	return createHardLink(target, linkName)
}

func createHardLink(target, linkName string) error {
	fi, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("failed to access '%s': %s", target, sysError(err))
	}
	if fi.IsDir() {
		return fmt.Errorf("'%s': hard link not allowed for directory", target)
	}
	if err := os.Link(target, linkName); err != nil {
		return fmt.Errorf("failed to create hard link '%s': %s",
			linkName, sysError(err))
	}
	return nil
}

func isDirectory(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.IsDir()
}

func sysError(err error) string {
	for u := errors.Unwrap(err); u != nil; u = errors.Unwrap(err) {
		err = u
	}
	return err.Error()
}

func parseArgs(args []string) (options, []string, error) {
	var opts options
	var targets []string

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			targets = append(targets, args[i+1:]...)
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
		targets = append(targets, arg)
		i++
	}

	if len(targets) == 0 {
		return opts, nil, fmt.Errorf("missing file operand")
	}

	return opts, targets, nil
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
	case "--symbolic":
		opts.symbolic = true
		return 1, nil
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", flag)
	}
}

func parseShortFlags(flags string, opts *options) error {
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 's':
			opts.symbolic = true
		default:
			return fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return nil
}
