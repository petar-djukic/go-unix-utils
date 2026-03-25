// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd105-stty R1.1–R1.3: display terminal settings.
package main

import (
	"fmt"
	"strings"

	"golang.org/x/sys/unix"
)

// displaySettings routes to the appropriate display mode.
func displaySettings(t *unix.Termios, opts options, fd int) error {
	if opts.showSave {
		return printSave(t)
	}
	if opts.showAll {
		return printAll(t, fd)
	}
	return printSummary(t, fd)
}

// printSave prints termios in machine-readable -g format.
// R1.3: colon-separated hex values restorable via stty arguments.
func printSave(t *unix.Termios) error {
	fmt.Printf("%x:%x:%x:%x",
		getFieldBits(t, fieldInput),
		getFieldBits(t, fieldOutput),
		getFieldBits(t, fieldControl),
		getFieldBits(t, fieldLocal))
	for _, c := range t.Cc {
		fmt.Printf(":%x", c)
	}
	fmt.Printf(":%x:%x\n", getInputSpeed(t), getOutputSpeed(t))
	return nil
}

// printAll prints all terminal settings in human-readable format.
// R1.2: shows speeds, rows/columns, characters, and all flag groups.
func printAll(t *unix.Termios, fd int) error {
	printSpeedLine(t, fd, true)
	printAllChars(t)
	printAllFlagGroups(t)
	return nil
}

// printSummary prints changed-from-default terminal settings.
// R1.1: shows speed, line discipline, and non-default settings.
func printSummary(t *unix.Termios, fd int) error {
	printSpeedLine(t, fd, false)
	printChangedChars(t)
	printChangedFlags(t)
	return nil
}

// printSpeedLine prints baud rate, and optionally rows/columns/line discipline.
func printSpeedLine(t *unix.Termios, fd int, showAll bool) {
	ispeed := getInputSpeed(t)
	ospeed := getOutputSpeed(t)
	if ispeed == ospeed {
		fmt.Printf("speed %d baud;", ospeed)
	} else {
		fmt.Printf("ispeed %d baud; ospeed %d baud;", ispeed, ospeed)
	}
	if showAll {
		rows, cols := getWinSize(fd)
		fmt.Printf(" rows %d; columns %d;", rows, cols)
	}
	if line, has := getLineDiscipline(t); has {
		fmt.Printf(" line = %d;", line)
	}
	fmt.Println()
}

// getWinSize returns the terminal window size (rows, columns).
func getWinSize(fd int) (int, int) {
	ws, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
	if err != nil {
		return 0, 0
	}
	return int(ws.Row), int(ws.Col)
}

// printAllChars prints all special character assignments.
func printAllChars(t *unix.Termios) {
	var entries []string
	for _, name := range charOrder {
		idx := charDefs[name]
		entries = append(entries, fmt.Sprintf("%s = %s;", name, formatCC(t.Cc[idx])))
	}
	printWrapped(entries, 80)
}

// printWrapped prints entries separated by spaces, wrapping at maxWidth.
func printWrapped(entries []string, maxWidth int) {
	line := ""
	for _, e := range entries {
		candidate := e
		if line != "" {
			candidate = line + " " + e
		}
		if len(candidate) > maxWidth && line != "" {
			fmt.Println(line)
			line = e
		} else {
			line = candidate
		}
	}
	if line != "" {
		fmt.Println(line)
	}
}

// formatCC formats a control character value for display.
func formatCC(val uint8) string {
	switch {
	case val == 0:
		return "<undef>"
	case val == 127:
		return "^?"
	case val < 32:
		return fmt.Sprintf("^%c", val+'@')
	default:
		return fmt.Sprintf("%d", val)
	}
}

// printAllFlagGroups prints all flag settings grouped by category.
func printAllFlagGroups(t *unix.Termios) {
	printFlagGroup(t, cflagDisplayOrder)
	printFlagGroup(t, iflagDisplayOrder)
	printFlagGroup(t, oflagDisplayOrder)
	printFlagGroup(t, lflagDisplayOrder)
}

// printFlagGroup prints one category of flags on a single line.
// Multi-bit fields (CSIZE) show only the active value.
func printFlagGroup(t *unix.Termios, names []string) {
	var parts []string
	for _, name := range names {
		def, ok := flagDefs[name]
		if !ok {
			continue
		}
		if def.mask != 0 {
			if isFlagSet(t, def) {
				parts = append(parts, name)
			}
			continue
		}
		if isFlagSet(t, def) {
			parts = append(parts, name)
		} else {
			parts = append(parts, "-"+name)
		}
	}
	if len(parts) > 0 {
		fmt.Println(strings.Join(parts, " "))
	}
}

// printChangedChars prints control characters that differ from sane defaults.
func printChangedChars(t *unix.Termios) {
	var entries []string
	for _, name := range charOrder {
		idx := charDefs[name]
		dflt, ok := defaultChars[name]
		if ok && t.Cc[idx] == dflt {
			continue
		}
		entries = append(entries, fmt.Sprintf("%s = %s;", name, formatCC(t.Cc[idx])))
	}
	if len(entries) > 0 {
		printWrapped(entries, 80)
	}
}

// printChangedFlags prints flags that differ from sane defaults.
func printChangedFlags(t *unix.Termios) {
	var parts []string
	allNames := concatStrings(cflagDisplayOrder, iflagDisplayOrder,
		oflagDisplayOrder, lflagDisplayOrder)
	for _, name := range allNames {
		changed := isChangedFlag(t, name)
		if changed != "" {
			parts = append(parts, changed)
		}
	}
	if len(parts) > 0 {
		fmt.Println(strings.Join(parts, " "))
	}
}

// isChangedFlag returns the display name if the flag differs from default,
// or "" if it matches the default.
func isChangedFlag(t *unix.Termios, name string) string {
	def, ok := flagDefs[name]
	if !ok {
		return ""
	}
	isSet := isFlagSet(t, def)
	if def.mask != 0 {
		if isSet && name != defaultCSIZE {
			return name
		}
		return ""
	}
	if isSet != defaultFlagSet[name] {
		if isSet {
			return name
		}
		return "-" + name
	}
	return ""
}

// concatStrings concatenates multiple string slices into one.
func concatStrings(slices ...[]string) []string {
	var result []string
	for _, s := range slices {
		result = append(result, s...)
	}
	return result
}
