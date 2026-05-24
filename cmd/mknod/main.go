// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd093-mknod R1.1, R1.2, R1.3, R2.1, R2.2, R2.3.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: mknod [OPTION]... NAME TYPE [MAJOR MINOR]
Create the special file NAME of the given TYPE.

Mandatory arguments to long options are mandatory for short options too.
  -m, --mode=MODE    set file permission bits to MODE, not a=rw - umask
      --help         display this help and exit
      --version      output version information and exit

Both MAJOR and MINOR must be specified when TYPE is b, c, or u, and they must
be omitted when TYPE is p.  If MAJOR or MINOR begins with 0x or 0X, it is
interpreted as hexadecimal; otherwise, if it begins with 0, as octal; else, as
decimal.  TYPE may be:

  b      create a block (buffered) special file
  c, u   create a character (unbuffered) special file
  p      create a FIFO
`

const versionText = `mknod (go-unix-utils) dev
`

type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

func main() {
	sys.InstallSIGPIPEHandler()
	args := os.Args[1:]
	modeStr, name, nodeType, major, minor, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mknod: %s\n", err)
		if _, ok := err.(*usageError); ok {
			fmt.Fprintf(os.Stderr, "Try 'mknod --help' for more information.\n")
		}
		os.Exit(1)
	}
	os.Exit(run(modeStr, name, nodeType, major, minor))
}

func run(modeStr, name, nodeType string, major, minor int) int {
	var fileMode uint32
	var dev int

	switch nodeType {
	case "b":
		fileMode = syscall.S_IFBLK
		dev = major<<24 | minor
	case "c", "u":
		fileMode = syscall.S_IFCHR
		dev = major<<24 | minor
	case "p":
		fileMode = syscall.S_IFIFO
		dev = 0
	}

	var permBits uint32
	if modeStr != "" {
		mode, err := parseMode(modeStr, nodeType)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mknod: invalid mode\n")
			return 1
		}
		permBits = uint32(mode)
	} else {
		umaskVal := currentUmask()
		if nodeType == "p" {
			permBits = 0o666 &^ uint32(umaskVal)
		} else {
			permBits = 0o660 &^ uint32(umaskVal)
		}
	}

	fileMode |= permBits

	if err := syscall.Mknod(name, fileMode, dev); err != nil {
		fmt.Fprintf(os.Stderr, "mknod: %s: %s\n", name, sysErrMsg(err))
		return 1
	}

	if modeStr != "" {
		mode, _ := parseMode(modeStr, nodeType)
		if err := os.Chmod(name, mode); err != nil {
			fmt.Fprintf(os.Stderr, "mknod: cannot set permissions on %q: %s\n", name, sysErrMsg(err))
			return 1
		}
	}

	return 0
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
	case syscall.EACCES, syscall.EPERM:
		return "Permission denied"
	case syscall.ENOTDIR:
		return "Not a directory"
	default:
		return se.Error()
	}
}

func parseArgs(args []string) (string, string, string, int, int, error) {
	var modeStr string
	var positional []string
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			n, err := parseLongFlag(args, i, &modeStr)
			if err != nil {
				return "", "", "", 0, 0, err
			}
			i += n
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			n, err := parseShortFlag(args, i, &modeStr)
			if err != nil {
				return "", "", "", 0, 0, err
			}
			i += n
			continue
		}
		positional = append(positional, arg)
		i++
	}

	if len(positional) == 0 {
		return "", "", "", 0, 0, &usageError{"missing operand"}
	}
	if len(positional) == 1 {
		return "", "", "", 0, 0, &usageError{fmt.Sprintf("missing operand after '%s'", positional[0])}
	}

	name := positional[0]
	nodeType := positional[1]

	if nodeType == "p" {
		if len(positional) > 2 {
			return "", "", "", 0, 0, &usageError{fmt.Sprintf("extra operand '%s'", positional[2])}
		}
		return modeStr, name, nodeType, 0, 0, nil
	}

	// For non-FIFO types (b, c, u, or invalid), require MAJOR MINOR
	if len(positional) < 4 {
		last := positional[len(positional)-1]
		if len(positional) == 2 {
			return "", "", "", 0, 0, &usageError{fmt.Sprintf("missing operand after '%s'\nSpecial files require major and minor device numbers.", last)}
		}
		return "", "", "", 0, 0, &usageError{fmt.Sprintf("missing operand after '%s'", last)}
	}
	if len(positional) > 4 {
		return "", "", "", 0, 0, &usageError{fmt.Sprintf("extra operand '%s'", positional[4])}
	}

	// Now validate the type
	switch nodeType {
	case "b", "c", "u":
	default:
		return "", "", "", 0, 0, &usageError{fmt.Sprintf("invalid device type '%s'", nodeType)}
	}

	major, err := parseDeviceNumber(positional[2])
	if err != nil {
		return "", "", "", 0, 0, fmt.Errorf("invalid major device number '%s'", positional[2])
	}
	minor, err := parseDeviceNumber(positional[3])
	if err != nil {
		return "", "", "", 0, 0, fmt.Errorf("invalid minor device number '%s'", positional[3])
	}
	return modeStr, name, nodeType, major, minor, nil
}

func parseDeviceNumber(s string) (int, error) {
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		val, err := strconv.ParseInt(s[2:], 16, 64)
		return int(val), err
	}
	if len(s) > 1 && s[0] == '0' {
		val, err := strconv.ParseInt(s, 8, 64)
		return int(val), err
	}
	val, err := strconv.ParseInt(s, 10, 64)
	return int(val), err
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
			return 0, &usageError{"option '--mode' requires an argument"}
		}
		*modeStr = args[idx+1]
		return 2, nil
	default:
		return 0, &usageError{fmt.Sprintf("unrecognized option '%s'", flag)}
	}
	return 0, nil
}

func parseShortFlag(args []string, idx int, modeStr *string) (int, error) {
	flags := args[idx][1:]
	if flags[0] != 'm' {
		return 0, &usageError{fmt.Sprintf("invalid option -- '%c'", flags[0])}
	}
	rest := flags[1:]
	if rest != "" {
		*modeStr = rest
		return 1, nil
	}
	if idx+1 >= len(args) {
		return 0, &usageError{"option requires an argument -- 'm'"}
	}
	*modeStr = args[idx+1]
	return 2, nil
}

func parseMode(modeStr string, nodeType string) (os.FileMode, error) {
	if len(modeStr) == 0 {
		return 0, fmt.Errorf("invalid mode '%s'", modeStr)
	}
	if modeStr[0] >= '0' && modeStr[0] <= '7' {
		return parseOctalMode(modeStr)
	}
	return parseSymbolicMode(modeStr, nodeType)
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

func parseSymbolicMode(modeStr string, nodeType string) (os.FileMode, error) {
	umaskVal := currentUmask()
	var baseMode os.FileMode
	if nodeType == "p" {
		baseMode = 0o666
	} else {
		baseMode = 0o660
	}
	mode := baseMode
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
