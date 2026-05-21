// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/mv implements srd057-mv: move or rename files.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()
	args := parseFlags(os.Args[1:])
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "mv: missing file operand\n")
		os.Exit(1)
	}
	if len(args) == 1 {
		fmt.Fprintf(os.Stderr,
			"mv: missing destination file operand after '%s'\n", args[0])
		os.Exit(1)
	}
	dest := args[len(args)-1]
	sources := args[:len(args)-1]
	code := 0
	if len(sources) > 1 {
		code = moveMultiple(sources, dest)
	} else {
		code = moveSingle(sources[0], dest)
	}
	if code != 0 {
		os.Exit(code)
	}
}

func parseFlags(args []string) []string {
	var files []string
	endOfFlags := false
	for _, arg := range args {
		if endOfFlags || arg == "" || arg[0] != '-' {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
	}
	return files
}

func moveSingle(src, dest string) int {
	info, err := os.Stat(dest)
	if err == nil && info.IsDir() {
		dest = filepath.Join(dest, filepath.Base(src))
	}
	if err := moveFile(src, dest); err != nil {
		fmt.Fprintf(os.Stderr, "mv: %s\n", err)
		return 1
	}
	return 0
}

func moveMultiple(sources []string, dest string) int {
	info, err := os.Stat(dest)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "mv: target '%s': not a directory\n", dest)
		return 1
	}
	code := 0
	for _, src := range sources {
		target := filepath.Join(dest, filepath.Base(src))
		if err := moveFile(src, target); err != nil {
			fmt.Fprintf(os.Stderr, "mv: %s\n", err)
			code = 1
		}
	}
	return code
}

func moveFile(src, dest string) error {
	if _, err := os.Lstat(src); err != nil {
		return fmtPathErr("cannot stat", src, err)
	}
	err := os.Rename(src, dest)
	if err == nil {
		return nil
	}
	if isCrossDevice(err) {
		return crossDeviceMove(src, dest)
	}
	return fmtPathErr("cannot move", src, err)
}

func isCrossDevice(err error) bool {
	if linkErr, ok := errors.AsType[*os.LinkError](err); ok {
		return errors.Is(linkErr.Err, syscall.EXDEV)
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
