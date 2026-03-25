// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin

// Platform-specific termios operations for Darwin.
package main

import "golang.org/x/sys/unix"

// reqGetTermios is the Darwin ioctl request to read terminal attributes.
const reqGetTermios = unix.TIOCGETA

// reqSetTermios is the Darwin ioctl request to write terminal attributes.
const reqSetTermios = unix.TIOCSETA

// setIflagBits sets bits in the input flag field.
func setIflagBits(t *unix.Termios, bits uint64) { t.Iflag |= bits }

// clearIflag clears bits in the input flag field.
func clearIflag(t *unix.Termios, bits uint64) { t.Iflag &^= bits }

// setOflagBits sets bits in the output flag field.
func setOflagBits(t *unix.Termios, bits uint64) { t.Oflag |= bits }

// clearOflag clears bits in the output flag field.
func clearOflag(t *unix.Termios, bits uint64) { t.Oflag &^= bits }

// setCflagBits sets bits in the control flag field.
func setCflagBits(t *unix.Termios, bits uint64) { t.Cflag |= bits }

// clearCflag clears bits in the control flag field.
func clearCflag(t *unix.Termios, bits uint64) { t.Cflag &^= bits }

// setLflagBits sets bits in the local flag field.
func setLflagBits(t *unix.Termios, bits uint64) { t.Lflag |= bits }

// clearLflag clears bits in the local flag field.
func clearLflag(t *unix.Termios, bits uint64) { t.Lflag &^= bits }

// getFieldBits returns the value of a termios flag field as uint64.
func getFieldBits(t *unix.Termios, field flagField) uint64 {
	switch field {
	case fieldInput:
		return t.Iflag
	case fieldOutput:
		return t.Oflag
	case fieldControl:
		return t.Cflag
	case fieldLocal:
		return t.Lflag
	default:
		return 0
	}
}

// setInputSpeed sets the input baud rate.
// R2.4: on Darwin, speed values are stored directly.
func setInputSpeed(t *unix.Termios, speed uint64) { t.Ispeed = speed }

// setOutputSpeed sets the output baud rate.
func setOutputSpeed(t *unix.Termios, speed uint64) { t.Ospeed = speed }

// getInputSpeed returns the input baud rate.
func getInputSpeed(t *unix.Termios) uint64 { return t.Ispeed }

// getOutputSpeed returns the output baud rate.
func getOutputSpeed(t *unix.Termios) uint64 { return t.Ospeed }

// isValidSpeed checks if the given speed is a valid baud rate.
func isValidSpeed(speed uint64) bool {
	switch speed {
	case 0, 50, 75, 110, 134, 150, 200, 300, 600, 1200,
		1800, 2400, 4800, 9600, 19200, 38400, 57600,
		115200, 230400:
		return true
	default:
		return false
	}
}

// getLineDiscipline returns the line discipline.
// Darwin does not expose line discipline in Termios.
func getLineDiscipline(_ *unix.Termios) (int, bool) { return 0, false }
