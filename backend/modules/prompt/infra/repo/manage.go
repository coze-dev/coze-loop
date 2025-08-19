// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/samber/lo"
	"golang.org/x/exp/maps"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	metricsinfra "github.com/coze-dev/coze-loop/backend/modules/prompt/infra/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/query"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/redis"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ManageRepoImpl struct {
	db    db.Provider
	idgen idgen.IIDGenerator

	promptBasicDAO  mysql.IPromptBasicDAO
	promptCommitDAO mysql.IPromptCommitDAO
	promptDraftDAO  mysql.IPromptUserDraftDAO

	promptBasicCacheDAO redis.IPromptBasicDAO
	promptCacheDAO      redis.IPromptDAO

	promptCacheMetrics *metricsinfra.PromptCacheMetrics
}

func NewManageRepo(
	db db.Provider,
	idgen idgen.IIDGenerator,
	meter metrics.Meter,
	promptBasicDao mysql.IPromptBasicDAO,
	promptCommitDao mysql.IPromptCommitDAO,
	promptDraftDao mysql.IPromptUserDraftDAO,
	promptBasicCacheDAO redis.IPromptBasicDAO,
	promptCacheDAO redis.IPromptDAO,
) repo.IManageRepo {
	return &ManageRepoImpl{
		db:                  db,
		idgen:               idgen,
		promptBasicDAO:      promptBasicDao,
		promptCommitDAO:     promptCommitDao,
		promptDraftDAO:      promptDraftDao,
		promptBasicCacheDAO: promptBasicCacheDAO,
		promptCacheDAO:      promptCacheDAO,
		promptCacheMetrics:  metricsinfra.NewPromptCacheMetrics(meter),
	}
}

func (d *ManageRepoImpl) CreatePrompt(ctx context.Context, promptDO *entity.Prompt) (promptID int64, err error) {
	if promptDO == nil || promptDO.PromptBasic == nil {
		return 0, errorx.New("promptDO or promptDO.PromptBasic is empty")
	}

	promptID, err = d.idgen.GenID(ctx)
	if err != nil {
		return 0, err
	}
	var draftID int64
	if promptDO.PromptDraft != nil {
		draftID, err = d.idgen.GenID(ctx)
		if err != nil {
			return 0, err
		}
	}

	return promptID, d.db.Transaction(ctx, func(tx *gorm.DB) error {
		opt := db.WithTransaction(tx)

		basicPO := convertor.PromptDO2BasicPO(promptDO)
		basicPO.ID = promptID
		err = d.promptBasicDAO.Create(ctx, basicPO, opt)
		if err != nil {
			return err
		}

		if promptDO.PromptDraft != nil {
			draftPO := convertor.PromptDO2DraftPO(promptDO)
			draftPO.ID = draftID
			draftPO.PromptID = promptID
			err = d.promptDraftDAO.Create(ctx, draftPO, opt)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (d *ManageRepoImpl) DeletePrompt(ctx context.Context, promptID int64) (err error) {
	if promptID <= 0 {
		return errorx.New("promptID is invalid, promptID = %d", promptID)
	}
	promptBasicPO, err := d.promptBasicDAO.Get(ctx, promptID, false)
	if err != nil {
		return err
	}
	if promptBasicPO == nil {
		return errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("prompt is not found, prompt id = %d", promptID)))
	}
	err = d.promptBasicDAO.Delete(ctx, promptID)
	if err != nil {
		return err
	}
	cacheErr := d.promptBasicCacheDAO.DelByPromptKey(ctx, promptBasicPO.SpaceID, promptBasicPO.PromptKey)
	if cacheErr != nil {
		logs.CtxError(ctx, "delete prompt basic cache failed, prompt id = %d, err = %v", promptID, cacheErr)
	}
	return nil
}

func (d *ManageRepoImpl) GetPrompt(ctx context.Context, param repo.GetPromptParam) (promptDO *entity.Prompt, err error) {
	if param.PromptID <= 0 {
		return nil, errorx.New("param.PromptID is invalid, param = %s", json.Jsonify(param))
	}
	if param.WithCommit && lo.IsEmpty(param.CommitVersion) {
		return nil, errorx.New("Get with commit, but param.CommitVersion is empty, param = %s", json.Jsonify(param))
	}
	if param.WithDraft && lo.IsEmpty(param.UserID) {
		return nil, errorx.New("Get with draft, but param.UserID is empty, param = %s", json.Jsonify(param))
	}

	err = d.db.Transaction(ctx, func(tx *gorm.DB) error {
		opt := db.WithTransaction(tx)

		var basicPO *model.PromptBasic
		basicPO, err = d.promptBasicDAO.Get(ctx, param.PromptID, false, opt)
		if err != nil {
			return err
		}
		if basicPO == nil {
			return errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("prompt id = %d", param.PromptID)))
		}

		var commitPO *model.PromptCommit
		if param.WithCommit {
			commitPO, err = d.promptCommitDAO.Get(ctx, param.PromptID, param.CommitVersion, opt)
			if err != nil {
				return err
			}
			if commitPO == nil {
				return errorx.New("Get with commit, but it's not found, prompt id = %d, commit version = %s", param.PromptID, param.CommitVersion)
			}
		}

		var draftPO *model.PromptUserDraft
		if param.WithDraft {
			draftPO, err = d.promptDraftDAO.Get(ctx, param.PromptID, param.UserID, opt)
			if err != nil {
				return err
			}
		}

		promptDO = convertor.PromptPO2DO(basicPO, commitPO, draftPO)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return promptDO, nil
}

func (d *ManageRepoImpl) MGetPrompt(ctx context.Context, queries []repo.GetPromptParam, opts ...repo.GetPromptOptionFunc) (promptDOMap map[repo.GetPromptParam]*entity.Prompt, err error) {
	promptDOMap = make(map[repo.GetPromptParam]*entity.Prompt)
	if len(queries) == 0 {
		return nil, nil
	}
	options := &repo.GetPromptOption{}
	for _, opt := range opts {
		opt(options)
	}

	// try get from cache
	var cachedPromptMap map[redis.PromptQuery]*entity.Prompt
	var cacheErr error
	if options.CacheEnable {
		var cacheQueries []redis.PromptQuery
		for _, query := range queries {
			if query.WithDraft || !query.WithCommit {
				return nil, errorx.New("enable cache is allowed only when getting prompt with commit")
			}
			cacheQueries = append(cacheQueries, redis.PromptQuery{
				PromptID:      query.PromptID,
				WithCommit:    query.WithCommit,
				CommitVersion: query.CommitVersion,
			})
		}
		cachedPromptMap, cacheErr = d.promptCacheDAO.MGet(ctx, cacheQueries)
		if cacheErr != nil {
			logs.CtxError(ctx, "get prompt from cache error, queries=%s, err=%v", json.MarshalStringIgnoreErr(cacheQueries), cacheErr)
		}
		d.promptCacheMetrics.MEmit(ctx, metricsinfra.PromptCacheMetricsParam{
			QueryType:  metricsinfra.QueryTypePromptID,
			WithCommit: true,
			HitNum:     len(cachedPromptMap),
			MissNum:    len(cacheQueries) - len(cachedPromptMap),
		})
	}
	var missedQueries []repo.GetPromptParam
	for _, query := range queries {
		if cachedPrompt, ok := cachedPromptMap[redis.PromptQuery{
			PromptID:      query.PromptID,
			WithCommit:    query.WithCommit,
			CommitVersion: query.CommitVersion,
		}]; ok && cachedPrompt != nil {
			promptDOMap[query] = cachedPrompt
		} else {
			missedQueries = append(missedQueries, query)
		}
	}

	missedPromptMap, err := d.mGetPromptFromDB(ctx, missedQueries)
	if err != nil {
		return nil, err
	}
	for missedQuery, missedPrompt := range missedPromptMap {
		promptDOMap[missedQuery] = missedPrompt
	}

	// try set to cache
	if options.CacheEnable {
		cacheErr = d.promptCacheDAO.MSet(ctx, maps.Values(missedPromptMap))
		if cacheErr != nil {
			logs.CtxError(ctx, "get prompt from cache error, err=%v", cacheErr)
		}
	}
	return promptDOMap, nil
}

func (d *ManageRepoImpl) mGetPromptFromDB(ctx context.Context, queries []repo.GetPromptParam) (promptDOMap map[repo.GetPromptParam]*entity.Prompt, err error) {
	promptDOMap = make(map[repo.GetPromptParam]*entity.Prompt)
	if len(queries) == 0 {
		return nil, nil
	}
	var allPromptIDs []int64
	needDraftPromptIDUserIDMap := make(map[repo.GetPromptParam]bool)
	needCommitPromptIDVersionMap := make(map[repo.GetPromptParam]bool)
	for _, query := range queries {
		allPromptIDs = append(allPromptIDs, query.PromptID)
		if query.WithDraft {
			needDraftPromptIDUserIDMap[query] = true
		}
		if query.WithCommit {
			needCommitPromptIDVersionMap[query] = true
		}
	}

	idPromptBasicPOMap, err := d.promptBasicDAO.MGet(ctx, allPromptIDs)
	if err != nil {
		return nil, err
	}

	draftPOMap := make(map[mysql.PromptIDUserIDPair]*model.PromptUserDraft)
	if len(needDraftPromptIDUserIDMap) > 0 {
		var promptDraftQueries []mysql.PromptIDUserIDPair
		for promptQuery := range needDraftPromptIDUserIDMap {
			promptDraftQueries = append(promptDraftQueries, mysql.PromptIDUserIDPair{
				PromptID: promptQuery.PromptID,
				UserID:   promptQuery.UserID,
			})
		}
		draftPOMap, err = d.promptDraftDAO.MGet(ctx, promptDraftQueries)
		if err != nil {
			return nil, err
		}
	}

	commitPOMap := make(map[mysql.PromptIDCommitVersionPair]*model.PromptCommit)
	if len(needCommitPromptIDVersionMap) > 0 {
		var promptCommitQueries []mysql.PromptIDCommitVersionPair
		for promptQuery := range needCommitPromptIDVersionMap {
			promptCommitQueries = append(promptCommitQueries, mysql.PromptIDCommitVersionPair{
				PromptID:      promptQuery.PromptID,
				CommitVersion: promptQuery.CommitVersion,
			})
		}
		commitPOMap, err = d.promptCommitDAO.MGet(ctx, promptCommitQueries)
		if err != nil {
			return nil, err
		}
	}

	for _, query := range queries {
		promptBasicPO := idPromptBasicPOMap[query.PromptID]
		if promptBasicPO == nil {
			return nil, errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("prompt not found, prompt_id=%d", query.PromptID)))
		}
		var promptDraftPO *model.PromptUserDraft
		if query.WithDraft {
			promptDraftPO = draftPOMap[mysql.PromptIDUserIDPair{
				PromptID: query.PromptID,
				UserID:   query.UserID,
			}]
			if promptDraftPO == nil {
				return nil, errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("prompt draft not found, prompt_id=%d, user_id=%s", query.PromptID, query.UserID)))
			}
		}
		var promptCommitPO *model.PromptCommit
		if query.WithCommit {
			promptCommitPO = commitPOMap[mysql.PromptIDCommitVersionPair{
				PromptID:      query.PromptID,
				CommitVersion: query.CommitVersion,
			}]
			if promptCommitPO == nil {
				return nil, errorx.NewByCode(prompterr.PromptVersionNotExistCode,
					errorx.WithExtraMsg(fmt.Sprintf("prompt commit not found, prompt_id=%d, commit_version=%s", query.PromptID, query.CommitVersion)),
					errorx.WithExtra(map[string]string{"prompt_id": strconv.FormatInt(query.PromptID, 10), "version": query.CommitVersion}))
			}
		}
		promptDOMap[query] = convertor.PromptPO2DO(promptBasicPO, promptCommitPO, promptDraftPO)
	}
	return promptDOMap, nil
}

func (d *ManageRepoImpl) MGetPromptBasicByPromptKey(ctx context.Context, spaceID int64, promptKeys []string, opts ...repo.GetPromptBasicOptionFunc) (promptDOs []*entity.Prompt, err error) {
	if len(promptKeys) == 0 {
		return nil, nil
	}
	options := &repo.GetPromptBasicOption{}
	for _, opt := range opts {
		opt(options)
	}
	var cacheResultMap map[string]*entity.Prompt
	var cacheErr error
	if options.CacheEnable {
		// try get from cache
		cacheResultMap, cacheErr = d.promptBasicCacheDAO.MGetByPromptKey(ctx, spaceID, promptKeys)
		if cacheErr != nil {
			logs.CtxError(ctx, "get prompt basic from cache failed, space_id=%d, prompt_keys=%s, err=%v", spaceID, json.MarshalStringIgnoreErr(promptKeys), err)
		}
		d.promptCacheMetrics.MEmit(ctx, metricsinfra.PromptCacheMetricsParam{
			QueryType:  metricsinfra.QueryTypePromptKey,
			WithCommit: false,
			HitNum:     len(cacheResultMap),
			MissNum:    len(promptKeys) - len(cacheResultMap),
		})
	}

	var missedPromptKeys []string
	for _, promptKey := range promptKeys {
		if promptDO, ok := cacheResultMap[promptKey]; ok && promptDO != nil {
			promptDOs = append(promptDOs, promptDO)
		} else {
			missedPromptKeys = append(missedPromptKeys, promptKey)
		}
	}
	// get from rds
	missedPrompts, err := d.mGetPromptBasicByPromptKeyFromDB(ctx, spaceID, missedPromptKeys)
	if err != nil {
		return nil, err
	}
	promptDOs = append(promptDOs, missedPrompts...)

	if options.CacheEnable {
		// try set to cache
		cacheErr = d.promptBasicCacheDAO.MSetByPromptKey(ctx, missedPrompts)
		if cacheErr != nil {
			logs.CtxError(ctx, "set prompt basic to cache failed, err=%v", cacheErr)
		}
	}
	return promptDOs, nil
}

func (d *ManageRepoImpl) mGetPromptBasicByPromptKeyFromDB(ctx context.Context, spaceID int64, promptKeys []string) (promptDOs []*entity.Prompt, err error) {
	if len(promptKeys) == 0 {
		return nil, nil
	}
	basicPOs, err := d.promptBasicDAO.MGetByPromptKey(ctx, spaceID, promptKeys)
	if err != nil {
		return nil, err
	}
	promptDOs = append(promptDOs, convertor.BatchBasicPO2PromptDO(basicPOs)...)
	return promptDOs, nil
}

func (d *ManageRepoImpl) ListPrompt(ctx context.Context, param repo.ListPromptParam) (result *repo.ListPromptResult, err error) {
	if param.SpaceID <= 0 || param.PageNum < 1 || param.PageSize <= 0 {
		return nil, errorx.New("param(SpaceID or PageNum or PageSize) is invalid, param = %s", json.Jsonify(param))
	}

	listBasicParam := mysql.ListPromptBasicParam{
		SpaceID: param.SpaceID,

		KeyWord:       param.KeyWord,
		CreatedBys:    param.CreatedBys,
		CommittedOnly: param.CommittedOnly,

		Offset:  (param.PageNum - 1) * param.PageSize,
		Limit:   param.PageSize,
		OrderBy: param.OrderBy,
		Asc:     param.Asc,
	}
	basicPOs, total, err := d.promptBasicDAO.List(ctx, listBasicParam)
	if err != nil {
		return nil, err
	}

	draftPOMap := make(map[mysql.PromptIDUserIDPair]*model.PromptUserDraft)
	if len(basicPOs) > 0 {
		var promptDraftQueries []mysql.PromptIDUserIDPair
		for _, basicPO := range basicPOs {
			promptDraftQueries = append(promptDraftQueries, mysql.PromptIDUserIDPair{
				PromptID: basicPO.ID,
				UserID:   param.UserID,
			})
		}
		draftPOMap, err = d.promptDraftDAO.MGet(ctx, promptDraftQueries)
		if err != nil {
			return nil, err
		}
	}

	return &repo.ListPromptResult{
		Total:     total,
		PromptDOs: convertor.BatchBasicAndDraftPO2PromptDO(basicPOs, draftPOMap, param.UserID),
	}, nil
}

func (d *ManageRepoImpl) UpdatePrompt(ctx context.Context, param repo.UpdatePromptParam) (err error) {
	if param.PromptID <= 0 || lo.IsEmpty(param.PromptName) {
		return errorx.New("param(PromptID or PromptName) is invalid, param = %s", json.Jsonify(param))
	}

	basicPO, err := d.promptBasicDAO.Get(ctx, param.PromptID, false)
	if err != nil {
		return err
	}
	if basicPO == nil {
		return errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("prompt not found, prompt_id=%d", param.PromptID)))
	}

	q := query.Use(d.db.NewSession(ctx))
	updateFields := map[string]interface{}{
		q.PromptBasic.UpdatedBy.ColumnName().String(): param.UpdatedBy,

		q.PromptBasic.Name.ColumnName().String():        param.PromptName,
		q.PromptBasic.Description.ColumnName().String(): param.PromptDescription,
	}
	err = d.promptBasicDAO.Update(ctx, param.PromptID, updateFields)
	if err != nil {
		return err
	}
	cacheErr := d.promptBasicCacheDAO.DelByPromptKey(ctx, basicPO.SpaceID, basicPO.PromptKey)
	if cacheErr != nil {
		logs.CtxError(ctx, "delete prompt basic cache failed, prompt id = %d, err = %v", param.PromptID, cacheErr)
	}
	return nil
}

func (d *ManageRepoImpl) SaveDraft(ctx context.Context, promptDO *entity.Prompt) (draftInfo *entity.DraftInfo, err error) {
	if promptDO == nil || promptDO.PromptDraft == nil {
		return nil, errorx.New("promptDO or promptDO.PromptDraft is empty")
	}

	err = d.db.Transaction(ctx, func(tx *gorm.DB) error {
		opt := db.WithTransaction(tx)

		var basicPO *model.PromptBasic
		basicPO, err = d.promptBasicDAO.Get(ctx, promptDO.ID, true, opt)
		if err != nil {
			return err
		}
		if basicPO == nil {
			return errorx.New("Prompt is not found, prompt id = %d", promptDO.ID)
		}

		var baseCommitPO *model.PromptCommit
		savingBaseVersion := promptDO.PromptDraft.DraftInfo.BaseVersion
		if !lo.IsEmpty(savingBaseVersion) {
			baseCommitPO, err = d.promptCommitDAO.Get(ctx, promptDO.ID, savingBaseVersion, opt)
			if err != nil {
				return err
			}
			if baseCommitPO == nil {
				return errorx.New("Draft's base prompt commit is not found, prompt id = %d, base commit version = %s", promptDO.ID, savingBaseVersion)
			}
		}

		var originalDraftPO *model.PromptUserDraft
		userID := promptDO.PromptDraft.DraftInfo.UserID
		originalDraftPO, err = d.promptDraftDAO.Get(ctx, promptDO.ID, userID, opt)
		if err != nil {
			return err
		}

		// 创建
		if originalDraftPO == nil {
			promptDO.PromptDraft.DraftInfo.IsModified = true
			creatingDraftPO := convertor.PromptDO2DraftPO(promptDO)
			creatingDraftPO.ID, err = d.idgen.GenID(ctx)
			creatingDraftPO.SpaceID = basicPO.SpaceID
			if err != nil {
				return err
			}
			err = d.promptDraftDAO.Create(ctx, creatingDraftPO, opt)
			if err != nil {
				return err
			}
			createdDraftPO, err := d.promptDraftDAO.GetByID(ctx, creatingDraftPO.ID, opt)
			if err != nil {
				return err
			}
			if createdDraftPO != nil {
				draftInfo = convertor.DraftPO2DO(createdDraftPO).DraftInfo
			}

			return nil
		}

		originalDraftDO := convertor.DraftPO2DO(originalDraftPO)
		originalDraftDetailDO := originalDraftDO.PromptDetail
		updatingDraftDetailDO := promptDO.PromptDraft.PromptDetail
		// 草稿无变化
		if updatingDraftDetailDO.DeepEqual(originalDraftDetailDO) {
			return nil
		}
		// 草稿相对于base commit是否有变化
		if baseCommitPO == nil {
			promptDO.PromptDraft.DraftInfo.IsModified = true
		} else {
			baseCommitDO := convertor.CommitPO2DO(baseCommitPO)
			baseCommitDetailDO := baseCommitDO.PromptDetail
			promptDO.PromptDraft.DraftInfo.IsModified = !updatingDraftDetailDO.DeepEqual(baseCommitDetailDO)
		}
		// 持久化更新
		updatingDraftPO := convertor.PromptDO2DraftPO(promptDO)
		updatingDraftPO.ID = originalDraftPO.ID
		err = d.promptDraftDAO.Update(ctx, updatingDraftPO, opt)
		if err != nil {
			return err
		}
		updatedDraftPO, err := d.promptDraftDAO.GetByID(ctx, updatingDraftPO.ID, opt)
		if err != nil {
			return err
		}
		if updatedDraftPO != nil {
			draftInfo = convertor.DraftPO2DO(updatedDraftPO).DraftInfo
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return draftInfo, nil
}

func (d *ManageRepoImpl) CommitDraft(ctx context.Context, param repo.CommitDraftParam) (err error) {
	if param.PromptID <= 0 || lo.IsEmpty(param.UserID) || lo.IsEmpty(param.CommitVersion) {
		return errorx.New("param(PromptID or UserID or CommitVersion) is invalid, param = %s", json.Jsonify(param))
	}

	commitID, err := d.idgen.GenID(ctx)
	if err != nil {
		return err
	}

	var spaceID int64
	var promptKey string
	err = d.db.Transaction(ctx, func(tx *gorm.DB) error {
		opt := db.WithTransaction(tx)

		var basicPO *model.PromptBasic
		basicPO, err = d.promptBasicDAO.Get(ctx, param.PromptID, true, opt)
		if err != nil {
			return err
		}
		if basicPO == nil {
			return errorx.New("Prompt is not found, prompt id = %d", param.PromptID)
		}
		spaceID = basicPO.SpaceID
		promptKey = basicPO.PromptKey

		var draftPO *model.PromptUserDraft
		draftPO, err = d.promptDraftDAO.Get(ctx, param.PromptID, param.UserID, opt)
		if err != nil {
			return err
		}
		if draftPO == nil {
			return errorx.New("Prompt draft is not found, prompt id = %d, user id = %s", param.PromptID, param.UserID)
		}

		draftDO := convertor.DraftPO2DO(draftPO)
		commitDO := &entity.PromptCommit{
			CommitInfo: &entity.CommitInfo{
				Version:     param.CommitVersion,
				BaseVersion: draftPO.BaseVersion,
				Description: param.CommitDescription,
				CommittedBy: param.UserID,
			},
			PromptDetail: draftDO.PromptDetail,
		}
		promptDO := convertor.PromptPO2DO(basicPO, nil, nil)
		promptDO.PromptCommit = commitDO
		commitPO := convertor.PromptDO2CommitPO(promptDO)
		commitPO.ID = commitID
		timeNow := time.Now()
		err = d.promptCommitDAO.Create(ctx, commitPO, timeNow, opt)
		if err != nil {
			return err
		}
		err = d.promptDraftDAO.Delete(ctx, draftPO.ID, opt)
		if err != nil {
			return err
		}
		q := query.Use(d.db.NewSession(ctx, opt))
		err = d.promptBasicDAO.Update(ctx, basicPO.ID, map[string]interface{}{
			q.PromptBasic.LatestCommitTime.ColumnName().String(): timeNow,
			q.PromptBasic.LatestVersion.ColumnName().String():    param.CommitVersion,
		}, opt)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	cacheErr := d.promptBasicCacheDAO.DelByPromptKey(ctx, spaceID, promptKey)
	if cacheErr != nil {
		logs.CtxError(ctx, "delete prompt basic from cache failed, err=%v", cacheErr)
	}
	return nil
}

func (d *ManageRepoImpl) ListCommitInfo(ctx context.Context, param repo.ListCommitInfoParam) (result *repo.ListCommitResult, err error) {
	if param.PromptID <= 0 || param.PageSize <= 0 {
		return nil, errorx.New("Param(PromptID or PageSize) is invalid, param = %s", json.Jsonify(param))
	}

	listCommitParam := mysql.ListCommitParam{
		PromptID: param.PromptID,

		Cursor: param.PageToken,
		Limit:  param.PageSize + 1,
		Asc:    param.Asc,
	}
	commitPOs, err := d.promptCommitDAO.List(ctx, listCommitParam)
	if err != nil {
		return nil, err
	}
	if len(commitPOs) <= 0 {
		return nil, nil
	}

	result = &repo.ListCommitResult{}
	commitDOs := convertor.BatchCommitPO2DO(commitPOs)
	commitInfoDOs := convertor.BatchGetCommitInfoDOFromCommitDO(commitDOs)
	if len(commitPOs) <= param.PageSize {
		result.CommitInfoDOs = commitInfoDOs
		return result, nil
	}
	result.NextPageToken = commitPOs[param.PageSize].ID
	result.CommitInfoDOs = commitInfoDOs[:len(commitPOs)-1]
	return result, nil
}
