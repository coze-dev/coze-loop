// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package repo

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// IExptManualRetryFilterRepo enumerates canonical manual retries independently
// of the Logged-only scheduler scan. Returned IDs are candidates, never payloads.
//
//go:generate mockgen -destination=mocks/expt_manual_retry_filter.go -package=mocks . IExptManualRetryFilterRepo
type IExptManualRetryFilterRepo interface {
	ScanManualRetryFilterItems(context.Context, int64, int64, int64, int64, int) ([]int64, int64, error)
}

// IExptMaterializedFilterRepo reads the complete projection at the OSS sink.
// An async repository without this port can observe its existing reader but
// remains pending until full projection materialization can be established.
//
//go:generate mockgen -destination=mocks/expt_materialized_filter.go -package=mocks . IExptMaterializedFilterRepo
type IExptMaterializedFilterRepo interface {
	GetMaterializedByExptIDItemIDs(context.Context, string, string, string, []string) ([]*entity.ExptTurnResultFilterEntity, error)
}
