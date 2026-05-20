// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd037-ln R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: ln [OPTION]... TARGET LINK_NAME
  or:  ln [OPTION]... TARGET
  or:  ln [OPTION]... TARGET... DIRECTORY
In the 1st form, create a link to TARGET with the name LINK_NAME.
In the 2nd form, create a link to TARGET in the current directory.
In the 3rd form, create links to each TARGET in DIRECTORY.

  -s, --symbolic  make symbolic links instead of hard links
  -r, --relative  create symbolic links relative to link location
      --help      display this help and exit
      --version   output version information and exit
`

const versionText = `ln (go-unix-utils) dev
`

type options struct {
	symbolic bool
	relative bool
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
		linkName := "./" + filepath.Base(targets[0])
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
		fmt.Fprintf(os.Stderr, "ln: target '%s': %s\n",
			last, targetDirError(last))
		exitCode = 1
	}

	return exitCode
}

func createLink(target, linkName string, opts options) error {
	if opts.symbolic {
		t := target
		if opts.relative {
			rel, err := computeRelative(target, linkName)
			if err != nil {
				return err
			}
			t = rel
		}
		if err := os.Symlink(t, linkName); err != nil {
			return fmt.Errorf("failed to create symbolic link '%s': %s",
				linkName, sysErrMsg(err))
		}
		return nil
	}
	return createHardLink(target, linkName)
}

func computeRelative(target, linkName string) (string, error) {
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("failed to access '%s': %s", target, sysErrMsg(err))
	}
	linkDir := filepath.Dir(linkName)
	absLinkDir, err := filepath.Abs(linkDir)
	if err != nil {
		return "", fmt.Errorf("failed to access '%s': %s", linkDir, sysErrMsg(err))
	}
	rel, err := filepath.Rel(absLinkDir, absTarget)
	if err != nil {
		return "", fmt.Errorf("failed to compute relative path: %s", err)
	}
	return rel, nil
}

func createHardLink(target, linkName string) error {
	fi, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("failed to access '%s': %s", target, sysErrMsg(err))
	}
	if fi.IsDir() {
		return fmt.Errorf("%s: hard link not allowed for directory", target)
	}
	if err := os.Link(target, linkName); err != nil {
		return fmt.Errorf("failed to create hard link '%s': %s",
			linkName, sysErrMsg(err))
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

func targetDirError(path string) string {
	_, err := os.Stat(path)
	if err != nil {
		return "No such file or directory"
	}
	return "Not a directory"
}

func sysErrMsg(err error) string {
	pe, ok := err.(*os.PathError)
	if !ok {
		le, ok := err.(*os.LinkError)
		if !ok {
			return err.Error()
		}
		return errnoMsg(le.Err)
	}
	return errnoMsg(pe.Err)
}

func errnoMsg(err error) string {
	se, ok := err.(syscall.Errno)
	if !ok {
		return err.Error()
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
	case syscall.EPERM:
		return "Operation not permitted"
	case syscall.EXDEV:
		return "Invalid cross-device link"
	default:
		return se.Error()
	}
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
	case "--relative":
		opts.relative = true
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
		case 'r':
			opts.relative = true
		default:
			return fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return nil
}
