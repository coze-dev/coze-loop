// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination=mocks/evaluation_set_version.go -package=mocks . EvaluationSetVersionService
type EvaluationSetVersionService interface {
	CreateEvaluationSetVersion(ctx context.Context, param *entity.CreateEvaluationSetVersionParam) (id int64, err error)
	GetEvaluationSetVersion(ctx context.Context, spaceID, versionID int64, deletedAt *bool, sharedOption *entity.SharedResourceOption) (version *entity.EvaluationSetVersion, set *entity.EvaluationSet, err error)
	ListEvaluationSetVersions(ctx context.Context, param *entity.ListEvaluationSetVersionsParam) (sets []*entity.EvaluationSetVersion, total *int64, nextCursor *string, err error)
	BatchGetEvaluationSetVersions(ctx context.Context, spaceID *int64, evaluationSetIDs []int64, deletedAt *bool, sharedOption *entity.SharedResourceOption) (sets []*entity.BatchGetEvaluationSetVersionsResult, err error)
}
