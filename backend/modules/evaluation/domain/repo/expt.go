// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate  mockgen -destination  ./mocks/expt.go  --package mocks . IExperimentRepo,IExptStatsRepo,IExptItemResultRepo,IExptTurnResultRepo,IExptRunLogRepo,IExptAggrResultRepo,QuotaRepo,IExptTurnResultFilterRepo,IExptAnnotateRepo,IExptResultExportRecordRepo
type IExperimentRepo interface {
	Create(ctx context.Context, expt *entity.Experiment, exptEvaluatorRefs []*entity.ExptEvaluatorRef) error
	Update(ctx context.Context, expt *entity.Experiment) error
	Delete(ctx context.Context, id, spaceID int64) error
	MDelete(ctx context.Context, ids []int64, spaceID int64) error
	List(ctx context.Context, page, size int32, filter *entity.ExptListFilter, orders []*entity.OrderBy, spaceID int64) ([]*entity.Experiment, int64, error)
	GetByID(ctx context.Context, id, spaceID int64) (*entity.Experiment, error)
	MGetByID(ctx context.Context, ids []int64, spaceID int64) ([]*entity.Experiment, error)
	MGetBasicByID(ctx context.Context, ids []int64) ([]*entity.Experiment, error)
	GetByName(ctx context.Context, name string, spaceID int64) (*entity.Experiment, bool, error)
	GetEvaluatorRefByExptIDs(ctx context.Context, exptID []int64, spaceID int64) ([]*entity.ExptEvaluatorRef, error)
}

type IExptStatsRepo interface {
	Create(ctx context.Context, stats *entity.ExptStats) error
	Get(ctx context.Context, exptID, spaceID int64) (*entity.ExptStats, error)
	MGet(ctx context.Context, exptIDs []int64, spaceID int64) ([]*entity.ExptStats, error)
	UpdateByExptID(ctx context.Context, exptID, spaceID int64, stats *entity.ExptStats) error
	ArithOperateCount(ctx context.Context, exptID, spaceID int64, cntArithOp *entity.StatsCntArithOp) error
	Save(ctx context.Context, stats *entity.ExptStats) error
}

type IExptItemResultRepo interface {
	BatchGet(ctx context.Context, spaceID, exptID int64, itemIDs []int64) ([]*entity.ExptItemResult, error)
	BatchCreateNX(ctx context.Context, itemResults []*entity.ExptItemResult) error
	ScanItemResults(ctx context.Context, exptID, cursor, limit int64, status []int32, spaceID int64) (results []*entity.ExptItemResult, ncursor int64, err error)
	ListItemResultsByExptID(ctx context.Context, exptID, spaceID int64, page entity.Page, desc bool) ([]*entity.ExptItemResult, int64, error)
	GetItemIDListByExptID(ctx context.Context, exptID, spaceID int64) (itemIDList []int64, err error)
	SaveItemResults(ctx context.Context, itemResults []*entity.ExptItemResult) error
	GetItemTurnResults(ctx context.Context, spaceID, exptID, itemID int64) ([]*entity.ExptTurnResult, error)
	UpdateItemsResult(ctx context.Context, spaceID, exptID int64, itemID []int64, ufields map[string]any) error
	GetMaxItemIdxByExptID(ctx context.Context, exptID, spaceID int64) (int32, error)

	BatchCreateNXRunLogs(ctx context.Context, itemRunLogs []*entity.ExptItemResultRunLog) error
	ScanItemRunLogs(ctx context.Context, exptID, exptRunID int64, filter *entity.ExptItemRunLogFilter, cursor, limit, spaceID int64) ([]*entity.ExptItemResultRunLog, int64, error)
	UpdateItemRunLog(ctx context.Context, exptID, exptRunID int64, itemID []int64, ufields map[string]any, spaceID int64) error
	GetItemRunLog(ctx context.Context, exptID, exptRunID, itemID, spaceID int64) (*entity.ExptItemResultRunLog, error)
	MGetItemRunLog(ctx context.Context, exptID, exptRunID int64, itemIDs []int64, spaceID int64) ([]*entity.ExptItemResultRunLog, error)
}

type IExptTurnResultRepo interface {
	ListTurnResult(ctx context.Context, spaceID, exptID int64, filter *entity.ExptTurnResultFilter, page entity.Page, desc bool) ([]*entity.ExptTurnResult, int64, error)
	ListTurnResultByItemIDs(ctx context.Context, spaceID, exptID int64, itemIDs []int64, page entity.Page, desc bool) ([]*entity.ExptTurnResult, int64, error)
	BatchGet(ctx context.Context, spaceID, exptID int64, itemIDs []int64) ([]*entity.ExptTurnResult, error)
	Get(ctx context.Context, spaceID, exptID int64, itemID, turnID int64) (*entity.ExptTurnResult, error)
	CreateTurnEvaluatorRefs(ctx context.Context, turnResults []*entity.ExptTurnEvaluatorResultRef) error
	BatchCreateNX(ctx context.Context, turnResults []*entity.ExptTurnResult) error
	GetItemTurnResults(ctx context.Context, exptID, itemID, spaceID int64) ([]*entity.ExptTurnResult, error)
	SaveTurnResults(ctx context.Context, turnResults []*entity.ExptTurnResult) error
	ScanTurnResults(ctx context.Context, exptID int64, status []int32, cursor, limit, spaceID int64) ([]*entity.ExptTurnResult, int64, error)
	UpdateTurnResults(ctx context.Context, exptID int64, itemTurnIDs []*entity.ItemTurnID, spaceID int64, ufields map[string]any) error
	UpdateTurnResultsWithItemIDs(ctx context.Context, exptID int64, itemIDs []int64, spaceID int64, ufields map[string]any) error

	BatchCreateNXRunLog(ctx context.Context, turnResults []*entity.ExptTurnResultRunLog) error
	GetItemTurnRunLogs(ctx context.Context, exptID, exptRunID, itemID, spaceID int64) ([]*entity.ExptTurnResultRunLog, error)
	MGetItemTurnRunLogs(ctx context.Context, exptID, exptRunID int64, itemIDs []int64, spaceID int64) ([]*entity.ExptTurnResultRunLog, error)
	SaveTurnRunLogs(ctx context.Context, turnResults []*entity.ExptTurnResultRunLog) error
	CreateOrUpdateItemsTurnRunLogStatus(ctx context.Context, spaceID, exptID, exptRunID int64, itemIDs []int64, status entity.TurnRunState) error
	ScanTurnRunLogs(ctx context.Context, exptID, cursor, limit, spaceID int64) ([]*entity.ExptTurnResultRunLog, int64, error)

	BatchGetTurnEvaluatorResultRef(ctx context.Context, spaceID int64, exptTurnResultIDs []int64) ([]*entity.ExptTurnEvaluatorResultRef, error)
	GetTurnEvaluatorResultRefByExptID(ctx context.Context, spaceID, exptID int64) ([]*entity.ExptTurnEvaluatorResultRef, error)
	GetTurnEvaluatorResultRefByEvaluatorVersionID(ctx context.Context, spaceID, exptID, evaluatorVersionID int64) ([]*entity.ExptTurnEvaluatorResultRef, error)
}

type IExptRunLogRepo interface {
	Create(ctx context.Context, exptRunLog *entity.ExptRunLog) error
	Save(ctx context.Context, exptRunLog *entity.ExptRunLog) error
	Update(ctx context.Context, exptID, exptRunID int64, ufields map[string]any) error
	Get(ctx context.Context, exptID, exptRunID int64) (*entity.ExptRunLog, error)
}

type IExptAggrResultRepo interface {
	GetExptAggrResult(ctx context.Context, experimentID int64, fieldType int32, fieldKey string) (*entity.ExptAggrResult, error)
	GetExptAggrResultByExperimentID(ctx context.Context, experimentID int64) ([]*entity.ExptAggrResult, error)
	BatchGetExptAggrResultByExperimentIDs(ctx context.Context, experimentIDs []int64) ([]*entity.ExptAggrResult, error)
	CreateExptAggrResult(ctx context.Context, exptAggrResult *entity.ExptAggrResult) error
	BatchCreateExptAggrResult(ctx context.Context, exptAggrResults []*entity.ExptAggrResult) error
	UpdateExptAggrResultByVersion(ctx context.Context, exptAggrResult *entity.ExptAggrResult, taskVersion int64) error
	UpdateAndGetLatestVersion(ctx context.Context, experimentID int64, fieldType int32, fieldKey string) (int64, error)
	DeleteExptAggrResult(ctx context.Context, exptAggrResult *entity.ExptAggrResult, opts ...db.Option) error
}

type QuotaRepo interface {
	CreateOrUpdate(ctx context.Context, spaceID int64, updater func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), session *entity.Session) error
}

type IExptTurnResultFilterRepo interface {
	Save(ctx context.Context, filters []*entity.ExptTurnResultFilterEntity) error
	QueryItemIDStates(ctx context.Context, filter *entity.ExptTurnResultFilterAccelerator) (map[int64]entity.ItemRunState, int64, error)
	GetExptTurnResultFilterKeyMappings(ctx context.Context, spaceID, exptID int64) ([]*entity.ExptTurnResultFilterKeyMapping, error)
	InsertExptTurnResultFilterKeyMappings(ctx context.Context, mappings []*entity.ExptTurnResultFilterKeyMapping) error
	GetByExptIDItemIDs(ctx context.Context, spaceID, exptID, createdDate string, itemIDs []string) ([]*entity.ExptTurnResultFilterEntity, error)
	DeleteExptTurnResultFilterKeyMapping(ctx context.Context, mapping *entity.ExptTurnResultFilterKeyMapping, opts ...db.Option) error
}

type IExptAnnotateRepo interface {
	CreateExptTurnAnnotateRecordRefs(ctx context.Context, refs *entity.ExptTurnAnnotateRecordRef) error
	GetExptTurnAnnotateRecordRefs(ctx context.Context, exptID, spaceID int64) ([]*entity.ExptTurnAnnotateRecordRef, error)
	BatchGetExptTurnAnnotateRecordRefs(ctx context.Context, exptIDs []int64, spaceID int64) ([]*entity.ExptTurnAnnotateRecordRef, error)
	GetExptTurnAnnotateRecordRefsByTagKeyID(ctx context.Context, exptID, spaceID, tagKeyID int64) ([]*entity.ExptTurnAnnotateRecordRef, error)
	GetExptTurnAnnotateRecordRefsByTurnResultIDs(ctx context.Context, exptID int64, turnResultIDs []int64) ([]*entity.ExptTurnAnnotateRecordRef, error)
	DeleteTurnAnnotateRecordRef(ctx context.Context, exptID, spaceID, tagKeyID int64, opts ...db.Option) error

	CreateExptTurnResultTagRefs(ctx context.Context, refs []*entity.ExptTurnResultTagRef) error
	GetExptTurnResultTagRefs(ctx context.Context, exptID, spaceID int64) ([]*entity.ExptTurnResultTagRef, error)
	BatchGetExptTurnResultTagRefs(ctx context.Context, exptIDs []int64, spaceID int64) ([]*entity.ExptTurnResultTagRef, error)
	GetTagRefByTagKeyID(ctx context.Context, exptID, spaceID, tagKeyID int64) (*entity.ExptTurnResultTagRef, error)
	UpdateCompleteCount(ctx context.Context, exptID, spaceID, tagKeyID int64, opts ...db.Option) (int32, int32, error)
	DeleteExptTurnResultTagRef(ctx context.Context, exptID, spaceID, tagKeyID int64, opts ...db.Option) error

	SaveAnnotateRecord(ctx context.Context, exptTurnResultID int64, record *entity.AnnotateRecord, opts ...db.Option) error
	UpdateAnnotateRecord(ctx context.Context, record *entity.AnnotateRecord) error
	GetAnnotateRecordsByIDs(ctx context.Context, spaceID int64, recordIDs []int64) ([]*entity.AnnotateRecord, error)
	GetAnnotateRecordByID(ctx context.Context, spaceID, recordID int64) (*entity.AnnotateRecord, error)
}

type IExptResultExportRecordRepo interface {
	Create(ctx context.Context, exportRecord *entity.ExptResultExportRecord, opts ...db.Option) (int64, error)
	Update(ctx context.Context, exportRecord *entity.ExptResultExportRecord, opts ...db.Option) error
	List(ctx context.Context, spaceID, exptID int64, page entity.Page, csvExportStatus *int32) ([]*entity.ExptResultExportRecord, int64, error)
	Get(ctx context.Context, spaceID, exportID int64) (*entity.ExptResultExportRecord, error)
}
