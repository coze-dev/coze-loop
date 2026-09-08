// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import "testing"

func TestNormalizeDatasetKey(t *testing.T) {
	for _, tt := range []struct {
		key  string
		want string
	}{
		{key: "", want: ""},
		{key: "dataset", want: "dataset"},
		{key: "dataset_new_runtime", want: "dataset"},
		{key: "dataset_schema_v0_1", want: "dataset"},
		{key: "dataset_new_runtime_schema_v0_1", want: "dataset"},
		{key: "dataset_schema_v0_1_new_runtime", want: "dataset"},
		{key: "dataset_new_runtime_new_runtime_schema_v0_1_schema_v0_1", want: "dataset"},
		{key: "_new_runtime_schema_v0_1", want: ""},
		{key: "dataset_new_runtime_part", want: "dataset_new_runtime_part"},
		{key: "dataset_schema_v0_1_part", want: "dataset_schema_v0_1_part"},
		{key: "dataset_schema_v0_10", want: "dataset_schema_v0_10"},
		{key: "dataset_new_runtime ", want: "dataset_new_runtime "},
		{key: " dataset_new_runtime", want: " dataset"},
	} {
		t.Run(tt.key, func(t *testing.T) {
			got := NormalizeDatasetKey(tt.key)
			if got != tt.want {
				t.Errorf("NormalizeDatasetKey(%q) = %q, want %q", tt.key, got, tt.want)
			}
			if repeated := NormalizeDatasetKey(got); repeated != got {
				t.Errorf("normalizing %q again changed it to %q", got, repeated)
			}
		})
	}
}
