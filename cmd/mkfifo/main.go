// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd092-mkfifo R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3.
package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: mkfifo [OPTION]... NAME...
Create named pipes (FIFOs) with the given NAMEs.

Mandatory arguments to long options are mandatory for short options too.
  -m, --mode=MODE    set file permission bits to MODE, not a=rw - umask
      --help         display this help and exit
      --version      output version information and exit
`

const versionText = `mkfifo (go-unix-utils) dev
`

func main() {
	sys.InstallSIGPIPEHandler()
	modeStr, names, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "mkfifo: %s\n", err)
		fmt.Fprintf(os.Stderr, "Try 'mkfifo --help' for more information.\n")
		os.Exit(1)
	}
	os.Exit(run(modeStr, names))
}

func run(modeStr string, names []string) int {
	var mode os.FileMode
	var modeSet bool
	if modeStr != "" {
		var err error
		mode, err = parseMode(modeStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mkfifo: invalid mode\n")
			return 1
		}
		modeSet = true
	}
	exitCode := 0
	for _, name := range names {
		if err := syscall.Mkfifo(name, 0o666); err != nil {
			fmt.Fprintf(os.Stderr, "mkfifo: cannot create fifo '%s': %s\n",
				name, sysErrMsg(err))
			exitCode = 1
			continue
		}
		if modeSet {
			if err := os.Chmod(name, mode); err != nil {
				fmt.Fprintf(os.Stderr, "mkfifo: cannot create fifo '%s': %s\n",
					name, sysErrMsg(err))
				exitCode = 1
			}
		}
	}
	return exitCode
}

func sysErrMsg(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		err = pe.Err
	}
	se, ok := err.(syscall.Errno)
	if !ok {
		return err.Error()
	}
	switch se {
	case syscall.EEXIST:
		return "File exists"
	case syscall.ENOENT:
		return "No such file or directory"
	case syscall.EACCES:
		return "Permission denied"
	case syscall.ENOTDIR:
		return "Not a directory"
	default:
		return se.Error()
	}
}

func parseArgs(args []string) (string, []string, error) {
	var modeStr string
	var names []string
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			names = append(names, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			n, err := parseLongFlag(args, i, &modeStr)
			if err != nil {
				return "", nil, err
			}
			i += n
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			n, err := parseShortFlag(args, i, &modeStr)
			if err != nil {
				return "", nil, err
			}
			i += n
			continue
		}
		names = append(names, arg)
		i++
	}
	if len(names) == 0 {
		return "", nil, fmt.Errorf("missing operand")
	}
	return modeStr, names, nil
}

func parseLongFlag(args []string, idx int, modeStr *string) (int, error) {
	flag := args[idx]
	if strings.HasPrefix(flag, "--mode=") {
		*modeStr = flag[7:]
		return 1, nil
	}
	switch flag {
	case "--help":
		fmt.Fprint(os.Stdout, helpText)
		os.Exit(0)
	case "--version":
		fmt.Fprint(os.Stdout, versionText)
		os.Exit(0)
	case "--mode":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("option '--mode' requires an argument")
		}
		*modeStr = args[idx+1]
		return 2, nil
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", flag)
	}
	return 0, nil
}

func parseShortFlag(args []string, idx int, modeStr *string) (int, error) {
	flags := args[idx][1:]
	if flags[0] != 'm' {
		return 0, fmt.Errorf("invalid option -- '%c'", flags[0])
	}
	rest := flags[1:]
	if rest != "" {
		*modeStr = rest
		return 1, nil
	}
	if idx+1 >= len(args) {
		return 0, fmt.Errorf("option requires an argument -- 'm'")
	}
	*modeStr = args[idx+1]
	return 2, nil
}

func parseMode(modeStr string) (os.FileMode, error) {
	if len(modeStr) == 0 {
		return 0, fmt.Errorf("invalid mode '%s'", modeStr)
	}
	if modeStr[0] >= '0' && modeStr[0] <= '7' {
		return parseOctalMode(modeStr)
	}
	return parseSymbolicMode(modeStr)
}

func parseOctalMode(s string) (os.FileMode, error) {
	var val uint64
	for _, c := range s {
		if c < '0' || c > '7' {
			return 0, fmt.Errorf("invalid mode '%s'", s)
		}
		val = val*8 + uint64(c-'0')
		if val > 07777 {
			return 0, fmt.Errorf("invalid mode '%s'", s)
		}
	}
	mode := os.FileMode(val & 0o777)
	if val&0o4000 != 0 {
		mode |= os.ModeSetuid
	}
	if val&0o2000 != 0 {
		mode |= os.ModeSetgid
	}
	if val&0o1000 != 0 {
		mode |= os.ModeSticky
	}
	return mode, nil
}

func parseSymbolicMode(modeStr string) (os.FileMode, error) {
	umaskVal := currentUmask()
	mode := os.FileMode(0o666)
	for clause := range strings.SplitSeq(modeStr, ",") {
		var err error
		mode, err = applySymbolicClause(clause, mode, umaskVal)
		if err != nil {
			return 0, fmt.Errorf("invalid mode '%s'", modeStr)
		}
	}
	return mode, nil
}

func currentUmask() int {
	old := syscall.Umask(0)
	syscall.Umask(old)
	return old
}

func applySymbolicClause(clause string, mode os.FileMode, umask int) (os.FileMode, error) {
	i, whoMask, explicit := parseWhoChars(clause)
	if i >= len(clause) {
		return 0, fmt.Errorf("missing operator")
	}
	op := clause[i]
	if op != '+' && op != '-' && op != '=' {
		return 0, fmt.Errorf("invalid operator")
	}
	permBits, specialBits := parsePermChars(clause[i+1:], whoMask)
	effectivePerms := permBits
	if !explicit {
		effectivePerms &^= os.FileMode(umask)
	}
	return applyModeOp(mode, op, whoMask, effectivePerms, specialBits, explicit), nil
}

func parseWhoChars(clause string) (int, os.FileMode, bool) {
	var whoMask os.FileMode
	explicit := false
	i := 0
	for i < len(clause) {
		switch clause[i] {
		case 'u':
			whoMask |= 0o700
			explicit = true
		case 'g':
			whoMask |= 0o070
			explicit = true
		case 'o':
			whoMask |= 0o007
			explicit = true
		case 'a':
			whoMask |= 0o777
			explicit = true
		default:
			if !explicit {
				whoMask = 0o777
			}
			return i, whoMask, explicit
		}
		i++
	}
	if !explicit {
		whoMask = 0o777
	}
	return i, whoMask, explicit
}

func parsePermChars(s string, whoMask os.FileMode) (os.FileMode, os.FileMode) {
	var permBits, specialBits os.FileMode
	for _, c := range s {
		switch c {
		case 'r':
			permBits |= 0o444 & whoMask
		case 'w':
			permBits |= 0o222 & whoMask
		case 'x', 'X':
			permBits |= 0o111 & whoMask
		case 's':
			if whoMask&0o700 != 0 {
				specialBits |= os.ModeSetuid
			}
			if whoMask&0o070 != 0 {
				specialBits |= os.ModeSetgid
			}
		case 't':
			specialBits |= os.ModeSticky
		}
	}
	return permBits, specialBits
}

func applyModeOp(
	mode os.FileMode, op byte, whoMask, perms, special os.FileMode, explicit bool,
) os.FileMode {
	regular := mode & 0o777
	extra := mode &^ os.FileMode(0o777)
	switch op {
	case '=':
		regular = (regular &^ whoMask) | perms
		if explicit {
			if whoMask&0o700 != 0 {
				extra &^= os.ModeSetuid
			}
			if whoMask&0o070 != 0 {
				extra &^= os.ModeSetgid
			}
		} else {
			extra &^= os.ModeSetuid | os.ModeSetgid
		}
		extra |= special
	case '+':
		regular |= perms
		extra |= special
	case '-':
		regular &^= perms
		extra &^= special
	}
	return regular | extra
}
