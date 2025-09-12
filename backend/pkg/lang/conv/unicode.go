// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package conv

import (
	"regexp"
	"strconv"
	"unicode/utf8"
)

// UnescapeUnicode converts Unicode escape sequences in string to actual characters
// Supports \uXXXX format Unicode escape sequences
// Example: "Hello\u0026World" -> "Hello&World"
func UnescapeUnicode(str string) string {
	re := regexp.MustCompile(`\\u[0-9a-fA-F]{4}`)

	return re.ReplaceAllStringFunc(str, func(match string) string {
		hexStr := match[2:]
		if codePoint, err := strconv.ParseInt(hexStr, 16, 32); err == nil {
			if utf8.ValidRune(rune(codePoint)) {
				return string(rune(codePoint))
			}
		}
		return match
	})
}

// UnescapeUnicodeBytes processes Unicode escape sequences in byte array
func UnescapeUnicodeBytes(data []byte) []byte {
	return []byte(UnescapeUnicode(string(data)))
}
