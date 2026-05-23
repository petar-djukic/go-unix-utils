// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: stat [OPTION]... FILE...
Display file or file system status.

Mandatory arguments to long options are mandatory for short options too.
  -L, --dereference     follow links
  -f, --file-system     display file system status instead of file status
  -c  --format=FORMAT   use the specified FORMAT instead of the default;
                          output a newline after each use of FORMAT
      --printf=FORMAT   like --format, but interpret backslash escapes,
                          and do not output a mandatory trailing newline;
                          if you want a newline, include \n in FORMAT
  -t, --terse           print the information in terse form
      --help     display this help and exit
      --version  output version information and exit
`

const versionText = `stat (go-unix-utils) dev
`

type options struct {
	dereference bool
	fileSystem  bool
	format      string
	printf      string
	terse       bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	opts, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "stat: %s\n", err)
		fmt.Fprintf(os.Stderr, "Try 'stat --help' for more information.\n")
		os.Exit(1)
	}
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "stat: missing operand\n")
		fmt.Fprintf(os.Stderr, "Try 'stat --help' for more information.\n")
		os.Exit(1)
	}
	os.Exit(run(opts, files))
}

func run(opts options, files []string) int {
	exitCode := 0
	for _, path := range files {
		if err := statPath(opts, path); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

func statPath(opts options, path string) error {
	if opts.fileSystem {
		return statFS(opts, path)
	}
	return statFile(opts, path)
}

func statFile(opts options, path string) error {
	var fi *sys.FileInfo
	var err error
	if opts.dereference {
		fi, err = sys.Stat(path)
	} else {
		fi, err = sys.Lstat(path)
	}
	if err != nil {
		return fmtError(path, err)
	}
	output := fileOutput(opts, fi, path)
	fmt.Fprint(os.Stdout, output)
	return nil
}

func fileOutput(opts options, fi *sys.FileInfo, path string) string {
	if opts.printf != "" {
		return expandFormat(fi, path, opts.printf, false)
	}
	if opts.format != "" {
		return expandFormat(fi, path, opts.format, false) + "\n"
	}
	if opts.terse {
		return terseFile(fi, path) + "\n"
	}
	return defaultFile(fi, path)
}

func statFS(opts options, path string) error {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(path, &fs); err != nil {
		return fmtError(path, err)
	}
	output := fsOutput(opts, &fs, path)
	fmt.Fprint(os.Stdout, output)
	return nil
}

func fsOutput(opts options, fs *syscall.Statfs_t, path string) string {
	if opts.printf != "" {
		return expandFsFormat(fs, path, opts.printf, false)
	}
	if opts.format != "" {
		return expandFsFormat(fs, path, opts.format, false) + "\n"
	}
	if opts.terse {
		return terseFS(fs, path) + "\n"
	}
	return defaultFS(fs, path)
}

func parseArgs(args []string) (options, []string, error) {
	var opts options
	var files []string
	endFlags := false
	for i := 0; i < len(args); i++ {
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
			adv, err := parseLongFlag(arg, args[i+1:], &opts)
			if err != nil {
				return options{}, nil, err
			}
			i += adv
			continue
		}
		adv, err := parseShortFlags(arg[1:], args[i+1:], &opts)
		if err != nil {
			return options{}, nil, err
		}
		i += adv
	}
	return opts, files, nil
}

func parseLongFlag(flag string, rest []string, opts *options) (int, error) {
	if strings.HasPrefix(flag, "--format=") {
		opts.format = flag[len("--format="):]
		return 0, nil
	}
	if strings.HasPrefix(flag, "--printf=") {
		opts.printf = flag[len("--printf="):]
		return 0, nil
	}
	switch flag {
	case "--format":
		if len(rest) == 0 {
			return 0, fmt.Errorf("option '%s' requires an argument", flag)
		}
		opts.format = rest[0]
		return 1, nil
	case "--printf":
		if len(rest) == 0 {
			return 0, fmt.Errorf("option '%s' requires an argument", flag)
		}
		opts.printf = rest[0]
		return 1, nil
	case "--dereference":
		opts.dereference = true
	case "--file-system":
		opts.fileSystem = true
	case "--terse":
		opts.terse = true
	case "--help":
		fmt.Fprint(os.Stdout, helpText)
		os.Exit(0)
	case "--version":
		fmt.Fprint(os.Stdout, versionText)
		os.Exit(0)
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", flag)
	}
	return 0, nil
}

func parseShortFlags(flags string, rest []string, opts *options) (int, error) {
	for i, ch := range flags {
		switch ch {
		case 'L':
			opts.dereference = true
		case 'f':
			opts.fileSystem = true
		case 't':
			opts.terse = true
		case 'c':
			remaining := flags[i+1:]
			if remaining != "" {
				opts.format = remaining
				return 0, nil
			}
			if len(rest) == 0 {
				return 0, fmt.Errorf("option requires an argument -- 'c'")
			}
			opts.format = rest[0]
			return 1, nil
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return 0, nil
}
