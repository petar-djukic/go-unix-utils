// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/test evaluates conditional expressions and exits with status 0 (true),
// 1 (false), or 2 (error).
//
// Implements: srd104-test R1.1 (file test operators), R1.2 (file comparison
// operators), R2.1 (string operators), R2.2 (integer comparison operators),
// R3.1 (logical operators), R3.2 (exit codes), R4.1 (exit codes), R4.2 (SIGPIPE).
package main

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// Exit codes per POSIX test specification and srd104 R4.1.
const (
	exitTrue  = 0
	exitFalse = 1
	exitError = 2
)

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses and evaluates the expression from args, returning the exit code.
func run(args []string) int {
	p := &parser{args: args, pos: 0}
	result, err := p.parseExpr()
	if err != nil {
		fmt.Fprintf(os.Stderr, "test: %v\n", err)
		return exitError
	}
	if p.pos < len(p.args) {
		fmt.Fprintf(os.Stderr, "test: extra argument %q\n", p.args[p.pos])
		return exitError
	}
	if result {
		return exitTrue
	}
	return exitFalse
}

// parser holds the argument list and current position for recursive descent.
type parser struct {
	args []string
	pos  int
}

// peek returns the current token without advancing.
func (p *parser) peek() string {
	if p.pos >= len(p.args) {
		return ""
	}
	return p.args[p.pos]
}

// advance moves past the current token.
func (p *parser) advance() {
	p.pos++
}

// remaining returns the number of unconsumed tokens.
func (p *parser) remaining() int {
	return len(p.args) - p.pos
}

// parseExpr is the top-level expression parser (lowest precedence: -o).
// R3.1: EXPR1 -o EXPR2 (or).
func (p *parser) parseExpr() (bool, error) {
	left, err := p.parseAnd()
	if err != nil {
		return false, err
	}
	for p.peek() == "-o" {
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return false, err
		}
		left = left || right
	}
	return left, nil
}

// parseAnd handles -a (and) with higher precedence than -o.
// R3.1: EXPR1 -a EXPR2 (and).
func (p *parser) parseAnd() (bool, error) {
	left, err := p.parseNot()
	if err != nil {
		return false, err
	}
	for p.peek() == "-a" {
		p.advance()
		right, err := p.parseNot()
		if err != nil {
			return false, err
		}
		left = left && right
	}
	return left, nil
}

// parseNot handles ! (negation) with higher precedence than -a.
// R3.1: ! EXPR (not).
func (p *parser) parseNot() (bool, error) {
	if p.peek() == "!" {
		p.advance()
		val, err := p.parseNot()
		if err != nil {
			return false, err
		}
		return !val, nil
	}
	return p.parsePrimary()
}

// parsePrimary handles parenthesized groups and primary expressions.
// R3.1: ( EXPR ) (grouping).
func (p *parser) parsePrimary() (bool, error) {
	if p.peek() == "(" {
		return p.parseGroup()
	}
	return p.parsePrimaryExpr()
}

// parseGroup handles parenthesized sub-expressions.
func (p *parser) parseGroup() (bool, error) {
	p.advance() // skip "("
	val, err := p.parseExpr()
	if err != nil {
		return false, err
	}
	if p.peek() != ")" {
		return false, fmt.Errorf("missing )")
	}
	p.advance() // skip ")"
	return val, nil
}

// parsePrimaryExpr evaluates a primary expression: unary op, binary op, or string.
func (p *parser) parsePrimaryExpr() (bool, error) {
	if p.remaining() == 0 {
		return false, nil // zero args = false per R1.2
	}
	tok := p.peek()
	if isUnaryFileOp(tok) || tok == "-z" || tok == "-n" {
		return p.parseUnary()
	}
	if p.remaining() >= 3 && isBinaryOp(p.args[p.pos+1]) {
		return p.parseBinary()
	}
	// R2.1: bare STRING is true if non-empty.
	p.advance()
	return tok != "", nil
}

// parseUnary evaluates a unary operator expression.
func (p *parser) parseUnary() (bool, error) {
	op := p.peek()
	p.advance()
	if p.remaining() == 0 {
		return false, fmt.Errorf("missing argument after %q", op)
	}
	operand := p.peek()
	p.advance()
	return evalUnary(op, operand)
}

// parseBinary evaluates a binary operator expression.
func (p *parser) parseBinary() (bool, error) {
	left := p.peek()
	p.advance()
	op := p.peek()
	p.advance()
	if p.remaining() == 0 {
		return false, fmt.Errorf("missing argument after %q", op)
	}
	right := p.peek()
	p.advance()
	return evalBinary(op, left, right)
}

// evalUnary evaluates a unary operator with its operand.
func evalUnary(op, operand string) (bool, error) {
	switch op {
	case "-z":
		return len(operand) == 0, nil // R2.1
	case "-n":
		return len(operand) > 0, nil // R2.1
	default:
		return evalFileUnary(op, operand)
	}
}

// evalBinary dispatches binary operators to string, integer, or file comparisons.
func evalBinary(op, left, right string) (bool, error) {
	switch op {
	case "=":
		return left == right, nil // R2.1
	case "!=":
		return left != right, nil // R2.1
	case "-eq", "-ne", "-lt", "-le", "-gt", "-ge":
		return evalIntCompare(op, left, right) // R2.2
	case "-nt", "-ot", "-ef":
		return evalFileCompare(op, left, right) // R1.2
	default:
		return false, fmt.Errorf("unknown binary operator %q", op)
	}
}

// evalIntCompare evaluates integer comparison operators.
// R2.2: -eq, -ne, -lt, -le, -gt, -ge with proper numeric parsing.
func evalIntCompare(op, left, right string) (bool, error) {
	a, err := strconv.ParseInt(left, 10, 64)
	if err != nil {
		return false, fmt.Errorf("invalid integer '%s'", left)
	}
	b, err := strconv.ParseInt(right, 10, 64)
	if err != nil {
		return false, fmt.Errorf("invalid integer '%s'", right)
	}
	return compareInts(op, a, b), nil
}

// compareInts applies the integer comparison operator to two values.
func compareInts(op string, a, b int64) bool {
	switch op {
	case "-eq":
		return a == b
	case "-ne":
		return a != b
	case "-lt":
		return a < b
	case "-le":
		return a <= b
	case "-gt":
		return a > b
	case "-ge":
		return a >= b
	default:
		return false
	}
}

// isUnaryFileOp reports whether op is a unary file test operator.
// R1.1: -e, -f, -d, -s, -r, -w, -x, -L, -h, -b, -c, -p, -S, -g, -u, -k,
// -G, -O, -t.
func isUnaryFileOp(op string) bool {
	switch op {
	case "-e", "-f", "-d", "-s", "-r", "-w", "-x",
		"-L", "-h", "-b", "-c", "-p", "-S",
		"-g", "-u", "-k", "-G", "-O", "-t":
		return true
	}
	return false
}

// isBinaryOp reports whether op is a binary operator.
func isBinaryOp(op string) bool {
	switch op {
	case "=", "!=",
		"-eq", "-ne", "-lt", "-le", "-gt", "-ge",
		"-nt", "-ot", "-ef":
		return true
	}
	return false
}

// evalFileUnary evaluates unary file test operators.
// R1.1: file test operators.
func evalFileUnary(op, path string) (bool, error) {
	if op == "-t" {
		return evalTerminal(path)
	}
	return evalFileTest(op, path), nil
}

// evalTerminal checks if a file descriptor is a terminal.
// R1.1: -t FD (terminal).
func evalTerminal(fdStr string) (bool, error) {
	fd, err := strconv.Atoi(fdStr)
	if err != nil {
		return false, fmt.Errorf("invalid integer '%s'", fdStr)
	}
	return sys.IsTerminal(uintptr(fd)), nil
}

// evalFileTest evaluates a file test operator against a path.
func evalFileTest(op, path string) bool {
	switch op {
	case "-e":
		return fileExists(path)
	case "-f":
		return isRegular(path)
	case "-d":
		return isDir(path)
	case "-s":
		return isNonEmpty(path)
	case "-r":
		return isReadable(path)
	case "-w":
		return isWritable(path)
	case "-x":
		return isExecutable(path)
	case "-L", "-h":
		return isSymlink(path)
	case "-b":
		return isBlockDev(path)
	case "-c":
		return isCharDev(path)
	case "-p":
		return isNamedPipe(path)
	case "-S":
		return isSocket(path)
	case "-g":
		return hasSetgid(path)
	case "-u":
		return hasSetuid(path)
	case "-k":
		return hasSticky(path)
	case "-G":
		return isGroupOwned(path)
	case "-O":
		return isUserOwned(path)
	default:
		return false
	}
}

func fileExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func isRegular(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular()
}

func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

func isNonEmpty(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Size() > 0
}

func isReadable(path string) bool {
	return syscall.Access(path, syscall.O_RDONLY) == nil
}

func isWritable(path string) bool {
	return syscall.Access(path, 0x2) == nil // W_OK
}

func isExecutable(path string) bool {
	return syscall.Access(path, 0x1) == nil // X_OK
}

func isSymlink(path string) bool {
	fi, err := os.Lstat(path)
	return err == nil && fi.Mode()&os.ModeSymlink != 0
}

func isBlockDev(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode()&os.ModeDevice != 0 && fi.Mode()&os.ModeCharDevice == 0
}

func isCharDev(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func isNamedPipe(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode()&os.ModeNamedPipe != 0
}

func isSocket(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode()&os.ModeSocket != 0
}

func hasSetgid(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode()&os.ModeSetgid != 0
}

func hasSetuid(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode()&os.ModeSetuid != 0
}

func hasSticky(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode()&os.ModeSticky != 0
}

func isGroupOwned(path string) bool {
	fi, err := sys.Stat(path)
	return err == nil && fi.Gid == uint32(os.Getegid())
}

func isUserOwned(path string) bool {
	fi, err := sys.Stat(path)
	return err == nil && fi.Uid == uint32(os.Geteuid())
}

// evalFileCompare evaluates file comparison operators.
// R1.2: -nt (newer than), -ot (older than), -ef (same device and inode).
func evalFileCompare(op, left, right string) (bool, error) {
	switch op {
	case "-nt":
		return fileNewer(left, right), nil
	case "-ot":
		return fileNewer(right, left), nil
	case "-ef":
		return sameFile(left, right), nil
	default:
		return false, fmt.Errorf("unknown file comparison %q", op)
	}
}

// fileNewer returns true if a is newer than b by modification time.
func fileNewer(a, b string) bool {
	aStat, aErr := modTime(a)
	bStat, bErr := modTime(b)
	if aErr != nil || bErr != nil {
		return aErr == nil // a exists but b doesn't => a is newer
	}
	return aStat.After(bStat)
}

// modTime returns the modification time of a file.
func modTime(path string) (time.Time, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return fi.ModTime(), nil
}

// sameFile returns true if both paths refer to the same inode on the same device.
func sameFile(a, b string) bool {
	aInfo, err := sys.Stat(a)
	if err != nil {
		return false
	}
	bInfo, err := sys.Stat(b)
	if err != nil {
		return false
	}
	return aInfo.Dev == bInfo.Dev && aInfo.Ino == bInfo.Ino
}
