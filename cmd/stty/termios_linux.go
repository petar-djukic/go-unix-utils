// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build linux

// Platform-specific termios operations for Linux.
package main

import "golang.org/x/sys/unix"

// reqGetTermios is the Linux ioctl request to read terminal attributes.
const reqGetTermios = unix.TCGETS

// reqSetTermios is the Linux ioctl request to write terminal attributes.
const reqSetTermios = unix.TCSETS

// setIflagBits sets bits in the input flag field.
func setIflagBits(t *unix.Termios, bits uint64) { t.Iflag |= uint32(bits) }

// clearIflag clears bits in the input flag field.
func clearIflag(t *unix.Termios, bits uint64) { t.Iflag &^= uint32(bits) }

// setOflagBits sets bits in the output flag field.
func setOflagBits(t *unix.Termios, bits uint64) { t.Oflag |= uint32(bits) }

// clearOflag clears bits in the output flag field.
func clearOflag(t *unix.Termios, bits uint64) { t.Oflag &^= uint32(bits) }

// setCflagBits sets bits in the control flag field.
func setCflagBits(t *unix.Termios, bits uint64) { t.Cflag |= uint32(bits) }

// clearCflag clears bits in the control flag field.
func clearCflag(t *unix.Termios, bits uint64) { t.Cflag &^= uint32(bits) }

// setLflagBits sets bits in the local flag field.
func setLflagBits(t *unix.Termios, bits uint64) { t.Lflag |= uint32(bits) }

// clearLflag clears bits in the local flag field.
func clearLflag(t *unix.Termios, bits uint64) { t.Lflag &^= uint32(bits) }

// getFieldBits returns the value of a termios flag field as uint64.
func getFieldBits(t *unix.Termios, field flagField) uint64 {
	switch field {
	case fieldInput:
		return uint64(t.Iflag)
	case fieldOutput:
		return uint64(t.Oflag)
	case fieldControl:
		return uint64(t.Cflag)
	case fieldLocal:
		return uint64(t.Lflag)
	default:
		return 0
	}
}

// speedToBaud maps user-visible baud rates to Bxxx constants.
var speedToBaud = map[uint64]uint32{
	0:      unix.B0,
	50:     unix.B50,
	75:     unix.B75,
	110:    unix.B110,
	134:    unix.B134,
	150:    unix.B150,
	200:    unix.B200,
	300:    unix.B300,
	600:    unix.B600,
	1200:   unix.B1200,
	1800:   unix.B1800,
	2400:   unix.B2400,
	4800:   unix.B4800,
	9600:   unix.B9600,
	19200:  unix.B19200,
	38400:  unix.B38400,
	57600:  unix.B57600,
	115200: unix.B115200,
	230400: unix.B230400,
}

// setInputSpeed sets the input baud rate using Bxxx constants.
func setInputSpeed(t *unix.Termios, speed uint64) {
	if baud, ok := speedToBaud[speed]; ok {
		t.Ispeed = baud
	}
}

// setOutputSpeed sets the output baud rate using Bxxx constants.
func setOutputSpeed(t *unix.Termios, speed uint64) {
	if baud, ok := speedToBaud[speed]; ok {
		t.Ospeed = baud
	}
}

// baudToSpeed maps Bxxx constants back to user-visible baud rates.
var baudToSpeed = map[uint32]uint64{
	unix.B0:      0,
	unix.B50:     50,
	unix.B75:     75,
	unix.B110:    110,
	unix.B134:    134,
	unix.B150:    150,
	unix.B200:    200,
	unix.B300:    300,
	unix.B600:    600,
	unix.B1200:   1200,
	unix.B1800:   1800,
	unix.B2400:   2400,
	unix.B4800:   4800,
	unix.B9600:   9600,
	unix.B19200:  19200,
	unix.B38400:  38400,
	unix.B57600:  57600,
	unix.B115200: 115200,
	unix.B230400: 230400,
}

// getInputSpeed returns the input baud rate as a user-visible value.
func getInputSpeed(t *unix.Termios) uint64 { return baudToSpeed[t.Ispeed] }

// getOutputSpeed returns the output baud rate as a user-visible value.
func getOutputSpeed(t *unix.Termios) uint64 { return baudToSpeed[t.Ospeed] }

// isValidSpeed checks if the given speed is a valid baud rate.
func isValidSpeed(speed uint64) bool {
	_, ok := speedToBaud[speed]
	return ok
}

// getLineDiscipline returns the line discipline number.
func getLineDiscipline(t *unix.Termios) (int, bool) { return int(t.Line), true }
