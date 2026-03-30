// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/test evaluates conditional expressions (prd104-test R1-R4).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// Exit codes per R3.2 and R4.1.
const (
	exitTrue  = 0
	exitFalse = 1
	exitError = 2
)

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args, os.Stderr))
}

// run is the entry point. It strips the bracket wrapper if invoked as '[',
// then evaluates the expression. R4.1: returns 0, 1, or 2.
func run(args []string, stderr *os.File) int {
	exprs, err := prepareArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progBaseName(args[0]), err)
		return exitError
	}
	if len(exprs) == 0 {
		return exitFalse
	}
	result, evalErr := evaluate(exprs, stderr, progBaseName(args[0]))
	if evalErr != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progBaseName(args[0]), evalErr)
		return exitError
	}
	if result {
		return exitTrue
	}
	return exitFalse
}

// progBaseName returns the base name of the invocation path.
func progBaseName(arg0 string) string {
	return filepath.Base(arg0)
}

// prepareArgs handles '[' invocation by checking for closing ']'.
// Returns the expression arguments with program name and brackets stripped.
func prepareArgs(args []string) ([]string, error) {
	base := progBaseName(args[0])
	exprs := args[1:]
	if strings.HasSuffix(base, "[") {
		if len(exprs) == 0 || exprs[len(exprs)-1] != "]" {
			return nil, fmt.Errorf("missing ']'")
		}
		exprs = exprs[:len(exprs)-1]
	}
	return exprs, nil
}

// evaluate dispatches expression evaluation. R3.1: supports !, -a, -o, and
// parenthesized groups via recursive descent parsing.
func evaluate(args []string, _ *os.File, _ string) (bool, error) {
	pos := 0
	result, err := parseOr(args, &pos)
	if err != nil {
		return false, err
	}
	if pos < len(args) {
		return false, fmt.Errorf("extra argument '%s'", args[pos])
	}
	return result, nil
}

// parseOr handles EXPR1 -o EXPR2 (R3.1).
func parseOr(args []string, pos *int) (bool, error) {
	left, err := parseAnd(args, pos)
	if err != nil {
		return false, err
	}
	for *pos < len(args) && args[*pos] == "-o" {
		*pos++
		right, err := parseAnd(args, pos)
		if err != nil {
			return false, err
		}
		left = left || right
	}
	return left, nil
}

// parseAnd handles EXPR1 -a EXPR2 (R3.1).
func parseAnd(args []string, pos *int) (bool, error) {
	left, err := parseNot(args, pos)
	if err != nil {
		return false, err
	}
	for *pos < len(args) && args[*pos] == "-a" {
		*pos++
		right, err := parseNot(args, pos)
		if err != nil {
			return false, err
		}
		left = left && right
	}
	return left, nil
}

// parseNot handles ! EXPR (R3.1).
func parseNot(args []string, pos *int) (bool, error) {
	if *pos < len(args) && args[*pos] == "!" {
		*pos++
		result, err := parseNot(args, pos)
		if err != nil {
			return false, err
		}
		return !result, nil
	}
	return parsePrimary(args, pos)
}

// parsePrimary handles parenthesized groups and primary expressions.
func parsePrimary(args []string, pos *int) (bool, error) {
	if *pos >= len(args) {
		return false, fmt.Errorf("missing argument")
	}
	if args[*pos] == "(" {
		return parseGroup(args, pos)
	}
	return parsePrimaryExpr(args, pos)
}

// parseGroup handles ( EXPR ) grouping (R3.1).
func parseGroup(args []string, pos *int) (bool, error) {
	*pos++ // skip '('
	result, err := parseOr(args, pos)
	if err != nil {
		return false, err
	}
	if *pos >= len(args) || args[*pos] != ")" {
		return false, fmt.Errorf("missing ')'")
	}
	*pos++ // skip ')'
	return result, nil
}

// parsePrimaryExpr dispatches unary and binary test expressions.
func parsePrimaryExpr(args []string, pos *int) (bool, error) {
	if *pos+1 < len(args) {
		if isBinaryOp(args[*pos+1]) {
			return parseBinaryExpr(args, pos)
		}
	}
	return parseUnaryExpr(args, pos)
}

// parseUnaryExpr handles unary file tests and string tests.
func parseUnaryExpr(args []string, pos *int) (bool, error) {
	arg := args[*pos]
	if isUnaryFileOp(arg) && *pos+1 < len(args) {
		return evalUnaryFileTest(arg, args, pos)
	}
	if isUnaryStringOp(arg) && *pos+1 < len(args) {
		return evalUnaryStringTest(arg, args, pos)
	}
	// R2.1: bare STRING is true if non-empty.
	*pos++
	return arg != "", nil
}

// evalUnaryFileTest evaluates a unary file test operator (R1.1).
func evalUnaryFileTest(op string, args []string, pos *int) (bool, error) {
	*pos++
	operand := args[*pos]
	*pos++
	return fileTest(op, operand)
}

// evalUnaryStringTest evaluates a unary string test operator (R2.1).
func evalUnaryStringTest(op string, args []string, pos *int) (bool, error) {
	*pos++
	operand := args[*pos]
	*pos++
	return stringTest(op, operand)
}

// parseBinaryExpr handles binary operators (string, integer, file comparison).
func parseBinaryExpr(args []string, pos *int) (bool, error) {
	left := args[*pos]
	*pos++
	op := args[*pos]
	*pos++
	if *pos >= len(args) {
		return false, fmt.Errorf("missing argument after '%s'", op)
	}
	right := args[*pos]
	*pos++
	if isIntegerOp(op) {
		return integerCompare(left, op, right)
	}
	if isFileCompareOp(op) {
		return fileCompare(left, op, right)
	}
	return stringCompare(left, op, right)
}

// isUnaryFileOp returns true for unary file test operators (R1.1).
func isUnaryFileOp(op string) bool {
	switch op {
	case "-e", "-f", "-d", "-s", "-r", "-w", "-x",
		"-L", "-h", "-b", "-c", "-p", "-S",
		"-g", "-u", "-k", "-G", "-O", "-t":
		return true
	}
	return false
}

// isUnaryStringOp returns true for unary string operators (R2.1).
func isUnaryStringOp(op string) bool {
	return op == "-z" || op == "-n"
}

// isBinaryOp returns true for any binary operator.
func isBinaryOp(op string) bool {
	return isIntegerOp(op) || isStringBinaryOp(op) || isFileCompareOp(op)
}

// isIntegerOp returns true for integer comparison operators (R2.2).
func isIntegerOp(op string) bool {
	switch op {
	case "-eq", "-ne", "-lt", "-le", "-gt", "-ge":
		return true
	}
	return false
}

// isStringBinaryOp returns true for binary string operators (R2.1, R2.2).
func isStringBinaryOp(op string) bool {
	switch op {
	case "=", "!=", "<", ">":
		return true
	}
	return false
}

// isFileCompareOp returns true for binary file comparison operators (R1.2).
func isFileCompareOp(op string) bool {
	return op == "-nt" || op == "-ot" || op == "-ef"
}

// fileTest evaluates a unary file test operator (R1.1).
func fileTest(op, path string) (bool, error) {
	if op == "-t" {
		return terminalTest(path)
	}
	info, err := fileStat(op, path)
	if err != nil {
		return false, nil
	}
	return evalFileMode(op, info)
}

// terminalTest checks if FD is a terminal (R1.1: -t FD).
func terminalTest(fdStr string) (bool, error) {
	fd, err := strconv.Atoi(fdStr)
	if err != nil {
		return false, nil
	}
	return sys.IsTerminal(uintptr(fd)), nil
}

// fileStat calls os.Lstat or os.Stat depending on the operator.
func fileStat(op, path string) (os.FileInfo, error) {
	if op == "-L" || op == "-h" {
		return os.Lstat(path)
	}
	return os.Stat(path)
}

// evalFileMode evaluates file mode bits against the operator (R1.1).
func evalFileMode(op string, info os.FileInfo) (bool, error) {
	mode := info.Mode()
	switch op {
	case "-e":
		return true, nil
	case "-f":
		return mode.IsRegular(), nil
	case "-d":
		return mode.IsDir(), nil
	case "-s":
		return info.Size() > 0, nil
	case "-r":
		return mode&0o444 != 0, nil
	case "-w":
		return mode&0o222 != 0, nil
	case "-x":
		return mode&0o111 != 0, nil
	case "-L", "-h":
		return mode&os.ModeSymlink != 0, nil
	case "-b":
		return mode&os.ModeDevice != 0 && mode&os.ModeCharDevice == 0, nil
	case "-c":
		return mode&os.ModeCharDevice != 0, nil
	case "-p":
		return mode&os.ModeNamedPipe != 0, nil
	case "-S":
		return mode&os.ModeSocket != 0, nil
	case "-g":
		return mode&os.ModeSetgid != 0, nil
	case "-u":
		return mode&os.ModeSetuid != 0, nil
	case "-k":
		return mode&os.ModeSticky != 0, nil
	case "-G":
		return evalOwnerGID(info)
	case "-O":
		return evalOwnerUID(info)
	}
	return false, nil
}

// evalOwnerGID checks if the file is owned by the effective GID (R1.1: -G).
func evalOwnerGID(info os.FileInfo) (bool, error) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false, nil
	}
	return st.Gid == uint32(os.Getegid()), nil
}

// evalOwnerUID checks if the file is owned by the effective UID (R1.1: -O).
func evalOwnerUID(info os.FileInfo) (bool, error) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false, nil
	}
	return st.Uid == uint32(os.Geteuid()), nil
}

// stringTest evaluates a unary string test (R2.1).
func stringTest(op, s string) (bool, error) {
	switch op {
	case "-z":
		return len(s) == 0, nil
	case "-n":
		return len(s) > 0, nil
	}
	return false, fmt.Errorf("unknown operator '%s'", op)
}

// stringCompare evaluates a binary string comparison (R2.1, R2.2).
// D3: byte-level comparison for LC_ALL=C semantics.
func stringCompare(left, op, right string) (bool, error) {
	switch op {
	case "=":
		return left == right, nil
	case "!=":
		return left != right, nil
	case "<":
		return left < right, nil
	case ">":
		return left > right, nil
	}
	return false, fmt.Errorf("unknown operator '%s'", op)
}

// integerCompare evaluates an integer comparison (R2.2).
func integerCompare(left, op, right string) (bool, error) {
	l, err := strconv.ParseInt(left, 10, 64)
	if err != nil {
		return false, fmt.Errorf("invalid integer '%s'", left)
	}
	r, err := strconv.ParseInt(right, 10, 64)
	if err != nil {
		return false, fmt.Errorf("invalid integer '%s'", right)
	}
	return evalIntOp(l, op, r)
}

// evalIntOp compares two integers with the given operator (R2.2).
func evalIntOp(l int64, op string, r int64) (bool, error) {
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
	}
	return false, fmt.Errorf("unknown operator '%s'", op)
}

// fileCompare evaluates binary file comparison operators (R1.2).
func fileCompare(left, op, right string) (bool, error) {
	switch op {
	case "-nt":
		return fileNewer(left, right)
	case "-ot":
		return fileOlder(left, right)
	case "-ef":
		return fileSameInode(left, right)
	}
	return false, fmt.Errorf("unknown operator '%s'", op)
}

// fileNewer returns true if left is newer than right (R1.2: -nt).
// If left doesn't exist, returns false. If right doesn't exist, returns true.
func fileNewer(left, right string) (bool, error) {
	li, err := os.Stat(left)
	if err != nil {
		return false, nil
	}
	ri, err := os.Stat(right)
	if err != nil {
		return true, nil
	}
	return li.ModTime().After(ri.ModTime()), nil
}

// fileOlder returns true if left is older than right (R1.2: -ot).
// If left doesn't exist and right does, returns true. Otherwise false.
func fileOlder(left, right string) (bool, error) {
	li, lerr := os.Stat(left)
	ri, rerr := os.Stat(right)
	if lerr != nil {
		return rerr == nil, nil
	}
	if rerr != nil {
		return false, nil
	}
	return li.ModTime().Before(ri.ModTime()), nil
}

// fileSameInode returns true if both files have same device and inode (R1.2: -ef).
func fileSameInode(left, right string) (bool, error) {
	li, err := os.Stat(left)
	if err != nil {
		return false, nil
	}
	ri, err := os.Stat(right)
	if err != nil {
		return false, nil
	}
	return os.SameFile(li, ri), nil
}
