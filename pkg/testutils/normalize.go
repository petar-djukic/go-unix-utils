// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

// ComposeNormalizers chains multiple NormalizeFunc values into a single
// NormalizeFunc that applies each in order.
//
// R4.3, R4.4: composition of normalizers.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(data []byte) []byte {
		for _, fn := range fns {
			if fn != nil {
				data = fn(data)
			}
		}
		return data
	}
}
