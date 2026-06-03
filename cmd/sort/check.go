// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bufio"
	"fmt"
	"os"
)

func runCheck(files []string, opts options) int {
	keyCmp := buildKeyCmp(opts)
	hasKeys := len(opts.keys) > 0
	cmp := buildSortCmp(keyCmp, opts, hasKeys)
	var prev string
	var hasPrev bool
	for _, file := range files {
		code := checkFile(file, cmp, opts, &prev, &hasPrev)
		if code != 0 {
			return code
		}
	}
	return 0
}

func checkFile(
	file string, cmp func(string, string) int,
	opts options, prev *string, hasPrev *bool,
) int {
	r, closer, err := openInput(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sort: %s\n", err)
		return 2
	}
	if closer != nil {
		defer closer.Close()
	}
	scanner := bufio.NewScanner(r)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if *hasPrev && isDisorder(cmp, *prev, line, opts.unique) {
			if opts.check == checkDiagnose {
				fmt.Fprintf(os.Stderr, "sort: %s:%d: disorder: %s\n",
					file, lineNum, line)
			}
			return 1
		}
		*prev = line
		*hasPrev = true
	}
	return 0
}

func isDisorder(cmp func(string, string) int, prev, curr string, unique bool) bool {
	r := cmp(prev, curr)
	return r > 0 || (unique && r == 0)
}
