// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

type cursorPagingSource struct {
	EvaluationSetItemService
	t     *testing.T
	pages int
	calls int
}

func (s *cursorPagingSource) ListEvaluationSetItems(_ context.Context, p *entity.ListEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, *int64, *int64, *string, error) {
	require.Equal(s.t, int32(100), gptr.Indirect(p.PageSize))
	page := 0
	if p.PageToken != nil {
		var err error
		page, err = strconv.Atoi(*p.PageToken)
		require.NoError(s.t, err)
	}
	require.GreaterOrEqual(s.t, page, 0)
	require.Less(s.t, page, s.pages)
	s.calls++
	var next *string
	if page+1 < s.pages {
		next = gptr.Of(strconv.Itoa(page + 1))
	}
	return []*entity.EvaluationSetItem{{ItemID: p.EvaluationSetID*100000 + int64(page+1)}}, gptr.Of(int64(s.pages)), nil, next, nil
}

func cursorFilterOnlyLastItem() *entity.ExptItemFilter {
	values := make([]string, 1001)
	for i := range values {
		values[i] = strconv.Itoa(1010000 + i)
	}
	return &entity.ExptItemFilter{FilterFields: []*entity.ExptItemFilterField{{FieldName: "item_id", FieldType: "long", QueryType: "in", Values: values}}}
}

func TestHookSelectionCursorTenThousandPagesAndResume(t *testing.T) {
	for _, multi := range []bool{false, true} {
		t.Run(strconv.FormatBool(multi), func(t *testing.T) {
			source := &cursorPagingSource{t: t, pages: 10000}
			in := selectionInput(entity.EvaluationModeSubmit)
			if multi {
				in.Experiment.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
				in.Experiment.EvalConf = &entity.EvaluationConfiguration{EvalSetConfigs: []*entity.EvalSetConfig{{EvalSetID: 10, EvalSetVersionID: 20, SourceSpaceID: 8, ItemFilter: cursorFilterOnlyLastItem()}}}
			}
			selector := NewHookPlanSelector(source, nil, nil, nil)
			maxCursor, selected := 0, 0
			for page := 1; page <= 10000; page++ {
				if page%997 == 0 {
					selector = NewHookPlanSelector(source, nil, nil, nil)
				}
				out, err := selector.SelectPage(context.Background(), in)
				require.NoError(t, err, "legal page %d; prior cursor %d bytes", page, len(in.Cursor))
				require.Equal(t, page == 10000, out.Done, "page %d", page)
				if !multi || page == 10000 {
					require.Equal(t, []entity.HookPlanItem{{SourceSpaceID: 8, EvalSetID: 10, EvalSetVersionID: 20, ItemID: 1000000 + int64(page)}}, out.Items)
				} else {
					require.Empty(t, out.Items, "filtered empty page must still advance")
				}
				selected += len(out.Items)
				if len(out.NextCursor) > maxCursor {
					maxCursor = len(out.NextCursor)
				}
				in.Cursor = out.NextCursor
			}
			require.Equal(t, 10000, source.calls)
			if multi {
				require.Equal(t, 1, selected)
			} else {
				require.Equal(t, 10000, selected)
			}
			require.LessOrEqual(t, maxCursor, 1024, "cursor must not grow with page history")
			terminal, err := NewHookPlanSelector(source, nil, nil, nil).SelectPage(context.Background(), in)
			require.NoError(t, err)
			require.True(t, terminal.Done)
			require.Empty(t, terminal.Items)
			require.Equal(t, 10000, source.calls)
			t.Logf("pages=10000 selected=%d max_cursor_bytes=%d", selected, maxCursor)
		})
	}
}

func TestHookSelectionCursorResetsAcrossFullSizedSets(t *testing.T) {
	source := &cursorPagingSource{t: t, pages: 10000}
	in := selectionInput(entity.EvaluationModeSubmit)
	in.Experiment.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
	in.Experiment.EvalConf = &entity.EvaluationConfiguration{EvalSetConfigs: []*entity.EvalSetConfig{{EvalSetID: 10, EvalSetVersionID: 20, SourceSpaceID: 8}, {EvalSetID: 11, EvalSetVersionID: 21}}}
	maxCursor := 0
	for set := int64(10); set <= 11; set++ {
		for page := 1; page <= 10000; page++ {
			out, err := NewHookPlanSelector(source, nil, nil, nil).SelectPage(context.Background(), in)
			require.NoError(t, err, "set %d page %d", set, page)
			require.Len(t, out.Items, 1)
			require.Equal(t, set, out.Items[0].EvalSetID)
			require.Equal(t, set*100000+int64(page), out.Items[0].ItemID)
			require.Equal(t, set == 11 && page == 10000, out.Done)
			if len(out.NextCursor) > maxCursor {
				maxCursor = len(out.NextCursor)
			}
			in.Cursor = out.NextCursor
		}
	}
	require.Equal(t, 20000, source.calls)
	require.LessOrEqual(t, maxCursor, 1024)
	t.Logf("two_sets_pages=20000 max_cursor_bytes=%d", maxCursor)
}

type cursorCycleSource struct {
	EvaluationSetItemService
	t     *testing.T
	next  map[string]string
	calls int
	total *int64
}

func (s *cursorCycleSource) ListEvaluationSetItems(_ context.Context, p *entity.ListEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, *int64, *int64, *string, error) {
	token := gptr.Indirect(p.PageToken)
	next, ok := s.next[token]
	require.True(s.t, ok, "unexpected source token %q", token)
	s.calls++
	return []*entity.EvaluationSetItem{{ItemID: 7}}, s.total, nil, gptr.Of(next), nil
}

func TestHookSelectionCursorCyclesFailAfterResume(t *testing.T) {
	for _, multi := range []bool{false, true} {
		for _, tc := range []struct {
			name     string
			next     map[string]string
			minReads int
		}{
			{"self", map[string]string{"": "a", "a": "a"}, 2},
			{"two", map[string]string{"": "a", "a": "b", "b": "a"}, 3},
			{"prefix_and_five", map[string]string{"": "x", "x": "y", "y": "a", "a": "b", "b": "c", "c": "d", "d": "e", "e": "a"}, 8},
		} {
			t.Run(strconv.FormatBool(multi)+"/"+tc.name, func(t *testing.T) {
				source := &cursorCycleSource{t: t, next: tc.next}
				in := selectionInput(entity.EvaluationModeSubmit)
				if multi {
					in.Experiment.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
					in.Experiment.EvalConf = &entity.EvaluationConfiguration{EvalSetConfigs: []*entity.EvalSetConfig{{EvalSetID: 10, EvalSetVersionID: 20, ItemFilter: cursorFilterOnlyLastItem()}}}
				}
				for page := 1; page <= 32; page++ {
					out, err := NewHookPlanSelector(source, nil, nil, nil).SelectPage(context.Background(), in)
					require.False(t, out.Done, "a loop must never be reported as ready")
					if err != nil {
						require.GreaterOrEqual(t, source.calls, tc.minReads, "do not reject a still-unique token sequence")
						if tc.name == "self" {
							require.Equal(t, 2, source.calls)
						}
						t.Logf("cycle rejected after %d reads", source.calls)
						return
					}
					if multi {
						require.Empty(t, out.Items)
					}
					require.LessOrEqual(t, len(out.NextCursor), 1024)
					in.Cursor = out.NextCursor
				}
				t.Fatal("cycle did not fail within the bounded probe")
			})
		}
	}
}

func TestHookSelectionCursorPageLimitIsNotSuccess(t *testing.T) {
	for _, multi := range []bool{false, true} {
		t.Run(strconv.FormatBool(multi), func(t *testing.T) {
			source := &cursorPagingSource{t: t, pages: 10001}
			in := selectionInput(entity.EvaluationModeSubmit)
			if multi {
				in.Experiment.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
				in.Experiment.EvalConf = &entity.EvaluationConfiguration{EvalSetConfigs: []*entity.EvalSetConfig{{EvalSetID: 10, EvalSetVersionID: 20}}}
			}
			for page := 1; page <= 10000; page++ {
				out, err := NewHookPlanSelector(source, nil, nil, nil).SelectPage(context.Background(), in)
				require.False(t, out.Done)
				if page == 10000 {
					require.Error(t, err, "nonterminal budget exhaustion must not freeze a partial plan")
					require.Empty(t, out.Items)
					require.Equal(t, 10000, source.calls)
					return
				}
				require.NoError(t, err)
				in.Cursor = out.NextCursor
			}
		})
	}
}

func TestHookSelectionCursorCycleCannotFinishAtReportedTotal(t *testing.T) {
	for _, multi := range []bool{false, true} {
		t.Run(strconv.FormatBool(multi), func(t *testing.T) {
			source := &cursorCycleSource{t: t, next: map[string]string{"": "a", "a": "b", "b": "a"}, total: gptr.Of(int64(3))}
			in := selectionInput(entity.EvaluationModeSubmit)
			if multi {
				in.Experiment.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
				in.Experiment.EvalConf = &entity.EvaluationConfiguration{EvalSetConfigs: []*entity.EvalSetConfig{{EvalSetID: 10, EvalSetVersionID: 20, ItemFilter: cursorFilterOnlyLastItem()}}}
			}
			for page := 1; page <= 8; page++ {
				out, err := NewHookPlanSelector(source, nil, nil, nil).SelectPage(context.Background(), in)
				require.False(t, out.Done, "reported total must not turn a still-looping source into success")
				if err != nil {
					return
				}
				in.Cursor = out.NextCursor
			}
			t.Fatal("looping source must fail")
		})
	}
}

func TestHookSelectionCursorRejectsCorruptCycleStateBeforeRead(t *testing.T) {
	source := &cursorPagingSource{t: t, pages: 2}
	in := selectionInput(entity.EvaluationModeSubmit)
	page, err := NewHookPlanSelector(source, nil, nil, nil).SelectPage(context.Background(), in)
	require.NoError(t, err)
	var valid hookSelectionCursor
	require.NoError(t, json.Unmarshal([]byte(page.NextCursor), &valid))
	for _, mutate := range []func(*hookSelectionCursor){
		func(c *hookSelectionCursor) { c.Version = 1 },
		func(c *hookSelectionCursor) { c.Pages = -1 },
		func(c *hookSelectionCursor) { c.Pages = 10000 },
		func(c *hookSelectionCursor) { c.CyclePower = 3 },
		func(c *hookSelectionCursor) { c.CyclePower = 1 << 30 },
		func(c *hookSelectionCursor) { c.CycleSpan = -1 },
		func(c *hookSelectionCursor) { c.CycleSpan = 1 },
		func(c *hookSelectionCursor) { c.CycleAnchor = "bad" },
		func(c *hookSelectionCursor) { c.Token = "different" },
		func(c *hookSelectionCursor) { c.Token = "" },
	} {
		changed := valid
		mutate(&changed)
		data, err := json.Marshal(changed)
		require.NoError(t, err)
		in.Cursor = string(data)
		_, err = NewHookPlanSelector(source, nil, nil, nil).SelectPage(context.Background(), in)
		require.Error(t, err)
	}
	require.Equal(t, 1, source.calls)
}
