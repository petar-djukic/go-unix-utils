// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/sha512sum (prd033-sha512sum R1–R4).
package main

import (
	"bufio"
	"crypto/sha512"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()
	var bin, txt, chk, tag, warn, quiet, status bool
	flag.BoolVar(&bin, "b", false, "")
	flag.BoolVar(&bin, "binary", false, "")
	flag.BoolVar(&txt, "t", false, "")
	flag.BoolVar(&txt, "text", false, "")
	flag.BoolVar(&chk, "c", false, "")
	flag.BoolVar(&chk, "check", false, "")
	flag.BoolVar(&tag, "tag", false, "")
	flag.BoolVar(&warn, "w", false, "")
	flag.BoolVar(&warn, "warn", false, "")
	flag.BoolVar(&quiet, "quiet", false, "")
	flag.BoolVar(&status, "status", false, "")
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		args = []string{"-"}
	}
	rc := 0
	if chk {
		rc = check(args, warn, quiet, status)
	} else {
		rc = compute(args, bin, tag)
	}
	os.Exit(rc)
}

func hash(r io.Reader) string {
	h := sha512.New()
	io.Copy(h, r) //nolint:errcheck // best-effort read
	return hex.EncodeToString(h.Sum(nil))
}

func hashFile(name string) (string, error) {
	if name == "-" {
		return hash(os.Stdin), nil
	}
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return hash(f), nil
}

func compute(files []string, bin, tag bool) int {
	rc := 0
	for _, name := range files {
		h, err := hashFile(name)
		if err != nil {
			printErr(err)
			rc = 1
			continue
		}
		if tag {
			fmt.Printf("SHA512 (%s) = %s\n", name, h)
		} else if bin {
			fmt.Printf("%s *%s\n", h, name)
		} else {
			fmt.Printf("%s  %s\n", h, name)
		}
	}
	return rc
}

func check(files []string, warn, quiet, status bool) int {
	rc, total := 0, 0
	for _, cf := range files {
		var r io.ReadCloser
		if cf == "-" {
			r = os.Stdin
		} else {
			f, err := os.Open(cf)
			if err != nil {
				printErr(err)
				rc = 1
				continue
			}
			r = f
		}
		failed, ln := 0, 0
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			ln++
			exp, fn, ok := parseLine(sc.Text())
			if !ok {
				if warn {
					fmt.Fprintf(os.Stderr, "sha512sum: %s: %d: improperly formatted SHA512 checksum line\n", cf, ln)
				}
				continue
			}
			got, err := hashFile(fn)
			if err != nil {
				printErr(err)
				if !status {
					fmt.Printf("%s: FAILED open or read\n", fn)
				}
				failed++
				continue
			}
			if strings.EqualFold(got, exp) {
				if !quiet && !status {
					fmt.Printf("%s: OK\n", fn)
				}
			} else {
				if !status {
					fmt.Printf("%s: FAILED\n", fn)
				}
				failed++
			}
		}
		if cf != "-" {
			r.Close() // best-effort close
		}
		if failed > 0 {
			rc = 1
			total += failed
		}
	}
	if total > 0 && !status {
		s := ""
		if total > 1 {
			s = "s"
		}
		fmt.Fprintf(os.Stderr, "sha512sum: WARNING: %d computed checksum%s did NOT match\n", total, s)
	}
	return rc
}

func parseLine(line string) (string, string, bool) {
	if _, after, ok := strings.Cut(line, " ("); ok {
		if fn, hash, ok := strings.Cut(after, ") = "); ok {
			return hash, fn, len(hash) > 0 && len(fn) > 0
		}
	}
	i := strings.IndexByte(line, ' ')
	if i < 1 || i >= len(line)-1 {
		return "", "", false
	}
	if line[i+1] == ' ' || line[i+1] == '*' {
		return line[:i], line[i+2:], true
	}
	return "", "", false
}

func printErr(err error) {
	if pe, ok := err.(*os.PathError); ok {
		msg := pe.Err.Error()
		if len(msg) > 0 {
			msg = strings.ToUpper(msg[:1]) + msg[1:]
		}
		fmt.Fprintf(os.Stderr, "sha512sum: %s: %s\n", pe.Path, msg)
		return
	}
	fmt.Fprintf(os.Stderr, "sha512sum: %s\n", err)
}
