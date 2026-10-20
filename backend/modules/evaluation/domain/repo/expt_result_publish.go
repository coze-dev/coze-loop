// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package repo

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// IExptResultPublishRepo owns the canonical-first publication transaction.
//
//go:generate mockgen -destination=mocks/expt_result_publish.go -package=mocks . IExptResultPublishRepo
type IExptResultPublishRepo interface {
	GetItemResultPublishSource(context.Context, int64, int64, int64, int64) (*entity.ExptItemResultPublishSource, error)
	IsItemResultPublishSourceCurrent(context.Context, *entity.ExptItemResultPublishSource) (bool, error)
	CommitItemResult(context.Context, *entity.ExptItemResultPublication) (bool, error)
}
