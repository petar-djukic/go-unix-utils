// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/stty implements GNU stty: change and print terminal line settings.
//
// Implements prd105-stty R1.1, R2.1, R3.1, R3.2, R4.1, R5.1, R6.1, R6.2.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "stty"

// defaultWrapCol is the wrap width when stdout is not a terminal.
const defaultWrapCol = 80

// posixVDisable is the value indicating a disabled special character.
const posixVDisable = 0xff

// version is set at build time via -ldflags.
var version = "dev"

// displayMode selects the output format.
type displayMode int

const (
	modeDefault displayMode = iota
	modeAll
	modeSave
	modeHelp
	modeVersion
)

// saneState describes a flag's value in "sane" mode.
type saneState int

const (
	saneNone  saneState = iota // no sane default; not shown in default display
	saneSet                    // sane enables this flag
	saneUnset                  // sane disables this flag
)

// modeFlag describes a single terminal mode flag.
type modeFlag struct {
	name string
	bits uint64
	sane saneState
}

// ccEntry describes a special character slot.
type ccEntry struct {
	name    string
	index   uint8
	saneVal uint8
	isNum   bool // min and time are displayed as decimal numbers
}

// config holds parsed command-line options.
type config struct {
	mode   displayMode
	device string
	ops    []settingOp
}

// Control flags in GNU stty display order (macOS).
// R2.1: CSIZE is inserted between parodd and hupcl in display code.
var controlFlags = []modeFlag{
	{"parenb", unix.PARENB, saneNone},
	{"parodd", unix.PARODD, saneNone},
	{"hupcl", unix.HUPCL, saneUnset},
	{"cstopb", unix.CSTOPB, saneNone},
	{"cread", unix.CREAD, saneSet},
	{"clocal", unix.CLOCAL, saneNone},
	{"crtscts", unix.CRTSCTS, saneNone},
}

// Input flags in GNU stty display order (macOS).
var inputFlags = []modeFlag{
	{"ignbrk", unix.IGNBRK, saneUnset},
	{"brkint", unix.BRKINT, saneSet},
	{"ignpar", unix.IGNPAR, saneNone},
	{"parmrk", unix.PARMRK, saneNone},
	{"inpck", unix.INPCK, saneNone},
	{"istrip", unix.ISTRIP, saneUnset},
	{"inlcr", unix.INLCR, saneUnset},
	{"igncr", unix.IGNCR, saneUnset},
	{"icrnl", unix.ICRNL, saneSet},
	{"ixon", unix.IXON, saneNone},
	{"ixoff", unix.IXOFF, saneUnset},
	{"ixany", unix.IXANY, saneNone},
	{"imaxbel", unix.IMAXBEL, saneSet},
	{"iutf8", unix.IUTF8, saneSet},
}

// Output flags in GNU stty display order (macOS).
var outputFlags = []modeFlag{
	{"opost", unix.OPOST, saneSet},
	{"ocrnl", unix.OCRNL, saneUnset},
	{"onlcr", unix.ONLCR, saneSet},
	{"onocr", unix.ONOCR, saneUnset},
	{"onlret", unix.ONLRET, saneUnset},
	{"oxtabs", unix.OXTABS, saneUnset},
	{"onoeot", unix.ONOEOT, saneUnset},
}

// Local flags in GNU stty display order (macOS).
var localFlags = []modeFlag{
	{"isig", unix.ISIG, saneSet},
	{"icanon", unix.ICANON, saneSet},
	{"iexten", unix.IEXTEN, saneSet},
	{"echo", unix.ECHO, saneSet},
	{"echoe", unix.ECHOE, saneSet},
	{"echok", unix.ECHOK, saneSet},
	{"echonl", unix.ECHONL, saneUnset},
	{"noflsh", unix.NOFLSH, saneUnset},
	{"tostop", unix.TOSTOP, saneUnset},
	{"echoprt", unix.ECHOPRT, saneUnset},
	{"echoctl", unix.ECHOCTL, saneSet},
	{"echoke", unix.ECHOKE, saneSet},
	{"altwerase", unix.ALTWERASE, saneUnset},
	{"flusho", unix.FLUSHO, saneUnset},
	{"pendin", unix.PENDIN, saneUnset},
	{"nokerninfo", unix.NOKERNINFO, saneUnset},
	{"extproc", unix.EXTPROC, saneUnset},
}

// specialChars lists special character slots in GNU stty display order (macOS).
// R2.1: displayed in -a mode; R1.1: changed-from-sane shown in default mode.
var specialChars = []ccEntry{
	{"intr", unix.VINTR, 0x03, false},
	{"quit", unix.VQUIT, 0x1c, false},
	{"erase", unix.VERASE, 0x7f, false},
	{"kill", unix.VKILL, 0x15, false},
	{"eof", unix.VEOF, 0x04, false},
	{"eol", unix.VEOL, posixVDisable, false},
	{"eol2", unix.VEOL2, posixVDisable, false},
	{"start", unix.VSTART, 0x11, false},
	{"stop", unix.VSTOP, 0x13, false},
	{"susp", unix.VSUSP, 0x1a, false},
	{"dsusp", unix.VDSUSP, 0x19, false},
	{"rprnt", unix.VREPRINT, 0x12, false},
	{"werase", unix.VWERASE, 0x17, false},
	{"lnext", unix.VLNEXT, 0x16, false},
	{"discard", unix.VDISCARD, 0x0f, false},
	{"status", unix.VSTATUS, 0x14, false},
	{"min", unix.VMIN, 1, true},
	{"time", unix.VTIME, 0, true},
}

// R7.3: install SIGPIPE handler for graceful pipe termination.
func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run implements the main logic. Returns exit code.
// R1.1: default display. R2.1: -a display. R3.1: -g display. R3.2: -F device.
func run(args []string, stdout, stderr *os.File) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)             //nolint:errcheck
		fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck
		return 1
	}
	if cfg.mode == modeHelp {
		printHelp(stdout)
		return 0
	}
	if cfg.mode == modeVersion {
		printVersion(stdout)
		return 0
	}
	if len(cfg.ops) > 0 {
		return applySettings(cfg, stderr)
	}
	return runDisplay(cfg, stdout, stderr)
}

// runDisplay opens the terminal and prints settings. Returns exit code.
func runDisplay(cfg *config, stdout, stderr *os.File) int {
	fd, cleanup, source, err := openTermFD(cfg.device)
	if err != nil {
		printQuotedErr(stderr, source, err)
		return 1
	}
	defer cleanup()

	termios, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		printQuotedErr(stderr, source, err)
		return 1
	}
	switch cfg.mode {
	case modeSave:
		displaySave(stdout, termios)
	case modeAll:
		ws, _ := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ) // best-effort
		displayAll(stdout, termios, ws)
	default:
		displayDefault(stdout, termios)
	}
	return 0
}

// parseArgs extracts display mode and device from command-line arguments.
func parseArgs(args []string) (*config, error) {
	cfg := &config{mode: modeDefault}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-a" || arg == "--all":
			if cfg.mode == modeSave {
				return nil, fmt.Errorf("only one of -a and -g may be used")
			}
			cfg.mode = modeAll
		case arg == "-g" || arg == "--save":
			if cfg.mode == modeAll {
				return nil, fmt.Errorf("only one of -a and -g may be used")
			}
			cfg.mode = modeSave
		case arg == "-F":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("option requires an argument -- 'F'")
			}
			i++
			cfg.device = args[i]
		case strings.HasPrefix(arg, "--file="):
			cfg.device = strings.TrimPrefix(arg, "--file=")
		case arg == "--help":
			cfg.mode = modeHelp
		case arg == "--version":
			cfg.mode = modeVersion
		default:
			consumed, err := parseSetting(cfg, args[i:])
			if err != nil {
				return nil, err
			}
			i += consumed - 1
		}
	}
	if len(cfg.ops) > 0 && (cfg.mode == modeAll || cfg.mode == modeSave) {
		return nil, fmt.Errorf("when specifying an output style, modes may not be set")
	}
	return cfg, nil
}

// openTermFD returns the file descriptor, cleanup, source name, and error.
// R3.2: opens the specified device instead of stdin.
func openTermFD(device string) (int, func(), string, error) {
	if device == "" {
		return int(os.Stdin.Fd()), func() {}, "standard input", nil
	}
	f, err := os.OpenFile(device, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		if pe, ok := err.(*os.PathError); ok {
			return 0, func() {}, device, pe.Err
		}
		return 0, func() {}, device, err
	}
	return int(f.Fd()), func() { f.Close() }, device, nil //nolint:errcheck
}

// displayDefault prints only settings that differ from "sane" defaults.
// R1.1: speed line followed by changed cc values and mode flags.
func displayDefault(w io.Writer, t *unix.Termios) {
	wr := newWrapper(w)
	writeSpeedLine(wr, t, nil, false)
	wr.newline()
	ccWrote := writeChangedCC(wr, t)
	flagWrote := writeChangedFlags(wr, t)
	if ccWrote || flagWrote {
		wr.newline()
	}
}

// displayAll prints all terminal settings in human-readable format.
// R2.1: speed, rows, columns, special characters, and all mode flags.
func displayAll(w io.Writer, t *unix.Termios, ws *unix.Winsize) {
	wr := newWrapper(w)
	writeSpeedLine(wr, t, ws, true)
	wr.newline()
	writeAllCC(wr, t)
	wr.newline()
	writeControlFlagsAll(wr, t.Cflag)
	wr.newline()
	writeGroupFlags(wr, inputFlags, t.Iflag)
	wr.newline()
	writeGroupFlags(wr, outputFlags, t.Oflag)
	wr.newline()
	writeGroupFlags(wr, localFlags, t.Lflag)
	wr.newline()
}

// displaySave prints all settings in machine-readable hex format.
// R3.1: colon-separated hex values of iflag:oflag:cflag:lflag:cc[0..19]:ispeed:ospeed.
func displaySave(w io.Writer, t *unix.Termios) {
	fmt.Fprintf(w, "%x:%x:%x:%x", t.Iflag, t.Oflag, t.Cflag, t.Lflag) //nolint:errcheck
	for _, v := range t.Cc {
		fmt.Fprintf(w, ":%x", v) //nolint:errcheck
	}
	fmt.Fprintf(w, ":%x:%x\n", t.Ispeed, t.Ospeed) //nolint:errcheck
}

// writeSpeedLine prints the speed line, with optional rows and columns.
func writeSpeedLine(wr *wrapper, t *unix.Termios, ws *unix.Winsize, showSize bool) {
	if t.Ispeed == t.Ospeed {
		wr.write(fmt.Sprintf("speed %d baud;", t.Ospeed))
	} else {
		wr.write(fmt.Sprintf("ispeed %d baud;", t.Ispeed))
		wr.write(fmt.Sprintf("ospeed %d baud;", t.Ospeed))
	}
	if showSize && ws != nil {
		wr.write(fmt.Sprintf("rows %d;", ws.Row))
		wr.write(fmt.Sprintf("columns %d;", ws.Col))
	}
}

// writeAllCC prints all special character values for -a display.
func writeAllCC(wr *wrapper, t *unix.Termios) {
	for _, cc := range specialChars {
		wr.write(fmt.Sprintf("%s = %s;", cc.name, formatCC(t.Cc[cc.index], cc.isNum)))
	}
}

// writeControlFlagsAll prints all control flags with CSIZE for -a display.
func writeControlFlagsAll(wr *wrapper, cflag uint64) {
	writeSingleFlag(wr, "parenb", cflag, unix.PARENB)
	writeSingleFlag(wr, "parodd", cflag, unix.PARODD)
	wr.write(csizeStr(cflag))
	for _, f := range controlFlags[2:] {
		writeSingleFlag(wr, f.name, cflag, f.bits)
	}
}

// writeGroupFlags prints all flags in a group for -a display.
func writeGroupFlags(wr *wrapper, flags []modeFlag, bitfield uint64) {
	for _, f := range flags {
		writeSingleFlag(wr, f.name, bitfield, f.bits)
	}
}

// writeSingleFlag writes a flag as "name" (set) or "-name" (not set).
func writeSingleFlag(wr *wrapper, name string, bitfield, bits uint64) {
	if bitfield&bits != 0 {
		wr.write(name)
	} else {
		wr.write("-" + name)
	}
}

// writeChangedCC prints cc values that differ from sane defaults.
func writeChangedCC(wr *wrapper, t *unix.Termios) bool {
	wrote := false
	for _, cc := range specialChars {
		if cc.isNum && t.Lflag&unix.ICANON != 0 {
			continue // min/time not shown when icanon is set
		}
		if t.Cc[cc.index] != cc.saneVal {
			wr.write(fmt.Sprintf("%s = %s;", cc.name, formatCC(t.Cc[cc.index], cc.isNum)))
			wrote = true
		}
	}
	return wrote
}

// writeChangedFlags prints mode flags that differ from sane defaults.
func writeChangedFlags(wr *wrapper, t *unix.Termios) bool {
	wrote := false
	// Control flags with CSIZE special case
	for i, f := range controlFlags {
		if i == 2 && t.Cflag&unix.CSIZE != unix.CS8 {
			wr.write(csizeStr(t.Cflag))
			wrote = true
		}
		wrote = writeIfChanged(wr, f.name, t.Cflag, f.bits, f.sane) || wrote
	}
	wrote = checkChangedGroup(wr, inputFlags, t.Iflag) || wrote
	wrote = checkChangedGroup(wr, outputFlags, t.Oflag) || wrote
	wrote = checkChangedGroup(wr, localFlags, t.Lflag) || wrote
	return wrote
}

// checkChangedGroup writes flags in a group that differ from sane.
func checkChangedGroup(wr *wrapper, flags []modeFlag, bitfield uint64) bool {
	wrote := false
	for _, f := range flags {
		wrote = writeIfChanged(wr, f.name, bitfield, f.bits, f.sane) || wrote
	}
	return wrote
}

// writeIfChanged writes a flag if it differs from its sane default.
func writeIfChanged(wr *wrapper, name string, bitfield, bits uint64, sane saneState) bool {
	isSet := bitfield&bits != 0
	if sane == saneSet && !isSet {
		wr.write("-" + name)
		return true
	}
	if sane == saneUnset && isSet {
		wr.write(name)
		return true
	}
	return false
}

// wrapper handles space-separated output with line wrapping.
type wrapper struct {
	w      io.Writer
	col    int
	maxCol int
}

// newWrapper creates a wrapper with appropriate wrap width.
func newWrapper(w io.Writer) *wrapper {
	col := defaultWrapCol
	if ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ); err == nil && ws.Col > 0 {
		col = int(ws.Col)
	}
	return &wrapper{w: w, maxCol: col}
}

// write adds an entry to the current line with space separation and wrapping.
func (wr *wrapper) write(s string) {
	extra := 0
	if wr.col > 0 {
		extra = 1
	}
	if wr.col+extra+len(s) > wr.maxCol {
		fmt.Fprint(wr.w, "\n") //nolint:errcheck
		wr.col = 0
		extra = 0
	}
	if extra > 0 {
		fmt.Fprint(wr.w, " ") //nolint:errcheck
		wr.col++
	}
	fmt.Fprint(wr.w, s) //nolint:errcheck
	wr.col += len(s)
}

// newline forces a line break and resets the column counter.
func (wr *wrapper) newline() {
	if wr.col > 0 {
		fmt.Fprint(wr.w, "\n") //nolint:errcheck
	}
	wr.col = 0
}

// formatCC formats a cc value for display.
func formatCC(val byte, isNum bool) string {
	if isNum {
		return fmt.Sprintf("%d", val)
	}
	if val == posixVDisable {
		return "<undef>"
	}
	ch := val
	prefix := ""
	if ch >= 128 {
		prefix = "M-"
		ch -= 128
	}
	if ch < 32 {
		return fmt.Sprintf("%s^%c", prefix, ch+64)
	}
	if ch == 127 {
		return prefix + "^?"
	}
	return fmt.Sprintf("%s%c", prefix, ch)
}

// csizeStr returns the character size string for the given cflag.
func csizeStr(cflag uint64) string {
	switch cflag & unix.CSIZE {
	case unix.CS5:
		return "cs5"
	case unix.CS6:
		return "cs6"
	case unix.CS7:
		return "cs7"
	default:
		return "cs8"
	}
}

// capitalizeFirst returns s with the first byte uppercased.
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// printQuotedErr prints an error with GNU-style quoting.
func printQuotedErr(w io.Writer, source string, err error) {
	msg := capitalizeFirst(err.Error())
	fmt.Fprintf(w, "%s: '%s': %s\n", progName, source, msg) //nolint:errcheck
}

// printHelp writes usage information.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [-F DEVICE | --file=DEVICE] [SETTING]...\n", progName) //nolint:errcheck
	fmt.Fprintf(w, "  or:  %s [-F DEVICE | --file=DEVICE] [-a|--all]\n", progName)   //nolint:errcheck
	fmt.Fprintf(w, "  or:  %s [-F DEVICE | --file=DEVICE] [-g|--save]\n", progName)  //nolint:errcheck
	fmt.Fprintln(w, "Print or change terminal line settings.")                        //nolint:errcheck
	fmt.Fprintln(w)                                                                   //nolint:errcheck
	fmt.Fprintln(w, "  -a, --all      print all current settings in human-readable form") //nolint:errcheck
	fmt.Fprintln(w, "  -g, --save     print all current settings in a stty-readable form") //nolint:errcheck
	fmt.Fprintln(w, "  -F, --file=DEVICE  open and use the specified DEVICE instead of stdin") //nolint:errcheck
	fmt.Fprintln(w, "      --help     display this help and exit")                    //nolint:errcheck
	fmt.Fprintln(w, "      --version  output version information and exit")           //nolint:errcheck
}

// printVersion writes version information.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils) %s\n", progName, version) //nolint:errcheck
}
