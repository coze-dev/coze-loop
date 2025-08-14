// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"strconv"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/ck"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/ck/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/ck/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	mysqlmodel "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// 假设存在对应的 DAO 接口
type ExptTurnResultFilterRepoImpl struct {
	exptTurnResultFilterDAO           ck.IExptTurnResultFilterDAO
	exptTurnResultFilterKeyMappingDAO mysql.IExptTurnResultFilterKeyMappingDAO
}

// NewExptTurnResultFilterRepo 创建 ExptTurnResultFilterRepoImpl 实例
func NewExptTurnResultFilterRepo(exptTurnResultFilterDAO ck.IExptTurnResultFilterDAO, exptTurnResultFilterKeyMappingDAO mysql.IExptTurnResultFilterKeyMappingDAO) repo.IExptTurnResultFilterRepo {
	return &ExptTurnResultFilterRepoImpl{
		exptTurnResultFilterDAO:           exptTurnResultFilterDAO,
		exptTurnResultFilterKeyMappingDAO: exptTurnResultFilterKeyMappingDAO,
	}
}

// Save 实现 IExptTurnResultFilterRepo 接口的 Save 方法
func (e *ExptTurnResultFilterRepoImpl) Save(ctx context.Context, filter []*entity.ExptTurnResultFilterEntity) error {
	// 转换为 model.ExptTurnResultFilterAccelerator
	models := make([]*model.ExptTurnResultFilter, 0, len(filter))
	for _, filterEntity := range filter {
		models = append(models, convertor.ExptTurnResultFilterEntity2PO(filterEntity))
	}
	logs.CtxInfo(ctx, "ExptTurnResultFilterRepoImpl.Save: %v", json.Jsonify(models))
	return e.exptTurnResultFilterDAO.Save(ctx, models)
}

func fieldFilterEntityToCK(src *entity.FieldFilter) *ck.FieldFilter {
	return &ck.FieldFilter{
		Key:    src.Key,
		Op:     src.Op,
		Values: src.Values,
	}
}

func fieldFiltersEntityToCK(src []*entity.FieldFilter) []*ck.FieldFilter {
	res := make([]*ck.FieldFilter, 0, len(src))
	for _, f := range src {
		res = append(res, &ck.FieldFilter{
			Key:    f.Key,
			Op:     f.Op,
			Values: f.Values,
		})
	}
	return res
}

// QueryItemIDStates 实现 IExptTurnResultFilterRepo 接口的 QueryItemIDStates 方法
func (e *ExptTurnResultFilterRepoImpl) QueryItemIDStates(ctx context.Context, filter *entity.ExptTurnResultFilterAccelerator) (map[int64]entity.ItemRunState, int64, error) {
	cond := &ck.ExptTurnResultFilterQueryCond{}
	// 主表字段
	if filter.SpaceID != 0 {
		s := strconv.FormatInt(filter.SpaceID, 10)
		cond.SpaceID = ptr.Of(s)
	}
	if filter.ExptID != 0 {
		s := strconv.FormatInt(filter.ExptID, 10)
		cond.ExptID = ptr.Of(s)
	}
	// 支持多组item_id、item状态、turn状态 filter
	cond.ItemIDs = fieldFiltersEntityToCK(filter.ItemIDs)
	cond.ItemRunStatus = fieldFiltersEntityToCK(filter.ItemRunStatus)
	cond.TurnRunStatus = fieldFiltersEntityToCK(filter.TurnRunStatus)
	if !filter.CreatedDate.IsZero() {
		createdDate := filter.CreatedDate
		cond.CreatedDate = ptr.Of(createdDate)
	}
	if filter.EvaluatorScoreCorrected != nil {
		cond.EvaluatorScoreCorrected = fieldFilterEntityToCK(filter.EvaluatorScoreCorrected)
	}
	// MapCond
	if filter.MapCond != nil {
		cond.MapCond = &ck.ExptTurnResultFilterMapCond{
			EvalTargetDataFilters:   fieldFiltersEntityToCK(filter.MapCond.EvalTargetDataFilters),
			EvaluatorScoreFilters:   fieldFiltersEntityToCK(filter.MapCond.EvaluatorScoreFilters),
			AnnotationFloatFilters:  fieldFiltersEntityToCK(filter.MapCond.AnnotationFloatFilters),
			AnnotationBoolFilters:   fieldFiltersEntityToCK(filter.MapCond.AnnotationBoolFilters),
			AnnotationStringFilters: fieldFiltersEntityToCK(filter.MapCond.AnnotationStringFilters),
		}
	}
	// ItemSnapshotCond
	cond.EvalSetSyncCkDate = filter.EvalSetSyncCkDate
	if filter.ItemSnapshotCond != nil {
		cond.ItemSnapshotCond = &ck.ItemSnapshotFilter{
			BoolMapFilters:   fieldFiltersEntityToCK(filter.ItemSnapshotCond.BoolMapFilters),
			FloatMapFilters:  fieldFiltersEntityToCK(filter.ItemSnapshotCond.FloatMapFilters),
			IntMapFilters:    fieldFiltersEntityToCK(filter.ItemSnapshotCond.IntMapFilters),
			StringMapFilters: fieldFiltersEntityToCK(filter.ItemSnapshotCond.StringMapFilters),
		}
	}
	if filter.KeywordSearch != nil {
		cond.KeywordSearch = &ck.KeywordMapCond{
			ItemSnapshotFilter: &ck.ItemSnapshotFilter{
				BoolMapFilters:   fieldFiltersEntityToCK(filter.KeywordSearch.ItemSnapshotFilter.BoolMapFilters),
				FloatMapFilters:  fieldFiltersEntityToCK(filter.KeywordSearch.ItemSnapshotFilter.FloatMapFilters),
				IntMapFilters:    fieldFiltersEntityToCK(filter.KeywordSearch.ItemSnapshotFilter.IntMapFilters),
				StringMapFilters: fieldFiltersEntityToCK(filter.KeywordSearch.ItemSnapshotFilter.StringMapFilters),
			},
			EvalTargetDataFilters: fieldFiltersEntityToCK(filter.KeywordSearch.EvalTargetDataFilters),
			Keyword:               filter.KeywordSearch.Keyword,
		}
	}
	// 分页
	cond.Page = ck.Page{
		Offset: filter.Page.Offset(),
		Limit:  filter.Page.Limit(),
	}
	itemIDStates, total, err := e.exptTurnResultFilterDAO.QueryItemIDStates(ctx, cond)
	if err != nil {
		logs.CtxError(ctx, "QueryItemIDStates failed: %v", err)
	}
	itemID2ItemRunState := make(map[int64]entity.ItemRunState)
	for itemIDStr, status := range itemIDStates {
		itemID, err := strconv.ParseInt(itemIDStr, 10, 64)
		if err != nil {
			logs.CtxError(ctx, "ParseInt failed: %v", err)
		}
		itemID2ItemRunState[itemID] = entity.ItemRunState(status)
	}
	return itemID2ItemRunState, total, nil
}

func (e *ExptTurnResultFilterRepoImpl) GetExptTurnResultFilterKeyMappings(ctx context.Context, spaceID, exptID int64) ([]*entity.ExptTurnResultFilterKeyMapping, error) {
	pos, err := e.exptTurnResultFilterKeyMappingDAO.GetByExptID(ctx, spaceID, exptID)
	if err != nil {
		return nil, err
	}
	dos := make([]*entity.ExptTurnResultFilterKeyMapping, 0)
	for _, po := range pos {
		dos = append(dos, convert.ExptTurnResultFilterKeyMappingPO2DO(po))
	}
	return dos, nil
}

func (e *ExptTurnResultFilterRepoImpl) InsertExptTurnResultFilterKeyMappings(ctx context.Context, mappings []*entity.ExptTurnResultFilterKeyMapping) error {
	if len(mappings) == 0 {
		return nil
	}
	pos := make([]*mysqlmodel.ExptTurnResultFilterKeyMapping, 0, len(mappings))
	for _, mapping := range mappings {
		pos = append(pos, convert.ExptTurnResultFilterKeyMappingDO2PO(mapping))
	}
	return e.exptTurnResultFilterKeyMappingDAO.Insert(ctx, pos)
}

func (e *ExptTurnResultFilterRepoImpl) GetByExptIDItemIDs(ctx context.Context, spaceID, exptID, createdDate string, itemIDs []string) ([]*entity.ExptTurnResultFilterEntity, error) {
	pos, err := e.exptTurnResultFilterDAO.GetByExptIDItemIDs(ctx, spaceID, exptID, createdDate, itemIDs)
	if err != nil {
		return nil, err
	}
	dos := make([]*entity.ExptTurnResultFilterEntity, 0)
	for _, po := range pos {
		do := &entity.ExptTurnResultFilterEntity{
			SpaceID:        convertor.ParseStringToInt64(po.SpaceID),
			ExptID:         convertor.ParseStringToInt64(po.ExptID),
			ItemID:         convertor.ParseStringToInt64(po.ItemID),
			ItemIdx:        po.ItemIdx,
			TurnID:         convertor.ParseStringToInt64(po.TurnID),
			Status:         entity.ItemRunState(po.Status),
			EvalTargetData: make(map[string]string),
			EvaluatorScore: make(map[string]float64),
		}
		if po.ActualOutput != nil {
			do.EvalTargetData["actual_output"] = *po.ActualOutput
		}
		if po.EvaluatorScoreKey1 != nil {
			do.EvaluatorScore["key1"] = *po.EvaluatorScoreKey1
		}
		if po.EvaluatorScoreKey2 != nil {
			do.EvaluatorScore["key2"] = *po.EvaluatorScoreKey2
		}
		if po.EvaluatorScoreKey3 != nil {
			do.EvaluatorScore["key3"] = *po.EvaluatorScoreKey3
		}
		if po.EvaluatorScoreKey4 != nil {
			do.EvaluatorScore["key4"] = *po.EvaluatorScoreKey4
		}
		if po.EvaluatorScoreKey5 != nil {
			do.EvaluatorScore["key5"] = *po.EvaluatorScoreKey5
		}
		if po.EvaluatorScoreKey6 != nil {
			do.EvaluatorScore["key6"] = *po.EvaluatorScoreKey6
		}
		if po.EvaluatorScoreKey7 != nil {
			do.EvaluatorScore["key7"] = *po.EvaluatorScoreKey7
		}
		if po.EvaluatorScoreKey8 != nil {
			do.EvaluatorScore["key8"] = *po.EvaluatorScoreKey8
		}
		if po.EvaluatorScoreKey9 != nil {
			do.EvaluatorScore["key9"] = *po.EvaluatorScoreKey9
		}
		if po.EvaluatorScoreKey10 != nil {
			do.EvaluatorScore["key10"] = *po.EvaluatorScoreKey10
		}
		dos = append(dos, do)
	}
	return dos, nil
}
