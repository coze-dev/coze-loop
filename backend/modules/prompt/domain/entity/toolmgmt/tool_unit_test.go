package toolmgmt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommitInfo_IsPublicDraft(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		version  string
		expected bool
	}{
		{name: "public draft version", version: PublicDraftVersion, expected: true},
		{name: "normal version", version: "1.0.0", expected: false},
		{name: "empty version", version: "", expected: false},
		{name: "similar but not exact", version: "$PublicDraf", expected: false},
		{name: "with extra suffix", version: PublicDraftVersion + "x", expected: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ci := CommitInfo{Version: tt.version}
			assert.Equal(t, tt.expected, ci.IsPublicDraft())
		})
	}
}

func TestPublicDraftVersionConst(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "$PublicDraft", PublicDraftVersion)
}
