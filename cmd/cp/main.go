// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/cp implements srd056-cp: copy files and directories.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

type overwriteMode int

const (
	owDefault   overwriteMode = iota
	owAskUser                 // -i
	owNoClobber               // -n
)

type derefMode int

const (
	derefDefault derefMode = iota
	derefAlways
	derefNever
)

var (
	owMode      overwriteMode
	unlinkDest  bool
	recursive   bool
	deref       derefMode
	stdinReader = bufio.NewReader(os.Stdin)
	errDeclined = errors.New("declined")
	errPartial         = errors.New("partial failure")
	preserveMode       bool
	preserveOwnership  bool
	preserveTimestamps bool
	verbose            bool
	targetDir          string
)

func main() {
	sys.InstallSIGPIPEHandler()
	args := parseFlags(os.Args[1:])
	if targetDir != "" {
		if len(args) == 0 {
			fmt.Fprintf(os.Stderr, "cp: missing file operand\n")
			os.Exit(1)
		}
		code := copyMultiple(args, targetDir)
		if code != 0 {
			os.Exit(code)
		}
		return
	}
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "cp: missing file operand\n")
		os.Exit(1)
	}
	if len(args) == 1 {
		fmt.Fprintf(os.Stderr,
			"cp: missing destination file operand after '%s'\n", args[0])
		os.Exit(1)
	}
	dest := args[len(args)-1]
	sources := args[:len(args)-1]
	code := 0
	if len(sources) > 1 {
		code = copyMultiple(sources, dest)
	} else {
		code = copySingle(sources[0], dest)
	}
	if code != 0 {
		os.Exit(code)
	}
}

func parseFlags(args []string) []string {
	var files []string
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
			applyLongFlag(arg[2:])
			continue
		}
		rest := arg[1:]
		for j, c := range rest {
			if c == 't' {
				suffix := rest[j+1:]
				if suffix != "" {
					targetDir = suffix
				} else if i+1 < len(args) {
					i++
					targetDir = args[i]
				}
				break
			}
			applyFlag(c)
		}
	}
	return files
}

func applyFlag(c rune) {
	switch c {
	case 'f':
		owMode = owDefault
		unlinkDest = true
	case 'i':
		owMode = owAskUser
		unlinkDest = false
	case 'n':
		owMode = owNoClobber
	case 'r', 'R':
		recursive = true
	case 'L':
		deref = derefAlways
	case 'P':
		deref = derefNever
	case 'p':
		preserveMode = true
		preserveOwnership = true
		preserveTimestamps = true
	case 'a':
		applyArchive()
	case 'v':
		verbose = true
	}
}

func applyLongFlag(name string) {
	if attr, ok := strings.CutPrefix(name, "preserve="); ok {
		parsePreserve(attr)
		return
	}
	if dir, ok := strings.CutPrefix(name, "target-directory="); ok {
		targetDir = dir
		return
	}
	switch name {
	case "force":
		owMode = owDefault
		unlinkDest = true
	case "interactive":
		owMode = owAskUser
		unlinkDest = false
	case "no-clobber":
		owMode = owNoClobber
	case "recursive":
		recursive = true
	case "dereference":
		deref = derefAlways
	case "no-dereference":
		deref = derefNever
	case "preserve":
		parsePreserve("mode,ownership,timestamps")
	case "archive":
		applyArchive()
	case "verbose":
		verbose = true
	}
}

func applyArchive() {
	recursive = true
	deref = derefNever
	preserveMode = true
	preserveOwnership = true
	preserveTimestamps = true
}

func parsePreserve(attrList string) {
	for attr := range strings.SplitSeq(attrList, ",") {
		switch attr {
		case "mode":
			preserveMode = true
		case "ownership":
			preserveOwnership = true
		case "timestamps":
			preserveTimestamps = true
		case "all":
			preserveMode = true
			preserveOwnership = true
			preserveTimestamps = true
		}
	}
}

func shouldDeref() bool {
	switch deref {
	case derefAlways:
		return true
	case derefNever:
		return false
	default:
		return !recursive
	}
}

func statSource(path string) (os.FileInfo, error) {
	if shouldDeref() {
		return os.Stat(path)
	}
	return os.Lstat(path)
}

func copySingle(src, dest string) int {
	info, err := os.Stat(dest)
	if err == nil && info.IsDir() {
		dest = filepath.Join(dest, filepath.Base(src))
	}
	if err := copyFile(src, dest); err != nil {
		if err != errDeclined && err != errPartial {
			fmt.Fprintf(os.Stderr, "cp: %s\n", err)
		}
		return 1
	}
	return 0
}

func copyMultiple(sources []string, dest string) int {
	info, err := os.Stat(dest)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "cp: target '%s': not a directory\n", dest)
		return 1
	}
	code := 0
	for _, src := range sources {
		target := filepath.Join(dest, filepath.Base(src))
		if err := copyFile(src, target); err != nil {
			if err != errDeclined && err != errPartial {
				fmt.Fprintf(os.Stderr, "cp: %s\n", err)
			}
			code = 1
		}
	}
	return code
}

func copyFile(src, dest string) error {
	srcInfo, err := statSource(src)
	if err != nil {
		return fmtPathErr("cannot stat", src, err)
	}
	if srcInfo.IsDir() {
		if !recursive {
			return fmt.Errorf("-r not specified; omitting directory '%s'", src)
		}
		return copyDir(src, dest)
	}
	isLink := srcInfo.Mode()&os.ModeSymlink != 0
	if destInfo, err := os.Stat(dest); err == nil {
		if !isLink && os.SameFile(srcInfo, destInfo) {
			return fmt.Errorf("'%s' and '%s' are the same file", src, dest)
		}
		switch owMode {
		case owNoClobber:
			return nil
		case owAskUser:
			if !confirmOverwrite(dest) {
				return errDeclined
			}
		}
	}
	if isLink {
		return copyLink(src, dest)
	}
	return doCopy(src, dest, srcInfo.Mode())
}

func copyDir(src, dest string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmtPathErr("cannot stat", src, err)
	}
	if err := os.Mkdir(dest, srcInfo.Mode().Perm()); err != nil && !os.IsExist(err) {
		return fmtPathErr("cannot create directory", dest, err)
	}
	printVerbose(src, dest)
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmtPathErr("cannot read directory", src, err)
	}
	failed := false
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dest, e.Name())
		if err := copyFile(s, d); err != nil {
			if err != errDeclined && err != errPartial {
				fmt.Fprintf(os.Stderr, "cp: %s\n", err)
			}
			failed = true
		}
	}
	if err := preserveAttrs(src, dest); err != nil {
		fmt.Fprintf(os.Stderr, "cp: %s\n", err)
		failed = true
	}
	if failed {
		return errPartial
	}
	return nil
}

func copyLink(src, dest string) error {
	target, err := os.Readlink(src)
	if err != nil {
		return fmtPathErr("cannot readlink", src, err)
	}
	os.Remove(dest)
	if err := os.Symlink(target, dest); err != nil {
		return fmtPathErr("cannot create symlink", dest, err)
	}
	printVerbose(src, dest)
	return preserveAttrs(src, dest)
}

func doCopy(src, dest string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmtPathErr("cannot open", src, err)
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil && unlinkDest {
		os.Remove(dest)
		out, err = os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	}
	if err != nil {
		return fmtPathErr("cannot create regular file", dest, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	printVerbose(src, dest)
	return preserveAttrs(src, dest)
}

func confirmOverwrite(dest string) bool {
	fmt.Fprintf(os.Stderr, "cp: overwrite '%s'? ", dest)
	line, _ := stdinReader.ReadString('\n')
	return len(line) > 0 && (line[0] == 'y' || line[0] == 'Y')
}

func fmtPathErr(verb, path string, err error) error {
	if pe, ok := errors.AsType[*os.PathError](err); ok {
		return fmt.Errorf("%s '%s': %s", verb, path, pe.Err)
	}
	return fmt.Errorf("%s '%s': %s", verb, path, err)
}

func preserveAttrs(src, dest string) error {
	if !preserveMode && !preserveOwnership && !preserveTimestamps {
		return nil
	}
	fi, err := sys.Lstat(src)
	if err != nil {
		return fmtPathErr("cannot stat", src, err)
	}
	isLink := fi.Mode&os.ModeSymlink != 0
	if preserveMode && !isLink {
		perm := fi.Mode & (os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky)
		if err := os.Chmod(dest, perm); err != nil {
			return fmtPathErr("preserving permissions for", dest, err)
		}
	}
	if preserveTimestamps && !isLink {
		if err := os.Chtimes(dest, fi.AccessTime, fi.ModTime); err != nil {
			return fmtPathErr("preserving times for", dest, err)
		}
	}
	if preserveOwnership {
		if err := os.Lchown(dest, int(fi.Uid), int(fi.Gid)); err != nil {
			fmt.Fprintf(os.Stderr, "cp: %s\n",
				fmtPathErr("preserving ownership for", dest, err))
		}
	}
	return nil
}

func printVerbose(src, dest string) {
	if verbose {
		fmt.Printf("'%s' -> '%s'\n", src, dest)
	}
}
