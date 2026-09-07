// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import "strings"

// NormalizeDatasetKey removes runtime and schema suffixes from dataset identities.
// Removing all trailing suffixes keeps normalization idempotent across output paths.
func NormalizeDatasetKey(key string) string {
	for {
		normalized := strings.TrimSuffix(key, "_new_runtime")
		normalized = strings.TrimSuffix(normalized, "_schema_v0_1")
		if normalized == key {
			return key
		}
		key = normalized
	}
}
