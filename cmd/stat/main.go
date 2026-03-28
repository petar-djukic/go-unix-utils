// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd082-stat R1.1-R1.4, R2.1-R2.3, R3.1-R3.2, R4.1-R4.3: core stat
// output, format directives, terse mode, error handling, exit codes, version/help
// output, and SIGPIPE handling.
// Covers default multi-line output, -c/--format, --printf, -L/--dereference,
// -t/--terse, and format directives for file metadata (%a, %A, %b, %B, %d, %D,
// %f, %F, %g, %G, %h, %i, %n, %N, %o, %s, %t, %T, %u, %U, %w, %W, %x, %X,
// %y, %Y, %z, %Z).
package main

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "stat"

// terseFileFormat is the predefined format string for --terse file output.
// R2.3: matches GNU stat --terse format.
const terseFileFormat = "%n %s %b %f %u %g %D %i %h %t %T %X %Y %Z %W %o"

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// outputMode selects how format strings are handled.
type outputMode int

const (
	modeDefault outputMode = iota
	modeFormat             // -c/--format: custom format with trailing newline
	modePrintf             // --printf: custom format with escape interpretation, no trailing newline
)

// config holds parsed command-line options.
type config struct {
	mode        outputMode
	format      string
	dereference bool // -L/--dereference
	terse       bool // -t/--terse
	showHelp    bool
	showVersion bool
	args        []string
}

// statInfo extends sys.FileInfo with birth time from the platform syscall.
type statInfo struct {
	fi        *sys.FileInfo
	birthTime time.Time
	hasBirth  bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and prints file status. Returns exit code.
func run(args []string) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		printTryHelp()
		return 1
	}
	if cfg.showHelp {
		return printHelp()
	}
	if cfg.showVersion {
		return printVersion()
	}
	if len(cfg.args) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)
		printTryHelp()
		return 1
	}
	// R2.3: resolve terse mode to a predefined format string.
	resolveTerse(&cfg)
	return processFiles(cfg)
}

// resolveTerse applies the terse format when --terse is set and no explicit
// format was specified.
func resolveTerse(cfg *config) {
	if cfg.terse && cfg.mode == modeDefault {
		cfg.mode = modeFormat
		cfg.format = terseFileFormat
	}
}

// processFiles stats each file argument and prints output.
// R1.2: processes multiple files in order.
// R1.4: continues on error, exits 1 if any file fails.
func processFiles(cfg config) int {
	exitCode := 0
	for _, path := range cfg.args {
		if err := statFile(cfg, path); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
			exitCode = 1
		}
	}
	return exitCode
}

// statFile retrieves and displays status for a single file.
func statFile(cfg config, path string) error {
	fi, err := statFunc(cfg.dereference)(path)
	if err != nil {
		return formatStatError(path, err)
	}
	si := buildStatInfo(fi)
	printFileStatus(cfg, path, si)
	return nil
}

// buildStatInfo creates a statInfo with birth time extracted from the underlying syscall.
func buildStatInfo(fi *sys.FileInfo) statInfo {
	si := statInfo{fi: fi}
	if st, ok := fi.Info.Sys().(*syscall.Stat_t); ok {
		si.birthTime = time.Unix(st.Birthtimespec.Sec, st.Birthtimespec.Nsec)
		si.hasBirth = st.Birthtimespec.Sec != 0 || st.Birthtimespec.Nsec != 0
	}
	return si
}

// statFunc returns Stat or Lstat based on the dereference flag.
// R1.3: -L follows symlinks via sys.Stat.
func statFunc(deref bool) func(string) (*sys.FileInfo, error) {
	if deref {
		return sys.Stat
	}
	return sys.Lstat
}

// formatStatError formats an error for display matching GNU stat style.
// R3.1/R3.2: error messages match gstat format exactly.
func formatStatError(path string, err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return fmt.Errorf("cannot stat '%s': %v", path, pe.Err)
	}
	return fmt.Errorf("cannot stat '%s': %v", path, err)
}

// printFileStatus outputs file status using the configured format.
func printFileStatus(cfg config, path string, si statInfo) {
	switch cfg.mode {
	case modeFormat:
		fmt.Println(expandFormat(cfg.format, path, si))
	case modePrintf:
		fmt.Print(expandPrintf(cfg.format, path, si))
	default:
		printDefaultOutput(path, si)
	}
}

// printDefaultOutput prints the GNU stat default multi-line output.
// R1.1: matches GNU stat format for regular files, directories, symlinks.
func printDefaultOutput(path string, si statInfo) {
	printFileHeader(path, si.fi)
	printSizeBlock(si.fi)
	printDeviceInode(si.fi)
	printPermissions(si.fi)
	printTimestamps(si)
}

// printFileHeader prints the File: line with type info.
// GNU stat does not quote filenames in default output. Symlinks show " -> target".
func printFileHeader(path string, fi *sys.FileInfo) {
	nameStr := path
	if fi.Mode&os.ModeSymlink != 0 {
		if target, err := os.Readlink(path); err == nil {
			nameStr = path + " -> " + target
		}
	}
	fmt.Printf("  File: %s\n", nameStr)
}

// printSizeBlock prints Size/Blocks/IO Block/file type line.
func printSizeBlock(fi *sys.FileInfo) {
	fmt.Printf("  Size: %-10d\tBlocks: %-10d IO Block: %-6d %s\n",
		fi.Size, fi.Blocks, fi.Blksize, fileTypeName(fi.Mode))
}

// printDeviceInode prints Device/Inode/Links line.
// GNU stat on macOS formats device as "major,minor".
func printDeviceInode(fi *sys.FileInfo) {
	maj := majorDev(fi.Dev)
	min := minorDev(fi.Dev)
	fmt.Printf("Device: %d,%d\tInode: %-10d  Links: %d\n",
		maj, min, fi.Ino, fi.Nlink)
}

// printPermissions prints Access/Uid/Gid line.
func printPermissions(fi *sys.FileInfo) {
	octal := fmt.Sprintf("(%04o/%s)", unixMode(fi.Mode), humanPerms(fi.Mode))
	uid := fmt.Sprintf("(%5d/%8s)", fi.Uid, lookupUser(fi.Uid))
	gid := fmt.Sprintf("(%5d/%8s)", fi.Gid, lookupGroup(fi.Gid))
	fmt.Printf("Access: %-17s  Uid: %-14s   Gid: %s\n", octal, uid, gid)
}

// printTimestamps prints Access/Modify/Change/Birth time lines.
func printTimestamps(si statInfo) {
	fmt.Printf("Access: %s\n", formatTimestamp(si.fi.AccessTime))
	fmt.Printf("Modify: %s\n", formatTimestamp(si.fi.ModTime))
	fmt.Printf("Change: %s\n", formatTimestamp(si.fi.ChangeTime))
	printBirthLine(si)
}

// printBirthLine prints the Birth time line.
func printBirthLine(si statInfo) {
	if si.hasBirth {
		fmt.Printf(" Birth: %s\n", formatTimestamp(si.birthTime))
	} else {
		fmt.Printf(" Birth: -\n")
	}
}

// expandFormat processes a -c format string, expanding % directives.
// R2.1: each %X directive is replaced with the corresponding file metadata.
func expandFormat(format, path string, si statInfo) string {
	var buf strings.Builder
	for i := 0; i < len(format); {
		if format[i] != '%' {
			buf.WriteByte(format[i])
			i++
			continue
		}
		i++ // skip %
		if i >= len(format) {
			buf.WriteByte('%')
			break
		}
		if format[i] == '%' {
			buf.WriteByte('%')
			i++
			continue
		}
		buf.WriteString(expandStatDirective(format[i], path, si))
		i++
	}
	return buf.String()
}

// expandPrintf processes a --printf format string with escape sequences.
// R2.2: interprets \n, \t, \0NNN, \xHH and does not append trailing newline.
func expandPrintf(format, path string, si statInfo) string {
	escaped := interpretEscapes(format)
	return expandFormat(escaped, path, si)
}

// interpretEscapes processes backslash escape sequences in a format string.
func interpretEscapes(s string) string {
	var buf strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '\\' {
			buf.WriteByte(s[i])
			i++
			continue
		}
		i++ // skip backslash
		if i >= len(s) {
			buf.WriteByte('\\')
			break
		}
		ch, adv := expandEscape(s, i)
		buf.WriteString(ch)
		i += adv
	}
	return buf.String()
}

// expandEscape returns the expanded escape and how many chars were consumed.
func expandEscape(s string, i int) (string, int) {
	switch s[i] {
	case 'n':
		return "\n", 1
	case 't':
		return "\t", 1
	case '\\':
		return "\\", 1
	case 'a':
		return "\a", 1
	case 'b':
		return "\b", 1
	case 'f':
		return "\f", 1
	case 'r':
		return "\r", 1
	case 'v':
		return "\v", 1
	case '0':
		return expandOctalEscape(s, i)
	case 'x':
		return expandHexEscape(s, i)
	default:
		return "\\" + string(s[i]), 1
	}
}

// expandOctalEscape parses \0NNN octal sequences.
func expandOctalEscape(s string, i int) (string, int) {
	end := i + 1
	for end < len(s) && end < i+4 && s[end] >= '0' && s[end] <= '7' {
		end++
	}
	if end == i+1 {
		return "\x00", 1
	}
	val, _ := strconv.ParseUint(s[i+1:end], 8, 8)
	return string(rune(val)), end - i
}

// expandHexEscape parses \xHH hex sequences.
func expandHexEscape(s string, i int) (string, int) {
	end := i + 1
	for end < len(s) && end < i+3 && isHexDigit(s[end]) {
		end++
	}
	if end == i+1 {
		return "\\x", 1
	}
	val, _ := strconv.ParseUint(s[i+1:end], 16, 8)
	return string(rune(val)), end - i
}

// isHexDigit returns true if c is a valid hexadecimal digit.
func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// expandStatDirective returns the value for a single stat % directive.
// R2.1/R2.2: supports all file-mode directives.
func expandStatDirective(ch byte, path string, si statInfo) string {
	switch ch {
	case 'a':
		return fmt.Sprintf("%o", unixMode(si.fi.Mode))
	case 'A':
		return humanPerms(si.fi.Mode)
	case 'b':
		return strconv.FormatInt(si.fi.Blocks, 10)
	case 'B':
		return "512"
	case 'd':
		return strconv.FormatUint(si.fi.Dev, 10)
	case 'D':
		return fmt.Sprintf("%x", si.fi.Dev)
	case 'f':
		return fmt.Sprintf("%x", rawModeHex(si.fi.Mode))
	case 'F':
		return fileTypeName(si.fi.Mode)
	case 'g':
		return strconv.FormatUint(uint64(si.fi.Gid), 10)
	case 'G':
		return lookupGroup(si.fi.Gid)
	case 'h':
		return strconv.FormatUint(si.fi.Nlink, 10)
	case 'i':
		return strconv.FormatUint(si.fi.Ino, 10)
	case 'm':
		return "?" // mount point not implemented in this task
	case 'n':
		return path
	case 'N':
		return formatNameDirective(path, si.fi)
	case 'o':
		return strconv.FormatInt(si.fi.Blksize, 10)
	case 's':
		return strconv.FormatInt(si.fi.Size, 10)
	case 't':
		return fmt.Sprintf("%x", majorDev(si.fi.Rdev))
	case 'T':
		return fmt.Sprintf("%x", minorDev(si.fi.Rdev))
	case 'u':
		return strconv.FormatUint(uint64(si.fi.Uid), 10)
	case 'U':
		return lookupUser(si.fi.Uid)
	default:
		return expandTimeDirective(ch, si)
	}
}

// expandTimeDirective handles time-related format directives.
func expandTimeDirective(ch byte, si statInfo) string {
	switch ch {
	case 'w':
		return formatBirthHuman(si)
	case 'W':
		return formatBirthEpoch(si)
	case 'x':
		return formatTimestamp(si.fi.AccessTime)
	case 'X':
		return strconv.FormatInt(si.fi.AccessTime.Unix(), 10)
	case 'y':
		return formatTimestamp(si.fi.ModTime)
	case 'Y':
		return strconv.FormatInt(si.fi.ModTime.Unix(), 10)
	case 'z':
		return formatTimestamp(si.fi.ChangeTime)
	case 'Z':
		return strconv.FormatInt(si.fi.ChangeTime.Unix(), 10)
	default:
		return "%" + string(ch)
	}
}

// formatBirthHuman returns birth time in human-readable form, or "-" if unavailable.
func formatBirthHuman(si statInfo) string {
	if si.hasBirth {
		return formatTimestamp(si.birthTime)
	}
	return "-"
}

// formatBirthEpoch returns birth time as epoch seconds, or "0" if unavailable.
func formatBirthEpoch(si statInfo) string {
	if si.hasBirth {
		return strconv.FormatInt(si.birthTime.Unix(), 10)
	}
	return "0"
}

// fileTypeName returns the human-readable file type string.
// R2.1: %F directive.
func fileTypeName(mode os.FileMode) string {
	switch {
	case mode&os.ModeSymlink != 0:
		return "symbolic link"
	case mode.IsDir():
		return "directory"
	case mode&os.ModeNamedPipe != 0:
		return "fifo"
	case mode&os.ModeSocket != 0:
		return "socket"
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		return "character special file"
	case mode&os.ModeDevice != 0:
		return "block special file"
	default:
		return "regular file"
	}
}

// formatNameDirective returns the %N format: quoted name with link target for symlinks.
// R2.1: %N shows quoted name, with ' -> target' for symlinks.
func formatNameDirective(path string, fi *sys.FileInfo) string {
	quoted := "'" + path + "'"
	if fi.Mode&os.ModeSymlink == 0 {
		return quoted
	}
	target, err := os.Readlink(path)
	if err != nil {
		return quoted
	}
	return quoted + " -> '" + target + "'"
}

// unixMode extracts the traditional Unix permission bits from Go's FileMode.
func unixMode(mode os.FileMode) uint32 {
	perm := uint32(mode.Perm())
	if mode&os.ModeSetuid != 0 {
		perm |= 0o4000
	}
	if mode&os.ModeSetgid != 0 {
		perm |= 0o2000
	}
	if mode&os.ModeSticky != 0 {
		perm |= 0o1000
	}
	return perm
}

// rawModeHex returns the raw stat mode including file type bits for %f.
func rawModeHex(mode os.FileMode) uint32 {
	raw := unixMode(mode)
	raw |= fileTypeBits(mode)
	return raw
}

// fileTypeBits returns the Unix file type bits (S_IFMT) for Go's FileMode.
func fileTypeBits(mode os.FileMode) uint32 {
	switch {
	case mode.IsRegular():
		return 0o100000
	case mode.IsDir():
		return 0o040000
	case mode&os.ModeSymlink != 0:
		return 0o120000
	case mode&os.ModeNamedPipe != 0:
		return 0o010000
	case mode&os.ModeSocket != 0:
		return 0o140000
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		return 0o020000
	case mode&os.ModeDevice != 0:
		return 0o060000
	default:
		return 0o100000
	}
}

// humanPerms returns the ls-style permission string (e.g., "-rwxr-xr-x").
func humanPerms(mode os.FileMode) string {
	var buf [10]byte
	buf[0] = fileTypeChar(mode)
	fillPermTriple(buf[1:4], mode, 6, os.ModeSetuid, 's', 'S')
	fillPermTriple(buf[4:7], mode, 3, os.ModeSetgid, 's', 'S')
	fillPermTriple(buf[7:10], mode, 0, os.ModeSticky, 't', 'T')
	return string(buf[:])
}

// fileTypeChar returns the single character for the file type field.
func fileTypeChar(mode os.FileMode) byte {
	switch {
	case mode.IsRegular():
		return '-'
	case mode.IsDir():
		return 'd'
	case mode&os.ModeSymlink != 0:
		return 'l'
	case mode&os.ModeNamedPipe != 0:
		return 'p'
	case mode&os.ModeSocket != 0:
		return 's'
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		return 'c'
	case mode&os.ModeDevice != 0:
		return 'b'
	default:
		return '-'
	}
}

// fillPermTriple fills a 3-byte rwx permission group with special bit handling.
func fillPermTriple(buf []byte, mode os.FileMode, shift uint, special os.FileMode, lower, upper byte) {
	perm := mode.Perm()
	if perm&(1<<(shift+2)) != 0 {
		buf[0] = 'r'
	} else {
		buf[0] = '-'
	}
	if perm&(1<<(shift+1)) != 0 {
		buf[1] = 'w'
	} else {
		buf[1] = '-'
	}
	hasExec := perm&(1<<shift) != 0
	hasSpecial := mode&special != 0
	buf[2] = execSpecialChar(hasExec, hasSpecial, lower, upper)
}

// execSpecialChar returns the character for the execute position.
func execSpecialChar(hasExec, hasSpecial bool, lower, upper byte) byte {
	switch {
	case hasExec && hasSpecial:
		return lower
	case !hasExec && hasSpecial:
		return upper
	case hasExec:
		return 'x'
	default:
		return '-'
	}
}

// formatTimestamp formats a time in GNU stat's default timestamp format.
func formatTimestamp(t time.Time) string {
	return t.Format("2006-01-02 15:04:05.000000000 -0700")
}

// majorDev extracts the major device number (macOS layout).
func majorDev(dev uint64) uint32 {
	return uint32((dev >> 24) & 0xff)
}

// minorDev extracts the minor device number (macOS layout).
func minorDev(dev uint64) uint32 {
	return uint32(dev & 0xffffff)
}

// lookupUser returns the username for a UID, or the UID string if not found.
func lookupUser(uid uint32) string {
	u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
	if err != nil {
		return strconv.FormatUint(uint64(uid), 10)
	}
	return u.Username
}

// lookupGroup returns the group name for a GID, or the GID string if not found.
func lookupGroup(gid uint32) string {
	g, err := user.LookupGroupId(strconv.FormatUint(uint64(gid), 10))
	if err != nil {
		return strconv.FormatUint(uint64(gid), 10)
	}
	return g.Name
}

// parseArgs processes all command-line arguments into a config.
func parseArgs(args []string) (config, error) {
	var cfg config
	for i := 0; i < len(args); {
		if args[i] == "--" {
			cfg.args = append(cfg.args, args[i+1:]...)
			break
		}
		adv, err := parseArg(&cfg, args, i)
		if err != nil {
			return cfg, err
		}
		i += adv
	}
	return cfg, nil
}

// parseArg processes one argument, returning how many args were consumed.
func parseArg(cfg *config, args []string, i int) (int, error) {
	arg := args[i]
	switch {
	case arg == "--help":
		cfg.showHelp = true
		return 1, nil
	case arg == "--version":
		cfg.showVersion = true
		return 1, nil
	case arg == "--dereference":
		cfg.dereference = true
		return 1, nil
	case arg == "--terse":
		cfg.terse = true
		return 1, nil
	case strings.HasPrefix(arg, "--format="):
		cfg.mode = modeFormat
		cfg.format = arg[len("--format="):]
		return 1, nil
	case arg == "--format":
		return consumeFormatArg(cfg, modeFormat, args, i, arg)
	case strings.HasPrefix(arg, "--printf="):
		cfg.mode = modePrintf
		cfg.format = arg[len("--printf="):]
		return 1, nil
	case arg == "--printf":
		return consumeFormatArg(cfg, modePrintf, args, i, arg)
	case strings.HasPrefix(arg, "--"):
		return 0, fmt.Errorf("unrecognized option '%s'", arg)
	case strings.HasPrefix(arg, "-") && len(arg) > 1:
		return parseShortFlags(cfg, args, i)
	default:
		cfg.args = append(cfg.args, arg)
		return 1, nil
	}
}

// consumeFormatArg reads the next argument as a format string.
func consumeFormatArg(cfg *config, mode outputMode, args []string, i int, opt string) (int, error) {
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option '%s' requires an argument", opt)
	}
	cfg.mode = mode
	cfg.format = args[i+1]
	return 2, nil
}

// parseShortFlags processes combined short flags.
func parseShortFlags(cfg *config, args []string, i int) (int, error) {
	flags := args[i][1:]
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'L':
			cfg.dereference = true
		case 't':
			cfg.terse = true
		case 'c':
			return consumeShortFormat(cfg, modeFormat, flags[j+1:], flags[j], args, i)
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return 1, nil
}

// consumeShortFormat handles -c FORMAT.
func consumeShortFormat(cfg *config, mode outputMode, rest string, ch byte, args []string, i int) (int, error) {
	cfg.mode = mode
	if rest != "" {
		cfg.format = rest
		return 1, nil
	}
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option requires an argument -- '%c'", ch)
	}
	cfg.format = args[i+1]
	return 2, nil
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp() {
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
}

const helpText = `Usage: stat [OPTION]... FILE...
Display file or file system status.

  -L, --dereference     follow links
  -c  --format=FORMAT   use the specified FORMAT instead of the default;
                         output a newline after each use of FORMAT
      --printf=FORMAT   like --format, but interpret backslash escapes,
                         and do not output a mandatory trailing newline
  -t, --terse           print the information in terse form
      --help            display this help and exit
      --version         output version information and exit

Valid format sequences for files:
  %a   access rights in octal
  %A   access rights in human readable form
  %b   number of blocks allocated
  %B   the size in bytes of each block reported by %b
  %d   device number in decimal
  %D   device number in hex
  %f   raw mode in hex
  %F   file type
  %g   group ID of owner
  %G   group name of owner
  %h   number of hard links
  %i   inode number
  %n   file name
  %N   quoted file name with dereference if symbolic link
  %o   optimal I/O transfer size hint
  %s   total size, in bytes
  %t   major device type in hex, for character/block device special files
  %T   minor device type in hex, for character/block device special files
  %u   user ID of owner
  %U   user name of owner
  %w   time of file birth, human-readable; - if unknown
  %W   time of file birth, seconds since Epoch; 0 if unknown
  %x   time of last access, human-readable
  %X   time of last access, seconds since Epoch
  %y   time of last data modification, human-readable
  %Y   time of last data modification, seconds since Epoch
  %z   time of last status change, human-readable
  %Z   time of last status change, seconds since Epoch
`

// printHelp writes usage information to stdout and returns the exit code.
func printHelp() int {
	_, err := os.Stdout.WriteString(helpText)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns the exit code.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout, "%s (go-unix-utils) %s\n", progName, version)
	if err != nil {
		return 1
	}
	return 0
}
