// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluation_set

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/common"
	openapi_eval_set "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/eval_set"
)

func TestOpenAPIFieldDataOmittedCandidate(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		omitted *bool
		want    bool
	}{
		{name: "absent"},
		{name: "explicit false", omitted: gptr.Of(false)},
		{name: "omitted", omitted: gptr.Of(true), want: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			content := &common.Content{
				ContentType:    gptr.Of(common.ContentTypeText),
				Text:           gptr.Of(`{"detail":{},"rounds":{"source_round_1":{}}}`),
				ContentOmitted: tt.omitted,
			}
			field := OpenAPIFieldDataDTO2DO(&openapi_eval_set.FieldData{
				Name:    gptr.Of("FORNAX_output"),
				Content: content,
			})

			require.Equal(t, tt.want, field.Content.IsContentOmitted())
			require.Equal(t, tt.omitted, field.Content.ContentOmitted)
			require.Equal(t, content.Text, field.Content.Text)
		})
	}
}
