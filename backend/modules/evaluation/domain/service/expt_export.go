// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate  mockgen -destination  ./mocks/expt_export.go  --package mocks . IExptResultExportService
type IExptResultExportService interface {
	ExportCSV(ctx context.Context, spaceID, exptID int64, session *entity.Session) (int64, error)
	DoExportCSV(ctx context.Context, spaceID, exptID, exportID int64) error
	UpdateExportRecord(ctx context.Context, exportRecord *entity.ExptResultExportRecord) error
	ListExportRecord(ctx context.Context, spaceID, exptID int64, page entity.Page) ([]*entity.ExptResultExportRecord, int64, error)
	GetExptExportRecord(ctx context.Context, spaceID, exportID int64) (*entity.ExptResultExportRecord, error)
}
