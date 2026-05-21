// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/rm implements srd058-rm: remove files or directories.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

type interactiveMode int

const (
	interactiveNever  interactiveMode = iota // -f / --interactive=never
	interactiveOnce                          // -I / --interactive=once
	interactiveAlways                        // -i / --interactive=always
)

type options struct {
	recursive   bool            // -r, -R, --recursive (R2.1)
	force       bool            // -f, --force (R2.2)
	dir         bool            // -d, --dir (R2.4)
	interactive interactiveMode // -i, -I, --interactive=WHEN (R3.1, R3.2, R3.4)
	verbose     bool            // -v, --verbose (R3.3)
}

var stdinScanner = bufio.NewScanner(os.Stdin)

func main() {
	sys.InstallSIGPIPEHandler()
	files, opts := parseFlags(os.Args[1:])

	if len(files) == 0 {
		if !opts.force {
			fmt.Fprintf(os.Stderr, "rm: missing operand\n")
			os.Exit(1)
		}
		return
	}

	code := removeFiles(files, opts)
	if code != 0 {
		os.Exit(code)
	}
}

func parseFlags(args []string) ([]string, options) {
	var files []string
	var opts options
	endOfFlags := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if endOfFlags || arg == "" || arg[0] != '-' {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if strings.HasPrefix(arg, "--") {
			switch {
			case arg == "--recursive":
				opts.recursive = true
			case arg == "--force":
				opts.force = true
				opts.interactive = interactiveNever
			case arg == "--dir":
				opts.dir = true
			case arg == "--verbose":
				opts.verbose = true
			case arg == "--interactive=never":
				opts.interactive = interactiveNever
			case arg == "--interactive=once":
				opts.interactive = interactiveOnce
			case arg == "--interactive=always":
				opts.interactive = interactiveAlways
			case arg == "--interactive":
				opts.interactive = interactiveAlways
			}
			continue
		}
		chars := arg[1:]
		for j := 0; j < len(chars); j++ {
			switch chars[j] {
			case 'r', 'R':
				opts.recursive = true
			case 'f':
				opts.force = true
				opts.interactive = interactiveNever
			case 'd':
				opts.dir = true
			case 'i':
				opts.interactive = interactiveAlways
				opts.force = false
			case 'I':
				opts.interactive = interactiveOnce
				opts.force = false
			case 'v':
				opts.verbose = true
			}
		}
	}
	return files, opts
}

func removeFiles(paths []string, opts options) int {
	if shouldPromptOnce(paths, opts) {
		n := len(paths)
		noun := "arguments"
		if n == 1 {
			noun = "argument"
		}
		var ok bool
		if opts.recursive {
			ok = promptUser("rm: remove %d %s recursively? ", n, noun)
		} else {
			ok = promptUser("rm: remove %d %s? ", n, noun)
		}
		if !ok {
			return 0
		}
	}
	exitCode := 0
	for _, path := range paths {
		if err := removePath(path, opts); err != nil {
			fmt.Fprintf(os.Stderr, "rm: %s\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

func removePath(path string, opts options) error {
	info, err := os.Lstat(path)
	if err != nil {
		if opts.force && os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("cannot remove '%s': %s", path, sysErrMsg(err))
	}
	if info.IsDir() {
		if opts.recursive {
			if isDotOrDotDot(path) {
				return fmt.Errorf(
					"refusing to remove '.' or '..' directory: skipping '%s'", path)
			}
			if err := removeDir(path, opts); err != nil {
				return fmt.Errorf("cannot remove '%s': %s", path, sysErrMsg(err))
			}
			return nil
		}
		if opts.dir {
			if err := removeEmptyDir(path, opts); err != nil {
				return fmt.Errorf("cannot remove '%s': %s", path, sysErrMsg(err))
			}
			return nil
		}
		return fmt.Errorf("cannot remove '%s': Is a directory", path)
	}
	if err := removeFile(path, info, opts); err != nil {
		return fmt.Errorf("cannot remove '%s': %s", path, sysErrMsg(err))
	}
	return nil
}

func removeFile(path string, info os.FileInfo, opts options) error {
	if opts.interactive == interactiveAlways {
		desc := "regular file"
		if info.Size() == 0 {
			desc = "regular empty file"
		}
		if !promptUser("rm: remove %s '%s'? ", desc, path) {
			return nil
		}
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	if opts.verbose {
		fmt.Fprintf(os.Stdout, "removed '%s'\n", path)
	}
	return nil
}

func removeDir(path string, opts options) error {
	if opts.interactive == interactiveAlways {
		if !promptUser("rm: descend into directory '%s'? ", path) {
			return nil
		}
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		child := filepath.Join(path, entry.Name())
		if err := removePath(child, opts); err != nil {
			fmt.Fprintf(os.Stderr, "rm: %s\n", err)
		}
	}
	return removeEmptyDir(path, opts)
}

func removeEmptyDir(path string, opts options) error {
	if opts.interactive == interactiveAlways {
		if !promptUser("rm: remove directory '%s'? ", path) {
			return nil
		}
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	if opts.verbose {
		fmt.Fprintf(os.Stdout, "removed directory '%s'\n", path)
	}
	return nil
}

func isDotOrDotDot(path string) bool {
	return path == "." || path == ".."
}

func sysErrMsg(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return errnoMsg(pe.Err)
	}
	return err.Error()
}

func errnoMsg(err error) string {
	errno, ok := err.(syscall.Errno)
	if !ok {
		return err.Error()
	}
	switch errno {
	case syscall.ENOENT:
		return "No such file or directory"
	case syscall.EISDIR:
		return "Is a directory"
	case syscall.EACCES:
		return "Permission denied"
	case syscall.EPERM:
		return "Operation not permitted"
	case syscall.ENOTEMPTY:
		return "Directory not empty"
	default:
		return errno.Error()
	}
}

func promptUser(format string, args ...any) bool {
	fmt.Fprintf(os.Stderr, format, args...)
	if !stdinScanner.Scan() {
		return false
	}
	resp := stdinScanner.Text()
	return len(resp) > 0 && (resp[0] == 'y' || resp[0] == 'Y')
}

func shouldPromptOnce(paths []string, opts options) bool {
	if opts.interactive != interactiveOnce {
		return false
	}
	return len(paths) > 3 || opts.recursive
}
