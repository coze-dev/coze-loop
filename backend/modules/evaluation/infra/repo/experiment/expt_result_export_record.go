// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
)

type ExptResultExportRecordRepoImpl struct {
	exptResultExportRecordDAO mysql.ExptResultExportRecordDAO
	idgenerator               idgen.IIDGenerator
}

func NewExptResultExportRecordRepo(exptResultExportRecordDAO mysql.ExptResultExportRecordDAO, idgenerator idgen.IIDGenerator) repo.IExptResultExportRecordRepo {
	return &ExptResultExportRecordRepoImpl{
		exptResultExportRecordDAO: exptResultExportRecordDAO,
		idgenerator:               idgenerator,
	}
}

func (e ExptResultExportRecordRepoImpl) Create(ctx context.Context, exportRecord *entity.ExptResultExportRecord, opts ...db.Option) (int64, error) {
	id, err := e.idgenerator.GenID(ctx)
	if err != nil {
		return 0, err
	}
	exportRecord.ID = id

	po := convert.ExptResultExportRecordDOToPO(exportRecord)

	return id, e.exptResultExportRecordDAO.Create(ctx, po, opts...)
}

func (e ExptResultExportRecordRepoImpl) Update(ctx context.Context, exportRecord *entity.ExptResultExportRecord, opts ...db.Option) error {
	po := convert.ExptResultExportRecordDOToPO(exportRecord)

	return e.exptResultExportRecordDAO.Update(ctx, po, opts...)
}

func (e ExptResultExportRecordRepoImpl) List(ctx context.Context, spaceID, exptID int64, page entity.Page, csvExportStatus *int32) ([]*entity.ExptResultExportRecord, int64, error) {
	pos, total, err := e.exptResultExportRecordDAO.List(ctx, spaceID, exptID, page, csvExportStatus)
	if err != nil {
		return nil, 0, err
	}

	dos := make([]*entity.ExptResultExportRecord, 0, len(pos))
	for _, po := range pos {
		dos = append(dos, convert.ExptResultExportRecordPOToDO(po))
	}

	return dos, total, nil
}

func (e ExptResultExportRecordRepoImpl) Get(ctx context.Context, spaceID, exportID int64) (*entity.ExptResultExportRecord, error) {
	po, err := e.exptResultExportRecordDAO.Get(ctx, spaceID, exportID, db.WithMaster())
	if err != nil {
		return nil, err
	}

	return convert.ExptResultExportRecordPOToDO(po), nil
}
