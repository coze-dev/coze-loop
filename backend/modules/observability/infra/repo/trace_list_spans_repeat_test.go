// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/dao"
	daomock "github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/dao/mocks"
)

// 说明：针对 ListSpansRepeat 的UT，风格与现有 trace_test.go 保持一致。
func TestTraceRepoImpl_ListSpansRepeat(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	spansDaoMock := daomock.NewMockISpansDao(ctrl)
	annoDaoMock := daomock.NewMockIAnnotationDao(ctrl)
	traceConfigMock := confmocks.NewMockITraceConfig(ctrl)

	// 返回租户表配置，避免 ListSpans 内部报错
	traceConfigMock.EXPECT().GetTenantConfig(gomock.Any()).Return(&config.TenantCfg{
		TenantTables: map[string]map[loop_span.TTL]config.TableCfg{
			"tenant": {
				loop_span.TTL3d: {
					SpanTable: "spans",
				},
			},
		},
		TenantsSupportAnnotation: map[string]bool{},
	}, nil).MinTimes(1)

	// 第一次分页：返回 limit+1 条，触发 HasMore=true 和 PageToken 生成
	spansDaoMock.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p *dao.QueryParam) ([]*dao.Span, error) {
			// 验证：ListSpansRepeat 会强制按 StartTime 倒序
			assert.True(t, p.OrderByStartTime)
			// 验证：SelectColumns 中会包含 start_time
			foundStart := false
			for _, c := range p.SelectColumns {
				if c == loop_span.SpanFieldStartTime {
					foundStart = true
					break
				}
			}
			assert.True(t, foundStart)

			// 返回 p.Limit 条（即 req.Limit+1）数据，最后一条用于翻页
			n := int(p.Limit)
			spans := make([]*dao.Span, 0, n)
			ids := []string{"s1", "s2", "s3", "sX"}
			for i := 0; i < n && i < len(ids); i++ {
				spans = append(spans, &dao.Span{SpanID: ids[i]})
			}
			return spans, nil
		}).Times(1)

	// 第二次分页：返回少于 limit 的数据，结束循环。包含与第一页重复的ID以测试 Uniq()
	spansDaoMock.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ *dao.QueryParam) ([]*dao.Span, error) {
			return []*dao.Span{
				{SpanID: "s3"},
				{SpanID: "s4"},
			}, nil
		}).Times(1)

	repoImpl, err := NewTraceRepoImpl(
		traceConfigMock,
		&mockStorageProvider{},
		nil,
		nil,
		nil,
		nil,
		WithTraceStorageDaos("ck", spansDaoMock, annoDaoMock),
	)
	assert.NoError(t, err)

	// 初始 SelectColumns 不包含 start_time，以覆盖追加逻辑；初始 DescByStartTime=false
	req := &repo.ListSpansParam{
		WorkSpaceID:     "",
		Tenants:         []string{"tenant"},
		StartAt:         0,
		EndAt:           0,
		Limit:           3,
		DescByStartTime: false,
		SelectColumns:   []string{"trace_id"},
	}

	res, err := repoImpl.ListSpansRepeat(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.False(t, res.HasMore)
	assert.Equal(t, "", res.PageToken)

	// totalSpans.Uniq() 后应为 s1, s2, s3, sX, s4 共5个唯一Span
	gotIDs := make([]string, 0, len(res.Spans))
	for _, s := range res.Spans {
		gotIDs = append(gotIDs, s.SpanID)
	}
	assert.ElementsMatch(t, []string{"s1", "s2", "s3", "s4"}, gotIDs)

	// 验证：入口 req 已被置为按开始时间倒序
	assert.True(t, req.DescByStartTime)
	// 验证：入口 req 的 SelectColumns 已包含 start_time
	containsStart := false
	for _, c := range req.SelectColumns {
		if c == loop_span.SpanFieldStartTime {
			containsStart = true
			break
		}
	}
	assert.True(t, containsStart)
}

func TestTraceRepoImpl_ListSpansRepeat_NilRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	traceConfigMock := confmocks.NewMockITraceConfig(ctrl)
	repoImpl, err := NewTraceRepoImpl(
		traceConfigMock,
		&mockStorageProvider{},
		nil,
		nil,
		nil,
		nil,
	)
	assert.NoError(t, err)

	res, err := repoImpl.ListSpansRepeat(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, res)
}
