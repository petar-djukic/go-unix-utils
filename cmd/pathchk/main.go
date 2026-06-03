// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	posixNameMax  = 14
	posixPathMax  = 255
	darwinNameMax = 255
	darwinPathMax = 1023
)

const portableChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789._-/"

const helpText = `Usage: pathchk [OPTION]... NAME...
Diagnose invalid or non-portable file names.

  -p     check for most POSIX systems
  -P     check for empty names and leading "-"
      --portability
         check for all POSIX systems (equivalent to -p -P)
      --help     display this help and exit
      --version  output version information and exit
`

const versionText = `pathchk (go-unix-utils) dev
`

type options struct {
	posix      bool
	leadHyphen bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	opts, names, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName(), err)
		if strings.Contains(err.Error(), "missing operand") {
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName())
		}
		os.Exit(1)
	}
	code := 0
	for _, name := range names {
		if !checkPath(name, opts) {
			code = 1
		}
	}
	os.Exit(code)
}

func progName() string {
	return filepath.Base(os.Args[0])
}

func parseArgs(args []string) (options, []string, error) {
	var opts options
	var names []string
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			names = append(names, args[i+1:]...)
			break
		}
		if err := parseSingleArg(arg, &opts, &names); err != nil {
			return opts, nil, err
		}
		i++
	}
	if len(names) == 0 {
		return opts, nil, fmt.Errorf("missing operand")
	}
	return opts, names, nil
}

func parseSingleArg(arg string, opts *options, names *[]string) error {
	switch {
	case arg == "--help":
		fmt.Fprint(os.Stdout, helpText)
		os.Exit(0)
	case arg == "--version":
		fmt.Fprint(os.Stdout, versionText)
		os.Exit(0)
	case arg == "--portability":
		opts.posix = true
		opts.leadHyphen = true
	case strings.HasPrefix(arg, "--"):
		return fmt.Errorf("unrecognized option '%s'", arg)
	case len(arg) > 1 && arg[0] == '-':
		return parseShortFlags(arg[1:], opts)
	default:
		*names = append(*names, arg)
	}
	return nil
}

func parseShortFlags(flags string, opts *options) error {
	for _, c := range flags {
		switch c {
		case 'p':
			opts.posix = true
		case 'P':
			opts.leadHyphen = true
		default:
			return fmt.Errorf("invalid option -- '%c'", c)
		}
	}
	return nil
}

func checkPath(name string, opts options) bool {
	if opts.leadHyphen && !checkLeadHyphen(name) {
		return false
	}
	if opts.posix {
		return checkPosix(name)
	}
	return checkDefault(name)
}

func checkLeadHyphen(name string) bool {
	if name == "" {
		fmt.Fprintf(os.Stderr, "%s: empty file name\n", progName())
		return false
	}
	for _, comp := range strings.Split(name, "/") {
		if strings.HasPrefix(comp, "-") {
			fmt.Fprintf(os.Stderr,
				"%s: leading '-' in a component of file name '%s'\n",
				progName(), name)
			return false
		}
	}
	return true
}

func checkPosix(name string) bool {
	if name == "" {
		fmt.Fprintf(os.Stderr, "%s: empty file name\n", progName())
		return false
	}
	if len(name) > posixPathMax {
		fmt.Fprintf(os.Stderr,
			"%s: limit %d exceeded by length %d of file name '%s'\n",
			progName(), posixPathMax, len(name), name)
		return false
	}
	return checkPosixComponents(name)
}

func checkPosixComponents(name string) bool {
	for _, comp := range strings.Split(name, "/") {
		if len(comp) > posixNameMax {
			fmt.Fprintf(os.Stderr,
				"%s: limit %d exceeded by length %d of file name component '%s'\n",
				progName(), posixNameMax, len(comp), comp)
			return false
		}
		if ch, ok := findNonPortable(comp); ok {
			fmt.Fprintf(os.Stderr,
				"%s: non-portable character '%s' in file name '%s'\n",
				progName(), quoteChar(ch), name)
			return false
		}
	}
	return true
}

func findNonPortable(comp string) (byte, bool) {
	for i := 0; i < len(comp); i++ {
		if !strings.ContainsRune(portableChars, rune(comp[i])) {
			return comp[i], true
		}
	}
	return 0, false
}

func quoteChar(c byte) string {
	if c >= 0x20 && c < 0x7f {
		return string(c)
	}
	switch c {
	case '\t':
		return `\t`
	case '\n':
		return `\n`
	case '\r':
		return `\r`
	default:
		return fmt.Sprintf("\\%03o", c)
	}
}

func checkDefault(name string) bool {
	if name == "" {
		fmt.Fprintf(os.Stderr, "%s: '': No such file or directory\n", progName())
		return false
	}
	pathMax := sysPathMax()
	nameMax := sysNameMax()
	if len(name) > pathMax {
		fmt.Fprintf(os.Stderr, "%s: %s: File name too long\n", progName(), name)
		return false
	}
	return checkDefaultComponents(name, nameMax)
}

func checkDefaultComponents(name string, nameMax int) bool {
	for _, comp := range strings.Split(name, "/") {
		if len(comp) > nameMax {
			fmt.Fprintf(os.Stderr,
				"%s: %s: File name too long\n", progName(), name)
			return false
		}
	}
	return checkAccessibility(name)
}

func checkAccessibility(name string) bool {
	prefix := ""
	sep := ""
	if strings.HasPrefix(name, "/") {
		prefix = "/"
		sep = ""
	}
	components := strings.Split(name, "/")
	for i, comp := range components {
		if comp == "" {
			continue
		}
		prefix = prefix + sep + comp
		sep = "/"
		if i == len(components)-1 {
			break
		}
		if err := syscall.Access(prefix, syscall.F_OK); err != nil {
			break
		}
		if err := syscall.Access(prefix, 0x01); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %s\n",
				progName(), name, accessErrMsg(err))
			return false
		}
	}
	return true
}

func accessErrMsg(err error) string {
	if errno, ok := err.(syscall.Errno); ok {
		switch errno {
		case syscall.EACCES:
			return "Permission denied"
		case syscall.ENOTDIR:
			return "Not a directory"
		default:
			return errno.Error()
		}
	}
	return err.Error()
}

func sysPathMax() int {
	return darwinPathMax
}

func sysNameMax() int {
	return darwinNameMax
}
