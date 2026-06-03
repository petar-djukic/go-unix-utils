// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sizeparse"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: truncate OPTION... FILE...
Shrink or extend the size of each FILE to the specified size

A FILE argument that does not exist is created.

If a FILE is larger than the specified size, the extra data is lost.
If a FILE is shorter, it is extended and the sparse extended part (hole)
reads as zero bytes.

Mandatory arguments to long options are mandatory for short options too.
  -c, --no-create        do not create any files
  -o, --io-blocks        treat SIZE as number of IO blocks instead of bytes
  -r, --reference=RFILE  base size on RFILE
  -s, --size=SIZE        set or adjust the file size by SIZE bytes
      --help     display this help and exit
      --version  output version information and exit

The SIZE argument is an integer and optional unit (example: 10K is 10*1024).
Units are K,M,G,T,P,E,Z,Y,R,Q (powers of 1024) or KB,MB,... (powers of 1000).
Binary prefixes can be used, too: KiB=K, MiB=M, and so on.

SIZE may also be prefixed by one of the following modifying characters:
'+' extend by, '-' reduce by, '<' at most, '>' at least,
'/' round down to multiple of, '%' round up to multiple of.
`

const versionText = `truncate (go-unix-utils) dev
`

type options struct {
	noCreate  bool
	ioBlocks  bool
	reference string
	sizeStr   string
	sizeSet   bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	opts, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "truncate: %s\n", err)
		fmt.Fprintf(os.Stderr, "Try 'truncate --help' for more information.\n")
		os.Exit(1)
	}
	if !opts.sizeSet && opts.reference == "" {
		fmt.Fprintf(os.Stderr, "truncate: you must specify either '--size' or '--reference'\n")
		fmt.Fprintf(os.Stderr, "Try 'truncate --help' for more information.\n")
		os.Exit(1)
	}
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "truncate: missing file operand\n")
		fmt.Fprintf(os.Stderr, "Try 'truncate --help' for more information.\n")
		os.Exit(1)
	}
	os.Exit(run(opts, files))
}

func run(opts options, files []string) int {
	var baseSize int64
	if opts.reference != "" {
		fi, err := os.Stat(opts.reference)
		if err != nil {
			fmt.Fprintf(os.Stderr, "truncate: cannot stat %s: %s\n",
				quote(opts.reference), errMsg(err))
			return 1
		}
		baseSize = fi.Size()
	}

	var modifier byte
	var sizeVal int64
	if opts.sizeSet {
		var err error
		modifier, sizeVal, err = parseSize(opts.sizeStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "truncate: %s\n", err)
			return 1
		}
		if opts.ioBlocks {
			sizeVal *= ioBlockSize(files[0])
		}
	}

	exitCode := 0
	for _, path := range files {
		if err := truncateFile(opts, path, baseSize, modifier, sizeVal); err != nil {
			fmt.Fprintf(os.Stderr, "truncate: %s\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

func truncateFile(opts options, path string, baseSize int64, modifier byte, sizeVal int64) error {
	fi, err := ensureFile(opts, path)
	if err != nil {
		return err
	}
	if fi == nil {
		return nil
	}
	newSize, err := computeSize(opts, fi.Size(), baseSize, modifier, sizeVal)
	if err != nil {
		return err
	}
	if err := os.Truncate(path, newSize); err != nil {
		return fmt.Errorf("cannot open %s for writing: %s",
			quote(path), errMsg(err))
	}
	return nil
}

func ensureFile(opts options, path string) (os.FileInfo, error) {
	fi, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		if opts.noCreate {
			return nil, nil
		}
		f, createErr := os.Create(path)
		if createErr != nil {
			return nil, fmt.Errorf("cannot open %s for writing: %s",
				quote(path), errMsg(createErr))
		}
		f.Close()
		fi, err = os.Stat(path)
	}
	if err != nil {
		return nil, fmt.Errorf("cannot open %s for writing: %s",
			quote(path), errMsg(err))
	}
	return fi, nil
}

func computeSize(opts options, currentSize, baseSize int64, modifier byte, sizeVal int64) (int64, error) {
	if opts.reference != "" && !opts.sizeSet {
		return baseSize, nil
	}
	base := baseSize
	if opts.reference == "" {
		base = currentSize
	}
	result, err := applySize(base, modifier, sizeVal)
	if err != nil {
		return 0, err
	}
	if result < 0 {
		return 0, nil
	}
	return result, nil
}

func applySize(current int64, modifier byte, size int64) (int64, error) {
	switch modifier {
	case 0:
		return size, nil
	case '+':
		return current + size, nil
	case '-':
		return current - size, nil
	case '<':
		if current < size {
			return current, nil
		}
		return size, nil
	case '>':
		if current > size {
			return current, nil
		}
		return size, nil
	case '/':
		if size == 0 {
			return 0, fmt.Errorf("division by zero")
		}
		return (current / size) * size, nil
	case '%':
		if size == 0 {
			return 0, fmt.Errorf("division by zero")
		}
		return ((current + size - 1) / size) * size, nil
	}
	return size, nil
}

func parseSize(s string) (byte, int64, error) {
	if s == "" {
		return 0, 0, fmt.Errorf("Invalid number: %s", quote(s))
	}
	var modifier byte
	rest := s
	if len(rest) > 0 {
		switch rest[0] {
		case '+', '-', '<', '>', '/', '%':
			modifier = rest[0]
			rest = rest[1:]
		}
	}
	if rest == "" {
		return 0, 0, fmt.Errorf("Invalid number: %s", quote(s))
	}
	val, err := sizeparse.Parse(rest)
	if err != nil {
		return 0, 0, fmt.Errorf("Invalid number: %s", quote(s))
	}
	return modifier, val, nil
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
	if strings.HasPrefix(flag, "--size=") {
		opts.sizeStr = flag[len("--size="):]
		opts.sizeSet = true
		return 0, nil
	}
	if strings.HasPrefix(flag, "--reference=") {
		opts.reference = flag[len("--reference="):]
		return 0, nil
	}
	switch flag {
	case "--size":
		if len(rest) == 0 {
			return 0, fmt.Errorf("option '%s' requires an argument", flag)
		}
		opts.sizeStr = rest[0]
		opts.sizeSet = true
		return 1, nil
	case "--reference":
		if len(rest) == 0 {
			return 0, fmt.Errorf("option '%s' requires an argument", flag)
		}
		opts.reference = rest[0]
		return 1, nil
	case "--no-create":
		opts.noCreate = true
	case "--io-blocks":
		opts.ioBlocks = true
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
		case 'c':
			opts.noCreate = true
		case 'o':
			opts.ioBlocks = true
		case 's':
			opts.sizeSet = true
			remaining := flags[i+1:]
			if remaining != "" {
				opts.sizeStr = remaining
				return 0, nil
			}
			if len(rest) == 0 {
				return 0, fmt.Errorf("option requires an argument -- 's'")
			}
			opts.sizeStr = rest[0]
			return 1, nil
		case 'r':
			remaining := flags[i+1:]
			if remaining != "" {
				opts.reference = remaining
				return 0, nil
			}
			if len(rest) == 0 {
				return 0, fmt.Errorf("option requires an argument -- 'r'")
			}
			opts.reference = rest[0]
			return 1, nil
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return 0, nil
}

func ioBlockSize(path string) int64 {
	dir := filepath.Dir(path)
	var fs syscall.Statfs_t
	if err := syscall.Statfs(dir, &fs); err != nil {
		return 512
	}
	return int64(fs.Bsize)
}

func quote(s string) string {
	return "'" + s + "'"
}

func errMsg(err error) string {
	msg := err.Error()
	for _, prefix := range []string{"stat ", "open ", "truncate "} {
		if strings.HasPrefix(msg, prefix) {
			idx := strings.LastIndex(msg, ": ")
			if idx >= 0 {
				return capitalizeFirst(msg[idx+2:])
			}
		}
	}
	return capitalizeFirst(msg)
}

func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
