// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: basename NAME [SUFFIX]
  or:  basename OPTION... NAME...
Print NAME with any leading directory components removed.
If specified, also remove a trailing SUFFIX.

Mandatory arguments to long options are mandatory for short options too.
  -a, --multiple       support multiple arguments and treat each as a NAME
  -s, --suffix=SUFFIX  remove a trailing SUFFIX; implies -a
  -z, --zero           end each output line with NUL, not newline
      --help     display this help and exit
      --version  output version information and exit

Examples:
  basename /usr/bin/sort          -> "sort"
  basename include/stdio.h .h     -> "stdio"
  basename -s .h include/stdio.h  -> "stdio"
  basename -a any/str1 any/str2   -> "str1" followed by "str2"
`

const versionText = `basename (go-unix-utils) dev
`

type options struct {
	multiple bool
	suffix   string
	zero     bool
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, names, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "basename: %s\n", err)
		fmt.Fprintf(os.Stderr, "Try 'basename --help' for more information.\n")
		os.Exit(1)
	}

	terminator := "\n"
	if opts.zero {
		terminator = "\x00"
	}

	for _, name := range names {
		result := basename(name, opts.suffix)
		fmt.Fprint(os.Stdout, result+terminator)
	}
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
		if strings.HasPrefix(arg, "--") {
			n, err := parseLongFlag(arg, args[i:], &opts)
			if err != nil {
				return opts, nil, err
			}
			i += n
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			n, err := parseShortFlags(arg[1:], args[i+1:], &opts)
			if err != nil {
				return opts, nil, err
			}
			i += 1 + n
			continue
		}
		names = append(names, arg)
		i++
	}

	if len(names) == 0 {
		return opts, nil, fmt.Errorf("missing operand")
	}
	if !opts.multiple && len(names) > 2 {
		return opts, nil, fmt.Errorf("extra operand '%s'", names[2])
	}
	// R1.2: single-argument mode treats second positional arg as suffix
	if !opts.multiple && len(names) == 2 {
		opts.suffix = names[1]
		names = names[:1]
	}

	return opts, names, nil
}

func parseLongFlag(flag string, remaining []string, opts *options) (int, error) {
	switch {
	case flag == "--help":
		fmt.Fprint(os.Stdout, helpText)
		os.Exit(0)
		return 0, nil
	case flag == "--version":
		fmt.Fprint(os.Stdout, versionText)
		os.Exit(0)
		return 0, nil
	case flag == "--multiple":
		opts.multiple = true
		return 1, nil
	case flag == "--zero":
		opts.zero = true
		return 1, nil
	case flag == "--suffix":
		if len(remaining) < 2 {
			return 0, fmt.Errorf("option '--suffix' requires an argument")
		}
		opts.suffix = remaining[1]
		opts.multiple = true
		return 2, nil
	case strings.HasPrefix(flag, "--suffix="):
		opts.suffix = flag[len("--suffix="):]
		opts.multiple = true
		return 1, nil
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", flag)
	}
}

func parseShortFlags(flags string, remaining []string, opts *options) (int, error) {
	consumed := 0
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'a':
			opts.multiple = true
		case 'z':
			opts.zero = true
		case 's':
			opts.multiple = true
			if rest := flags[j+1:]; rest != "" {
				opts.suffix = rest
			} else if len(remaining) > consumed {
				opts.suffix = remaining[consumed]
				consumed++
			} else {
				return 0, fmt.Errorf("option requires an argument -- 's'")
			}
			return consumed, nil
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return consumed, nil
}

// R1.5: empty string produces empty output
// R1.4: all-slash input produces "/"
// R1.1: strip directory prefix
// R1.2: remove literal suffix match
func basename(name, suffix string) string {
	if name == "" {
		return ""
	}

	name = strings.TrimRight(name, "/")
	if name == "" {
		return "/"
	}

	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}

	if suffix != "" && name != suffix && strings.HasSuffix(name, suffix) {
		name = name[:len(name)-len(suffix)]
	}

	return name
}
