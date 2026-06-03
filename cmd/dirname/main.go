// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd016-dirname.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: dirname [OPTION] NAME...
Output each NAME with its last non-slash component and trailing slashes
removed; if NAME contains no /'s, output '.' (meaning the current directory).

  -z, --zero     end each output line with NUL, not newline
      --help     display this help and exit
      --version  output version information and exit

Examples:
  dirname /usr/bin/          -> "/usr"
  dirname dir1/str1 dir2/str2 -> "dir1" followed by "dir2"
`

const versionText = `dirname (go-unix-utils) dev
`

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	nulDelimited := false

	for len(args) > 0 {
		switch args[0] {
		case "--help":
			fmt.Fprint(os.Stdout, helpText)
			os.Exit(0)
		case "--version":
			fmt.Fprint(os.Stdout, versionText)
			os.Exit(0)
		case "-z", "--zero":
			nulDelimited = true
			args = args[1:]
		case "--":
			args = args[1:]
			goto done
		default:
			if strings.HasPrefix(args[0], "-") && len(args[0]) > 1 {
				fmt.Fprintf(os.Stderr, "dirname: unrecognized option '%s'\n", args[0])
				fmt.Fprintln(os.Stderr, "Try 'dirname --help' for more information.")
				os.Exit(1)
			}
			goto done
		}
	}
done:

	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "dirname: missing operand")
		fmt.Fprintln(os.Stderr, "Try 'dirname --help' for more information.")
		os.Exit(1)
	}

	delim := byte('\n')
	if nulDelimited {
		delim = 0
	}

	for _, arg := range args {
		result := dirname(arg)
		_, err := fmt.Fprintf(os.Stdout, "%s%c", result, delim)
		if err != nil {
			os.Exit(1)
		}
	}
}

func dirname(path string) string {
	// R1.1: strip trailing slashes
	for len(path) > 1 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}

	// R1.3: if path is "/" or was all slashes, return "/"
	if path == "/" {
		return "/"
	}

	// R1.2: find last slash
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return "."
	}

	// Remove the last component
	dir := path[:i]

	// R1.4: strip trailing slashes from result
	for len(dir) > 1 && dir[len(dir)-1] == '/' {
		dir = dir[:len(dir)-1]
	}

	if dir == "" {
		return "/"
	}

	return dir
}
