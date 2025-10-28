// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCozeLoopSnippetParser_ParseReferences(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content string
		want    []*SnippetReference
		wantErr bool
	}{
		{
			name:    "empty content",
			content: "",
			want:    nil,
		},
		{
			name:    "single reference with version",
			content: "prefix <cozeloop_snippet>id=123&version=v1</cozeloop_snippet> suffix",
			want: []*SnippetReference{{
				PromptID:      123,
				CommitVersion: "v1",
			}},
		},
		{
			name:    "multiple references",
			content: "<cozeloop_snippet>id=1&version=v1</cozeloop_snippet> text <cozeloop_snippet>id=2&version=v2</cozeloop_snippet>",
			want: []*SnippetReference{
				{PromptID: 1, CommitVersion: "v1"},
				{PromptID: 2, CommitVersion: "v2"},
			},
		},
		{
			name:    "reference without version",
			content: "<cozeloop_snippet>id=5&version=</cozeloop_snippet>",
			want: []*SnippetReference{{
				PromptID:      5,
				CommitVersion: "",
			}},
		},
		{
			name:    "non matching pattern",
			content: "<cozeloop_snippet>id=abc&version=v1</cozeloop_snippet>",
			want:    nil,
		},
	}

	for _, tt := range tests {
		ttt := tt
		t.Run(ttt.name, func(t *testing.T) {
			t.Parallel()
			parser := NewCozeLoopSnippetParser()

			refs, err := parser.ParseReferences(ttt.content)
			if ttt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, ttt.want, refs)
		})
	}
}

func TestCozeLoopSnippetParser_SerializeReference(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ref  *SnippetReference
		want string
	}{
		{
			name: "with version",
			ref: &SnippetReference{
				PromptID:      10,
				CommitVersion: "v1",
			},
			want: "<cozeloop_snippet>id=10&version=v1</cozeloop_snippet>",
		},
		{
			name: "empty version",
			ref: &SnippetReference{
				PromptID:      20,
				CommitVersion: "",
			},
			want: "<cozeloop_snippet>id=20&version=</cozeloop_snippet>",
		},
	}

	for _, tt := range tests {
		ttt := tt
		t.Run(ttt.name, func(t *testing.T) {
			t.Parallel()
			parser := NewCozeLoopSnippetParser()
			got := parser.SerializeReference(ttt.ref)
			assert.Equal(t, ttt.want, got)
		})
	}
}
