// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/query"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

//go:generate mockgen -destination=mocks/expt_result_export_record.go -package=mocks . ExptResultExportRecordDAO
type ExptResultExportRecordDAO interface {
	Create(ctx context.Context, exptResultExportRecord *model.ExptResultExportRecord, opts ...db.Option) error
	Update(ctx context.Context, exptResultExportRecord *model.ExptResultExportRecord, opts ...db.Option) error
	List(ctx context.Context, spaceID, exptID int64, page entity.Page, csvExportStatus *int32) ([]*model.ExptResultExportRecord, int64, error)
	Get(ctx context.Context, spaceID, exportID int64, opts ...db.Option) (*model.ExptResultExportRecord, error)
}

func NewExptResultExportRecordDAO(db db.Provider) ExptResultExportRecordDAO {
	return &exptResultExportRecordDAO{
		db:    db,
		query: query.Use(db.NewSession(context.Background())),
	}
}

type exptResultExportRecordDAO struct {
	db    db.Provider
	query *query.Query
}

func (e exptResultExportRecordDAO) Create(ctx context.Context, exptResultExportRecord *model.ExptResultExportRecord, opts ...db.Option) error {
	if err := e.db.NewSession(ctx, opts...).Create(exptResultExportRecord).Error; err != nil {
		return errorx.Wrapf(err, "create exptResultExportRecord fail, model: %v", json.Jsonify(exptResultExportRecord))
	}
	return nil
}

func (e exptResultExportRecordDAO) Update(ctx context.Context, exptResultExportRecord *model.ExptResultExportRecord, opts ...db.Option) error {
	if err := e.db.NewSession(ctx, opts...).Model(&model.ExptResultExportRecord{}).Where("id = ?", exptResultExportRecord.ID).Updates(exptResultExportRecord).Error; err != nil {
		return errorx.Wrapf(err, "create expt fail, model: %v", json.Jsonify(exptResultExportRecord))
	}
	return nil
}

func (e exptResultExportRecordDAO) List(ctx context.Context, spaceID, exptID int64, page entity.Page, csvExportStatus *int32) ([]*model.ExptResultExportRecord, int64, error) {
	var (
		finds []*model.ExptResultExportRecord
		total int64
	)

	db := e.db.NewSession(ctx).Model(&model.ExptResultExportRecord{}).Where("space_id = ?", spaceID).Where("expt_id = ?", exptID)

	if csvExportStatus != nil {
		db = db.Where("csv_export_status =?", *csvExportStatus)
	}

	db = db.Order("created_at desc")
	// 总记录数
	db = db.Count(&total)
	// 分页
	db = db.Offset(page.Offset()).Limit(page.Limit())
	err := db.Find(&finds).Error
	if err != nil {
		return nil, 0, err
	}
	return finds, total, nil
}

func (e exptResultExportRecordDAO) Get(ctx context.Context, spaceID, exportID int64, opts ...db.Option) (*model.ExptResultExportRecord, error) {
	record := &model.ExptResultExportRecord{}
	err := e.db.NewSession(ctx, opts...).Where("space_id = ?", spaceID).Where("id = ?", exportID).First(record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.WrapByCode(err, errno.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("ExptResultExportRecord %d not found", exportID)))
		}
		return nil, errorx.Wrapf(err, "mysql get ExptResultExportRecord fail, id: %v", exportID)
	}
	return record, nil
}
