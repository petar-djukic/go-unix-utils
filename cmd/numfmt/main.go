// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd071-numfmt R1.1-R1.4, R2.1-R2.4.
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

type opts struct {
	fromUnit string
	toUnit   string
	format   string
	padding  int
	round    string
	suffix   string
}

type fmtSpec struct {
	leftAlign bool
	width     int
	prec      int
}

func main() {
	sys.InstallSIGPIPEHandler()
	toUnit := flag.String("to", "none", "")
	fromUnit := flag.String("from", "none", "")
	format := flag.String("format", "", "")
	padding := flag.Int("padding", 0, "")
	round := flag.String("round", "from-zero", "")
	suffix := flag.String("suffix", "", "")
	flag.Parse()
	if !isValidUnit(*toUnit) || !isValidUnit(*fromUnit) {
		fmt.Fprintf(os.Stderr, "numfmt: invalid unit\n")
		os.Exit(1)
	}
	if !isValidRound(*round) {
		fmt.Fprintf(os.Stderr, "numfmt: invalid --round method: '%s'\n", *round)
		os.Exit(1)
	}
	o := opts{fromUnit: *fromUnit, toUnit: *toUnit, format: *format, padding: *padding, round: *round, suffix: *suffix}
	hasError := false
	if flag.NArg() > 0 {
		for _, arg := range flag.Args() {
			if err := processToken(arg, o); err != nil {
				fmt.Fprintf(os.Stderr, "numfmt: %v\n", err)
				hasError = true
			}
		}
	} else {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			if err := processLine(scanner.Text(), o); err != nil {
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

func isValidRound(r string) bool {
	switch r {
	case "up", "down", "from-zero", "towards-zero", "nearest":
		return true
	}
	return false
}

func processLine(line string, o opts) error {
	prefix, token, trailing := splitWhitespace(line)
	if token == "" {
		return fmt.Errorf("invalid number: %q", "")
	}
	val, err := parseNumber(token, o.fromUnit)
	if err != nil {
		return err
	}
	out := formatOutput(val, o)
	if prefix != "" {
		fieldWidth := len(prefix) + len(token)
		if len(out) < fieldWidth {
			out = strings.Repeat(" ", fieldWidth-len(out)) + out
		}
	}
	fmt.Printf("%s%s\n", out, trailing)
	return nil
}

func processToken(token string, o opts) error {
	val, err := parseNumber(token, o.fromUnit)
	if err != nil {
		return err
	}
	fmt.Println(formatOutput(val, o))
	return nil
}

func formatOutput(val float64, o opts) string {
	spec := parseFmtSpec(o.format)
	out := formatNumber(val, o.toUnit, o.round, spec.prec)
	out += o.suffix
	width := spec.width
	leftAlign := spec.leftAlign
	if o.padding != 0 {
		pw := o.padding
		if pw < 0 {
			leftAlign = true
			pw = -pw
		}
		if pw > width {
			width = pw
		}
	}
	if width > 0 && len(out) < width {
		out = padString(out, width, leftAlign)
	}
	return out
}

func parseFmtSpec(format string) fmtSpec {
	spec := fmtSpec{prec: -1}
	idx := strings.IndexByte(format, '%')
	if idx < 0 {
		return spec
	}
	s := format[idx+1:]
	for len(s) > 0 && (s[0] == '-' || s[0] == '+' || s[0] == '\'' || s[0] == ' ') {
		if s[0] == '-' {
			spec.leftAlign = true
		}
		s = s[1:]
	}
	for len(s) > 0 && s[0] >= '0' && s[0] <= '9' {
		spec.width = spec.width*10 + int(s[0]-'0')
		s = s[1:]
	}
	if len(s) > 0 && s[0] == '.' {
		s = s[1:]
		spec.prec = 0
		for len(s) > 0 && s[0] >= '0' && s[0] <= '9' {
			spec.prec = spec.prec*10 + int(s[0]-'0')
			s = s[1:]
		}
	}
	return spec
}

func padString(s string, width int, leftAlign bool) string {
	pad := width - len(s)
	if pad <= 0 {
		return s
	}
	spaces := strings.Repeat(" ", pad)
	if leftAlign {
		return s + spaces
	}
	return spaces + s
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

func formatNumber(val float64, unit, round string, prec int) string {
	if unit == "none" {
		return formatValPrec(val, prec)
	}
	base := baseForUnit(unit)
	suffixes := suffixesForUnit(unit)
	return formatWithSuffix(val, base, suffixes, round, prec)
}

func formatRaw(val float64) string {
	if val == math.Trunc(val) && !math.IsInf(val, 0) {
		return strconv.FormatInt(int64(val), 10)
	}
	return strconv.FormatFloat(val, 'f', -1, 64)
}

func formatValPrec(val float64, prec int) string {
	if prec >= 0 {
		return strconv.FormatFloat(val, 'f', prec, 64)
	}
	return formatRaw(val)
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

func formatWithSuffix(val, base float64, suffixes []string, round string, prec int) string {
	negative := val < 0
	abs := math.Abs(val)
	level := 0
	scaled := abs
	for level+1 < len(suffixes) && scaled >= base {
		scaled /= base
		level++
	}
	if level == 0 {
		return formatValPrec(val, prec)
	}
	sign := ""
	if negative {
		sign = "-"
	}
	if prec >= 0 {
		r := roundToPrec(scaled, prec, negative, round)
		if r >= base && level+1 < len(suffixes) {
			level++
			r = roundToPrec(abs/math.Pow(base, float64(level)), prec, negative, round)
		}
		return fmt.Sprintf("%s%.*f%s", sign, prec, r, suffixes[level])
	}
	rounded1 := roundToPrec(scaled, 1, negative, round)
	if rounded1 < 10 {
		return fmt.Sprintf("%s%.1f%s", sign, rounded1, suffixes[level])
	}
	intVal := int64(roundToPrec(scaled, 0, negative, round))
	if float64(intVal) >= base && level+1 < len(suffixes) {
		level++
		newScaled := abs / math.Pow(base, float64(level))
		r := roundToPrec(newScaled, 1, negative, round)
		if r < 10 {
			return fmt.Sprintf("%s%.1f%s", sign, r, suffixes[level])
		}
		intVal = int64(roundToPrec(newScaled, 0, negative, round))
	}
	return fmt.Sprintf("%s%d%s", sign, intVal, suffixes[level])
}

func roundToPrec(absVal float64, prec int, negative bool, method string) float64 {
	factor := math.Pow(10, float64(prec))
	return roundAbs(absVal*factor, negative, method) / factor
}

func roundAbs(absVal float64, negative bool, method string) float64 {
	switch method {
	case "up":
		if negative {
			return math.Floor(absVal)
		}
		return math.Ceil(absVal)
	case "down":
		if negative {
			return math.Ceil(absVal)
		}
		return math.Floor(absVal)
	case "from-zero":
		return math.Ceil(absVal)
	case "towards-zero":
		return math.Floor(absVal)
	case "nearest":
		return math.Round(absVal)
	}
	return math.Ceil(absVal)
}
