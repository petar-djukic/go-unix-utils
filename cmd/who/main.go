// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/who: show who is logged on.
// Implements srd097-who R1.1-R1.4: core utmpx reading and default output.
package main

/*
#include <utmpx.h>
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "who"

// utmpEntry holds one logged-in user session from the utmpx database.
type utmpEntry struct {
	user string
	line string
	time time.Time
	host string
}

func main() {
	// R1.4: handle SIGPIPE gracefully.
	sys.InstallSIGPIPEHandler()

	entries := readEntries()
	for _, e := range entries {
		printEntry(e)
	}
}

// readEntries reads the utmpx database and returns USER_PROCESS entries.
// R1.1: read utmpx to enumerate logged-in user sessions.
// R1.3: filter to USER_PROCESS entries only.
func readEntries() []utmpEntry {
	C.setutxent()
	defer C.endutxent()

	var entries []utmpEntry
	for {
		entry := C.getutxent()
		if entry == nil {
			break
		}
		if entry.ut_type != C.USER_PROCESS {
			continue
		}
		entries = append(entries, extractEntry(entry))
	}
	return entries
}

// extractEntry converts a C utmpx entry to a Go utmpEntry.
func extractEntry(entry *C.struct_utmpx) utmpEntry {
	return utmpEntry{
		user: C.GoString(&entry.ut_user[0]),
		line: C.GoString(&entry.ut_line[0]),
		time: time.Unix(int64(entry.ut_tv.tv_sec), 0),
		host: C.GoString(&entry.ut_host[0]),
	}
}

// printEntry prints one user session line in GNU who default format.
// R1.2: format is NAME LINE TIME (HOST).
// D3: date format is YYYY-MM-DD HH:MM.
func printEntry(e utmpEntry) {
	timeStr := e.time.Format("2006-01-02 15:04")
	if e.host != "" {
		fmt.Printf("%-8s %-12s %s (%s)\n", e.user, e.line, timeStr, e.host)
	} else {
		fmt.Printf("%-8s %-12s %s\n", e.user, e.line, timeStr)
	}
}
