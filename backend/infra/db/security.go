// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"bytes"
)

func QuoteSQLData(data string) string {
	return "'" + escapeSQL(data) + "'"
}

func EscapeSQLData(data string) string {
	return escapeSQL(data)
}

func escapeSQL(data string) string {
	buf := bytes.NewBuffer(nil)
	for _, c := range data {
		switch c {
		case 0x00:
			buf.WriteByte('\\')
			buf.WriteByte('0')
		case '\'':
			buf.WriteByte('\\')
			buf.WriteByte('\'')
		case '"':
			buf.WriteByte('\\')
			buf.WriteByte('"')
		case 0x08:
			buf.WriteByte('\\')
			buf.WriteByte('b')
		case 0x0A:
			buf.WriteByte('\\')
			buf.WriteByte('n')
		case 0x0D:
			buf.WriteByte('\\')
			buf.WriteByte('r')
		case 0x09:
			buf.WriteByte('\\')
			buf.WriteByte('t')
		case 0x1A:
			buf.WriteByte('\\')
			buf.WriteByte('Z')
		case '\\':
			buf.WriteByte('\\')
			buf.WriteByte('\\')
		// case '%':
		//	buf.WriteByte('\\')
		//	buf.WriteByte('%')
		// case '_':
		//	buf.WriteByte('\\')
		//	buf.WriteByte('_')
		default:
			buf.WriteRune(c)
		}
	}
	return buf.String()
}
