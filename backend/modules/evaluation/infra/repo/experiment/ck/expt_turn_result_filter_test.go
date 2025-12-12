// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package ck

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestExptTurnResultFilterDAOImpl_buildQueryConditions(t *testing.T) {
	d := &exptTurnResultFilterDAOImpl{}
	ctx := context.Background()

	tests := []struct {
		name string
		cond *ExptTurnResultFilterQueryCond

		wantArgs []interface{}
	}{
		{
			name: "full_condition",
			cond: &ExptTurnResultFilterQueryCond{
				SpaceID: ptr.Of("1"),
				ExptID:  ptr.Of("1"),
				ItemIDs: []*FieldFilter{
					{Key: "1", Op: "=", Values: []any{"1"}},
					{Key: "2", Op: "!=", Values: []any{"2"}},
					{Key: "3", Op: "in", Values: []any{"3"}},
					{Key: "4", Op: "NOT IN", Values: []any{"4"}},
					{Key: "5", Op: "between", Values: []any{"5", "6"}},
				},
				ItemRunStatus: []*FieldFilter{
					{Key: "1", Op: "!=", Values: []any{"1"}},
					{Key: "2", Op: "in", Values: []any{"2"}},
					{Key: "3", Op: "NOT IN", Values: []any{"3"}},
					{Key: "4", Op: "between", Values: []any{"4", "5"}},
					{Key: "5", Op: "=", Values: []any{"5"}},
				},
				EvaluatorScoreCorrected: &FieldFilter{Key: "1", Op: "NOT IN", Values: []any{"1"}},
				CreatedDate:             ptr.Of(time.Now()),
				EvalSetVersionID:        ptr.Of("1"),
				MapCond: &ExptTurnResultFilterMapCond{
					EvalTargetDataFilters: []*FieldFilter{
						{Key: "2", Op: "!=", Values: []any{"2"}},
						{Key: "3", Op: "in", Values: []any{"3"}},
						{Key: "4", Op: "LIKE", Values: []any{"4", "5"}},
						{Key: "5", Op: "NOT LIKE", Values: []any{"5"}},
					},
					EvaluatorScoreFilters: []*FieldFilter{
						{Key: "1", Op: "=", Values: []any{"1"}},
						{Key: "2", Op: "!=", Values: []any{"2"}},
						{Key: "3", Op: "BETWEEN", Values: []any{"3", "4"}},
					},
					AnnotationFloatFilters: []*FieldFilter{
						{Key: "1", Op: "=", Values: []any{"1"}},
						{Key: "2", Op: "!=", Values: []any{"2"}},
						{Key: "3", Op: "BETWEEN", Values: []any{"3", "4"}},
					},
					AnnotationStringFilters: []*FieldFilter{
						{Key: "1", Op: "=", Values: []any{"1"}},
						{Key: "2", Op: "!=", Values: []any{"2"}},
						{Key: "3", Op: "in", Values: []any{"3"}},
						{Key: "4", Op: "LIKE", Values: []any{"4", "5"}},
						{Key: "5", Op: "NOT LIKE", Values: []any{"5"}},
						{Key: "6", Op: "NOT IN", Values: []any{"3"}},
					},
					EvalTargetMetricsFilters: []*FieldFilter{
						{Key: "total_latency", Op: "=", Values: []any{"100"}},
						{Key: "input_tokens", Op: ">", Values: []any{"10"}},
						{Key: "output_tokens", Op: "<=", Values: []any{"20"}},
						{Key: "total_tokens", Op: "BETWEEN", Values: []any{"30", "40"}},
						{Key: "input_tokens", Op: "IN", Values: []any{"50", "60"}},
						{Key: "output_tokens", Op: "NOT IN", Values: []any{"70", "80"}},
					},
				},
				ItemSnapshotCond: &ItemSnapshotFilter{
					BoolMapFilters: []*FieldFilter{
						{Key: "1", Op: "=", Values: []any{"true"}},
						{Key: "2", Op: "!=", Values: []any{"false"}},
					},
					FloatMapFilters: []*FieldFilter{
						{Key: "1", Op: "=", Values: []any{"1"}},
						{Key: "2", Op: "!=", Values: []any{"2"}},
						{Key: "3", Op: "BETWEEN", Values: []any{"3", "4"}},
					},
					IntMapFilters: []*FieldFilter{
						{Key: "1", Op: "=", Values: []any{"1"}},
						{Key: "2", Op: "!=", Values: []any{"2"}},
						{Key: "3", Op: "BETWEEN", Values: []any{"3", "4"}},
					},
					StringMapFilters: []*FieldFilter{
						{Key: "1", Op: "=", Values: []any{"1"}},
						{Key: "2", Op: "!=", Values: []any{"2"}},
						{Key: "3", Op: "LIKE", Values: []any{"3"}},
						{Key: "4", Op: "NOT LIKE", Values: []any{"4"}},
					},
				},
				EvalSetSyncCkDate: "1",
				KeywordSearch: &KeywordMapCond{
					Keyword: ptr.Of("1"),
					EvalTargetDataFilters: []*FieldFilter{
						{Key: "1", Op: "=", Values: []any{"1"}},
					},
					ItemSnapshotFilter: &ItemSnapshotFilter{
						BoolMapFilters: []*FieldFilter{
							{Key: "1", Op: "=", Values: []any{"true"}},
							{Key: "2", Op: "!=", Values: []any{"false"}},
						},
						FloatMapFilters: []*FieldFilter{
							{Key: "1", Op: "=", Values: []any{"1"}},
							{Key: "2", Op: "!=", Values: []any{"2"}},
							{Key: "3", Op: "BETWEEN", Values: []any{"3", "4"}},
						},
						IntMapFilters: []*FieldFilter{
							{Key: "1", Op: "=", Values: []any{"1"}},
							{Key: "2", Op: "!=", Values: []any{"2"}},
						},
						StringMapFilters: []*FieldFilter{
							{Key: "1", Op: "=", Values: []any{"1"}},
							{Key: "2", Op: "!=", Values: []any{"2"}},
							{Key: "3", Op: "LIKE", Values: []any{"3"}},
							{Key: "4", Op: "NOT LIKE", Values: []any{"4"}},
						},
					},
				},
				Page: Page{
					Offset: 0,
					Limit:  10,
				},
			},
			wantArgs: []interface{}{},
		},
		{
			name: "bool_condition",
			cond: &ExptTurnResultFilterQueryCond{
				SpaceID: ptr.Of("1"),
				ExptID:  ptr.Of("1"),
				ItemIDs: []*FieldFilter{
					{Key: "1", Op: "=", Values: []any{"1"}},
					{Key: "2", Op: "!=", Values: []any{"2"}},
					{Key: "3", Op: "in", Values: []any{"3"}},
					{Key: "4", Op: "NOT IN", Values: []any{"4"}},
					{Key: "5", Op: "between", Values: []any{"5", "6"}},
				},
				EvalSetSyncCkDate: "1",
				KeywordSearch: &KeywordMapCond{
					Keyword: ptr.Of("true"),
					EvalTargetDataFilters: []*FieldFilter{
						{Key: "1", Op: "=", Values: []any{"1"}},
					},
					ItemSnapshotFilter: &ItemSnapshotFilter{
						BoolMapFilters: []*FieldFilter{
							{Key: "1", Op: "=", Values: []any{"true"}},
							{Key: "2", Op: "!=", Values: []any{"false"}},
						},
					},
				},
				Page: Page{
					Offset: 0,
					Limit:  10,
				},
			},
			wantArgs: []interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			whereSQL, keywordCond, gotArgs := d.buildQueryConditions(ctx, tt.cond)
			assert.NotNil(t, whereSQL)
			assert.NotNil(t, keywordCond)
			assert.NotNil(t, gotArgs)
		})
	}
}

func TestExptTurnResultFilterDAOImpl_buildBaseSQL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfig := mocks.NewMockIConfiger(ctrl)
	d := &exptTurnResultFilterDAOImpl{
		configer: mockConfig,
	}
	ctx := context.Background()

	tests := []struct {
		name        string
		whereSQL    string
		keywordCond string
		args        *[]interface{}
		want        string
	}{
		{
			name:        "empty_conditions",
			whereSQL:    "2",
			keywordCond: "3",
			args:        &[]interface{}{},
			want:        "SELECT  etrf.item_id, etrf.status FROM `cozeloop-clickhouse`.expt_turn_result_filter etrf FINAL WHERE 1=123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConfig.EXPECT().GetCKDBName(gomock.Any()).Return(&entity.CKDBConfig{
				ExptTurnResultFilterDBName: "ck",
			}).AnyTimes()
			got := d.buildBaseSQL(ctx, tt.whereSQL, tt.keywordCond, tt.args)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExptTurnResultFilterDAOImpl_appendPaginationArgs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfig := mocks.NewMockIConfiger(ctrl)
	d := &exptTurnResultFilterDAOImpl{
		configer: mockConfig,
	}
	tests := []struct {
		name string
		cond *ExptTurnResultFilterQueryCond
		args []interface{}
		want string
	}{
		{
			name: "empty_conditions",
			cond: &ExptTurnResultFilterQueryCond{
				Page: Page{
					Offset: 0,
					Limit:  10,
				},
			},
			args: []interface{}{},
			want: "LIMIT 10 OFFSET 0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := d.appendPaginationArgs(tt.args, tt.cond)
			assert.Equal(t, tt.want, fmt.Sprintf("LIMIT %d OFFSET %d", args[len(args)-2], args[len(args)-1]))
		})
	}
}

func TestExptTurnResultFilterDAOImpl_buildGetByExptIDItemIDsSQL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfig := mocks.NewMockIConfiger(ctrl)
	d := &exptTurnResultFilterDAOImpl{
		configer: mockConfig,
	}
	ctx := context.Background()
	tests := []struct {
		name        string
		spaceID     string
		exptID      string
		createdDate string
		itemIDs     []string
	}{
		{
			name:        "empty_conditions",
			spaceID:     "1",
			exptID:      "1",
			createdDate: "2025-01-01",
			itemIDs:     []string{"1", "2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConfig.EXPECT().GetCKDBName(gomock.Any()).Return(&entity.CKDBConfig{
				ExptTurnResultFilterDBName: "ck",
			}).AnyTimes()
			got, args := d.buildGetByExptIDItemIDsSQL(ctx, tt.spaceID, tt.exptID, tt.createdDate, tt.itemIDs)
			assert.NotNil(t, got)
			if len(args) != 4 {
				t.Errorf("buildGetByExptIDItemIDsSQL failed, args len not equal 4, args: %v", args)
			}
		})
	}
}

func TestExptTurnResultFilterDAOImpl_parseOutput(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		sql  string
		args []map[string]interface{}
		want map[string]int32
	}{
		{
			name: "empty_conditions",
			args: []map[string]interface{}{
				{
					"item_id": "1",
					"status":  "1",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseOutput(ctx, tt.args)
			assert.NotNil(t, got)
		})
	}
}

func TestExptTurnResultFilterDAOImpl_buildMapFieldConditions_EvalTargetMetricsFilters(t *testing.T) {
	d := &exptTurnResultFilterDAOImpl{}

	tests := []struct {
		name     string
		cond     *ExptTurnResultFilterQueryCond
		wantSQL  string
		wantArgs int
	}{
		{
			name: "eval_target_metrics_equal",
			cond: &ExptTurnResultFilterQueryCond{
				MapCond: &ExptTurnResultFilterMapCond{
					EvalTargetMetricsFilters: []*FieldFilter{
						{Key: "total_latency", Op: "=", Values: []any{"100"}},
					},
				},
			},
			wantSQL:  " AND etrf.eval_target_metrics['total_latency'] = ?",
			wantArgs: 1,
		},
		{
			name: "eval_target_metrics_comparison_ops",
			cond: &ExptTurnResultFilterQueryCond{
				MapCond: &ExptTurnResultFilterMapCond{
					EvalTargetMetricsFilters: []*FieldFilter{
						{Key: "input_tokens", Op: ">", Values: []any{"10"}},
						{Key: "output_tokens", Op: ">=", Values: []any{"20"}},
						{Key: "total_tokens", Op: "<", Values: []any{"30"}},
						{Key: "total_latency", Op: "<=", Values: []any{"40"}},
						{Key: "input_tokens", Op: "!=", Values: []any{"50"}},
					},
				},
			},
			wantSQL:  " AND etrf.eval_target_metrics['input_tokens'] > ? AND etrf.eval_target_metrics['output_tokens'] >= ? AND etrf.eval_target_metrics['total_tokens'] < ? AND etrf.eval_target_metrics['total_latency'] <= ? AND etrf.eval_target_metrics['input_tokens'] != ?",
			wantArgs: 5,
		},
		{
			name: "eval_target_metrics_between",
			cond: &ExptTurnResultFilterQueryCond{
				MapCond: &ExptTurnResultFilterMapCond{
					EvalTargetMetricsFilters: []*FieldFilter{
						{Key: "total_tokens", Op: "BETWEEN", Values: []any{"100", "200"}},
					},
				},
			},
			wantSQL:  " AND etrf.eval_target_metrics['total_tokens'] BETWEEN ? AND ?",
			wantArgs: 2,
		},
		{
			name: "eval_target_metrics_in",
			cond: &ExptTurnResultFilterQueryCond{
				MapCond: &ExptTurnResultFilterMapCond{
					EvalTargetMetricsFilters: []*FieldFilter{
						{Key: "input_tokens", Op: "IN", Values: []any{"10", "20", "30"}},
					},
				},
			},
			wantSQL:  " AND etrf.eval_target_metrics['input_tokens'] IN ?",
			wantArgs: 1,
		},
		{
			name: "eval_target_metrics_not_in",
			cond: &ExptTurnResultFilterQueryCond{
				MapCond: &ExptTurnResultFilterMapCond{
					EvalTargetMetricsFilters: []*FieldFilter{
						{Key: "output_tokens", Op: "NOT IN", Values: []any{"40", "50"}},
					},
				},
			},
			wantSQL:  " AND etrf.eval_target_metrics['output_tokens'] NOT IN ?",
			wantArgs: 1,
		},
		{
			name: "eval_target_metrics_invalid_value",
			cond: &ExptTurnResultFilterQueryCond{
				MapCond: &ExptTurnResultFilterMapCond{
					EvalTargetMetricsFilters: []*FieldFilter{
						{Key: "total_latency", Op: "=", Values: []any{"invalid"}},
					},
				},
			},
			wantSQL:  "",
			wantArgs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			whereSQL := ""
			args := []interface{}{}
			d.buildMapFieldConditions(tt.cond, &whereSQL, &args)
			if tt.wantSQL != "" {
				assert.Contains(t, whereSQL, tt.wantSQL)
			}
			assert.Equal(t, tt.wantArgs, len(args))
		})
	}
}
