// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/install implements srd101-install R1.1-R1.4.
package main

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: install [OPTION]... [-T] SOURCE DEST
  or:  install [OPTION]... SOURCE... DIRECTORY
  or:  install [OPTION]... -d DIRECTORY...

Copy files and set attributes.

  -c                  (ignored)
  -g GROUP            set group ownership
  -m MODE             set permission mode (as in chmod), instead of rwxr-xr-x
  -o OWNER            set ownership
      --help          display this help and exit
      --version       output version information and exit
`

const versionText = `install (go-unix-utils) dev
`

type options struct {
	mode  string
	owner string
	group string
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, args := parseArgs(os.Args[1:])
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "install: missing file operand")
		os.Exit(1)
	}
	if len(args) == 1 {
		fmt.Fprintf(os.Stderr,
			"install: missing destination file operand after '%s'\n", args[0])
		os.Exit(1)
	}

	dest := args[len(args)-1]
	sources := args[:len(args)-1]

	exitCode := 0
	if len(sources) > 1 {
		exitCode = installMultiple(opts, sources, dest)
	} else {
		if err := installSingle(opts, sources[0], dest); err != nil {
			fmt.Fprintf(os.Stderr, "install: %s\n", err)
			exitCode = 1
		}
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func installMultiple(opts options, sources []string, dest string) int {
	fi, err := os.Stat(dest)
	if err != nil || !fi.IsDir() {
		fmt.Fprintf(os.Stderr,
			"install: target '%s' is not a directory\n", dest)
		return 1
	}
	exitCode := 0
	for _, src := range sources {
		target := filepath.Join(dest, filepath.Base(src))
		if err := installSingle(opts, src, target); err != nil {
			fmt.Fprintf(os.Stderr, "install: %s\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

func installSingle(opts options, src, dest string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("cannot stat '%s': %s", src, unwrapErr(err))
	}
	if srcInfo.IsDir() {
		return fmt.Errorf("omitting directory '%s'", src)
	}
	if fi, err := os.Stat(dest); err == nil && fi.IsDir() {
		dest = filepath.Join(dest, filepath.Base(src))
	}
	if err := copyFile(src, dest); err != nil {
		return err
	}
	if err := setMode(opts, dest); err != nil {
		return err
	}
	return setOwnership(opts, dest)
}

func copyFile(src, dest string) error {
	sf, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("cannot open '%s': %s", src, unwrapErr(err))
	}
	defer sf.Close()

	df, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("cannot create regular file '%s': %s",
			dest, unwrapErr(err))
	}
	defer df.Close()

	if _, err := io.Copy(df, sf); err != nil {
		return fmt.Errorf("writing '%s': %s", dest, unwrapErr(err))
	}
	return df.Close()
}

func setMode(opts options, path string) error {
	if opts.mode == "" {
		return os.Chmod(path, 0755)
	}
	perm, err := resolveMode(opts.mode)
	if err != nil {
		return fmt.Errorf("invalid mode '%s'", opts.mode)
	}
	return os.Chmod(path, perm)
}

func resolveMode(mode string) (os.FileMode, error) {
	if isOctalMode(mode) {
		val, err := strconv.ParseUint(mode, 8, 32)
		if err != nil {
			return 0, err
		}
		return os.FileMode(val), nil
	}
	return evalSymbolic(mode, 0, getUmask())
}

func getUmask() os.FileMode {
	old := syscall.Umask(0)
	syscall.Umask(old)
	return os.FileMode(old)
}

func setOwnership(opts options, path string) error {
	uid, gid := -1, -1
	if opts.owner != "" {
		var err error
		uid, err = lookupUser(opts.owner)
		if err != nil {
			return err
		}
	}
	if opts.group != "" {
		var err error
		gid, err = lookupGroup(opts.group)
		if err != nil {
			return err
		}
	}
	if uid == -1 && gid == -1 {
		return nil
	}
	if err := os.Chown(path, uid, gid); err != nil {
		return fmt.Errorf("cannot change ownership of '%s': %s",
			path, unwrapErr(err))
	}
	return nil
}

func lookupUser(name string) (int, error) {
	if uid, err := strconv.Atoi(name); err == nil {
		return uid, nil
	}
	u, err := user.Lookup(name)
	if err != nil {
		return 0, fmt.Errorf("invalid user: '%s'", name)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return 0, fmt.Errorf("invalid user: '%s'", name)
	}
	return uid, nil
}

func lookupGroup(name string) (int, error) {
	if gid, err := strconv.Atoi(name); err == nil {
		return gid, nil
	}
	g, err := user.LookupGroup(name)
	if err != nil {
		return 0, fmt.Errorf("invalid group: '%s'", name)
	}
	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		return 0, fmt.Errorf("invalid group: '%s'", name)
	}
	return gid, nil
}

func unwrapErr(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

func parseArgs(args []string) (options, []string) {
	var opts options
	var operands []string
	endOfFlags := false
	i := 0
	for i < len(args) {
		arg := args[i]
		if endOfFlags {
			operands = append(operands, arg)
			i++
			continue
		}
		if arg == "--" {
			endOfFlags = true
			i++
			continue
		}
		if strings.HasPrefix(arg, "--") {
			i = parseLongFlag(args, i, &opts)
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			i = parseShortFlags(args, i, &opts)
			continue
		}
		operands = append(operands, arg)
		i++
	}
	return opts, operands
}

func parseLongFlag(args []string, i int, opts *options) int {
	arg := args[i]
	switch {
	case arg == "--help":
		fmt.Fprint(os.Stdout, helpText)
		os.Exit(0)
	case arg == "--version":
		fmt.Fprint(os.Stdout, versionText)
		os.Exit(0)
	case strings.HasPrefix(arg, "--mode="):
		opts.mode = arg[len("--mode="):]
	case strings.HasPrefix(arg, "--owner="):
		opts.owner = arg[len("--owner="):]
	case strings.HasPrefix(arg, "--group="):
		opts.group = arg[len("--group="):]
	}
	return i + 1
}

func parseShortFlags(args []string, i int, opts *options) int {
	flags := args[i][1:]
	j := 0
	for j < len(flags) {
		switch flags[j] {
		case 'm':
			opts.mode = consumeValue(flags, j+1, args, &i)
			return i + 1
		case 'o':
			opts.owner = consumeValue(flags, j+1, args, &i)
			return i + 1
		case 'g':
			opts.group = consumeValue(flags, j+1, args, &i)
			return i + 1
		case 'c':
			j++
			continue
		default:
			fmt.Fprintf(os.Stderr,
				"install: invalid option -- '%c'\n", flags[j])
			os.Exit(1)
		}
		j++
	}
	return i + 1
}

func consumeValue(flags string, pos int, args []string, i *int) string {
	if pos < len(flags) {
		return flags[pos:]
	}
	if *i+1 < len(args) {
		*i++
		return args[*i]
	}
	return ""
}
