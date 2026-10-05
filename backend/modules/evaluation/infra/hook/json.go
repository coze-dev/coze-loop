// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"bytes"
	"encoding/json"
	"io"
	"unicode/utf8"
)

// Unknown response fields may exceed parameter depth/key limits. Track each
// object's keys iteratively while the caller enforces the overall byte limit.
func validJSONObject(data []byte) bool {
	if !utf8.Valid(data) {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return false
	}
	type frame struct {
		object bool
		key    bool
		seen   map[string]struct{}
	}
	stack := []frame{{object: true, key: true, seen: make(map[string]struct{})}}
	for len(stack) > 0 {
		token, err = decoder.Token()
		if err != nil {
			return false
		}
		current := &stack[len(stack)-1]
		if delimiter, ok := token.(json.Delim); ok {
			switch delimiter {
			case '}', ']':
				stack = stack[:len(stack)-1]
			case '{', '[':
				current.key = true
				stack = append(stack, frame{object: delimiter == '{', key: true, seen: make(map[string]struct{})})
			default:
				return false
			}
		} else if current.object && current.key {
			key, ok := token.(string)
			if !ok {
				return false
			}
			if _, exists := current.seen[key]; exists {
				return false
			}
			current.seen[key] = struct{}{}
			current.key = false
		} else {
			current.key = true
		}
	}
	_, err = decoder.Token()
	return err == io.EOF
}
