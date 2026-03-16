// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd015-basename R1.1-R1.4:
// cmd/basename strips directory components from pathnames and optionally
// removes a suffix. Installs SIGPIPE handler for clean exit on broken pipe.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the name used in diagnostic output.
const progName = "basename"

func main() {
	// R1.4: install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R1.3: exit >0 on incorrect argument count.
	if len(args) == 0 || len(args) > 2 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName) //nolint:errcheck // best-effort diagnostic
		if len(args) > 2 {
			fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", progName, args[2]) //nolint:errcheck // best-effort diagnostic
		}
		os.Exit(1)
	}

	name := args[0]
	result := basename(name)

	// R1.2: suffix removal when second operand is provided.
	if len(args) == 2 {
		suffix := args[1]
		// Do not remove suffix if the result equals the suffix (GNU behavior).
		if suffix != "" && result != suffix && strings.HasSuffix(result, suffix) {
			result = result[:len(result)-len(suffix)]
		}
	}

	// R1.1: print result followed by newline.
	fmt.Println(result)
}

// basename strips the directory prefix from name, matching GNU basename behavior.
// R1.1: strips longest prefix ending in '/'.
// R1.3 (PRD): strips trailing slashes before processing.
// R1.4 (PRD): all-slash input returns "/".
// R1.5 (PRD): empty input returns "".
func basename(name string) string {
	// R1.5: empty string produces empty result.
	if name == "" {
		return ""
	}

	// R1.3 (PRD): strip trailing slashes.
	for len(name) > 1 && name[len(name)-1] == '/' {
		name = name[:len(name)-1]
	}

	// R1.4 (PRD): if name is just "/", return "/".
	if name == "/" {
		return "/"
	}

	// R1.1: strip longest prefix ending in '/'.
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}

	return name
}
