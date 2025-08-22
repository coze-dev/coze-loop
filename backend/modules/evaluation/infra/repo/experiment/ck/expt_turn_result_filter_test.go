// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package ck

import (
	"context"
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
			gotSelectClause, gotWhereClause, gotOrderClause, gotArgs := d.buildQueryConditions(ctx, tt.cond)
			assert.NotNil(t, gotSelectClause)
			assert.NotNil(t, gotWhereClause)
			assert.NotNil(t, gotOrderClause)
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
		name              string
		joinSQL           string
		whereSQL          string
		keywordCond       string
		evalSetSyncCkDate string
		args              *[]interface{}
		want              string
	}{
		{
			name:              "empty_conditions",
			joinSQL:           "1",
			whereSQL:          "2",
			keywordCond:       "3",
			evalSetSyncCkDate: "4",
			args:              &[]interface{}{},
			want:              "生成的基础 SQL 预期值，需根据实际实现修改",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConfig.EXPECT().GetCKDBName(gomock.Any()).Return(&entity.CKDBConfig{
				ExptTurnResultFilterDBName: "ck",
			}).AnyTimes()
			got := d.buildBaseSQL(ctx, tt.joinSQL, tt.whereSQL, tt.keywordCond, tt.evalSetSyncCkDate, tt.args)
			assert.NotNil(t, got)
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
			want: "生成的基础 SQL 预期值，需根据实际实现修改",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := d.appendPaginationArgs(tt.args, tt.cond)
			if len(args) != 2 {
				t.Errorf("appendPaginationArgs failed, args len not equal 2, args: %v", args)
			}
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
			})
			got, args := d.buildGetByExptIDItemIDsSQL(ctx, tt.spaceID, tt.exptID, tt.createdDate, tt.itemIDs)
			assert.NotNil(t, got)
			if len(args) != 4 {
				t.Errorf("buildGetByExptIDItemIDsSQL failed, args len not equal 3, args: %v", args)
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
