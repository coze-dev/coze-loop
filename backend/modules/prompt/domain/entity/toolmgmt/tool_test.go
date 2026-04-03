// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package toolmgmt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPublicDraftVersion(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "$PublicDraft", PublicDraftVersion)
}

func TestCommitInfo_IsPublicDraft(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{
			name:    "version equals PublicDraftVersion",
			version: PublicDraftVersion,
			want:    true,
		},
		{
			name:    "version is 1.0.0",
			version: "1.0.0",
			want:    false,
		},
		{
			name:    "empty version",
			version: "",
			want:    false,
		},
	}

	for _, tt := range tests {
		ttt := tt
		t.Run(ttt.name, func(t *testing.T) {
			t.Parallel()
			c := CommitInfo{Version: ttt.version}
			assert.Equal(t, ttt.want, c.IsPublicDraft())
		})
	}
}
