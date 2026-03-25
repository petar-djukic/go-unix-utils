// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/test implements the POSIX test (and [) conditional expression evaluator.
// Implements prd104-test R1.1, R1.2, R2.1, R2.2.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	exitTrue  = 0
	exitFalse = 1
	exitError = 2

	// POSIX access mode constants for syscall.Access.
	accessRead    = 0x04
	accessWrite   = 0x02
	accessExecute = 0x01
)

func main() {
	sys.InstallSIGPIPEHandler()
	args := os.Args[1:]
	prog := filepath.Base(os.Args[0])

	if prog == "[" {
		if len(args) == 0 || args[len(args)-1] != "]" {
			fmt.Fprintf(os.Stderr, "[: missing ']'\n")
			os.Exit(exitError)
		}
		args = args[:len(args)-1]
	}

	result, err := evaluate(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", prog, err)
		os.Exit(exitError)
	}
	if result {
		os.Exit(exitTrue)
	}
	os.Exit(exitFalse)
}

// evaluate dispatches expression evaluation based on argument count.
func evaluate(args []string) (bool, error) {
	switch len(args) {
	case 0:
		return false, nil
	case 1:
		// R2.1: bare STRING is true if non-empty
		return args[0] != "", nil
	case 2:
		return evalUnary(args[0], args[1])
	case 3:
		return evalBinary(args[0], args[1], args[2])
	default:
		// TODO: R3.1 will add logical operators for multi-argument expressions
		return false, fmt.Errorf("too many arguments")
	}
}

// evalUnary evaluates a two-argument unary expression.
func evalUnary(op, arg string) (bool, error) {
	switch op {
	// R2.1: string operators
	case "-z":
		return arg == "", nil
	case "-n":
		return arg != "", nil
	// R1.1: terminal test
	case "-t":
		return evalTerminal(arg)
	default:
		// R1.1: file test operators
		return evalFileTest(op, arg)
	}
}

// evalBinary evaluates a three-argument binary expression.
func evalBinary(left, op, right string) (bool, error) {
	switch op {
	// R2.1: string comparison
	case "=":
		return left == right, nil
	case "!=":
		return left != right, nil
	// R2.2: integer comparison
	case "-eq", "-ne", "-lt", "-le", "-gt", "-ge":
		return evalInteger(left, op, right)
	// R1.2: file comparison
	case "-nt", "-ot", "-ef":
		return evalFileCompare(left, op, right)
	default:
		return false, fmt.Errorf("%s: binary operator expected", op)
	}
}

// R1.1: evalTerminal checks if a file descriptor refers to a terminal.
func evalTerminal(arg string) (bool, error) {
	fd, err := strconv.Atoi(arg)
	if err != nil {
		return false, fmt.Errorf("-t: %s: integer expression expected", arg)
	}
	return sys.IsTerminal(uintptr(fd)), nil
}

// R1.1: evalFileTest evaluates file test operators.
func evalFileTest(op, path string) (bool, error) {
	switch op {
	case "-L", "-h":
		fi, err := os.Lstat(path)
		if err != nil {
			return false, nil
		}
		return fi.Mode()&os.ModeSymlink != 0, nil
	case "-r", "-w", "-x":
		return evalAccess(op, path), nil
	case "-e", "-f", "-d", "-s", "-b", "-c", "-p", "-S",
		"-g", "-u", "-k", "-G", "-O":
		fi, err := os.Stat(path)
		if err != nil {
			return false, nil
		}
		return evalFileMode(op, fi)
	default:
		return false, fmt.Errorf("%s: unary operator expected", op)
	}
}

// R1.1: evalAccess checks file access permissions.
func evalAccess(op, path string) bool {
	var mode uint32
	switch op {
	case "-r":
		mode = accessRead
	case "-w":
		mode = accessWrite
	case "-x":
		mode = accessExecute
	}
	return syscall.Access(path, mode) == nil
}

// R1.1: evalFileMode evaluates file property tests using stat info.
func evalFileMode(op string, fi os.FileInfo) (bool, error) {
	switch op {
	case "-e":
		return true, nil
	case "-f":
		return fi.Mode().IsRegular(), nil
	case "-d":
		return fi.IsDir(), nil
	case "-s":
		return fi.Size() > 0, nil
	case "-b":
		return fi.Mode()&os.ModeDevice != 0 &&
			fi.Mode()&os.ModeCharDevice == 0, nil
	case "-c":
		return fi.Mode()&os.ModeCharDevice != 0, nil
	case "-p":
		return fi.Mode()&os.ModeNamedPipe != 0, nil
	case "-S":
		return fi.Mode()&os.ModeSocket != 0, nil
	case "-g":
		return fi.Mode()&os.ModeSetgid != 0, nil
	case "-u":
		return fi.Mode()&os.ModeSetuid != 0, nil
	case "-k":
		return fi.Mode()&os.ModeSticky != 0, nil
	case "-G":
		return isEffectiveGroup(fi), nil
	case "-O":
		return isEffectiveUser(fi), nil
	default:
		return false, fmt.Errorf("%s: unary operator expected", op)
	}
}

// R1.1: isEffectiveGroup checks if the file is owned by the effective GID.
func isEffectiveGroup(fi os.FileInfo) bool {
	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	return stat.Gid == uint32(os.Getegid())
}

// R1.1: isEffectiveUser checks if the file is owned by the effective UID.
func isEffectiveUser(fi os.FileInfo) bool {
	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	return stat.Uid == uint32(os.Geteuid())
}

// R1.2: evalFileCompare evaluates file comparison operators.
func evalFileCompare(left, op, right string) (bool, error) {
	switch op {
	case "-nt":
		return fileNewerThan(left, right), nil
	case "-ot":
		return fileNewerThan(right, left), nil
	case "-ef":
		return sameFile(left, right), nil
	default:
		return false, fmt.Errorf("%s: binary operator expected", op)
	}
}

// R1.2: fileNewerThan returns true if file a is newer than file b.
func fileNewerThan(a, b string) bool {
	aInfo, aErr := os.Stat(a)
	bInfo, bErr := os.Stat(b)
	if aErr != nil {
		return false
	}
	if bErr != nil {
		return true
	}
	return aInfo.ModTime().After(bInfo.ModTime())
}

// R1.2: sameFile returns true if both paths refer to the same device and inode.
func sameFile(a, b string) bool {
	aInfo, aErr := os.Stat(a)
	bInfo, bErr := os.Stat(b)
	if aErr != nil || bErr != nil {
		return false
	}
	return os.SameFile(aInfo, bInfo)
}

// R2.2: evalInteger evaluates integer comparison operators.
func evalInteger(left, op, right string) (bool, error) {
	l, err := strconv.ParseInt(left, 10, 64)
	if err != nil {
		return false, fmt.Errorf("%s: integer expression expected", left)
	}
	r, err := strconv.ParseInt(right, 10, 64)
	if err != nil {
		return false, fmt.Errorf("%s: integer expression expected", right)
	}
	return compareIntegers(l, op, r)
}

// R2.2: compareIntegers performs the actual integer comparison.
func compareIntegers(l int64, op string, r int64) (bool, error) {
	switch op {
	case "-eq":
		return l == r, nil
	case "-ne":
		return l != r, nil
	case "-lt":
		return l < r, nil
	case "-le":
		return l <= r, nil
	case "-gt":
		return l > r, nil
	case "-ge":
		return l >= r, nil
	default:
		return false, fmt.Errorf("%s: unknown integer operator", op)
	}
}
