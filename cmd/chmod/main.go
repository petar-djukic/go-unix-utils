// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/chmod implements srd089-chmod R1.1-R1.4, R4.1-R4.3.
package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: chmod [OPTION]... MODE[,MODE]... FILE...
  or:  chmod [OPTION]... OCTAL-MODE FILE...
Change the mode of each FILE to MODE.

  -c, --changes        like verbose but report only when a change is made
  -f, --silent, --quiet  suppress most error messages
  -v, --verbose        output a diagnostic for every file processed
  -R, --recursive      change files and directories recursively
      --help     display this help and exit
      --version  output version information and exit

Each MODE is of the form '[ugoa]*([-+=]([rwxXst]*|[ugo]))+|[-+=][0-7]+'.
`

const versionText = `chmod (go-unix-utils) dev
`

func main() {
	sys.InstallSIGPIPEHandler()

	mode, files := parseArgs(os.Args[1:])
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "chmod: missing operand after '%s'\n", mode)
		fmt.Fprintln(os.Stderr, "Try 'chmod --help' for more information.")
		os.Exit(1)
	}

	exitCode := 0
	for _, file := range files {
		if err := applyMode(mode, file); err != nil {
			fmt.Fprintf(os.Stderr, "chmod: %s\n", formatErr(err))
			exitCode = 1
		}
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func parseArgs(args []string) (string, []string) {
	var files []string
	mode := ""
	endOfFlags := false
	for _, arg := range args {
		if endOfFlags {
			files = append(files, arg)
			continue
		}
		switch {
		case arg == "--help":
			fmt.Fprint(os.Stdout, helpText)
			os.Exit(0)
		case arg == "--version":
			fmt.Fprint(os.Stdout, versionText)
			os.Exit(0)
		case arg == "--":
			endOfFlags = true
		case mode == "":
			mode = arg
		default:
			files = append(files, arg)
		}
	}
	if mode == "" {
		fmt.Fprintln(os.Stderr, "chmod: missing operand")
		fmt.Fprintln(os.Stderr, "Try 'chmod --help' for more information.")
		os.Exit(1)
	}
	return mode, files
}

func applyMode(mode string, path string) error {
	if isOctalMode(mode) {
		return applyOctalMode(mode, path)
	}
	return applySymbolicMode(mode, path)
}

func isOctalMode(mode string) bool {
	s := mode
	if strings.HasPrefix(s, "0") && len(s) > 1 {
		s = s[1:]
	}
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '7' {
			return false
		}
	}
	return true
}

func applyOctalMode(mode string, path string) error {
	val, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return fmt.Errorf("invalid mode: %q", mode)
	}
	return os.Chmod(path, os.FileMode(val))
}

func getUmask() os.FileMode {
	old := syscall.Umask(0)
	syscall.Umask(old)
	return os.FileMode(old)
}

func applySymbolicMode(mode string, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	current := info.Mode()
	newMode, err := evalSymbolic(mode, current, getUmask())
	if err != nil {
		return err
	}
	return os.Chmod(path, newMode)
}

func evalSymbolic(spec string, current os.FileMode, umask os.FileMode) (os.FileMode, error) {
	perm := current.Perm() | (current & (os.ModeSetuid | os.ModeSetgid | os.ModeSticky))
	for clause := range strings.SplitSeq(spec, ",") {
		var err error
		perm, err = evalClause(clause, perm, umask)
		if err != nil {
			return 0, err
		}
	}
	return perm, nil
}

func evalClause(clause string, perm os.FileMode, umask os.FileMode) (os.FileMode, error) {
	i := 0
	who := parseWho(clause, &i)
	if i >= len(clause) {
		return 0, fmt.Errorf("invalid mode: %q", clause)
	}
	for i < len(clause) {
		op := clause[i]
		if op != '+' && op != '-' && op != '=' {
			return 0, fmt.Errorf("invalid mode: %q", clause)
		}
		i++
		bits := parsePerms(clause, &i)
		perm = applyOp(perm, who, op, bits, umask)
	}
	return perm, nil
}

type permBits struct {
	read    bool
	write   bool
	exec    bool
	execX   bool
	setuid  bool
	setgid  bool
	sticky  bool
}

func parseWho(clause string, pos *int) string {
	start := *pos
	for *pos < len(clause) {
		c := clause[*pos]
		if c == 'u' || c == 'g' || c == 'o' || c == 'a' {
			*pos++
		} else {
			break
		}
	}
	return clause[start:*pos]
}

func parsePerms(clause string, pos *int) permBits {
	var bits permBits
	for *pos < len(clause) {
		c := clause[*pos]
		switch c {
		case 'r':
			bits.read = true
		case 'w':
			bits.write = true
		case 'x':
			bits.exec = true
		case 'X':
			bits.execX = true
		case 's':
			bits.setuid = true
			bits.setgid = true
		case 't':
			bits.sticky = true
		default:
			return bits
		}
		*pos++
	}
	return bits
}

func applyOp(perm os.FileMode, who string, op byte, bits permBits, umask os.FileMode) os.FileMode {
	implicit := who == ""
	if implicit || strings.ContainsRune(who, 'a') {
		who = "ugo"
	}

	var mask os.FileMode
	for _, w := range who {
		mask |= buildMask(w, bits, perm)
	}

	if implicit {
		mask &^= umask
	}

	switch op {
	case '+':
		perm |= mask
	case '-':
		perm &^= mask
	case '=':
		perm = applyEquals(perm, who, mask)
	}
	return perm
}

func buildMask(who rune, bits permBits, current os.FileMode) os.FileMode {
	var rBit, wBit, xBit os.FileMode
	var specialBit os.FileMode
	useSpecial := false

	switch who {
	case 'u':
		rBit, wBit, xBit = 0400, 0200, 0100
		specialBit = os.ModeSetuid
		useSpecial = bits.setuid || bits.setgid
	case 'g':
		rBit, wBit, xBit = 0040, 0020, 0010
		specialBit = os.ModeSetgid
		useSpecial = bits.setuid || bits.setgid
	case 'o':
		rBit, wBit, xBit = 0004, 0002, 0001
		specialBit = os.ModeSticky
		useSpecial = bits.sticky
	}

	var mask os.FileMode
	if bits.read {
		mask |= rBit
	}
	if bits.write {
		mask |= wBit
	}
	if bits.exec {
		mask |= xBit
	}
	if bits.execX && current&0111 != 0 {
		mask |= xBit
	}
	if useSpecial {
		mask |= specialBit
	}
	return mask
}

func applyEquals(perm os.FileMode, who string, mask os.FileMode) os.FileMode {
	var clear os.FileMode
	for _, w := range who {
		switch w {
		case 'u':
			clear |= 0700 | os.ModeSetuid
		case 'g':
			clear |= 0070 | os.ModeSetgid
		case 'o':
			clear |= 0007 | os.ModeSticky
		}
	}
	perm &^= clear
	perm |= mask
	return perm
}

func formatErr(err error) string {
	if pe, ok := errors.AsType[*os.PathError](err); ok {
		return fmt.Sprintf("cannot access '%s': %s", pe.Path, pe.Err)
	}
	return err.Error()
}
