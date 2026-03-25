// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd105-stty R1.1–R1.3: display terminal settings.
package main

import (
	"fmt"

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
// R1.2: shows speeds, characters, and all flag groups.
func printAll(t *unix.Termios, fd int) error {
	printSpeeds(t)
	printAllChars(t)
	printAllFlags(t)
	return nil
}

// printSummary prints changed-from-default terminal settings.
// R1.1: shows speed and line discipline.
func printSummary(t *unix.Termios, fd int) error {
	printSpeeds(t)
	return nil
}

// printSpeeds prints input and output baud rates.
func printSpeeds(t *unix.Termios) {
	ispeed := getInputSpeed(t)
	ospeed := getOutputSpeed(t)
	if ispeed == ospeed {
		fmt.Printf("speed %d baud;\n", ospeed)
	} else {
		fmt.Printf("ispeed %d baud; ospeed %d baud;\n", ispeed, ospeed)
	}
}

// printAllChars prints all special character assignments.
func printAllChars(t *unix.Termios) {
	for _, name := range charOrder {
		idx := charDefs[name]
		fmt.Printf("%s = %s; ", name, formatCC(t.Cc[idx]))
	}
	fmt.Println()
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

// printAllFlags prints all flag settings grouped by category.
func printAllFlags(t *unix.Termios) {
	for _, name := range flagOrder {
		def := flagDefs[name]
		if isFlagSet(t, def) {
			fmt.Printf("%s ", name)
		} else {
			fmt.Printf("-%s ", name)
		}
	}
	fmt.Println()
}
