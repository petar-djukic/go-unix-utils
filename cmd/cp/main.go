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
	owDefault  overwriteMode = iota
	owAskUser                // -i
	owNoClobber              // -n
)

var (
	owMode      overwriteMode
	unlinkDest  bool
	stdinReader = bufio.NewReader(os.Stdin)
	errDeclined = errors.New("declined")
)

func main() {
	sys.InstallSIGPIPEHandler()
	args := parseFlags(os.Args[1:])
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
	for _, arg := range args {
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
		for _, c := range arg[1:] {
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
	}
}

func applyLongFlag(name string) {
	switch name {
	case "force":
		owMode = owDefault
		unlinkDest = true
	case "interactive":
		owMode = owAskUser
		unlinkDest = false
	case "no-clobber":
		owMode = owNoClobber
	}
}

func copySingle(src, dest string) int {
	info, err := os.Stat(dest)
	if err == nil && info.IsDir() {
		dest = filepath.Join(dest, filepath.Base(src))
	}
	if err := copyFile(src, dest); err != nil {
		if err != errDeclined {
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
			if err != errDeclined {
				fmt.Fprintf(os.Stderr, "cp: %s\n", err)
			}
			code = 1
		}
	}
	return code
}

func copyFile(src, dest string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmtPathErr("cannot stat", src, err)
	}
	if srcInfo.IsDir() {
		return fmt.Errorf("-r not specified; omitting directory '%s'", src)
	}
	if destInfo, err := os.Stat(dest); err == nil {
		if os.SameFile(srcInfo, destInfo) {
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
	return doCopy(src, dest, srcInfo.Mode())
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
	return out.Close()
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
