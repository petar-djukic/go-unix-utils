// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/chmod implements srd089-chmod R1.1-R1.4, R2.1-R2.4, R3.1-R3.2, R4.1-R4.3.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: chmod [OPTION]... MODE[,MODE]... FILE...
  or:  chmod [OPTION]... OCTAL-MODE FILE...
  or:  chmod [OPTION]... --reference=RFILE FILE...
Change the mode of each FILE to MODE.

  -c, --changes          like verbose but report only when a change is made
  -f, --silent, --quiet  suppress most error messages
  -v, --verbose          output a diagnostic for every file processed
      --no-preserve-root  do not treat '/' specially (the default)
      --preserve-root    fail to operate recursively on '/'
      --reference=RFILE  use RFILE's mode instead of MODE values
  -R, --recursive        change files and directories recursively
      --help     display this help and exit
      --version  output version information and exit

Each MODE is of the form '[ugoa]*([-+=]([rwxXst]*|[ugo]))+|[-+=][0-7]+'.
`

const versionText = `chmod (go-unix-utils) dev
`

type options struct {
	verbose      bool
	changes      bool
	silent       bool
	recursive    bool
	preserveRoot bool
	reference    string
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, mode, files := parseArgs(os.Args[1:])
	if opts.reference != "" {
		mode = resolveReference(opts.reference)
	} else if err := validateMode(mode); err != nil {
		fmt.Fprintf(os.Stderr, "chmod: invalid mode: '%s'\n", mode)
		fmt.Fprintln(os.Stderr, "Try 'chmod --help' for more information.")
		os.Exit(1)
	}
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "chmod: missing operand after '%s'\n", mode)
		fmt.Fprintln(os.Stderr, "Try 'chmod --help' for more information.")
		os.Exit(1)
	}

	exitCode := 0
	for _, file := range files {
		if err := processFile(opts, mode, file); err != nil {
			exitCode = 1
		}
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func processFile(opts options, mode string, path string) error {
	if opts.recursive {
		return processRecursive(opts, mode, path)
	}
	return applyAndReport(opts, mode, path)
}

func processRecursive(opts options, mode string, root string) error {
	if opts.preserveRoot && isRootPath(root) {
		fmt.Fprintf(os.Stderr,
			"chmod: it is dangerous to operate recursively on '/'\nchmod: use --no-preserve-root to override this failsafe\n")
		return fmt.Errorf("refusing to operate on root")
	}
	var walkErr error
	err := filepath.WalkDir(root, func(path string, _ os.DirEntry, err error) error {
		if err != nil {
			reportError(opts, path, err)
			walkErr = err
			return nil
		}
		if applyErr := applyAndReport(opts, mode, path); applyErr != nil {
			walkErr = applyErr
		}
		return nil
	})
	if err != nil {
		return err
	}
	return walkErr
}

func applyAndReport(opts options, mode string, path string) error {
	oldMode, err := getMode(path)
	if err != nil {
		reportError(opts, path, err)
		return err
	}
	if err := applyMode(mode, path); err != nil {
		reportError(opts, path, err)
		return err
	}
	newMode, err := getMode(path)
	if err != nil {
		reportError(opts, path, err)
		return err
	}
	printDiagnostic(opts, path, oldMode, newMode)
	return nil
}

func getMode(path string) (os.FileMode, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, err
	}
	return info.Mode().Perm() | (info.Mode() & (os.ModeSetuid | os.ModeSetgid | os.ModeSticky)), nil
}

func reportError(opts options, _ string, err error) {
	if opts.silent {
		return
	}
	fmt.Fprintf(os.Stderr, "chmod: %s\n", formatErr(err))
}

func printDiagnostic(opts options, path string, oldMode, newMode os.FileMode) {
	if opts.verbose {
		fmt.Fprintf(os.Stdout, "mode of '%s' changed from %04o (%s) to %04o (%s)\n",
			path, uint32(oldMode), symbolicString(oldMode), uint32(newMode), symbolicString(newMode))
		return
	}
	if opts.changes && oldMode != newMode {
		fmt.Fprintf(os.Stdout, "mode of '%s' changed from %04o (%s) to %04o (%s)\n",
			path, uint32(oldMode), symbolicString(oldMode), uint32(newMode), symbolicString(newMode))
	}
}

func symbolicString(mode os.FileMode) string {
	var buf [9]byte
	const rwx = "rwx"
	for i := range 9 {
		if mode&(1<<uint(8-i)) != 0 {
			buf[i] = rwx[i%3]
		} else {
			buf[i] = '-'
		}
	}
	applySpecialBits(&buf, mode)
	return string(buf[:])
}

func applySpecialBits(buf *[9]byte, mode os.FileMode) {
	if mode&os.ModeSetuid != 0 {
		if buf[2] == 'x' {
			buf[2] = 's'
		} else {
			buf[2] = 'S'
		}
	}
	if mode&os.ModeSetgid != 0 {
		if buf[5] == 'x' {
			buf[5] = 's'
		} else {
			buf[5] = 'S'
		}
	}
	if mode&os.ModeSticky != 0 {
		if buf[8] == 'x' {
			buf[8] = 't'
		} else {
			buf[8] = 'T'
		}
	}
}

func parseArgs(args []string) (options, string, []string) {
	var opts options
	var files []string
	mode := ""
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
		if mode == "" {
			if strings.HasPrefix(arg, "-") && parseShortFlags(arg[1:], &opts) {
				continue
			}
			if opts.reference != "" {
				files = append(files, arg)
			} else {
				mode = arg
			}
		} else {
			files = append(files, arg)
		}
	}
	if mode == "" && opts.reference == "" {
		fmt.Fprintln(os.Stderr, "chmod: missing operand")
		fmt.Fprintln(os.Stderr, "Try 'chmod --help' for more information.")
		os.Exit(1)
	}
	return opts, mode, files
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
	case "--recursive":
		opts.recursive = true
	case "--preserve-root":
		opts.preserveRoot = true
	case "--no-preserve-root":
		opts.preserveRoot = false
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
		case 'R':
			opts.recursive = true
		default:
			return false
		}
	}
	return true
}

func validateMode(mode string) error {
	if isOctalMode(mode) {
		_, err := strconv.ParseUint(mode, 8, 32)
		return err
	}
	_, err := evalSymbolic(mode, 0, 0)
	return err
}

func applyMode(mode string, path string) error {
	if isOctalMode(mode) {
		return applyOctalMode(mode, path)
	}
	return applySymbolicMode(mode, path)
}

func isOctalMode(mode string) bool {
	s := mode
	if strings.HasPrefix(s, "0") && len(s) > 1 {
		s = s[1:]
	}
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '7' {
			return false
		}
	}
	return true
}

func applyOctalMode(mode string, path string) error {
	val, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return fmt.Errorf("invalid mode: %q", mode)
	}
	return os.Chmod(path, os.FileMode(val))
}

func getUmask() os.FileMode {
	old := syscall.Umask(0)
	syscall.Umask(old)
	return os.FileMode(old)
}

func applySymbolicMode(mode string, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	current := info.Mode()
	newMode, err := evalSymbolic(mode, current, getUmask())
	if err != nil {
		return err
	}
	return os.Chmod(path, newMode)
}

func resolveReference(rfile string) string {
	info, err := os.Stat(rfile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "chmod: failed to get attributes of '%s': %s\n",
			rfile, err.(*os.PathError).Err)
		os.Exit(1)
	}
	m := info.Mode().Perm() | (info.Mode() & (os.ModeSetuid | os.ModeSetgid | os.ModeSticky))
	return fmt.Sprintf("%04o", uint32(m))
}

func isRootPath(path string) bool {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path == "/"
	}
	return resolved == "/"
}

func formatErr(err error) string {
	if pe, ok := errors.AsType[*os.PathError](err); ok {
		return fmt.Sprintf("cannot access '%s': %s", pe.Path, pe.Err)
	}
	return err.Error()
}
