// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func (service *EvalTargetServiceImpl) sandboxVersionIDsForRecords(ctx context.Context, records []*entity.EvalTargetRecord) (map[int64]struct{}, error) {
	groups := make(map[int64]map[int64]struct{})
	for _, record := range records {
		if record == nil || record.TargetVersionID <= 0 || gptr.Indirect(record.Status) != entity.EvalTargetRunStatusAsyncInvoking {
			continue
		}
		spaceID := record.TargetSourceSpaceID()
		if spaceID <= 0 {
			return nil, fmt.Errorf("invalid target record source space: record %d", record.ID)
		}
		if groups[spaceID] == nil {
			groups[spaceID] = make(map[int64]struct{})
		}
		groups[spaceID][record.TargetVersionID] = struct{}{}
	}
	result := make(map[int64]struct{})
	for spaceID, versionSet := range groups {
		versionIDs := make([]int64, 0, len(versionSet))
		for versionID := range versionSet {
			versionIDs = append(versionIDs, versionID)
		}
		versions, err := service.evalTargetRepo.BatchGetEvalTargetVersion(ctx, spaceID, versionIDs)
		if err != nil {
			return nil, err
		}
		for _, version := range versions {
			if version != nil && version.EvalTargetVersion != nil && version.EvalTargetType == entity.EvalTargetTypeSandboxAgent {
				result[version.EvalTargetVersion.ID] = struct{}{}
			}
		}
	}
	return result, nil
}
