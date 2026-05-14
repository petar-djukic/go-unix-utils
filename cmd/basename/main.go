// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	if len(args) < 1 || len(args) > 2 {
		fmt.Fprintf(os.Stderr, "basename: missing operand\n")
		os.Exit(1)
	}

	name := args[0]
	suffix := ""
	if len(args) == 2 {
		suffix = args[1]
	}

	result := basename(name, suffix)
	fmt.Println(result)
}

func basename(name, suffix string) string {
	name = strings.TrimRight(name, "/")

	if name == "" {
		return "/"
	}

	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}

	if suffix != "" && name != suffix && strings.HasSuffix(name, suffix) {
		name = name[:len(name)-len(suffix)]
	}

	return name
}
