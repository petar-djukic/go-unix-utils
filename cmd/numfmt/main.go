// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd071-numfmt R1.1-R1.4.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

var (
	siSuffixes   = []string{"", "k", "M", "G", "T", "P", "E"}
	iecSuffixes  = []string{"", "K", "M", "G", "T", "P", "E"}
	ieciSuffixes = []string{"", "Ki", "Mi", "Gi", "Ti", "Pi", "Ei"}
)

func main() {
	sys.InstallSIGPIPEHandler()
	toUnit := flag.String("to", "none", "")
	fromUnit := flag.String("from", "none", "")
	flag.Parse()

	if !isValidUnit(*toUnit) || !isValidUnit(*fromUnit) {
		fmt.Fprintf(os.Stderr, "numfmt: invalid unit\n")
		os.Exit(1)
	}

	hasError := false
	if flag.NArg() > 0 {
		for _, arg := range flag.Args() {
			if err := processToken(arg, *fromUnit, *toUnit); err != nil {
				fmt.Fprintf(os.Stderr, "numfmt: %v\n", err)
				hasError = true
			}
		}
	} else {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := scanner.Text()
			if err := processLine(line, *fromUnit, *toUnit); err != nil {
				fmt.Fprintf(os.Stderr, "numfmt: %v\n", err)
				hasError = true
			}
		}
	}
	if hasError {
		os.Exit(2)
	}
}

func isValidUnit(u string) bool {
	switch u {
	case "none", "si", "iec", "iec-i":
		return true
	}
	return false
}

func processLine(line, fromUnit, toUnit string) error {
	prefix, token, trailing := splitWhitespace(line)
	if token == "" {
		return fmt.Errorf("invalid number: %q", "")
	}
	val, err := parseNumber(token, fromUnit)
	if err != nil {
		return err
	}
	out := formatNumber(val, toUnit)
	if prefix != "" {
		fieldWidth := len(prefix) + len(token)
		if len(out) < fieldWidth {
			out = strings.Repeat(" ", fieldWidth-len(out)) + out
		}
	}
	fmt.Printf("%s%s\n", out, trailing)
	return nil
}

func processToken(token, fromUnit, toUnit string) error {
	val, err := parseNumber(token, fromUnit)
	if err != nil {
		return err
	}
	out := formatNumber(val, toUnit)
	fmt.Println(out)
	return nil
}

func splitWhitespace(s string) (prefix, body, suffix string) {
	trimLeft := strings.TrimLeft(s, " \t")
	prefix = s[:len(s)-len(trimLeft)]
	trimRight := strings.TrimRight(trimLeft, " \t")
	suffix = trimLeft[len(trimRight):]
	body = trimRight
	return
}

func parseNumber(s, unit string) (float64, error) {
	if unit == "none" {
		val, err := strconv.ParseFloat(s, 64)
		if err != nil {
			if hasSuffix(s) {
				return 0, fmt.Errorf(
					"rejecting suffix in input: '%s' (consider using --from)", s)
			}
			return 0, fmt.Errorf("invalid number: '%s'", s)
		}
		return val, nil
	}
	numStr, mult, err := extractSuffix(s, unit)
	if err != nil {
		return 0, err
	}
	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number: '%s'", s)
	}
	return val * mult, nil
}

func hasSuffix(s string) bool {
	if len(s) == 0 {
		return false
	}
	if len(s) >= 2 {
		tail := s[len(s)-2:]
		switch tail {
		case "Ki", "Mi", "Gi", "Ti", "Pi", "Ei":
			return true
		}
	}
	switch s[len(s)-1] {
	case 'K', 'k', 'M', 'G', 'T', 'P', 'E':
		return true
	}
	return false
}

func extractSuffix(s, unit string) (string, float64, error) {
	if len(s) == 0 {
		return "", 0, fmt.Errorf("invalid number: ''")
	}
	base := baseForUnit(unit)
	suffixMap := suffixMultipliers(unit, base)
	if unit == "iec-i" && len(s) >= 2 {
		tail := s[len(s)-2:]
		if m, ok := suffixMap[tail]; ok {
			return s[:len(s)-2], m, nil
		}
	}
	tail := s[len(s)-1:]
	if m, ok := suffixMap[tail]; ok {
		return s[:len(s)-1], m, nil
	}
	if endsWithLetter(s) {
		return "", 0, fmt.Errorf("invalid suffix in input: '%s'", s)
	}
	return s, 1, nil
}

func endsWithLetter(s string) bool {
	if len(s) == 0 {
		return false
	}
	c := s[len(s)-1]
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func baseForUnit(unit string) float64 {
	if unit == "si" {
		return 1000
	}
	return 1024
}

func suffixMultipliers(unit string, base float64) map[string]float64 {
	m := make(map[string]float64)
	switch unit {
	case "si":
		m["K"] = base
		m["k"] = base
		m["M"] = math.Pow(base, 2)
		m["G"] = math.Pow(base, 3)
		m["T"] = math.Pow(base, 4)
		m["P"] = math.Pow(base, 5)
		m["E"] = math.Pow(base, 6)
	case "iec":
		m["K"] = base
		m["M"] = math.Pow(base, 2)
		m["G"] = math.Pow(base, 3)
		m["T"] = math.Pow(base, 4)
		m["P"] = math.Pow(base, 5)
		m["E"] = math.Pow(base, 6)
	case "iec-i":
		m["Ki"] = base
		m["Mi"] = math.Pow(base, 2)
		m["Gi"] = math.Pow(base, 3)
		m["Ti"] = math.Pow(base, 4)
		m["Pi"] = math.Pow(base, 5)
		m["Ei"] = math.Pow(base, 6)
	}
	return m
}

func formatNumber(val float64, unit string) string {
	if unit == "none" {
		return formatRaw(val)
	}
	base := baseForUnit(unit)
	suffixes := suffixesForUnit(unit)
	return formatWithSuffix(val, base, suffixes)
}

func formatRaw(val float64) string {
	if val == math.Trunc(val) && !math.IsInf(val, 0) {
		return strconv.FormatInt(int64(val), 10)
	}
	return strconv.FormatFloat(val, 'f', -1, 64)
}

func suffixesForUnit(unit string) []string {
	switch unit {
	case "si":
		return siSuffixes
	case "iec":
		return iecSuffixes
	case "iec-i":
		return ieciSuffixes
	}
	return nil
}

func formatWithSuffix(val, base float64, suffixes []string) string {
	negative := val < 0
	abs := math.Abs(val)
	level := 0
	scaled := abs
	for level+1 < len(suffixes) && scaled >= base {
		scaled /= base
		level++
	}
	if level == 0 {
		return formatRaw(val)
	}
	rounded1 := math.Ceil(scaled*10) / 10
	if rounded1 < 10 {
		if negative {
			return fmt.Sprintf("-%.1f%s", rounded1, suffixes[level])
		}
		return fmt.Sprintf("%.1f%s", rounded1, suffixes[level])
	}
	intVal := int64(math.Ceil(scaled))
	if float64(intVal) >= base && level+1 < len(suffixes) {
		level++
		newScaled := abs / math.Pow(base, float64(level))
		r := math.Ceil(newScaled*10) / 10
		if r < 10 {
			if negative {
				return fmt.Sprintf("-%.1f%s", r, suffixes[level])
			}
			return fmt.Sprintf("%.1f%s", r, suffixes[level])
		}
		intVal = int64(math.Ceil(newScaled))
	}
	if negative {
		return fmt.Sprintf("-%d%s", intVal, suffixes[level])
	}
	return fmt.Sprintf("%d%s", intVal, suffixes[level])
}
