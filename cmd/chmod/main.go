// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/chmod implements srd089-chmod R1.1-R1.4, R2.1-R2.4, R4.1-R4.3.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

type options struct {
	verbose   bool
	changes   bool
	silent    bool
	recursive bool
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, mode, files := parseArgs(os.Args[1:])
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "chmod: missing operand after '%s'\n", mode)
		fmt.Fprintln(os.Stderr, "Try 'chmod --help' for more information.")
		os.Exit(1)
	}

	exitCode := 0
	for _, file := range files {
		if err := processFile(opts, mode, file); err != nil {
			exitCode = 1
		}
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func processFile(opts options, mode string, path string) error {
	if opts.recursive {
		return processRecursive(opts, mode, path)
	}
	return applyAndReport(opts, mode, path)
}

func processRecursive(opts options, mode string, root string) error {
	var walkErr error
	err := filepath.WalkDir(root, func(path string, _ os.DirEntry, err error) error {
		if err != nil {
			reportError(opts, path, err)
			walkErr = err
			return nil
		}
		if applyErr := applyAndReport(opts, mode, path); applyErr != nil {
			walkErr = applyErr
		}
		return nil
	})
	if err != nil {
		return err
	}
	return walkErr
}

func applyAndReport(opts options, mode string, path string) error {
	oldMode, err := getMode(path)
	if err != nil {
		reportError(opts, path, err)
		return err
	}
	if err := applyMode(mode, path); err != nil {
		reportError(opts, path, err)
		return err
	}
	newMode, err := getMode(path)
	if err != nil {
		reportError(opts, path, err)
		return err
	}
	printDiagnostic(opts, path, oldMode, newMode)
	return nil
}

func getMode(path string) (os.FileMode, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, err
	}
	return info.Mode().Perm() | (info.Mode() & (os.ModeSetuid | os.ModeSetgid | os.ModeSticky)), nil
}

func reportError(opts options, _ string, err error) {
	if opts.silent {
		return
	}
	fmt.Fprintf(os.Stderr, "chmod: %s\n", formatErr(err))
}

func printDiagnostic(opts options, path string, oldMode, newMode os.FileMode) {
	if opts.verbose {
		fmt.Fprintf(os.Stdout, "mode of '%s' changed from %04o (%s) to %04o (%s)\n",
			path, uint32(oldMode), symbolicString(oldMode), uint32(newMode), symbolicString(newMode))
		return
	}
	if opts.changes && oldMode != newMode {
		fmt.Fprintf(os.Stdout, "mode of '%s' changed from %04o (%s) to %04o (%s)\n",
			path, uint32(oldMode), symbolicString(oldMode), uint32(newMode), symbolicString(newMode))
	}
}

func symbolicString(mode os.FileMode) string {
	var buf [9]byte
	const rwx = "rwx"
	for i := range 9 {
		if mode&(1<<uint(8-i)) != 0 {
			buf[i] = rwx[i%3]
		} else {
			buf[i] = '-'
		}
	}
	applySpecialBits(&buf, mode)
	return string(buf[:])
}

func applySpecialBits(buf *[9]byte, mode os.FileMode) {
	if mode&os.ModeSetuid != 0 {
		if buf[2] == 'x' {
			buf[2] = 's'
		} else {
			buf[2] = 'S'
		}
	}
	if mode&os.ModeSetgid != 0 {
		if buf[5] == 'x' {
			buf[5] = 's'
		} else {
			buf[5] = 'S'
		}
	}
	if mode&os.ModeSticky != 0 {
		if buf[8] == 'x' {
			buf[8] = 't'
		} else {
			buf[8] = 'T'
		}
	}
}

func parseArgs(args []string) (options, string, []string) {
	var opts options
	var files []string
	mode := ""
	endOfFlags := false
	for _, arg := range args {
		if endOfFlags {
			files = append(files, arg)
			continue
		}
		if handled := handleSpecialArg(arg, &endOfFlags); handled {
			continue
		}
		if mode == "" {
			if parseLongFlag(arg, &opts) {
				continue
			}
			if strings.HasPrefix(arg, "-") && parseShortFlags(arg[1:], &opts) {
				continue
			}
			mode = arg
		} else {
			files = append(files, arg)
		}
	}
	if mode == "" {
		fmt.Fprintln(os.Stderr, "chmod: missing operand")
		fmt.Fprintln(os.Stderr, "Try 'chmod --help' for more information.")
		os.Exit(1)
	}
	return opts, mode, files
}

func handleSpecialArg(arg string, endOfFlags *bool) bool {
	switch arg {
	case "--help":
		fmt.Fprint(os.Stdout, helpText)
		os.Exit(0)
	case "--version":
		fmt.Fprint(os.Stdout, versionText)
		os.Exit(0)
	case "--":
		*endOfFlags = true
	default:
		return false
	}
	return true
}

func parseLongFlag(arg string, opts *options) bool {
	switch arg {
	case "--verbose":
		opts.verbose = true
	case "--changes":
		opts.changes = true
	case "--silent", "--quiet":
		opts.silent = true
	case "--recursive":
		opts.recursive = true
	default:
		return false
	}
	return true
}

func parseShortFlags(flags string, opts *options) bool {
	for _, c := range flags {
		switch c {
		case 'v':
			opts.verbose = true
		case 'c':
			opts.changes = true
		case 'f':
			opts.silent = true
		case 'R':
			opts.recursive = true
		default:
			return false
		}
	}
	return true
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
