// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate  mockgen -destination  ./mocks/expt_annotate.go  --package mocks . IExptAnnotateService
type IExptAnnotateService interface {
	CreateExptTurnResultTagRefs(ctx context.Context, refs []*entity.ExptTurnResultTagRef) error
	GetExptTurnResultTagRefs(ctx context.Context, exptID, spaceID int64) ([]*entity.ExptTurnResultTagRef, error)
	SaveAnnotateRecord(ctx context.Context, exptID, itemID, turnID int64, record *entity.AnnotateRecord) error
	UpdateAnnotateRecord(ctx context.Context, itemID int64, turnID int64, record *entity.AnnotateRecord) error
	GetAnnotateRecordsByIDs(ctx context.Context, spaceID int64, recordIDs []int64) ([]*entity.AnnotateRecord, error)
	DeleteExptTurnResultTagRef(ctx context.Context, exptID int64, spaceID int64, tagKeyID int64) error
}
