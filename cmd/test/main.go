// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"golang.org/x/sys/unix"
)

var progName = "test"

func main() {
	sys.InstallSIGPIPEHandler()
	args := os.Args[1:]

	if filepath.Base(os.Args[0]) == "[" {
		progName = "["
		if len(args) == 0 || args[len(args)-1] != "]" {
			exitError("missing ']'")
		}
		args = args[:len(args)-1]
	}

	if len(args) == 0 {
		os.Exit(1)
	}

	p := &parser{args: args}
	result := p.expr()

	if p.pos < len(p.args) {
		exitError("extra argument '%s'", p.args[p.pos])
	}

	if result {
		os.Exit(0)
	}
	os.Exit(1)
}

func exitError(format string, args ...any) {
	fmt.Fprintf(os.Stderr, progName+": "+format+"\n", args...)
	os.Exit(2)
}

type parser struct {
	args []string
	pos  int
}

func (p *parser) peek() string {
	if p.pos >= len(p.args) {
		return ""
	}
	return p.args[p.pos]
}

func (p *parser) advance() string {
	s := p.args[p.pos]
	p.pos++
	return s
}

func (p *parser) remaining() int {
	return len(p.args) - p.pos
}

func (p *parser) expr() bool {
	return p.primary()
}

func (p *parser) primary() bool {
	if p.remaining() == 0 {
		exitError("missing argument")
	}

	if p.remaining() >= 3 && isBinaryOp(p.args[p.pos+1]) {
		left := p.advance()
		op := p.advance()
		right := p.advance()
		return evalBinary(left, op, right)
	}

	tok := p.peek()

	if isUnaryOp(tok) && p.remaining() >= 2 {
		p.advance()
		arg := p.advance()
		return evalUnary(tok, arg)
	}

	p.advance()
	return tok != ""
}

func isUnaryOp(op string) bool {
	switch op {
	case "-e", "-f", "-d", "-s", "-r", "-w", "-x",
		"-L", "-h", "-b", "-c", "-p", "-S",
		"-g", "-u", "-k", "-G", "-O", "-t",
		"-z", "-n":
		return true
	}
	return false
}

func isBinaryOp(op string) bool {
	switch op {
	case "=", "!=",
		"-eq", "-ne", "-lt", "-le", "-gt", "-ge",
		"-nt", "-ot", "-ef":
		return true
	}
	return false
}

func evalUnary(op, arg string) bool {
	switch op {
	case "-z":
		return len(arg) == 0
	case "-n":
		return len(arg) > 0
	case "-t":
		fd, err := strconv.Atoi(arg)
		if err != nil {
			return false
		}
		return sys.IsTerminal(uintptr(fd))
	case "-r":
		return unix.Access(arg, unix.R_OK) == nil
	case "-w":
		return unix.Access(arg, unix.W_OK) == nil
	case "-x":
		return unix.Access(arg, unix.X_OK) == nil
	default:
		return fileStatTest(op, arg)
	}
}

func evalBinary(left, op, right string) bool {
	switch op {
	case "=":
		return left == right
	case "!=":
		return left != right
	case "-eq", "-ne", "-lt", "-le", "-gt", "-ge":
		return intCompare(left, op, right)
	case "-nt", "-ot", "-ef":
		return fileCompare(left, op, right)
	}
	return false
}

func fileStatTest(op, path string) bool {
	var fi *sys.FileInfo
	var err error
	if op == "-L" || op == "-h" {
		fi, err = sys.Lstat(path)
	} else {
		fi, err = sys.Stat(path)
	}
	if err != nil {
		return false
	}
	return evalStatOp(op, fi)
}

func evalStatOp(op string, fi *sys.FileInfo) bool {
	m := fi.Mode
	switch op {
	case "-e":
		return true
	case "-f":
		return m.IsRegular()
	case "-d":
		return m.IsDir()
	case "-s":
		return fi.Size > 0
	case "-L", "-h":
		return m&os.ModeSymlink != 0
	case "-b":
		return m&os.ModeDevice != 0 && m&os.ModeCharDevice == 0
	case "-c":
		return m&os.ModeCharDevice != 0
	case "-p":
		return m&os.ModeNamedPipe != 0
	case "-S":
		return m&os.ModeSocket != 0
	case "-g":
		return m&os.ModeSetgid != 0
	case "-u":
		return m&os.ModeSetuid != 0
	case "-k":
		return m&os.ModeSticky != 0
	case "-G":
		return fi.Gid == uint32(os.Getegid())
	case "-O":
		return fi.Uid == uint32(os.Geteuid())
	}
	return false
}

func fileCompare(left, op, right string) bool {
	switch op {
	case "-nt":
		return isNewer(left, right)
	case "-ot":
		return isNewer(right, left)
	case "-ef":
		return sameFile(left, right)
	}
	return false
}

func isNewer(a, b string) bool {
	ai, aErr := os.Stat(a)
	bi, bErr := os.Stat(b)
	if aErr != nil {
		return false
	}
	if bErr != nil {
		return true
	}
	return ai.ModTime().After(bi.ModTime())
}

func sameFile(a, b string) bool {
	afi, err := sys.Stat(a)
	if err != nil {
		return false
	}
	bfi, err := sys.Stat(b)
	if err != nil {
		return false
	}
	return afi.Dev == bfi.Dev && afi.Ino == bfi.Ino
}

func intCompare(left, op, right string) bool {
	l, lErr := strconv.ParseInt(left, 10, 64)
	if lErr != nil {
		exitError("invalid integer '%s'", left)
	}
	r, rErr := strconv.ParseInt(right, 10, 64)
	if rErr != nil {
		exitError("invalid integer '%s'", right)
	}
	switch op {
	case "-eq":
		return l == r
	case "-ne":
		return l != r
	case "-lt":
		return l < r
	case "-le":
		return l <= r
	case "-gt":
		return l > r
	case "-ge":
		return l >= r
	}
	return false
}
