// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type ExptAnnotateRepoImpl struct {
	exptTurnAnnotateRecordRefDAO mysql.IExptTurnAnnotateRecordRefDAO
	exptTurnResultTagRefDAO      mysql.IExptTurnResultTagRefDAO
	annotateRecordDAO            mysql.IAnnotateRecordDAO
	idgenerator                  idgen.IIDGenerator
}

func NewExptAnnotateRepo(exptTurnAnnotateRecordRefDAO mysql.IExptTurnAnnotateRecordRefDAO,
	exptTurnResultTagRefDAO mysql.IExptTurnResultTagRefDAO,
	annotateRecordDAO mysql.IAnnotateRecordDAO, idgenerator idgen.IIDGenerator,
) repo.IExptAnnotateRepo {
	return &ExptAnnotateRepoImpl{
		exptTurnAnnotateRecordRefDAO: exptTurnAnnotateRecordRefDAO,
		exptTurnResultTagRefDAO:      exptTurnResultTagRefDAO,
		annotateRecordDAO:            annotateRecordDAO,
		idgenerator:                  idgenerator,
	}
}

func (e ExptAnnotateRepoImpl) GetTagRefByTagKeyID(ctx context.Context, exptID, spaceID, tagKeyID int64) (*entity.ExptTurnResultTagRef, error) {
	ref, err := e.exptTurnResultTagRefDAO.GetByTagKeyID(ctx, exptID, spaceID, tagKeyID)
	if err != nil {
		return nil, err
	}

	return convert.ExptTurnResultTagRefPOToDO(ref), nil
}

func (e ExptAnnotateRepoImpl) CreateExptTurnAnnotateRecordRefs(ctx context.Context, ref *entity.ExptTurnAnnotateRecordRef) error {
	id, err := e.idgenerator.GenID(ctx)
	if err != nil {
		return err
	}
	ref.ID = id

	return e.exptTurnAnnotateRecordRefDAO.Save(ctx, convert.ExptTurnAnnotateRecordRefDOToPO(ref))
}

func (e ExptAnnotateRepoImpl) CreateExptTurnResultTagRefs(ctx context.Context, refs []*entity.ExptTurnResultTagRef) error {
	ids, err := e.idgenerator.GenMultiIDs(ctx, len(refs))
	if err != nil {
		return err
	}
	for i, ref := range refs {
		ref.ID = ids[i]
	}

	exptTurnResultTagRefs := make([]*model.ExptTurnResultTagRef, 0)
	for _, ref := range refs {
		exptTurnResultTagRefs = append(exptTurnResultTagRefs, convert.ExptTurnResultTagRefDOToPO(ref))
	}
	return e.exptTurnResultTagRefDAO.Create(ctx, exptTurnResultTagRefs)
}

func (e ExptAnnotateRepoImpl) DeleteExptTurnResultTagRef(ctx context.Context, exptID, spaceID, tagKeyID int64, opts ...db.Option) error {
	err := e.exptTurnResultTagRefDAO.Delete(ctx, exptID, spaceID, tagKeyID, opts...)
	if err != nil {
		return err
	}

	return nil
}

func (e ExptAnnotateRepoImpl) DeleteTurnAnnotateRecordRef(ctx context.Context, exptID, spaceID, tagKeyID int64, opts ...db.Option) error {
	err := e.exptTurnAnnotateRecordRefDAO.DeleteByTagKeyID(ctx, spaceID, exptID, tagKeyID, opts...)
	if err != nil {
		return err
	}

	return nil
}

func (e ExptAnnotateRepoImpl) GetExptTurnAnnotateRecordRefs(ctx context.Context, exptID, spaceID int64) ([]*entity.ExptTurnAnnotateRecordRef, error) {
	refs, err := e.exptTurnAnnotateRecordRefDAO.GetByExptID(ctx, spaceID, exptID)
	if err != nil {
		return nil, err
	}

	exptTurnAnnotateRecordRefs := make([]*entity.ExptTurnAnnotateRecordRef, 0)
	for _, ref := range refs {
		exptTurnAnnotateRecordRefs = append(exptTurnAnnotateRecordRefs, convert.ExptTurnAnnotateRecordRefPOToDO(ref))
	}
	return exptTurnAnnotateRecordRefs, nil
}

func (e ExptAnnotateRepoImpl) BatchGetExptTurnAnnotateRecordRefs(ctx context.Context, exptIDs []int64, spaceID int64) ([]*entity.ExptTurnAnnotateRecordRef, error) {
	refs, err := e.exptTurnAnnotateRecordRefDAO.BatchGetByExptIDs(ctx, spaceID, exptIDs)
	if err != nil {
		return nil, err
	}

	exptTurnAnnotateRecordRefs := make([]*entity.ExptTurnAnnotateRecordRef, 0)
	for _, ref := range refs {
		exptTurnAnnotateRecordRefs = append(exptTurnAnnotateRecordRefs, convert.ExptTurnAnnotateRecordRefPOToDO(ref))
	}
	return exptTurnAnnotateRecordRefs, nil
}

func (e ExptAnnotateRepoImpl) GetExptTurnAnnotateRecordRefsByTurnResultIDs(ctx context.Context, spaceID int64, turnResultIDs []int64) ([]*entity.ExptTurnAnnotateRecordRef, error) {
	refs, err := e.exptTurnAnnotateRecordRefDAO.BatchGet(ctx, spaceID, turnResultIDs)
	if err != nil {
		return nil, err
	}

	exptTurnAnnotateRecordRefs := make([]*entity.ExptTurnAnnotateRecordRef, 0)
	for _, ref := range refs {
		exptTurnAnnotateRecordRefs = append(exptTurnAnnotateRecordRefs, convert.ExptTurnAnnotateRecordRefPOToDO(ref))
	}
	return exptTurnAnnotateRecordRefs, nil
}

func (e ExptAnnotateRepoImpl) GetExptTurnAnnotateRecordRefsByTagKeyID(ctx context.Context, exptID, spaceID, tagKeyID int64) ([]*entity.ExptTurnAnnotateRecordRef, error) {
	refs, err := e.exptTurnAnnotateRecordRefDAO.GetByTagKeyID(ctx, spaceID, exptID, tagKeyID)
	if err != nil {
		return nil, err
	}

	exptTurnAnnotateRecordRefs := make([]*entity.ExptTurnAnnotateRecordRef, 0)
	for _, ref := range refs {
		exptTurnAnnotateRecordRefs = append(exptTurnAnnotateRecordRefs, convert.ExptTurnAnnotateRecordRefPOToDO(ref))
	}
	return exptTurnAnnotateRecordRefs, nil
}

func (e ExptAnnotateRepoImpl) UpdateCompleteCount(ctx context.Context, exptID, spaceID, tagKeyID int64, opts ...db.Option) (int32, int32, error) {
	return e.exptTurnResultTagRefDAO.UpdateCompleteCount(ctx, exptID, spaceID, tagKeyID, opts...)
}

func (e ExptAnnotateRepoImpl) GetExptTurnResultTagRefs(ctx context.Context, exptID, spaceID int64) ([]*entity.ExptTurnResultTagRef, error) {
	refs, err := e.exptTurnResultTagRefDAO.GetByExptID(ctx, exptID, spaceID)
	if err != nil {
		return nil, err
	}

	exptTurnResultTagRefs := make([]*entity.ExptTurnResultTagRef, 0)
	for _, ref := range refs {
		exptTurnResultTagRefs = append(exptTurnResultTagRefs, convert.ExptTurnResultTagRefPOToDO(ref))
	}
	return exptTurnResultTagRefs, nil
}

func (e ExptAnnotateRepoImpl) BatchGetExptTurnResultTagRefs(ctx context.Context, exptIDs []int64, spaceID int64) ([]*entity.ExptTurnResultTagRef, error) {
	refs, err := e.exptTurnResultTagRefDAO.BatchGetByExptIDs(ctx, exptIDs, spaceID)
	if err != nil {
		return nil, err
	}

	exptTurnResultTagRefs := make([]*entity.ExptTurnResultTagRef, 0)
	for _, ref := range refs {
		exptTurnResultTagRefs = append(exptTurnResultTagRefs, convert.ExptTurnResultTagRefPOToDO(ref))
	}
	return exptTurnResultTagRefs, nil
}

func (e ExptAnnotateRepoImpl) SaveAnnotateRecord(ctx context.Context, exptTurnResultID int64, record *entity.AnnotateRecord, opts ...db.Option) error {
	id, err := e.idgenerator.GenID(ctx)
	if err != nil {
		return err
	}

	po, err := convert.AnnotateRecordDOToPO(record)
	if err != nil {
		return err
	}
	err = e.annotateRecordDAO.Save(ctx, po, opts...)
	if err != nil {
		return err
	}

	exptTurnAnnotateRecordRef := &model.ExptTurnAnnotateRecordRef{
		ID:               id,
		SpaceID:          record.SpaceID,
		ExptTurnResultID: exptTurnResultID,
		TagKeyID:         record.TagKeyID,
		AnnotateRecordID: record.ID,
		ExptID:           record.ExperimentID,
	}

	err = e.exptTurnAnnotateRecordRefDAO.Save(ctx, exptTurnAnnotateRecordRef, opts...)
	if err != nil {
		return err
	}

	return nil
}

func (e ExptAnnotateRepoImpl) UpdateAnnotateRecord(ctx context.Context, record *entity.AnnotateRecord) error {
	po, err := convert.AnnotateRecordDOToPO(record)
	if err != nil {
		return err
	}
	return e.annotateRecordDAO.Update(ctx, po)
}

func (e ExptAnnotateRepoImpl) GetAnnotateRecordsByIDs(ctx context.Context, spaceID int64, recordIDs []int64) ([]*entity.AnnotateRecord, error) {
	records, err := e.annotateRecordDAO.MGetByID(ctx, recordIDs)
	if err != nil {
		return nil, err
	}
	annotateRecords := make([]*entity.AnnotateRecord, 0)
	for _, record := range records {
		do, err := convert.AnnotateRecordPOToDO(record)
		if err != nil {
			return nil, err
		}
		annotateRecords = append(annotateRecords, do)
	}
	return annotateRecords, nil
}

func (e ExptAnnotateRepoImpl) GetAnnotateRecordByID(ctx context.Context, spaceID, recordID int64) (*entity.AnnotateRecord, error) {
	records, err := e.annotateRecordDAO.MGetByID(ctx, []int64{recordID})
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("record not found, recordID: %v", recordID)
	}
	record := records[0]

	do, err := convert.AnnotateRecordPOToDO(record)
	if err != nil {
		return nil, err
	}

	return do, nil
}
