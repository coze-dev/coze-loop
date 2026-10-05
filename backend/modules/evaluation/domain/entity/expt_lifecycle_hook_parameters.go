// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

// NormalizeHookParameters validates object text without re-encoding its values.
func NormalizeHookParameters(raw string) (string, error) {
	return normalizeHookParameters(raw)
}

func normalizeHookParameters(raw string) (string, error) {
	invalid := errors.New("parameters_json must be a UTF-8 JSON object within 16 KiB, depth 8 and 256 total keys, without duplicate keys or trailing data")
	if len(raw) > 16*1024 || !utf8.ValidString(raw) {
		return "", invalid
	}
	if strings.TrimSpace(raw) == "" {
		return "{}", nil
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return "", invalid
	}
	keys := 0
	if err := scanHookJSONContainer(decoder, '{', 1, &keys); err != nil {
		return "", invalid
	}
	if _, err := decoder.Token(); err != io.EOF {
		return "", invalid
	}
	// Validation never re-encodes caller numbers, escapes or whitespace.
	return raw, nil
}

func scanHookJSONContainer(decoder *json.Decoder, opening json.Delim, depth int, keys *int) error {
	if depth > 8 {
		return errors.New("JSON depth exceeded")
	}
	seen := make(map[string]struct{})
	for decoder.More() {
		if opening == '{' {
			token, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := token.(string)
			if !ok {
				return errors.New("JSON object key required")
			}
			if _, exists := seen[key]; exists {
				return errors.New("duplicate JSON key")
			}
			seen[key] = struct{}{}
			*keys++
			if *keys > 256 {
				return errors.New("JSON key limit exceeded")
			}
		}
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		if delimiter, ok := token.(json.Delim); ok {
			if delimiter != '{' && delimiter != '[' {
				return errors.New("unexpected JSON delimiter")
			}
			if err := scanHookJSONContainer(decoder, delimiter, depth+1, keys); err != nil {
				return err
			}
		}
	}
	closing := json.Delim('}')
	if opening == '[' {
		closing = ']'
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if token != closing {
		return errors.New("unclosed JSON container")
	}
	return nil
}
