// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd034-mkdir R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R3.1, R3.2, R3.3, R3.4, R4.1, R4.2, R4.3.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: mkdir [OPTION]... DIRECTORY...
Create the DIRECTORY(ies), if they do not already exist.

Mandatory arguments to long options are mandatory for short options too.
  -m, --mode=MODE   set file mode (as in chmod), not a=rwx - umask
  -p, --parents     no error if existing, make parent directories as needed,
                    with their file modes unaffected by any -m option
  -v, --verbose     print a message for each created directory
  -Z                set SELinux security context of each created directory
                    to the default type
      --context[=CTX]  like -Z, or if CTX is specified then set the
                    SELinux or SMACK security context to CTX
      --help        display this help and exit
      --version     output version information and exit
`

const versionText = `mkdir (go-unix-utils) dev
`

type options struct {
	parents bool
	verbose bool
	mode    string
	modeSet bool
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, dirs, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %s\n", err)
		fmt.Fprintf(os.Stderr, "Try 'mkdir --help' for more information.\n")
		os.Exit(1)
	}

	os.Exit(run(opts, dirs))
}

func run(opts options, dirs []string) int {
	var mode os.FileMode
	if opts.modeSet {
		var err error
		mode, err = parseMode(opts.mode)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mkdir: %s\n", err)
			return 1
		}
	}
	exitCode := 0
	for _, dir := range dirs {
		var err error
		if opts.parents {
			err = mkdirParents(dir, opts.verbose, opts.modeSet, mode)
		} else {
			err = mkdirSingle(dir, opts.verbose, opts.modeSet, mode)
		}
		if err != nil {
			failPath := dir
			if pe, ok := err.(*os.PathError); ok {
				failPath = pe.Path
			}
			fmt.Fprintf(os.Stderr, "mkdir: cannot create directory '%s': %s\n",
				failPath, sysErrMsg(err))
			exitCode = 1
		}
	}
	return exitCode
}

func mkdirSingle(dir string, verbose, modeSet bool, mode os.FileMode) error {
	if err := os.Mkdir(dir, 0o777); err != nil {
		return err
	}
	if modeSet {
		if err := os.Chmod(dir, mode); err != nil {
			return err
		}
	}
	if verbose {
		fmt.Fprintf(os.Stdout, "mkdir: created directory '%s'\n", dir)
	}
	return nil
}

func mkdirParents(dir string, verbose, modeSet bool, mode os.FileMode) error {
	if verbose {
		if err := mkdirAllVerbose(dir); err != nil {
			return err
		}
	} else {
		if err := os.MkdirAll(dir, 0o777); err != nil {
			return err
		}
	}
	if modeSet {
		return os.Chmod(dir, mode)
	}
	return nil
}

func mkdirAllVerbose(dir string) error {
	fi, err := os.Stat(dir)
	if err == nil {
		if fi.IsDir() {
			return nil
		}
		return &os.PathError{Op: "mkdir", Path: dir, Err: syscall.ENOTDIR}
	}
	parent := filepath.Dir(dir)
	if parent != dir {
		if fi, err := os.Stat(parent); err != nil {
			if err := mkdirAllVerbose(parent); err != nil {
				return err
			}
		} else if !fi.IsDir() {
			return &os.PathError{Op: "mkdir", Path: parent, Err: syscall.ENOTDIR}
		}
	}
	if err := os.Mkdir(dir, 0o777); err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	fmt.Fprintf(os.Stdout, "mkdir: created directory '%s'\n", dir)
	return nil
}

func sysErrMsg(err error) string {
	pe, ok := err.(*os.PathError)
	if !ok {
		return err.Error()
	}
	se, ok := pe.Err.(syscall.Errno)
	if !ok {
		return pe.Err.Error()
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

func parseArgs(args []string) (options, []string, error) {
	var opts options
	var dirs []string
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			dirs = append(dirs, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			n, err := parseLongFlag(args, i, &opts)
			if err != nil {
				return opts, nil, err
			}
			i += n
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			n, err := parseShortFlagGroup(args, i, &opts)
			if err != nil {
				return opts, nil, err
			}
			i += n
			continue
		}
		dirs = append(dirs, arg)
		i++
	}
	if len(dirs) == 0 {
		return opts, nil, fmt.Errorf("missing operand")
	}
	return opts, dirs, nil
}

func parseShortFlagGroup(args []string, i int, opts *options) (int, error) {
	modeVal, hasMode, err := parseShortFlags(args[i][1:], opts)
	if err != nil {
		return 0, err
	}
	if !hasMode {
		return 1, nil
	}
	if modeVal != "" {
		opts.mode = modeVal
		opts.modeSet = true
		return 1, nil
	}
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option requires an argument -- 'm'")
	}
	opts.mode = args[i+1]
	opts.modeSet = true
	return 2, nil
}

func parseLongFlag(args []string, idx int, opts *options) (int, error) {
	flag := args[idx]
	if strings.HasPrefix(flag, "--mode=") {
		opts.mode = flag[7:]
		opts.modeSet = true
		return 1, nil
	}
	switch flag {
	case "--help":
		fmt.Fprint(os.Stdout, helpText)
		os.Exit(0)
		return 0, nil
	case "--version":
		fmt.Fprint(os.Stdout, versionText)
		os.Exit(0)
		return 0, nil
	case "--parents":
		opts.parents = true
		return 1, nil
	case "--verbose":
		opts.verbose = true
		return 1, nil
	case "--mode":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("option '--mode' requires an argument")
		}
		opts.mode = args[idx+1]
		opts.modeSet = true
		return 2, nil
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", flag)
	}
}

func parseShortFlags(flags string, opts *options) (string, bool, error) {
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'p':
			opts.parents = true
		case 'v':
			opts.verbose = true
		case 'm':
			return flags[j+1:], true, nil
		default:
			return "", false, fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return "", false, nil
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
	mode := os.FileMode(0o777)
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

func applyModeOp(mode os.FileMode, op byte, whoMask, perms, special os.FileMode, explicit bool) os.FileMode {
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
