// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/stty: Change and print terminal line settings.
// Implements srd105-stty R1.1, R2.1, R3.1, R3.2, R4.1, R5.1, R6.1, R6.2, R7.1, R7.2, R7.3.
package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode"

	"golang.org/x/sys/unix"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// wrapCol is the column width for line wrapping in display output.
const wrapCol = 80

// specialChar represents a terminal special character for display.
type specialChar struct {
	Name  string
	Index int
	IsNum bool // min, time use numeric display
}

// displayEntry represents a single flag or multi-value entry for display.
type displayEntry struct {
	Name   string       // simple flag name
	Mask   uint64       // flag bitmask
	DefOn  bool         // true if flag is ON in default/sane state
	Values []multiValue // non-nil for multi-value flags (e.g., cs5-cs8)
	DefVal uint64       // default value for multi-value flags
}

// multiValue represents one named option in a multi-value flag set.
type multiValue struct {
	Name  string
	Value uint64
}

// Darwin output flag constants not always exported by x/sys/unix.
const (
	darwinOCRNL  uint64 = 0x00000010
	darwinONOCR  uint64 = 0x00000020
	darwinONLRET uint64 = 0x00000040
	darwinOFILL  uint64 = 0x00000080
	darwinOFDEL  uint64 = 0x00020000
	darwinNLDLY  uint64 = 0x00000300
	darwinNL1    uint64 = 0x00000100
	darwinCRDLY  uint64 = 0x00003000
	darwinCR1    uint64 = 0x00001000
	darwinCR2    uint64 = 0x00002000
	darwinCR3    uint64 = 0x00003000
	darwinTABDLY uint64 = 0x00000c04
	darwinTAB1   uint64 = 0x00000400
	darwinTAB2   uint64 = 0x00000800
	darwinTAB3   uint64 = 0x00000004 // OXTABS
	darwinBSDLY  uint64 = 0x00008000
	darwinBS1    uint64 = 0x00008000
	darwinVTDLY  uint64 = 0x00010000
	darwinVT1    uint64 = 0x00010000
	darwinFFDLY  uint64 = 0x00004000
	darwinFF1    uint64 = 0x00004000
	darwinIUTF8  uint64 = 0x00004000 // IUTF8 input flag on Darwin
)

// specialCharDisplay lists special characters in GNU stty display order.
var specialCharDisplay = []specialChar{
	{"intr", unix.VINTR, false},
	{"quit", unix.VQUIT, false},
	{"erase", unix.VERASE, false},
	{"kill", unix.VKILL, false},
	{"eof", unix.VEOF, false},
	{"eol", unix.VEOL, false},
	{"eol2", unix.VEOL2, false},
	{"start", unix.VSTART, false},
	{"stop", unix.VSTOP, false},
	{"susp", unix.VSUSP, false},
	{"dsusp", unix.VDSUSP, false},
	{"rprnt", unix.VREPRINT, false},
	{"werase", unix.VWERASE, false},
	{"lnext", unix.VLNEXT, false},
	{"discard", unix.VDISCARD, false},
	{"status", unix.VSTATUS, false},
	{"min", unix.VMIN, true},
	{"time", unix.VTIME, true},
}

// controlFlags defines control mode flag display entries in GNU stty order.
var controlFlags = []displayEntry{
	{Name: "parenb", Mask: unix.PARENB},
	{Name: "parodd", Mask: unix.PARODD},
	{Mask: unix.CSIZE, DefVal: uint64(unix.CS8), Values: []multiValue{
		{"cs5", uint64(unix.CS5)}, {"cs6", uint64(unix.CS6)},
		{"cs7", uint64(unix.CS7)}, {"cs8", uint64(unix.CS8)},
	}},
	{Name: "hupcl", Mask: unix.HUPCL, DefOn: true},
	{Name: "cstopb", Mask: unix.CSTOPB},
	{Name: "cread", Mask: unix.CREAD, DefOn: true},
	{Name: "clocal", Mask: unix.CLOCAL},
	{Name: "crtscts", Mask: unix.CRTSCTS},
}

// inputFlags defines input mode flag display entries in GNU stty order.
var inputFlags = []displayEntry{
	{Name: "ignbrk", Mask: unix.IGNBRK},
	{Name: "brkint", Mask: unix.BRKINT, DefOn: true},
	{Name: "ignpar", Mask: unix.IGNPAR},
	{Name: "parmrk", Mask: unix.PARMRK},
	{Name: "inpck", Mask: unix.INPCK},
	{Name: "istrip", Mask: unix.ISTRIP},
	{Name: "inlcr", Mask: unix.INLCR},
	{Name: "igncr", Mask: unix.IGNCR},
	{Name: "icrnl", Mask: unix.ICRNL, DefOn: true},
	{Name: "ixon", Mask: unix.IXON, DefOn: true},
	{Name: "ixoff", Mask: unix.IXOFF},
	{Name: "ixany", Mask: unix.IXANY},
	{Name: "imaxbel", Mask: unix.IMAXBEL, DefOn: true},
	{Name: "iutf8", Mask: darwinIUTF8},
}

// outputFlags defines output mode flag display entries in GNU stty order.
var outputFlags = []displayEntry{
	{Name: "opost", Mask: unix.OPOST, DefOn: true},
	{Name: "ocrnl", Mask: darwinOCRNL},
	{Name: "onlcr", Mask: unix.ONLCR, DefOn: true},
	{Name: "onocr", Mask: darwinONOCR},
	{Name: "onlret", Mask: darwinONLRET},
	{Name: "ofill", Mask: darwinOFILL},
	{Name: "ofdel", Mask: darwinOFDEL},
	{Mask: darwinNLDLY, Values: []multiValue{{"nl0", 0}, {"nl1", darwinNL1}}},
	{Mask: darwinCRDLY, Values: []multiValue{
		{"cr0", 0}, {"cr1", darwinCR1}, {"cr2", darwinCR2}, {"cr3", darwinCR3},
	}},
	{Mask: darwinTABDLY, Values: []multiValue{
		{"tab0", 0}, {"tab1", darwinTAB1}, {"tab2", darwinTAB2}, {"tab3", darwinTAB3},
	}},
	{Mask: darwinBSDLY, Values: []multiValue{{"bs0", 0}, {"bs1", darwinBS1}}},
	{Mask: darwinVTDLY, Values: []multiValue{{"vt0", 0}, {"vt1", darwinVT1}}},
	{Mask: darwinFFDLY, Values: []multiValue{{"ff0", 0}, {"ff1", darwinFF1}}},
}

// localFlags defines local mode flag display entries in GNU stty order.
var localFlags = []displayEntry{
	{Name: "isig", Mask: unix.ISIG, DefOn: true},
	{Name: "icanon", Mask: unix.ICANON, DefOn: true},
	{Name: "iexten", Mask: unix.IEXTEN, DefOn: true},
	{Name: "echo", Mask: unix.ECHO, DefOn: true},
	{Name: "echoe", Mask: unix.ECHOE, DefOn: true},
	{Name: "echok", Mask: unix.ECHOK, DefOn: true},
	{Name: "echonl", Mask: unix.ECHONL},
	{Name: "noflsh", Mask: unix.NOFLSH},
	{Name: "tostop", Mask: unix.TOSTOP},
	{Name: "echoprt", Mask: unix.ECHOPRT},
	{Name: "echoctl", Mask: unix.ECHOCTL, DefOn: true},
	{Name: "echoke", Mask: unix.ECHOKE, DefOn: true},
	{Name: "flusho", Mask: unix.FLUSHO},
	{Name: "extproc", Mask: unix.EXTPROC},
}

// formatChar formats a terminal control character value for display.
func formatChar(c uint8) string {
	switch {
	case c == 0xFF:
		return "<undef>"
	case c < 0x20:
		return fmt.Sprintf("^%c", c+0x40)
	case c == 0x7F:
		return "^?"
	default:
		return string(rune(c))
	}
}

// renderEntry returns the display string for a flag entry given current flags.
func renderEntry(e displayEntry, flags uint64) string {
	if len(e.Values) > 0 {
		val := flags & e.Mask
		for _, v := range e.Values {
			if v.Value == val {
				return v.Name
			}
		}
		return ""
	}
	if flags&e.Mask != 0 {
		return e.Name
	}
	return "-" + e.Name
}

// isChanged reports whether a flag entry differs from its default value.
func isChanged(e displayEntry, flags uint64) bool {
	if len(e.Values) > 0 {
		return (flags & e.Mask) != e.DefVal
	}
	return (flags&e.Mask != 0) != e.DefOn
}

// wrapWriter accumulates display output with line wrapping at wrapCol.
type wrapWriter struct {
	buf strings.Builder
	col int
}

// add appends an entry with space-separation and automatic line wrapping.
func (w *wrapWriter) add(entry string) {
	if w.col > 0 && w.col+1+len(entry) > wrapCol {
		w.buf.WriteByte('\n')
		w.col = 0
	} else if w.col > 0 {
		w.buf.WriteByte(' ')
		w.col++
	}
	w.buf.WriteString(entry)
	w.col += len(entry)
}

// newLine forces a line break if there is content on the current line.
func (w *wrapWriter) newLine() {
	if w.col > 0 {
		w.buf.WriteByte('\n')
		w.col = 0
	}
}

// getWinSize returns the terminal rows and columns for a file descriptor.
// Returns (0, 0) if the ioctl fails (best-effort).
func getWinSize(fd int) (int, int) {
	ws, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
	if err != nil {
		return 0, 0
	}
	return int(ws.Row), int(ws.Col)
}

// writeSpeedLine writes the speed line with optional window dimensions.
func writeSpeedLine(w *wrapWriter, t *unix.Termios, fd int, all bool) {
	if t.Ispeed == t.Ospeed {
		fmt.Fprintf(&w.buf, "speed %d baud;", t.Ispeed)
	} else {
		fmt.Fprintf(&w.buf, "ispeed %d baud; ospeed %d baud;", t.Ispeed, t.Ospeed)
	}
	if all {
		rows, cols := getWinSize(fd)
		fmt.Fprintf(&w.buf, " rows %d; columns %d;", rows, cols)
	}
	w.buf.WriteByte('\n')
}

// writeSpecialChars writes all special character settings with wrapping.
func writeSpecialChars(w *wrapWriter, cc [20]uint8) {
	for _, sc := range specialCharDisplay {
		var entry string
		if sc.IsNum {
			entry = fmt.Sprintf("%s = %d;", sc.Name, cc[sc.Index])
		} else {
			entry = fmt.Sprintf("%s = %s;", sc.Name, formatChar(cc[sc.Index]))
		}
		w.add(entry)
	}
	w.newLine()
}

// writeFlagGroupAll writes all flags in a group for -a display mode.
func writeFlagGroupAll(w *wrapWriter, entries []displayEntry, flags uint64) {
	for _, e := range entries {
		if name := renderEntry(e, flags); name != "" {
			w.add(name)
		}
	}
	w.newLine()
}

// writeFlagGroupChanged writes only flags that differ from defaults.
func writeFlagGroupChanged(w *wrapWriter, entries []displayEntry, flags uint64) {
	for _, e := range entries {
		if isChanged(e, flags) {
			if name := renderEntry(e, flags); name != "" {
				w.add(name)
			}
		}
	}
	w.newLine()
}

// R1.1: Print summary of current terminal settings matching GNU stty format.
func printDefaultDisplay(fd int) error {
	t, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return err
	}
	var w wrapWriter
	writeSpeedLine(&w, t, fd, false)
	writeFlagGroupChanged(&w, controlFlags, t.Cflag)
	writeFlagGroupChanged(&w, inputFlags, t.Iflag)
	writeFlagGroupChanged(&w, outputFlags, t.Oflag)
	writeFlagGroupChanged(&w, localFlags, t.Lflag)
	fmt.Print(w.buf.String())
	return nil
}

// R2.1: Print all current terminal settings in human-readable format.
func printAllSettings(fd int) error {
	t, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return err
	}
	var w wrapWriter
	writeSpeedLine(&w, t, fd, true)
	writeSpecialChars(&w, t.Cc)
	writeFlagGroupAll(&w, controlFlags, t.Cflag)
	writeFlagGroupAll(&w, inputFlags, t.Iflag)
	writeFlagGroupAll(&w, outputFlags, t.Oflag)
	writeFlagGroupAll(&w, localFlags, t.Lflag)
	fmt.Print(w.buf.String())
	return nil
}

// R3.1: Print all current terminal settings in machine-readable format.
func printSaveFormat(fd int) error {
	t, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return err
	}
	fmt.Printf("%x:%x:%x:%x", t.Iflag, t.Oflag, t.Cflag, t.Lflag)
	for _, c := range t.Cc {
		fmt.Printf(":%x", c)
	}
	fmt.Println()
	return nil
}

// R3.2: Open the specified terminal device and return its file descriptor.
func openDevice(path string) (int, *os.File, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return 0, nil, err
	}
	return int(f.Fd()), f, nil
}

// unwrapSyscallError extracts the underlying error message from os.PathError
// to match GNU coreutils strerror() formatting.
func unwrapSyscallError(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return capitalizeError(pe.Err)
	}
	return capitalizeError(err)
}

// resolveDevice returns the fd and optional file for the target terminal.
func resolveDevice(device string) (int, *os.File, error) {
	if device == "" {
		return int(os.Stdin.Fd()), nil, nil
	}
	fd, f, err := openDevice(device)
	if err != nil {
		return 0, nil, fmt.Errorf("%s: %s", device, unwrapSyscallError(err))
	}
	return fd, f, nil
}

// capitalizeError capitalizes the first letter of an error message to match
// GNU coreutils strerror() formatting (e.g., "Operation not supported").
func capitalizeError(err error) string {
	msg := err.Error()
	if msg == "" {
		return msg
	}
	runes := []rune(msg)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// dispatch calls the appropriate display function based on flags.
func dispatch(fd int, showAll, showSave bool, device string) error {
	var err error
	switch {
	case showSave:
		err = printSaveFormat(fd)
	case showAll:
		err = printAllSettings(fd)
	default:
		err = printDefaultDisplay(fd)
	}
	if err != nil {
		name := "'standard input'"
		if device != "" {
			name = fmt.Sprintf("'%s'", device)
		}
		return fmt.Errorf("%s: %s", name, capitalizeError(err))
	}
	return nil
}

// parseArgs extracts display flags, device, and setting arguments.
func parseArgs(args []string) (showAll, showSave bool, device string, settings []string, err error) {
	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "-a" || arg == "--all":
			showAll = true
		case arg == "-g" || arg == "--save":
			showSave = true
		case arg == "-F":
			if i+1 >= len(args) {
				return false, false, "", nil, fmt.Errorf("option requires an argument -- 'F'")
			}
			i++
			device = args[i]
		case strings.HasPrefix(arg, "-F"):
			device = arg[2:]
		case strings.HasPrefix(arg, "--file="):
			device = arg[len("--file="):]
		case arg == "--file":
			if i+1 >= len(args) {
				return false, false, "", nil, fmt.Errorf("option '--file' requires an argument")
			}
			i++
			device = args[i]
		default:
			// R4.1, R5.1, R6.1, R6.2: Collect setting arguments.
			settings = append(settings, arg)
		}
		i++
	}
	return showAll, showSave, device, settings, nil
}

// run is the main entry point logic, separated for testability.
// Returns the exit code per R7.1 and R7.2.
func run(args []string) int {
	showAll, showSave, device, settings, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "stty: %v\n", err)
		return 1
	}
	if showAll && showSave {
		fmt.Fprintln(os.Stderr, "stty: the options for verbose and stty-readable output styles are")
		fmt.Fprintln(os.Stderr, "mutually exclusive")
		return 1
	}
	fd, deviceFile, err := resolveDevice(device)
	if err != nil {
		fmt.Fprintf(os.Stderr, "stty: %v\n", err)
		return 1
	}
	if deviceFile != nil {
		defer deviceFile.Close()
	}
	if len(settings) > 0 {
		if err := processSettings(fd, settings); err != nil {
			fmt.Fprintf(os.Stderr, "stty: %v\n", err)
			return 1
		}
	}
	if showAll || showSave || len(settings) == 0 {
		if err := dispatch(fd, showAll, showSave, device); err != nil {
			fmt.Fprintf(os.Stderr, "stty: %v\n", err)
			return 1
		}
	}
	return 0
}

// R7.3: main entry point with SIGPIPE handler.
func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}
