// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/mv implements srd057-mv: move or rename files.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

type overwriteMode int

const (
	modeDefault     overwriteMode = iota
	modeInteractive
	modeForce
	modeNoClobber
)

type options struct {
	mode        overwriteMode
	verbose     bool
	targetDir   string
	noTargetDir bool
}

var stdinScanner = bufio.NewScanner(os.Stdin)

func main() {
	sys.InstallSIGPIPEHandler()
	files, opts := parseFlags(os.Args[1:])

	if opts.targetDir != "" && opts.noTargetDir {
		fmt.Fprintf(os.Stderr,
			"mv: cannot combine --target-directory (-t) and --no-target-directory (-T)\n")
		os.Exit(1)
	}

	if opts.targetDir != "" {
		if len(files) == 0 {
			fmt.Fprintf(os.Stderr, "mv: missing file operand\n")
			os.Exit(1)
		}
		code := moveMultiple(files, opts.targetDir, opts)
		if code != 0 {
			os.Exit(code)
		}
		return
	}

	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "mv: missing file operand\n")
		os.Exit(1)
	}
	if len(files) == 1 {
		fmt.Fprintf(os.Stderr,
			"mv: missing destination file operand after '%s'\n", files[0])
		os.Exit(1)
	}
	if opts.noTargetDir && len(files) > 2 {
		fmt.Fprintf(os.Stderr, "mv: extra operand '%s'\n", files[2])
		os.Exit(1)
	}
	dest := files[len(files)-1]
	sources := files[:len(files)-1]
	code := 0
	if len(sources) > 1 {
		code = moveMultiple(sources, dest, opts)
	} else {
		code = moveSingle(sources[0], dest, opts)
	}
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
			case arg == "--interactive":
				opts.mode = modeInteractive
			case arg == "--force":
				opts.mode = modeForce
			case arg == "--no-clobber":
				opts.mode = modeNoClobber
			case arg == "--verbose":
				opts.verbose = true
			case arg == "--no-target-directory":
				opts.noTargetDir = true
			case arg == "--target-directory":
				i++
				if i < len(args) {
					opts.targetDir = args[i]
				}
			case strings.HasPrefix(arg, "--target-directory="):
				opts.targetDir = arg[len("--target-directory="):]
			}
			continue
		}
		chars := arg[1:]
		for j := 0; j < len(chars); j++ {
			switch chars[j] {
			case 'i':
				opts.mode = modeInteractive
			case 'f':
				opts.mode = modeForce
			case 'n':
				opts.mode = modeNoClobber
			case 'v':
				opts.verbose = true
			case 'T':
				opts.noTargetDir = true
			case 't':
				rest := chars[j+1:]
				if len(rest) > 0 {
					opts.targetDir = rest
				} else {
					i++
					if i < len(args) {
						opts.targetDir = args[i]
					}
				}
				j = len(chars)
			}
		}
	}
	return files, opts
}

func moveSingle(src, dest string, opts options) int {
	if !opts.noTargetDir {
		info, err := os.Stat(dest)
		if err == nil && info.IsDir() {
			dest = filepath.Join(dest, filepath.Base(src))
		}
	}
	switch checkOverwrite(dest, opts.mode) {
	case owSkip:
		return 0
	case owSkipError:
		return 1
	}
	renamed, err := moveFile(src, dest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mv: %s\n", err)
		return 1
	}
	if opts.verbose {
		printVerbose(src, dest, renamed)
	}
	return 0
}

func moveMultiple(sources []string, dest string, opts options) int {
	info, err := os.Stat(dest)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "mv: target '%s': not a directory\n", dest)
		return 1
	}
	code := 0
	for _, src := range sources {
		target := filepath.Join(dest, filepath.Base(src))
		switch checkOverwrite(target, opts.mode) {
		case owSkip:
			continue
		case owSkipError:
			code = 1
			continue
		}
		renamed, err := moveFile(src, target)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mv: %s\n", err)
			code = 1
			continue
		}
		if opts.verbose {
			printVerbose(src, target, renamed)
		}
	}
	return code
}

type overwriteAction int

const (
	owProceed   overwriteAction = iota
	owSkip
	owSkipError
)

func checkOverwrite(dest string, mode overwriteMode) overwriteAction {
	if _, err := os.Lstat(dest); err != nil {
		return owProceed
	}
	switch mode {
	case modeNoClobber:
		return owSkip
	case modeInteractive:
		if promptOverwrite(dest) {
			return owProceed
		}
		return owSkipError
	}
	return owProceed
}

func promptOverwrite(dest string) bool {
	fmt.Fprintf(os.Stderr, "mv: overwrite '%s'? ", dest)
	if !stdinScanner.Scan() {
		return false
	}
	line := stdinScanner.Text()
	return len(line) > 0 && (line[0] == 'y' || line[0] == 'Y')
}

func moveFile(src, dest string) (bool, error) {
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return false, fmtPathErr("cannot stat", src, err)
	}
	err = os.Rename(src, dest)
	if err == nil {
		return true, nil
	}
	if isCrossDevice(err) {
		return false, crossDeviceMove(src, dest)
	}
	if srcInfo.IsDir() && isDirExistsErr(err) {
		if rmErr := os.Remove(dest); rmErr == nil {
			if err2 := os.Rename(src, dest); err2 == nil {
				return true, nil
			}
		}
	}
	if !srcInfo.IsDir() {
		if di, derr := os.Lstat(dest); derr == nil && di.IsDir() {
			return false, fmt.Errorf(
				"cannot overwrite directory '%s' with non-directory '%s'", dest, src)
		}
	}
	return false, fmtMoveErr(src, dest, err)
}

func printVerbose(src, dest string, renamed bool) {
	if renamed {
		fmt.Printf("renamed '%s' -> '%s'\n", src, dest)
	} else {
		fmt.Printf("'%s' -> '%s'\n", src, dest)
	}
}

func isCrossDevice(err error) bool {
	if linkErr, ok := errors.AsType[*os.LinkError](err); ok {
		return errors.Is(linkErr.Err, syscall.EXDEV)
	}
	return false
}

func isDirExistsErr(err error) bool {
	if linkErr, ok := errors.AsType[*os.LinkError](err); ok {
		return errors.Is(linkErr.Err, syscall.EEXIST) ||
			errors.Is(linkErr.Err, syscall.ENOTEMPTY)
	}
	return false
}

func crossDeviceMove(src, dest string) error {
	if err := copyTree(src, dest); err != nil {
		return err
	}
	if err := os.RemoveAll(src); err != nil {
		return fmtPathErr("cannot remove", src, err)
	}
	return nil
}

func copyTree(src, dest string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return fmtPathErr("cannot stat", src, err)
	}
	if info.IsDir() {
		return copyDir(src, dest, info)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return copySymlink(src, dest)
	}
	return copyRegular(src, dest, info.Mode())
}

func copyDir(src, dest string, info os.FileInfo) error {
	if err := os.Mkdir(dest, info.Mode().Perm()); err != nil {
		return fmtPathErr("cannot create directory", dest, err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmtPathErr("cannot read directory", src, err)
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dest, e.Name())
		if err := copyTree(s, d); err != nil {
			return err
		}
	}
	return nil
}

func copySymlink(src, dest string) error {
	target, err := os.Readlink(src)
	if err != nil {
		return fmtPathErr("cannot readlink", src, err)
	}
	return os.Symlink(target, dest)
}

func copyRegular(src, dest string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmtPathErr("cannot open", src, err)
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmtPathErr("cannot create", dest, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func fmtPathErr(verb, path string, err error) error {
	if pe, ok := errors.AsType[*os.PathError](err); ok {
		return fmt.Errorf("%s '%s': %s", verb, path, pe.Err)
	}
	return fmt.Errorf("%s '%s': %s", verb, path, err)
}

func fmtMoveErr(src, dest string, err error) error {
	if le, ok := errors.AsType[*os.LinkError](err); ok {
		return fmt.Errorf("cannot move '%s' to '%s': %s", src, dest, le.Err)
	}
	return fmt.Errorf("cannot move '%s' to '%s': %s", src, dest, err)
}
