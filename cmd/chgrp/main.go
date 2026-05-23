// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/chgrp implements srd090-chgrp R1.1-R1.4.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: chgrp [OPTION]... GROUP FILE...
  or:  chgrp [OPTION]... --reference=RFILE FILE...
Change the group of each FILE to GROUP.

  -c, --changes          like verbose but report only when a change is made
  -f, --silent, --quiet  suppress most error messages
  -v, --verbose          output a diagnostic for every file processed
      --dereference      affect the referent of each symbolic link (this is
                         the default), rather than the symbolic link itself
  -h, --no-dereference   affect symbolic links instead of any referenced file
      --no-preserve-root  do not treat '/' specially (the default)
      --preserve-root    fail to operate recursively on '/'
      --reference=RFILE  use RFILE's group rather than specifying a GROUP value
  -R, --recursive        operate on files and directories recursively
  -H                     if a command line argument is a symbolic link
                         to a directory, traverse it
  -L                     traverse every symbolic link to a directory
                         encountered
  -P                     do not traverse any symbolic links (default)
      --help     display this help and exit
      --version  output version information and exit

Examples:
  chgrp staff /u      Change the group of /u to "staff".
  chgrp -hR staff /u  Change the group of /u and subfiles to "staff".
`

const versionText = `chgrp (go-unix-utils) dev
`

type options struct {
	verbose   bool
	changes   bool
	silent    bool
	reference string
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, group, files := parseArgs(os.Args[1:])
	gid := resolveGID(opts, group)
	if len(files) == 0 {
		if opts.reference != "" {
			fmt.Fprintln(os.Stderr, "chgrp: missing operand")
		} else {
			fmt.Fprintf(os.Stderr, "chgrp: missing operand after '%s'\n", group)
		}
		fmt.Fprintln(os.Stderr, "Try 'chgrp --help' for more information.")
		os.Exit(1)
	}

	exitCode := 0
	for _, file := range files {
		if err := changeGroup(opts, gid, file); err != nil {
			exitCode = 1
		}
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func resolveGID(opts options, group string) int {
	if opts.reference != "" {
		return resolveReference(opts.reference)
	}
	return lookupGroup(group)
}

func lookupGroup(group string) int {
	if gid, err := strconv.Atoi(group); err == nil {
		return gid
	}
	g, err := user.LookupGroup(group)
	if err != nil {
		fmt.Fprintf(os.Stderr, "chgrp: invalid group: '%s'\n", group)
		os.Exit(1)
	}
	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "chgrp: invalid group: '%s'\n", group)
		os.Exit(1)
	}
	return gid
}

func resolveReference(rfile string) int {
	fi, err := sys.Stat(rfile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "chgrp: failed to get attributes of '%s': %s\n",
			rfile, err.(*os.PathError).Err)
		os.Exit(1)
	}
	return int(fi.Gid)
}

func changeGroup(opts options, gid int, path string) error {
	if err := os.Lchown(path, -1, gid); err != nil {
		reportError(opts, err)
		return err
	}
	return nil
}

func reportError(opts options, err error) {
	if opts.silent {
		return
	}
	fmt.Fprintf(os.Stderr, "chgrp: %s\n", formatErr(err))
}

func formatErr(err error) string {
	if pe, ok := errors.AsType[*os.PathError](err); ok {
		return fmt.Sprintf("changing group of '%s': %s", pe.Path, pe.Err)
	}
	return err.Error()
}

func parseArgs(args []string) (options, string, []string) {
	var opts options
	var files []string
	group := ""
	endOfFlags := false
	for _, arg := range args {
		if endOfFlags {
			files = append(files, arg)
			continue
		}
		if handled := handleSpecialArg(arg, &endOfFlags); handled {
			continue
		}
		if parseLongFlag(arg, &opts) {
			continue
		}
		if group == "" {
			if strings.HasPrefix(arg, "-") && parseShortFlags(arg[1:], &opts) {
				continue
			}
			if opts.reference != "" {
				files = append(files, arg)
			} else {
				group = arg
			}
		} else {
			files = append(files, arg)
		}
	}
	if group == "" && opts.reference == "" {
		fmt.Fprintln(os.Stderr, "chgrp: missing operand")
		fmt.Fprintln(os.Stderr, "Try 'chgrp --help' for more information.")
		os.Exit(1)
	}
	return opts, group, files
}

func handleSpecialArg(arg string, endOfFlags *bool) bool {
	switch arg {
	case "--help":
		fmt.Fprint(os.Stdout, helpText)
		os.Exit(0)
	case "--version":
		fmt.Fprint(os.Stdout, versionText)
		os.Exit(0)
	case "--":
		*endOfFlags = true
	default:
		return false
	}
	return true
}

func parseLongFlag(arg string, opts *options) bool {
	switch arg {
	case "--verbose":
		opts.verbose = true
	case "--changes":
		opts.changes = true
	case "--silent", "--quiet":
		opts.silent = true
	default:
		if strings.HasPrefix(arg, "--reference=") {
			opts.reference = arg[len("--reference="):]
			return true
		}
		return false
	}
	return true
}

func parseShortFlags(flags string, opts *options) bool {
	for _, c := range flags {
		switch c {
		case 'v':
			opts.verbose = true
		case 'c':
			opts.changes = true
		case 'f':
			opts.silent = true
		default:
			return false
		}
	}
	return true
}
