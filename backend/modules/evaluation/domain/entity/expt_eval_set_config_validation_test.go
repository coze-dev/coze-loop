// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateEvalSetConfigs(t *testing.T) {
	validEvaluator := func(verID int64, alias string) *ExptEvaluatorConf {
		return &ExptEvaluatorConf{EvaluatorVersionID: verID, Alias: alias}
	}

	tests := []struct {
		name    string
		configs []*EvalSetConfig
		wantErr bool
	}{
		{
			name:    "empty configs (老路径) 放行",
			configs: nil,
			wantErr: false,
		},
		{
			name: "合法: 单 set 单 evaluator",
			configs: []*EvalSetConfig{
				{EvalSetID: 1, EvalSetVersionID: 11, EvaluatorConfs: []*ExptEvaluatorConf{validEvaluator(100, "")}},
			},
			wantErr: false,
		},
		{
			name: "合法: 同 set 同 version 多 alias",
			configs: []*EvalSetConfig{
				{EvalSetID: 1, EvalSetVersionID: 11, EvaluatorConfs: []*ExptEvaluatorConf{
					validEvaluator(100, "judge_A"),
					validEvaluator(100, "judge_B"),
				}},
			},
			wantErr: false,
		},
		{
			name: "非法: eval_set_id 缺失",
			configs: []*EvalSetConfig{
				{EvalSetID: 0, EvalSetVersionID: 11},
			},
			wantErr: true,
		},
		{
			name: "非法: set 重复",
			configs: []*EvalSetConfig{
				{EvalSetID: 1, EvalSetVersionID: 11},
				{EvalSetID: 1, EvalSetVersionID: 11},
			},
			wantErr: true,
		},
		{
			name: "非法: (version,alias) set 内重复",
			configs: []*EvalSetConfig{
				{EvalSetID: 1, EvalSetVersionID: 11, EvaluatorConfs: []*ExptEvaluatorConf{
					validEvaluator(100, "judge_A"),
					validEvaluator(100, "judge_A"),
				}},
			},
			wantErr: true,
		},
		{
			name: "非法: target_confs len>1",
			configs: []*EvalSetConfig{
				{EvalSetID: 1, EvalSetVersionID: 11, TargetConfs: []*ExptTargetConf{
					{TargetID: 1}, {TargetID: 2},
				}},
			},
			wantErr: true,
		},
		{
			name: "非法: target_conf alias 非空",
			configs: []*EvalSetConfig{
				{EvalSetID: 1, EvalSetVersionID: 11, TargetConfs: []*ExptTargetConf{
					{TargetID: 1, Alias: "t1"},
				}},
			},
			wantErr: true,
		},
		{
			name: "非法: alias 含非法字符",
			configs: []*EvalSetConfig{
				{EvalSetID: 1, EvalSetVersionID: 11, EvaluatorConfs: []*ExptEvaluatorConf{
					validEvaluator(100, "judge A!"),
				}},
			},
			wantErr: true,
		},
		{
			name: "非法: filter field_type 超白名单",
			configs: []*EvalSetConfig{
				{EvalSetID: 1, EvalSetVersionID: 11, EvaluatorConfs: []*ExptEvaluatorConf{
					{EvaluatorVersionID: 100, Filter: &ExptItemFilter{
						FilterFields: []*ExptItemFilterField{
							{FieldName: "item_id", FieldType: "double", QueryType: "eq"},
						},
					}},
				}},
			},
			wantErr: true,
		},
		{
			name: "非法: filter query_type 超白名单",
			configs: []*EvalSetConfig{
				{EvalSetID: 1, EvalSetVersionID: 11, ItemFilter: &ExptItemFilter{
					FilterFields: []*ExptItemFilterField{
						{FieldName: "item_id", FieldType: "long", QueryType: "gt"},
					},
				}},
			},
			wantErr: true,
		},
		{
			name: "非法: item_id in 但 values 为空",
			configs: []*EvalSetConfig{
				{EvalSetID: 1, EvalSetVersionID: 11, ItemFilter: &ExptItemFilter{
					FilterFields: []*ExptItemFilterField{
						{FieldName: "item_id", FieldType: "long", QueryType: "in", Values: nil},
					},
				}},
			},
			wantErr: true,
		},
		{
			name: "合法: tag key 条件圈选",
			configs: []*EvalSetConfig{
				{EvalSetID: 1, EvalSetVersionID: 11, ItemFilter: &ExptItemFilter{
					QueryAndOr: "and",
					FilterFields: []*ExptItemFilterField{
						{FieldName: "lang", FieldType: "tag", QueryType: "eq", Values: []string{"zh"}},
					},
				}},
			},
			wantErr: false,
		},
		{
			name: "合法: item_id 点选",
			configs: []*EvalSetConfig{
				{EvalSetID: 1, EvalSetVersionID: 11, ItemFilter: &ExptItemFilter{
					FilterFields: []*ExptItemFilterField{
						{FieldName: "item_id", FieldType: "long", QueryType: "in", Values: []string{"1", "2"}},
					},
				}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEvalSetConfigs(tt.configs)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
