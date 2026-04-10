// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	tracedto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/trace"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
)

func ColumnExtractRulesDTO2DO(dtos []*tracedto.ColumnExtractRule) []entity.ColumnExtractRule {
	if len(dtos) == 0 {
		return nil
	}
	rules := make([]entity.ColumnExtractRule, 0, len(dtos))
	for _, dto := range dtos {
		if dto == nil {
			continue
		}
		rules = append(rules, entity.ColumnExtractRule{
			Column:   dto.GetColumn(),
			JSONPath: dto.GetJSONPath(),
		})
	}
	return rules
}

func ColumnExtractRulesDO2DTO(dos []entity.ColumnExtractRule) []*tracedto.ColumnExtractRule {
	if len(dos) == 0 {
		return nil
	}
	rules := make([]*tracedto.ColumnExtractRule, 0, len(dos))
	for _, do := range dos {
		rules = append(rules, &tracedto.ColumnExtractRule{
			Column:   do.Column,
			JSONPath: do.JSONPath,
		})
	}
	return rules
}
