// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/stat displays file and filesystem status information.
// Implements prd082-stat R1.1, R2.1, R2.2, R2.3, R3.1, R4.1, R4.2, R5.1, R7.1, R7.2, R7.3.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// options holds parsed command-line flags.
type options struct {
	dereference bool
	fileSystem  bool
	terse       bool
	format      string
	printfFmt   string
	files       []string
}

// R7.3: SIGPIPE handler for piped output.
func main() {
	sys.InstallSIGPIPEHandler()
	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "stat: %v\n", err)
		os.Exit(1)
	}
	os.Exit(run(opts))
}

// parseArgs processes command-line arguments into options.
func parseArgs(args []string) (options, error) {
	var opts options
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			opts.files = append(opts.files, args[i+1:]...)
			break
		}
		switch {
		case a == "--dereference":
			opts.dereference = true
		case a == "--file-system":
			opts.fileSystem = true
		case a == "--terse":
			opts.terse = true
		case strings.HasPrefix(a, "--format="):
			opts.format = a[len("--format="):]
		case a == "--format":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("option '--format' requires an argument")
			}
			opts.format = args[i]
		case strings.HasPrefix(a, "--printf="):
			opts.printfFmt = a[len("--printf="):]
		case a == "--printf":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("option '--printf' requires an argument")
			}
			opts.printfFmt = args[i]
		case len(a) > 1 && a[0] == '-' && a[1] != '-':
			var err error
			i, err = parseShortGroup(args, i, &opts)
			if err != nil {
				return opts, err
			}
		case strings.HasPrefix(a, "--"):
			return opts, fmt.Errorf("unrecognized option '%s'", a)
		default:
			opts.files = append(opts.files, a)
		}
	}
	if len(opts.files) == 0 {
		return opts, fmt.Errorf("missing operand")
	}
	return opts, nil
}

// parseShortGroup handles combined short flags like -Lt or -c FORMAT.
func parseShortGroup(args []string, idx int, opts *options) (int, error) {
	flags := args[idx][1:]
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'L':
			opts.dereference = true
		case 'f':
			opts.fileSystem = true
		case 't':
			opts.terse = true
		case 'c':
			if j+1 < len(flags) {
				opts.format = flags[j+1:]
			} else {
				idx++
				if idx >= len(args) {
					return idx, fmt.Errorf("option requires an argument -- 'c'")
				}
				opts.format = args[idx]
			}
			return idx, nil
		default:
			return idx, fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return idx, nil
}

// activeFormat returns the active format string and whether it's printf mode.
func activeFormat(opts options) (string, bool) {
	if opts.printfFmt != "" {
		return opts.printfFmt, true
	}
	if opts.format != "" {
		return opts.format, false
	}
	return "", false
}

// R2.1: process multiple files; R2.3, R7.1, R7.2: exit code handling.
func run(opts options) int {
	exitCode := 0
	fmtStr, isPrintf := activeFormat(opts)
	for _, path := range opts.files {
		var err error
		switch {
		case fmtStr != "":
			err = printFormatted(path, fmtStr, isPrintf, opts)
		case opts.fileSystem:
			err = printFileSystem(path, opts.terse)
		default:
			err = printFileStat(path, opts.dereference, opts.terse)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			exitCode = 1
		}
	}
	return exitCode
}

// printFormatted dispatches to file or filesystem format expansion.
func printFormatted(path, fmtStr string, isPrintf bool, opts options) error {
	if opts.fileSystem {
		return printFSFormatted(path, fmtStr, isPrintf)
	}
	return printFileFormatted(path, fmtStr, isPrintf, opts.dereference)
}

// printFileFormatted outputs file status using a format string.
// R3.1: --format appends newline; R4.1: --printf does not.
func printFileFormatted(path, fmtStr string, isPrintf, deref bool) error {
	fi, err := statPath(path, deref)
	if err != nil {
		return fmt.Errorf("stat: cannot stat '%s': %s", path, formatOSError(err))
	}
	output := expandFileFormat(fmtStr, path, fi, isPrintf)
	if !isPrintf {
		output += "\n"
	}
	fmt.Print(output)
	return nil
}

// printFSFormatted outputs filesystem status using a format string.
// R5.1: filesystem format with --format or --printf.
func printFSFormatted(path, fmtStr string, isPrintf bool) error {
	fsi, err := getStatfs(path)
	if err != nil {
		return fmt.Errorf("stat: cannot read file system information for '%s': %s",
			path, capitalizeFirst(err.Error()))
	}
	output := expandFSFormat(fmtStr, path, fsi, isPrintf)
	if !isPrintf {
		output += "\n"
	}
	fmt.Print(output)
	return nil
}

// printFileStat displays file status for a single path.
func printFileStat(path string, deref, terse bool) error {
	fi, err := statPath(path, deref)
	if err != nil {
		return fmt.Errorf("stat: cannot stat '%s': %s", path, formatOSError(err))
	}
	if terse {
		printTerseFile(path, fi)
	} else {
		printDefaultFile(path, fi)
	}
	return nil
}

// statPath calls Stat or Lstat based on the dereference flag.
// R2.2: -L follows symlinks via sys.Stat.
func statPath(path string, deref bool) (*sys.FileInfo, error) {
	if deref {
		return sys.Stat(path)
	}
	return sys.Lstat(path)
}

// printFileSystem displays filesystem status for a single path.
// R5.1: filesystem information via statfs.
func printFileSystem(path string, terse bool) error {
	fsi, err := getStatfs(path)
	if err != nil {
		return fmt.Errorf("stat: cannot read file system information for '%s': %s",
			path, capitalizeFirst(err.Error()))
	}
	if terse {
		printTerseFS(path, fsi)
	} else {
		printDefaultFS(path, fsi)
	}
	return nil
}

// R1.1: default multi-line file status output matching GNU stat.
func printDefaultFile(path string, fi *sys.FileInfo) {
	fmt.Printf("  File: %s\n", formatFileName(path, fi))
	fmt.Printf("  Size: %-10d\tBlocks: %-10d IO Block: %-6d %s\n",
		fi.Size, fi.Blocks, fi.Blksize, fileTypeStr(fi))
	printDeviceLine(fi)
	fmt.Printf("Access: (%04o/%s)  Uid: (%5d/%8s)   Gid: (%5d/%8s)\n",
		unixPerms(fi.Mode), modeString(fi.Mode),
		fi.Uid, lookupUser(fi.Uid), fi.Gid, lookupGroup(fi.Gid))
	fmt.Printf("Access: %s\n", formatTime(fi.AccessTime))
	fmt.Printf("Modify: %s\n", formatTime(fi.ModTime))
	fmt.Printf("Change: %s\n", formatTime(fi.ChangeTime))
	fmt.Printf(" Birth: %s\n", formatBirthTime(fi))
}

// printDeviceLine outputs the Device/Inode/Links line.
func printDeviceLine(fi *sys.FileInfo) {
	maj, min := deviceMajor(fi.Dev), deviceMinor(fi.Dev)
	if fi.Mode&os.ModeDevice != 0 {
		fmt.Printf("Device: %d,%d\tInode: %-10d  Links: %-5d Device type: %x,%x\n",
			maj, min, fi.Ino, fi.Nlink,
			deviceMajor(fi.Rdev), deviceMinor(fi.Rdev))
	} else {
		fmt.Printf("Device: %d,%d\tInode: %-10d  Links: %d\n",
			maj, min, fi.Ino, fi.Nlink)
	}
}

// R4.2: terse output format for files.
func printTerseFile(path string, fi *sys.FileInfo) {
	birthEpoch := int64(0)
	if bt, ok := getBirthTime(fi.Info); ok {
		birthEpoch = bt.Unix()
	}
	fmt.Printf("%s %d %d %x %d %d %x %d %d %x %x %d %d %d %d %d\n",
		path, fi.Size, fi.Blocks, rawMode(fi.Info),
		fi.Uid, fi.Gid, fi.Dev, fi.Ino, fi.Nlink,
		deviceMajor(fi.Rdev), deviceMinor(fi.Rdev),
		fi.AccessTime.Unix(), fi.ModTime.Unix(),
		fi.ChangeTime.Unix(), birthEpoch, fi.Blksize)
}

// R5.1: default multi-line filesystem status output.
func printDefaultFS(path string, fsi *statfsInfo) {
	fmt.Printf("  File: \"%s\"\n", path)
	fmt.Printf("    ID: %-8s Namelen: %-7s Type: %s\n",
		fsi.fsIDHex, fsi.maxName, fsi.typeName)
	fmt.Printf("Block size: %-10d Fundamental block size: %d\n",
		fsi.blockSize, fsi.fundBlockSize)
	fmt.Printf("Blocks: Total: %-10d Free: %-10d Available: %d\n",
		fsi.blocks, fsi.blocksFree, fsi.blocksAvail)
	fmt.Printf("Inodes: Total: %-10d Free: %d\n",
		fsi.files, fsi.filesFree)
}

// R4.2, R5.1: terse output format for filesystem.
func printTerseFS(path string, fsi *statfsInfo) {
	fmt.Printf("%s %s %s %x %d %d %d %d %d %d %d\n",
		path, fsi.fsIDHex, fsi.maxName, fsi.typeNum,
		fsi.blockSize, fsi.fundBlockSize,
		fsi.blocks, fsi.blocksFree, fsi.blocksAvail,
		fsi.files, fsi.filesFree)
}

// formatFileName returns the File: line content.
// Symlinks show path -> target; other types show path.
func formatFileName(path string, fi *sys.FileInfo) string {
	if fi.Mode&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return path
		}
		return fmt.Sprintf("%s -> %s", path, target)
	}
	return path
}

// fileTypeStr returns a human-readable file type string.
func fileTypeStr(fi *sys.FileInfo) string {
	m := fi.Mode
	switch {
	case m.IsRegular():
		if fi.Size == 0 {
			return "regular empty file"
		}
		return "regular file"
	case m.IsDir():
		return "directory"
	case m&os.ModeSymlink != 0:
		return "symbolic link"
	case m&os.ModeCharDevice != 0:
		return "character special file"
	case m&os.ModeDevice != 0:
		return "block special file"
	case m&os.ModeNamedPipe != 0:
		return "fifo"
	case m&os.ModeSocket != 0:
		return "socket"
	default:
		return "weird file"
	}
}

// fileTypeChar returns the single-character file type indicator.
func fileTypeChar(mode os.FileMode) byte {
	switch {
	case mode.IsDir():
		return 'd'
	case mode&os.ModeSymlink != 0:
		return 'l'
	case mode&os.ModeNamedPipe != 0:
		return 'p'
	case mode&os.ModeSocket != 0:
		return 's'
	case mode&os.ModeCharDevice != 0:
		return 'c'
	case mode&os.ModeDevice != 0:
		return 'b'
	default:
		return '-'
	}
}

// modeString returns the 10-character permission string (e.g. -rw-r--r--).
func modeString(mode os.FileMode) string {
	var buf [10]byte
	buf[0] = fileTypeChar(mode)
	const rwx = "rwxrwxrwx"
	perm := mode.Perm()
	for i := 0; i < 9; i++ {
		if perm&(1<<uint(8-i)) != 0 {
			buf[1+i] = rwx[i]
		} else {
			buf[1+i] = '-'
		}
	}
	applySpecialBits(&buf, mode)
	return string(buf[:])
}

// applySpecialBits applies setuid/setgid/sticky to the permission string.
func applySpecialBits(buf *[10]byte, mode os.FileMode) {
	if mode&os.ModeSetuid != 0 {
		if buf[3] == 'x' {
			buf[3] = 's'
		} else {
			buf[3] = 'S'
		}
	}
	if mode&os.ModeSetgid != 0 {
		if buf[6] == 'x' {
			buf[6] = 's'
		} else {
			buf[6] = 'S'
		}
	}
	if mode&os.ModeSticky != 0 {
		if buf[9] == 'x' {
			buf[9] = 't'
		} else {
			buf[9] = 'T'
		}
	}
}

// unixPerms returns the Unix-style octal permission bits (0-4095).
func unixPerms(mode os.FileMode) uint32 {
	perm := uint32(mode.Perm())
	if mode&os.ModeSetuid != 0 {
		perm |= 04000
	}
	if mode&os.ModeSetgid != 0 {
		perm |= 02000
	}
	if mode&os.ModeSticky != 0 {
		perm |= 01000
	}
	return perm
}

// lookupUser returns the username for a uid, or the uid string if lookup fails.
func lookupUser(uid uint32) string {
	u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
	if err != nil {
		return strconv.FormatUint(uint64(uid), 10)
	}
	return u.Username
}

// lookupGroup returns the group name for a gid, or the gid string if lookup fails.
func lookupGroup(gid uint32) string {
	g, err := user.LookupGroupId(strconv.FormatUint(uint64(gid), 10))
	if err != nil {
		return strconv.FormatUint(uint64(gid), 10)
	}
	return g.Name
}

// formatTime formats a timestamp matching GNU stat output.
func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05.000000000 -0700")
}

// formatBirthTime returns the birth time string, or "-" if unavailable.
func formatBirthTime(fi *sys.FileInfo) string {
	bt, ok := getBirthTime(fi.Info)
	if !ok {
		return "-"
	}
	return formatTime(bt)
}

// formatOSError extracts and capitalizes the underlying OS error message.
func formatOSError(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return capitalizeFirst(pe.Err.Error())
	}
	return capitalizeFirst(err.Error())
}

// capitalizeFirst uppercases the first character of a string.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
