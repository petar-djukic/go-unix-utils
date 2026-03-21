// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd003-format R3.5-R3.6: utility context for HumanSize.
//
// R3.5: ls -h uses HumanSize with Binary=true to format file sizes in -l
// output. The same conversion applies to ls -s block counts (caller
// converts blocks to bytes before calling HumanSize).
//
// R3.6: du -h uses HumanSize with the same binary/SI distinction as ls -h.
// du passes accumulated directory byte counts to HumanSize for display.
//
// No additional exported functions are required by R3.5 or R3.6; the
// existing HumanSize(bytes int64, opts HumanSizeOpts) string API serves
// both utilities.
package format
