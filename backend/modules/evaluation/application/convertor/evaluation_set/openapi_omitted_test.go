// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluation_set

import (
	"testing"
	"unicode/utf8"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/common"
	openapi_eval_set "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/eval_set"
)

func TestOpenAPIFieldDataOmittedCandidate(t *testing.T) {
	t.Parallel()
	asciiText := `{"detail":{},"rounds":{"source_round_1":{}}}`
	utf8Text := `{"detail":{"text":"完整内容"},"rounds":{"source_round_1":{}}}`

	for _, tt := range []struct {
		name      string
		text      string
		omitted   *bool
		fullBytes *int32
		want      bool
	}{
		{name: "absent", text: asciiText},
		{name: "explicit false", text: asciiText, omitted: gptr.Of(false)},
		{name: "omitted without byte count", text: asciiText, omitted: gptr.Of(true), want: true},
		{name: "hydrated ASCII", text: asciiText, omitted: gptr.Of(true), fullBytes: gptr.Of(int32(len(asciiText)))},
		{name: "mismatched ASCII bytes", text: asciiText, omitted: gptr.Of(true), fullBytes: gptr.Of(int32(len(asciiText) + 1)), want: true},
		{name: "hydrated UTF8", text: utf8Text, omitted: gptr.Of(true), fullBytes: gptr.Of(int32(len(utf8Text)))},
		{name: "UTF8 rune count is not bytes", text: utf8Text, omitted: gptr.Of(true), fullBytes: gptr.Of(int32(utf8.RuneCountInString(utf8Text))), want: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			content := &common.Content{
				ContentType:      gptr.Of(common.ContentTypeText),
				Text:             gptr.Of(tt.text),
				ContentOmitted:   tt.omitted,
				FullContentBytes: tt.fullBytes,
			}
			field := OpenAPIFieldDataDTO2DO(&openapi_eval_set.FieldData{
				Name:    gptr.Of("FORNAX_output"),
				Content: content,
			})

			require.Equal(t, tt.want, field.Content.IsContentOmitted())
			require.Equal(t, tt.omitted, field.Content.ContentOmitted)
			require.Equal(t, tt.fullBytes, field.Content.FullContentBytes)
			require.Equal(t, content.Text, field.Content.Text)
		})
	}
}
