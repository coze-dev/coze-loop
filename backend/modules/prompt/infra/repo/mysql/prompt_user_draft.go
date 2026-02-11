// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"fmt"
	"time"

	"github.com/samber/lo"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/query"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

//go:generate mockgen -destination=mocks/prompt_user_draft_dao.go -package=mocks . IPromptUserDraftDAO
type IPromptUserDraftDAO interface {
	Create(ctx context.Context, promptDraftPO *model.PromptUserDraft, opts ...db.Option) (err error)
	Delete(ctx context.Context, draftID int64, opts ...db.Option) (err error)
	Get(ctx context.Context, promptID int64, userID string, opts ...db.Option) (promptDraftPO *model.PromptUserDraft, err error)
	GetByID(ctx context.Context, draftID int64, opts ...db.Option) (promptDraftPO *model.PromptUserDraft, err error)
	MGet(ctx context.Context, pairs []PromptIDUserIDPair, opts ...db.Option) (pairDraftPOMap map[PromptIDUserIDPair]*model.PromptUserDraft, err error)
	Update(ctx context.Context, promptDraftPO *model.PromptUserDraft, opts ...db.Option) (err error)
}

type PromptUserDraftDAOImpl struct {
	db           db.Provider
	writeTracker platestwrite.ILatestWriteTracker
}

func NewPromptUserDraftDAO(db db.Provider, redisCli redis.Cmdable) IPromptUserDraftDAO {
	return &PromptUserDraftDAOImpl{
		db:           db,
		writeTracker: platestwrite.NewLatestWriteTracker(redisCli),
	}
}

type PromptIDUserIDPair struct {
	PromptID int64
	UserID   string
}

func (d *PromptUserDraftDAOImpl) Create(ctx context.Context, promptDraftPO *model.PromptUserDraft, opts ...db.Option) (err error) {
	if promptDraftPO == nil {
		return errorx.New("promptDraftPO is empty")
	}
	d.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypePromptDraft, promptDraftPO.ID, platestwrite.SetWithSearchParam(fmt.Sprintf("%d:%s", promptDraftPO.ID, promptDraftPO.UserID)))

	q := query.Use(d.db.NewSession(ctx, opts...)).WithContext(ctx)
	promptDraftPO.CreatedAt = time.Time{}
	promptDraftPO.UpdatedAt = time.Time{}
	err = q.PromptUserDraft.Create(promptDraftPO)
	if err != nil {
		return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	return nil
}

func (d *PromptUserDraftDAOImpl) Get(ctx context.Context, promptID int64, userID string, opts ...db.Option) (promptDraftPO *model.PromptUserDraft, err error) {
	if promptID <= 0 || lo.IsEmpty(userID) {
		return nil, errorx.New("promptID or userID is invalid param, promptID = %d, userID = %s", promptID, userID)
	}
	if d.writeTracker.CheckWriteFlagBySearchParam(ctx, platestwrite.ResourceTypePromptDraft, fmt.Sprintf("%d:%s", promptID, userID)) {
		opts = append(opts, db.WithMaster())
	}

	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).PromptUserDraft
	tx = tx.Where(q.PromptUserDraft.PromptID.Eq(promptID), q.PromptUserDraft.UserID.Eq(userID))
	promptDraftPOs, err := tx.Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	if len(promptDraftPOs) <= 0 {
		return nil, nil
	}
	return promptDraftPOs[0], nil
}

func (d *PromptUserDraftDAOImpl) GetByID(ctx context.Context, draftID int64, opts ...db.Option) (promptDraftPO *model.PromptUserDraft, err error) {
	if draftID <= 0 {
		return nil, errorx.New("draftID is invalid, draftID = %d", draftID)
	}
	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).PromptUserDraft
	tx = tx.Where(q.PromptUserDraft.ID.Eq(draftID))
	promptDraftPOs, err := tx.Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	if len(promptDraftPOs) <= 0 {
		return nil, nil
	}
	return promptDraftPOs[0], nil
}

func (d *PromptUserDraftDAOImpl) MGet(ctx context.Context, pairs []PromptIDUserIDPair, opts ...db.Option) (pairDraftPOMap map[PromptIDUserIDPair]*model.PromptUserDraft, err error) {
	if len(pairs) <= 0 {
		return nil, errorx.WrapByCode(err, prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("PromptUserDraftDAOImpl.MGet invalid param"))
	}
	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).PromptUserDraft
	for i, pair := range pairs {
		if i == 0 {
			tx = tx.Where(q.PromptUserDraft.PromptID.Eq(pair.PromptID), q.PromptUserDraft.UserID.Eq(pair.UserID))
		} else {
			tx = tx.Or(q.PromptUserDraft.PromptID.Eq(pair.PromptID), q.PromptUserDraft.UserID.Eq(pair.UserID))
		}
	}
	promptDraftPOs, err := tx.Find()
	if err != nil {
		return nil, err
	}
	if len(promptDraftPOs) <= 0 {
		return nil, nil
	}
	pairDraftPOMap = make(map[PromptIDUserIDPair]*model.PromptUserDraft, len(promptDraftPOs))
	for _, promptDraftPO := range promptDraftPOs {
		if promptDraftPO == nil {
			continue
		}
		pairDraftPOMap[PromptIDUserIDPair{
			PromptID: promptDraftPO.PromptID,
			UserID:   promptDraftPO.UserID,
		}] = promptDraftPO
	}
	if len(pairDraftPOMap) <= 0 {
		return nil, nil
	}
	return pairDraftPOMap, nil
}

func (d *PromptUserDraftDAOImpl) Update(ctx context.Context, promptDraftPO *model.PromptUserDraft, opts ...db.Option) (err error) {
	if promptDraftPO == nil {
		return errorx.New("promptDraftPO is empty")
	}

	q := query.Use(d.db.NewSession(ctx, opts...))
	_, err = q.PromptUserDraft.WithContext(ctx).Where(q.PromptUserDraft.ID.Eq(promptDraftPO.ID)).
		Updates(map[string]interface{}{
			q.PromptUserDraft.Messages.ColumnName().String():       promptDraftPO.Messages,
			q.PromptUserDraft.ModelConfig.ColumnName().String():    promptDraftPO.ModelConfig,
			q.PromptUserDraft.BaseVersion.ColumnName().String():    promptDraftPO.BaseVersion,
			q.PromptUserDraft.Tools.ColumnName().String():          promptDraftPO.Tools,
			q.PromptUserDraft.ToolCallConfig.ColumnName().String(): promptDraftPO.ToolCallConfig,
			q.PromptUserDraft.TemplateType.ColumnName().String():   promptDraftPO.TemplateType,
			q.PromptUserDraft.VariableDefs.ColumnName().String():   promptDraftPO.VariableDefs,
			q.PromptUserDraft.Metadata.ColumnName().String():       promptDraftPO.Metadata,
			q.PromptUserDraft.McpConfig.ColumnName().String():      promptDraftPO.McpConfig,
			q.PromptUserDraft.IsDraftEdited.ColumnName().String():  promptDraftPO.IsDraftEdited,
			q.PromptUserDraft.HasSnippets.ColumnName().String():    promptDraftPO.HasSnippets,
		})
	if err != nil {
		return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	d.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypePromptDraft, promptDraftPO.ID, platestwrite.SetWithSearchParam(fmt.Sprintf("%d:%s", promptDraftPO.ID, promptDraftPO.UserID)))
	return nil
}

func (d *PromptUserDraftDAOImpl) Delete(ctx context.Context, draftID int64, opts ...db.Option) (err error) {
	if draftID <= 0 {
		return errorx.New("draftID is invalid, draftID = %d", draftID)
	}
	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).PromptUserDraft
	tx = tx.Where(q.PromptUserDraft.ID.Eq(draftID))
	_, err = tx.Delete(&model.PromptUserDraft{})
	if err != nil {
		return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	d.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypePromptDraft, draftID)
	return nil
}
