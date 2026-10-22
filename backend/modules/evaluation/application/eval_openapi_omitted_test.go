// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"
	"unicode/utf8"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/eval_set"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

func TestBatchCreateOmittedCandidateReachesPreparation(t *testing.T) {
	t.Parallel()
	asciiText := `{"detail":{},"rounds":{"source_round_1":{}}}`
	utf8Text := `{"detail":{"text":"完整内容"},"rounds":{"source_round_1":{}}}`

	for _, tt := range []struct {
		name      string
		text      string
		omitted   *bool
		fullBytes *int32
		wantErr   bool
	}{
		{name: "absent", text: asciiText},
		{name: "explicit false", text: asciiText, omitted: gptr.Of(false)},
		{name: "omitted without byte count", text: asciiText, omitted: gptr.Of(true), wantErr: true},
		{name: "hydrated ASCII", text: asciiText, omitted: gptr.Of(true), fullBytes: gptr.Of(int32(len(asciiText)))},
		{name: "mismatched ASCII bytes", text: asciiText, omitted: gptr.Of(true), fullBytes: gptr.Of(int32(len(asciiText) + 1)), wantErr: true},
		{name: "hydrated UTF8", text: utf8Text, omitted: gptr.Of(true), fullBytes: gptr.Of(int32(len(utf8Text)))},
		{name: "UTF8 rune count is not bytes", text: utf8Text, omitted: gptr.Of(true), fullBytes: gptr.Of(int32(utf8.RuneCountInString(utf8Text))), wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			sets := servicemocks.NewMockIEvaluationSetService(ctrl)
			items := servicemocks.NewMockEvaluationSetItemService(ctrl)
			sets.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), int64(2), gomock.Nil(), gomock.Nil()).Return(&entity.EvaluationSet{ID: 2, SpaceID: 1}, nil)
			auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
			rejected := errors.New("candidate must be loaded in full before preparation")
			items.EXPECT().BatchCreateEvaluationSetItems(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, param *entity.BatchCreateEvaluationSetItemsParam) (map[int64]int64, []*entity.ItemErrorGroup, []*entity.DatasetItemOutput, error) {
					require.Len(t, param.Items, 1)
					require.Len(t, param.Items[0].Turns, 1)
					field := param.Items[0].Turns[0].FieldDataList[0]
					require.Equal(t, "FORNAX_output", field.Name)
					if field.Content.IsContentOmitted() {
						return nil, nil, nil, rejected
					}
					return nil, nil, nil, nil
				})
			app := &EvalOpenAPIApplication{
				auth:                     auth,
				evaluationSetService:     sets,
				evaluationSetItemService: items,
				metric:                   &fakeOpenAPIMetric{},
			}
			resp, err := app.BatchCreateEvaluationSetItemsOApi(context.Background(), &openapi.BatchCreateEvaluationSetItemsOApiRequest{
				WorkspaceID:     gptr.Of(int64(1)),
				EvaluationSetID: gptr.Of(int64(2)),
				Items: []*eval_set.EvaluationSetItem{{
					ItemKey: gptr.Of("omitted-candidate"),
					Turns: []*eval_set.Turn{{FieldDatas: []*eval_set.FieldData{{
						Name: gptr.Of("FORNAX_output"),
						Content: &common.Content{
							ContentType:      gptr.Of(common.ContentTypeText),
							Text:             gptr.Of(tt.text),
							ContentOmitted:   tt.omitted,
							FullContentBytes: tt.fullBytes,
						},
					}}}},
				}},
			})
			if tt.wantErr {
				require.ErrorIs(t, err, rejected)
				require.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
			}
		})
	}
}
