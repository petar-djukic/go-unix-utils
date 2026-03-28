// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/test implements the POSIX test (and [) conditional expression evaluator.
// Implements prd104-test R1.1, R1.2, R2.1, R2.2, R3.1, R3.2, R4.1, R4.2.
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

// evaluate dispatches expression evaluation using POSIX argument-count rules
// for 0–4 arguments and recursive descent for 5+.
// R3.1: supports !, -a, -o, and ( ) for multi-argument expressions.
// R3.2, R4.1: returns error for syntax errors (caller exits 2).
func evaluate(args []string) (bool, error) {
	switch len(args) {
	case 0:
		return false, nil
	case 1:
		// R2.1: bare STRING is true if non-empty.
		return args[0] != "", nil
	case 2:
		return evalTwoArgs(args)
	case 3:
		return evalThreeArgs(args)
	case 4:
		return evalFourArgs(args)
	default:
		return evalMulti(args)
	}
}

// evalTwoArgs handles POSIX 2-argument forms.
// R3.1: ! STRING negates the single-string test.
func evalTwoArgs(args []string) (bool, error) {
	if args[0] == "!" {
		return args[1] == "", nil
	}
	return evalUnary(args[0], args[1])
}

// evalThreeArgs handles POSIX 3-argument forms.
// R3.1: supports ! EXPR and ( EXPR ) grouping.
func evalThreeArgs(args []string) (bool, error) {
	if isBinaryOp(args[1]) {
		return evalBinary(args[0], args[1], args[2])
	}
	if args[0] == "!" {
		result, err := evalTwoArgs(args[1:])
		if err != nil {
			return false, err
		}
		return !result, nil
	}
	if args[0] == "(" && args[2] == ")" {
		return args[1] != "", nil
	}
	// Fall through: try as binary for GNU-compatible error messages.
	return evalBinary(args[0], args[1], args[2])
}

// evalFourArgs handles POSIX 4-argument forms.
// R3.1: supports ! 3-arg and ( 2-arg ) grouping.
func evalFourArgs(args []string) (bool, error) {
	if args[0] == "!" {
		result, err := evalThreeArgs(args[1:])
		if err != nil {
			return false, err
		}
		return !result, nil
	}
	if args[0] == "(" && args[3] == ")" {
		return evalTwoArgs(args[1:3])
	}
	return evalMulti(args)
}

// parser holds state for recursive descent expression parsing.
// R3.1: used for expressions with 5+ arguments.
type parser struct {
	args []string
	pos  int
}

// evalMulti evaluates expressions using recursive descent.
func evalMulti(args []string) (bool, error) {
	p := &parser{args: args}
	result, err := p.orExpr()
	if err != nil {
		return false, err
	}
	if p.pos < len(p.args) {
		return false, fmt.Errorf("extra argument %q", p.args[p.pos])
	}
	return result, nil
}

// orExpr parses: and_expr ('-o' and_expr)*
// R3.1: -o is the lowest-precedence logical operator.
func (p *parser) orExpr() (bool, error) {
	result, err := p.andExpr()
	if err != nil {
		return false, err
	}
	for p.pos < len(p.args) && p.args[p.pos] == "-o" {
		p.pos++
		right, err := p.andExpr()
		if err != nil {
			return false, err
		}
		result = result || right
	}
	return result, nil
}

// andExpr parses: not_expr ('-a' not_expr)*
// R3.1: -a has higher precedence than -o.
func (p *parser) andExpr() (bool, error) {
	result, err := p.notExpr()
	if err != nil {
		return false, err
	}
	for p.pos < len(p.args) && p.args[p.pos] == "-a" {
		p.pos++
		right, err := p.notExpr()
		if err != nil {
			return false, err
		}
		result = result && right
	}
	return result, nil
}

// notExpr parses: '!' not_expr | primary
// R3.1: ! has higher precedence than -a and -o.
func (p *parser) notExpr() (bool, error) {
	if p.pos < len(p.args) && p.args[p.pos] == "!" {
		p.pos++
		result, err := p.notExpr()
		if err != nil {
			return false, err
		}
		return !result, nil
	}
	return p.primary()
}

// primary parses: '(' expr ')' | unary_op arg | arg binary_op arg | STRING
func (p *parser) primary() (bool, error) {
	remaining := len(p.args) - p.pos
	if remaining == 0 {
		return false, fmt.Errorf("argument expected")
	}
	if p.args[p.pos] == "(" {
		return p.parenExpr()
	}
	if isUnaryOp(p.args[p.pos]) && remaining >= 2 {
		op := p.args[p.pos]
		arg := p.args[p.pos+1]
		p.pos += 2
		return evalUnary(op, arg)
	}
	if remaining >= 3 && isBinaryOp(p.args[p.pos+1]) {
		left := p.args[p.pos]
		op := p.args[p.pos+1]
		right := p.args[p.pos+2]
		p.pos += 3
		return evalBinary(left, op, right)
	}
	// R2.1: bare STRING is true if non-empty.
	s := p.args[p.pos]
	p.pos++
	return s != "", nil
}

// parenExpr parses a parenthesized expression: '(' expr ')'
// R3.1: parentheses override operator precedence.
func (p *parser) parenExpr() (bool, error) {
	p.pos++ // consume '('
	result, err := p.orExpr()
	if err != nil {
		return false, err
	}
	if p.pos >= len(p.args) || p.args[p.pos] != ")" {
		return false, fmt.Errorf("')' expected")
	}
	p.pos++ // consume ')'
	return result, nil
}

// isUnaryOp returns true if s is a recognized unary test operator.
func isUnaryOp(s string) bool {
	switch s {
	case "-e", "-f", "-d", "-s", "-r", "-w", "-x", "-L", "-h",
		"-b", "-c", "-p", "-S", "-g", "-u", "-k", "-G", "-O",
		"-t", "-z", "-n":
		return true
	}
	return false
}

// isBinaryOp returns true if s is a recognized binary test operator.
func isBinaryOp(s string) bool {
	switch s {
	case "=", "!=", "-eq", "-ne", "-lt", "-le", "-gt", "-ge",
		"-nt", "-ot", "-ef":
		return true
	}
	return false
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
