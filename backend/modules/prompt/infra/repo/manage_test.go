// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	dbmocks "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	metricsinfra "github.com/coze-dev/coze-loop/backend/modules/prompt/infra/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/model"
	daomocks "github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/redis"
	redismocks "github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/redis/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/unittest"
)

func TestManageRepoImpl_MGetPrompt(t *testing.T) {
	type fields struct {
		promptBasicDAO     mysql.IPromptBasicDAO
		promptCommitDAO    mysql.IPromptCommitDAO
		promptDraftDAO     mysql.IPromptUserDraftDAO
		promptCacheDAO     redis.IPromptDAO
		promptCacheMetrics *metricsinfra.PromptCacheMetrics
	}
	type args struct {
		ctx     context.Context
		queries []repo.GetPromptParam
		opts    []repo.GetPromptOptionFunc
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         map[repo.GetPromptParam]*entity.Prompt
		wantErr      error
	}{
		{
			name: "empty queries",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx:     context.Background(),
				queries: []repo.GetPromptParam{},
				opts:    []repo.GetPromptOptionFunc{},
			},
			want:    nil,
			wantErr: nil,
		},
		{
			name: "get draft with cache enabled",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				queries: []repo.GetPromptParam{
					{
						PromptID:  123,
						WithDraft: true,
						UserID:    "111222",
					},
				},
				opts: []repo.GetPromptOptionFunc{
					repo.WithPromptCacheEnable(),
				},
			},
			want:    nil,
			wantErr: errorx.New("enable cache is allowed only when getting prompt with commit"),
		},
		{
			name: "get without commit when cache enabled",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				queries: []repo.GetPromptParam{
					{
						PromptID: 123,
					},
				},
				opts: []repo.GetPromptOptionFunc{
					repo.WithPromptCacheEnable(),
				},
			},
			want:    nil,
			wantErr: errorx.New("enable cache is allowed only when getting prompt with commit"),
		},
		{
			name: "get prompt draft success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().MGet(gomock.Any(), []int64{123}, gomock.Any()).Return(map[int64]*model.PromptBasic{
					123: {
						ID:         123,
						SpaceID:    123456,
						PromptKey:  "test_key_1",
						PromptType: "normal",
					},
				}, nil)
				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().MGet(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[mysql.PromptIDUserIDPair]*model.PromptUserDraft{
					{
						PromptID: 123,
						UserID:   "111222",
					}: {
						PromptID: 123,
					},
				}, nil)
				return fields{
					promptBasicDAO: mockBasicDAO,
					promptDraftDAO: mockDraftDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				queries: []repo.GetPromptParam{
					{
						PromptID:  123,
						WithDraft: true,
						UserID:    "111222",
					},
				},
			},
			want: map[repo.GetPromptParam]*entity.Prompt{
				{
					PromptID:  123,
					WithDraft: true,
					UserID:    "111222",
				}: {
					ID:        123,
					SpaceID:   123456,
					PromptKey: "test_key_1",
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
					PromptDraft: &entity.PromptDraft{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{},
						},
						DraftInfo: &entity.DraftInfo{},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "get prompt with commit partial cache hit",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockCacheDAO := redismocks.NewMockIPromptDAO(ctrl)
				mockCacheDAO.EXPECT().MGet(gomock.Any(), gomock.Any()).Return(map[redis.PromptQuery]*entity.Prompt{
					{
						PromptID:      123,
						WithCommit:    true,
						CommitVersion: "1.0.0",
					}: {
						ID:        123,
						SpaceID:   123456,
						PromptKey: "test_key_1",
						PromptBasic: &entity.PromptBasic{
							PromptType: entity.PromptTypeNormal,
						},
						PromptCommit: &entity.PromptCommit{
							PromptDetail: &entity.PromptDetail{
								PromptTemplate: &entity.PromptTemplate{},
							},
							CommitInfo: &entity.CommitInfo{},
						},
					},
				}, nil)

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().MGet(gomock.Any(), []int64{456}, gomock.Any()).Return(map[int64]*model.PromptBasic{
					456: {
						ID:        456,
						SpaceID:   123456,
						PromptKey: "test_key_2",
					},
				}, nil)

				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().MGet(gomock.Any(), []mysql.PromptIDCommitVersionPair{
					{
						PromptID:      456,
						CommitVersion: "1.0.0",
					},
				}).Return(map[mysql.PromptIDCommitVersionPair]*model.PromptCommit{
					{
						PromptID:      456,
						CommitVersion: "1.0.0",
					}: {
						PromptID: 456,
					},
				}, nil)

				mockCacheDAO.EXPECT().MSet(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, prompts []*entity.Prompt) error {
					assert.Equal(t, 1, len(prompts))
					assert.Equal(t, int64(456), prompts[0].ID)
					return nil
				})

				// 添加缓存指标验证
				mockCacheMetrics := &metricsinfra.PromptCacheMetrics{}
				// 注意：这里我们无法直接mock PromptCacheMetrics.MEmit 方法，因为它不是接口
				// 在实际测试中，我们通过验证调用参数来确保指标正确发送

				return fields{
					promptBasicDAO:     mockBasicDAO,
					promptCommitDAO:    mockCommitDAO,
					promptCacheDAO:     mockCacheDAO,
					promptCacheMetrics: mockCacheMetrics,
				}
			},
			args: args{
				ctx: context.Background(),
				queries: []repo.GetPromptParam{
					{
						PromptID:      123,
						WithCommit:    true,
						CommitVersion: "1.0.0",
					},
					{
						PromptID:      456,
						WithCommit:    true,
						CommitVersion: "1.0.0",
					},
				},
				opts: []repo.GetPromptOptionFunc{
					repo.WithPromptCacheEnable(),
				},
			},
			want: map[repo.GetPromptParam]*entity.Prompt{
				{
					PromptID:      123,
					WithCommit:    true,
					CommitVersion: "1.0.0",
				}: {
					ID:        123,
					SpaceID:   123456,
					PromptKey: "test_key_1",
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
					PromptCommit: &entity.PromptCommit{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{},
						},
						CommitInfo: &entity.CommitInfo{},
					},
				},
				{
					PromptID:      456,
					WithCommit:    true,
					CommitVersion: "1.0.0",
				}: {
					ID:        456,
					SpaceID:   123456,
					PromptKey: "test_key_2",
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
					PromptCommit: &entity.PromptCommit{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{},
						},
						CommitInfo: &entity.CommitInfo{},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "get prompt basic without cache",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().MGet(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[int64]*model.PromptBasic{
					123: {
						ID:        123,
						SpaceID:   123456,
						PromptKey: "test_key",
					},
				}, nil)

				return fields{
					promptBasicDAO: mockBasicDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				queries: []repo.GetPromptParam{
					{
						PromptID: 123,
					},
				},
			},
			want: map[repo.GetPromptParam]*entity.Prompt{
				{
					PromptID: 123,
				}: {
					ID:        123,
					SpaceID:   123456,
					PromptKey: "test_key",
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "cache set error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockCacheDAO := redismocks.NewMockIPromptDAO(ctrl)
				mockCacheDAO.EXPECT().MGet(gomock.Any(), gomock.Any()).Return(map[redis.PromptQuery]*entity.Prompt{}, nil)

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().MGet(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[int64]*model.PromptBasic{
					123: {
						ID:        123,
						SpaceID:   123456,
						PromptKey: "test_key",
					},
				}, nil)

				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().MGet(gomock.Any(), []mysql.PromptIDCommitVersionPair{
					{
						PromptID:      123,
						CommitVersion: "1.0.0",
					},
				}).Return(map[mysql.PromptIDCommitVersionPair]*model.PromptCommit{
					{
						PromptID:      123,
						CommitVersion: "1.0.0",
					}: {
						PromptID: 123,
					},
				}, nil)

				mockCacheDAO.EXPECT().MSet(gomock.Any(), gomock.Any()).Return(errorx.New("cache set error"))

				return fields{
					promptBasicDAO:  mockBasicDAO,
					promptCommitDAO: mockCommitDAO,
					promptCacheDAO:  mockCacheDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				queries: []repo.GetPromptParam{
					{
						PromptID:      123,
						WithCommit:    true,
						CommitVersion: "1.0.0",
					},
				},
				opts: []repo.GetPromptOptionFunc{
					repo.WithPromptCacheEnable(),
				},
			},
			want: map[repo.GetPromptParam]*entity.Prompt{
				{
					PromptID:      123,
					WithCommit:    true,
					CommitVersion: "1.0.0",
				}: {
					ID:        123,
					SpaceID:   123456,
					PromptKey: "test_key",
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
					PromptCommit: &entity.PromptCommit{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								TemplateType: "",
							},
						},
						CommitInfo: &entity.CommitInfo{
							Version:     "",
							BaseVersion: "",
							Description: "",
							CommittedBy: "",
							CommittedAt: time.Time{},
						},
					},
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)

			d := &ManageRepoImpl{
				promptBasicDAO:     ttFields.promptBasicDAO,
				promptCommitDAO:    ttFields.promptCommitDAO,
				promptDraftDAO:     ttFields.promptDraftDAO,
				promptCacheDAO:     ttFields.promptCacheDAO,
				promptCacheMetrics: ttFields.promptCacheMetrics,
			}

			got, err := d.MGetPrompt(tt.args.ctx, tt.args.queries, tt.args.opts...)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, len(tt.want), len(got))
				for k, v := range tt.want {
					assert.Equal(t, v, got[k])
				}
			}
		})
	}
}

func TestManageRepoImpl_MGetPromptBasicByPromptKey(t *testing.T) {
	type fields struct {
		promptBasicDAO      mysql.IPromptBasicDAO
		promptBasicCacheDAO redis.IPromptBasicDAO
		promptCacheMetrics  *metricsinfra.PromptCacheMetrics
	}
	type args struct {
		ctx        context.Context
		spaceID    int64
		promptKeys []string
		opts       []repo.GetPromptBasicOptionFunc
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         []*entity.Prompt
		wantErr      error
	}{
		{
			name: "empty prompt keys",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx:        context.Background(),
				spaceID:    123,
				promptKeys: []string{},
				opts:       []repo.GetPromptBasicOptionFunc{},
			},
			want:    nil,
			wantErr: nil,
		},
		{
			name: "cache hit",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockBasicCacheDAO := redismocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicCacheDAO.EXPECT().MGetByPromptKey(gomock.Any(), int64(123), []string{"test_key_1", "test_key_2"}).Return(map[string]*entity.Prompt{
					"test_key_1": {
						ID:        123,
						SpaceID:   123,
						PromptKey: "test_key_1",
					},
					"test_key_2": {
						ID:        456,
						SpaceID:   123,
						PromptKey: "test_key_2",
					},
				}, nil)
				mockBasicCacheDAO.EXPECT().MSetByPromptKey(gomock.Any(), gomock.Any()).Return(nil)

				mockCacheMetrics := &metricsinfra.PromptCacheMetrics{}

				return fields{
					promptBasicCacheDAO: mockBasicCacheDAO,
					promptCacheMetrics:  mockCacheMetrics,
				}
			},
			args: args{
				ctx:        context.Background(),
				spaceID:    123,
				promptKeys: []string{"test_key_1", "test_key_2"},
				opts: []repo.GetPromptBasicOptionFunc{
					repo.WithPromptBasicCacheEnable(),
				},
			},
			want: []*entity.Prompt{
				{
					ID:        123,
					SpaceID:   123,
					PromptKey: "test_key_1",
				},
				{
					ID:        456,
					SpaceID:   123,
					PromptKey: "test_key_2",
				},
			},
			wantErr: nil,
		},
		{
			name: "cache miss",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockCacheDAO := redismocks.NewMockIPromptBasicDAO(ctrl)
				mockCacheDAO.EXPECT().MGetByPromptKey(gomock.Any(), int64(123), []string{"test_key_1", "test_key_2"}).Return(map[string]*entity.Prompt{}, nil)

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().MGetByPromptKey(gomock.Any(), int64(123), []string{"test_key_1", "test_key_2"}).Return([]*model.PromptBasic{
					{
						ID:        123,
						SpaceID:   123,
						PromptKey: "test_key_1",
					},
					{
						ID:        456,
						SpaceID:   123,
						PromptKey: "test_key_2",
					},
				}, nil)

				mockCacheDAO.EXPECT().MSetByPromptKey(gomock.Any(), gomock.Any()).Return(nil)

				mockCacheMetrics := &metricsinfra.PromptCacheMetrics{}

				return fields{
					promptBasicDAO:      mockBasicDAO,
					promptBasicCacheDAO: mockCacheDAO,
					promptCacheMetrics:  mockCacheMetrics,
				}
			},
			args: args{
				ctx:        context.Background(),
				spaceID:    123,
				promptKeys: []string{"test_key_1", "test_key_2"},
				opts: []repo.GetPromptBasicOptionFunc{
					repo.WithPromptBasicCacheEnable(),
				},
			},
			want: []*entity.Prompt{
				{
					ID:        123,
					SpaceID:   123,
					PromptKey: "test_key_1",
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
				},
				{
					ID:        456,
					SpaceID:   123,
					PromptKey: "test_key_2",
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "cache set error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockCacheDAO := redismocks.NewMockIPromptBasicDAO(ctrl)
				mockCacheDAO.EXPECT().MGetByPromptKey(gomock.Any(), int64(123), []string{"test_key_1", "test_key_2"}).Return(map[string]*entity.Prompt{}, nil)

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().MGetByPromptKey(gomock.Any(), int64(123), []string{"test_key_1", "test_key_2"}).Return([]*model.PromptBasic{
					{
						ID:        123,
						SpaceID:   123,
						PromptKey: "test_key_1",
					},
					{
						ID:        456,
						SpaceID:   123,
						PromptKey: "test_key_2",
					},
				}, nil)

				mockCacheDAO.EXPECT().MSetByPromptKey(gomock.Any(), gomock.Any()).Return(errorx.New("cache set error"))

				mockCacheMetrics := &metricsinfra.PromptCacheMetrics{}

				return fields{
					promptBasicDAO:      mockBasicDAO,
					promptBasicCacheDAO: mockCacheDAO,
					promptCacheMetrics:  mockCacheMetrics,
				}
			},
			args: args{
				ctx:        context.Background(),
				spaceID:    123,
				promptKeys: []string{"test_key_1", "test_key_2"},
				opts: []repo.GetPromptBasicOptionFunc{
					repo.WithPromptBasicCacheEnable(),
				},
			},
			want: []*entity.Prompt{
				{
					ID:        123,
					SpaceID:   123,
					PromptKey: "test_key_1",
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
				},
				{
					ID:        456,
					SpaceID:   123,
					PromptKey: "test_key_2",
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "db error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockCacheDAO := redismocks.NewMockIPromptBasicDAO(ctrl)
				mockCacheDAO.EXPECT().MGetByPromptKey(gomock.Any(), int64(123), []string{"test_key_1", "test_key_2"}).Return(map[string]*entity.Prompt{}, nil)

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().MGetByPromptKey(gomock.Any(), int64(123), []string{"test_key_1", "test_key_2"}).Return(nil, errorx.New("db error"))

				mockCacheMetrics := &metricsinfra.PromptCacheMetrics{}

				return fields{
					promptBasicDAO:      mockBasicDAO,
					promptBasicCacheDAO: mockCacheDAO,
					promptCacheMetrics:  mockCacheMetrics,
				}
			},
			args: args{
				ctx:        context.Background(),
				spaceID:    123,
				promptKeys: []string{"test_key_1", "test_key_2"},
				opts: []repo.GetPromptBasicOptionFunc{
					repo.WithPromptBasicCacheEnable(),
				},
			},
			want:    nil,
			wantErr: errorx.New("db error"),
		},
		{
			name: "partial cache hit",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockCacheDAO := redismocks.NewMockIPromptBasicDAO(ctrl)
				mockCacheDAO.EXPECT().MGetByPromptKey(gomock.Any(), int64(123), []string{"test_key_1", "test_key_2"}).Return(map[string]*entity.Prompt{
					"test_key_1": {
						ID:        123,
						SpaceID:   123,
						PromptKey: "test_key_1",
						PromptBasic: &entity.PromptBasic{
							PromptType: entity.PromptTypeNormal,
						},
					},
				}, nil)

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().MGetByPromptKey(gomock.Any(), int64(123), []string{"test_key_2"}).Return([]*model.PromptBasic{
					{
						ID:        456,
						SpaceID:   123,
						PromptKey: "test_key_2",
					},
				}, nil)

				mockCacheDAO.EXPECT().MSetByPromptKey(gomock.Any(), gomock.Any()).Return(nil)

				mockCacheMetrics := &metricsinfra.PromptCacheMetrics{}

				return fields{
					promptBasicDAO:      mockBasicDAO,
					promptBasicCacheDAO: mockCacheDAO,
					promptCacheMetrics:  mockCacheMetrics,
				}
			},
			args: args{
				ctx:        context.Background(),
				spaceID:    123,
				promptKeys: []string{"test_key_1", "test_key_2"},
				opts: []repo.GetPromptBasicOptionFunc{
					repo.WithPromptBasicCacheEnable(),
				},
			},
			want: []*entity.Prompt{
				{
					ID:        123,
					SpaceID:   123,
					PromptKey: "test_key_1",
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
				},
				{
					ID:        456,
					SpaceID:   123,
					PromptKey: "test_key_2",
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)

			d := &ManageRepoImpl{
				promptBasicDAO:      ttFields.promptBasicDAO,
				promptBasicCacheDAO: ttFields.promptBasicCacheDAO,
				promptCacheMetrics:  ttFields.promptCacheMetrics,
			}

			got, err := d.MGetPromptBasicByPromptKey(tt.args.ctx, tt.args.spaceID, tt.args.promptKeys, tt.args.opts...)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, len(tt.want), len(got))
				for i := range tt.want {
					assert.Equal(t, tt.want[i], got[i])
				}
			}
		})
	}
}

func TestManageRepoImpl_GetPrompt(t *testing.T) {
	type fields struct {
		db                db.Provider
		promptBasicDAO    mysql.IPromptBasicDAO
		promptCommitDAO   mysql.IPromptCommitDAO
		promptDraftDAO    mysql.IPromptUserDraftDAO
		promptRelationDAO mysql.IPromptRelationDAO
	}
	type args struct {
		ctx   context.Context
		param repo.GetPromptParam
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *entity.Prompt
		wantErr      error
	}{
		{
			name: "invalid prompt id",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				param: repo.GetPromptParam{
					PromptID: 0,
				},
			},
			want:    nil,
			wantErr: errorx.New("param.PromptID is invalid, param = {\"PromptID\":0,\"WithCommit\":false,\"CommitVersion\":\"\",\"WithDraft\":false,\"UserID\":\"\"}"),
		},
		{
			name: "with commit but no version",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				param: repo.GetPromptParam{
					PromptID:      1,
					WithCommit:    true,
					CommitVersion: "",
				},
			},
			want:    nil,
			wantErr: errorx.New("Get with commit, but param.CommitVersion is empty, param = {\"PromptID\":1,\"WithCommit\":true,\"CommitVersion\":\"\",\"WithDraft\":false,\"UserID\":\"\"}"),
		},
		{
			name: "with draft but no user id",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				param: repo.GetPromptParam{
					PromptID:  1,
					WithDraft: true,
					UserID:    "",
				},
			},
			want:    nil,
			wantErr: errorx.New("Get with draft, but param.UserID is empty, param = {\"PromptID\":1,\"WithCommit\":false,\"CommitVersion\":\"\",\"WithDraft\":true,\"UserID\":\"\"}"),
		},
		{
			name: "basic prompt not found",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(nil, nil)

				return fields{
					db:             mockDB,
					promptBasicDAO: mockBasicDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.GetPromptParam{
					PromptID: 1,
				},
			},
			want:    nil,
			wantErr: errorx.NewByCode(errno.ResourceNotFoundCode),
		},
		{
			name: "with commit but commit not found",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:        1,
					SpaceID:   100,
					PromptKey: "test_key",
				}, nil)

				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Get(gomock.Any(), int64(1), "1.0.0", gomock.Any()).Return(nil, nil)

				return fields{
					db:              mockDB,
					promptBasicDAO:  mockBasicDAO,
					promptCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.GetPromptParam{
					PromptID:      1,
					WithCommit:    true,
					CommitVersion: "1.0.0",
				},
			},
			want:    nil,
			wantErr: errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("Get with commit, but it's not found, prompt id = 1, commit version = 1.0.0")),
		},
		{
			name: "basic prompt only",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:            1,
					SpaceID:       100,
					PromptKey:     "test_key",
					Name:          "test_name",
					Description:   "test_description",
					CreatedBy:     "test_creator",
					UpdatedBy:     "test_updater",
					LatestVersion: "1.0.0",
					CreatedAt:     time.Unix(1000, 0),
					UpdatedAt:     time.Unix(2000, 0),
				}, nil)

				return fields{
					db:             mockDB,
					promptBasicDAO: mockBasicDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.GetPromptParam{
					PromptID: 1,
				},
			},
			want: &entity.Prompt{
				ID:        1,
				SpaceID:   100,
				PromptKey: "test_key",
				PromptBasic: &entity.PromptBasic{
					PromptType:    entity.PromptTypeNormal,
					DisplayName:   "test_name",
					Description:   "test_description",
					CreatedBy:     "test_creator",
					UpdatedBy:     "test_updater",
					LatestVersion: "1.0.0",
					CreatedAt:     time.Unix(1000, 0),
					UpdatedAt:     time.Unix(2000, 0),
				},
			},
			wantErr: nil,
		},
		{
			name: "complete prompt with commit and draft",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:            1,
					SpaceID:       100,
					PromptKey:     "test_key",
					Name:          "test_name",
					Description:   "test_description",
					CreatedBy:     "test_creator",
					UpdatedBy:     "test_updater",
					LatestVersion: "1.0.0",
					CreatedAt:     time.Unix(1000, 0),
					UpdatedAt:     time.Unix(2000, 0),
				}, nil)

				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Get(gomock.Any(), int64(1), "1.0.0", gomock.Any()).Return(&model.PromptCommit{
					Version:     "1.0.0",
					BaseVersion: "0.9.0",
					Description: ptr.Of("test commit"),
					CommittedBy: "test_user",
					CreatedAt:   time.Unix(1000, 0),
				}, nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(&model.PromptUserDraft{
					UserID:      "test_user",
					BaseVersion: "1.0.0",
				}, nil)

				return fields{
					db:                mockDB,
					promptBasicDAO:    mockBasicDAO,
					promptCommitDAO:   mockCommitDAO,
					promptDraftDAO:    mockDraftDAO,
					promptRelationDAO: nil,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.GetPromptParam{
					PromptID:      1,
					WithCommit:    true,
					CommitVersion: "1.0.0",
					WithDraft:     true,
					UserID:        "test_user",
				},
			},
			want: &entity.Prompt{
				ID:        1,
				SpaceID:   100,
				PromptKey: "test_key",
				PromptBasic: &entity.PromptBasic{
					PromptType:    entity.PromptTypeNormal,
					DisplayName:   "test_name",
					Description:   "test_description",
					CreatedBy:     "test_creator",
					UpdatedBy:     "test_updater",
					LatestVersion: "1.0.0",
					CreatedAt:     time.Unix(1000, 0),
					UpdatedAt:     time.Unix(2000, 0),
				},
				PromptCommit: &entity.PromptCommit{
					PromptDetail: &entity.PromptDetail{
						PromptTemplate: &entity.PromptTemplate{},
					},
					CommitInfo: &entity.CommitInfo{
						Version:     "1.0.0",
						BaseVersion: "0.9.0",
						Description: "test commit",
						CommittedBy: "test_user",
						CommittedAt: time.Unix(1000, 0),
					},
				},
				PromptDraft: &entity.PromptDraft{
					PromptDetail: &entity.PromptDetail{
						PromptTemplate: &entity.PromptTemplate{},
					},
					DraftInfo: &entity.DraftInfo{
						UserID:      "test_user",
						BaseVersion: "1.0.0",
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "db error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).Return(errorx.New("db error"))

				return fields{
					db: mockDB,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.GetPromptParam{
					PromptID: 1,
				},
			},
			want:    nil,
			wantErr: errorx.New("db error"),
		},
		{
			name: "basic dao error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(nil, errorx.New("basic dao error"))

				return fields{
					db:             mockDB,
					promptBasicDAO: mockBasicDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.GetPromptParam{
					PromptID: 1,
				},
			},
			want:    nil,
			wantErr: errorx.New("basic dao error"),
		},
		{
			name: "commit dao error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:        1,
					SpaceID:   100,
					PromptKey: "test_key",
				}, nil)

				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Get(gomock.Any(), int64(1), "1.0.0", gomock.Any()).Return(nil, errorx.New("commit dao error"))

				return fields{
					db:              mockDB,
					promptBasicDAO:  mockBasicDAO,
					promptCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.GetPromptParam{
					PromptID:      1,
					WithCommit:    true,
					CommitVersion: "1.0.0",
				},
			},
			want:    nil,
			wantErr: errorx.New("commit dao error"),
		},
		{
			name: "draft dao error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:        1,
					SpaceID:   100,
					PromptKey: "test_key",
				}, nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(nil, errorx.New("draft dao error"))

				return fields{
					db:             mockDB,
					promptBasicDAO: mockBasicDAO,
					promptDraftDAO: mockDraftDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.GetPromptParam{
					PromptID:  1,
					WithDraft: true,
					UserID:    "test_user",
				},
			},
			want:    nil,
			wantErr: errorx.New("draft dao error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)

			d := &ManageRepoImpl{
				db:                ttFields.db,
				promptBasicDAO:    ttFields.promptBasicDAO,
				promptCommitDAO:   ttFields.promptCommitDAO,
				promptDraftDAO:    ttFields.promptDraftDAO,
				promptRelationDAO: ttFields.promptRelationDAO,
			}

			got, err := d.GetPrompt(tt.args.ctx, tt.args.param)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestManageRepoImpl_ListCommitInfo(t *testing.T) {
	type fields struct {
		promptCommitDAO mysql.IPromptCommitDAO
	}
	type args struct {
		ctx   context.Context
		param repo.ListCommitInfoParam
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *repo.ListCommitResult
		wantErr      error
	}{
		{
			name: "invalid prompt id",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListCommitInfoParam{
					PromptID: 0,
					PageSize: 10,
				},
			},
			want:    nil,
			wantErr: errorx.New("Param(PromptID or PageSize) is invalid, param = {\"PromptID\":0,\"PageSize\":10,\"PageToken\":null,\"Asc\":false}"),
		},
		{
			name: "invalid page size",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListCommitInfoParam{
					PromptID: 1,
					PageSize: 0,
				},
			},
			want:    nil,
			wantErr: errorx.New("Param(PromptID or PageSize) is invalid, param = {\"PromptID\":1,\"PageSize\":0,\"PageToken\":null,\"Asc\":false}"),
		},
		{
			name: "empty result",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().List(gomock.Any(), mysql.ListCommitParam{
					PromptID: 1,
					Limit:    11,
					Asc:      false,
				}).Return([]*model.PromptCommit{}, nil)

				return fields{
					promptCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListCommitInfoParam{
					PromptID: 1,
					PageSize: 10,
				},
			},
			want:    nil,
			wantErr: nil,
		},
		{
			name: "list error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().List(gomock.Any(), mysql.ListCommitParam{
					PromptID: 1,
					Limit:    11,
					Asc:      false,
				}).Return(nil, errorx.New("list error"))

				return fields{
					promptCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListCommitInfoParam{
					PromptID: 1,
					PageSize: 10,
				},
			},
			want:    nil,
			wantErr: errorx.New("list error"),
		},
		{
			name: "single page result",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().List(gomock.Any(), mysql.ListCommitParam{
					PromptID: 1,
					Limit:    11,
					Asc:      false,
				}).Return([]*model.PromptCommit{
					{
						ID:          1,
						Version:     "1.0.0",
						BaseVersion: "0.9.0",
						Description: ptr.Of("test commit 1"),
						CommittedBy: "test_user",
						CreatedAt:   time.Unix(1000, 0),
					},
					{
						ID:          2,
						Version:     "1.1.0",
						BaseVersion: "1.0.0",
						Description: ptr.Of("test commit 2"),
						CommittedBy: "test_user",
						CreatedAt:   time.Unix(2000, 0),
					},
				}, nil)

				return fields{
					promptCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListCommitInfoParam{
					PromptID: 1,
					PageSize: 10,
				},
			},
			want: &repo.ListCommitResult{
				CommitInfoDOs: []*entity.CommitInfo{
					{
						Version:     "1.0.0",
						BaseVersion: "0.9.0",
						Description: "test commit 1",
						CommittedBy: "test_user",
						CommittedAt: time.Unix(1000, 0),
					},
					{
						Version:     "1.1.0",
						BaseVersion: "1.0.0",
						Description: "test commit 2",
						CommittedBy: "test_user",
						CommittedAt: time.Unix(2000, 0),
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "multiple pages result",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().List(gomock.Any(), mysql.ListCommitParam{
					PromptID: 1,
					Limit:    3,
					Asc:      false,
				}).Return([]*model.PromptCommit{
					{
						ID:          1,
						Version:     "1.0.0",
						BaseVersion: "0.9.0",
						Description: ptr.Of("test commit 1"),
						CommittedBy: "test_user",
						CreatedAt:   time.Unix(1000, 0),
					},
					{
						ID:          2,
						Version:     "1.1.0",
						BaseVersion: "1.0.0",
						Description: ptr.Of("test commit 2"),
						CommittedBy: "test_user",
						CreatedAt:   time.Unix(2000, 0),
					},
					{
						ID:          3,
						Version:     "1.2.0",
						BaseVersion: "1.1.0",
						Description: ptr.Of("test commit 3"),
						CommittedBy: "test_user",
						CreatedAt:   time.Unix(3000, 0),
					},
				}, nil)

				return fields{
					promptCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListCommitInfoParam{
					PromptID: 1,
					PageSize: 2,
				},
			},
			want: &repo.ListCommitResult{
				CommitInfoDOs: []*entity.CommitInfo{
					{
						Version:     "1.0.0",
						BaseVersion: "0.9.0",
						Description: "test commit 1",
						CommittedBy: "test_user",
						CommittedAt: time.Unix(1000, 0),
					},
					{
						Version:     "1.1.0",
						BaseVersion: "1.0.0",
						Description: "test commit 2",
						CommittedBy: "test_user",
						CommittedAt: time.Unix(2000, 0),
					},
				},
				NextPageToken: 3,
			},
			wantErr: nil,
		},
		{
			name: "with page token and asc",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().List(gomock.Any(), mysql.ListCommitParam{
					PromptID: 1,
					Cursor:   ptr.Of(int64(2)),
					Limit:    11,
					Asc:      true,
				}).Return([]*model.PromptCommit{
					{
						ID:          3,
						Version:     "1.2.0",
						BaseVersion: "1.1.0",
						Description: ptr.Of("test commit 3"),
						CommittedBy: "test_user",
						CreatedAt:   time.Unix(3000, 0),
					},
					{
						ID:          4,
						Version:     "1.3.0",
						BaseVersion: "1.2.0",
						Description: ptr.Of("test commit 4"),
						CommittedBy: "test_user",
						CreatedAt:   time.Unix(4000, 0),
					},
				}, nil)

				return fields{
					promptCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListCommitInfoParam{
					PromptID:  1,
					PageSize:  10,
					PageToken: ptr.Of(int64(2)),
					Asc:       true,
				},
			},
			want: &repo.ListCommitResult{
				CommitInfoDOs: []*entity.CommitInfo{
					{
						Version:     "1.2.0",
						BaseVersion: "1.1.0",
						Description: "test commit 3",
						CommittedBy: "test_user",
						CommittedAt: time.Unix(3000, 0),
					},
					{
						Version:     "1.3.0",
						BaseVersion: "1.2.0",
						Description: "test commit 4",
						CommittedBy: "test_user",
						CommittedAt: time.Unix(4000, 0),
					},
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)

			d := &ManageRepoImpl{
				promptCommitDAO: ttFields.promptCommitDAO,
			}

			got, err := d.ListCommitInfo(tt.args.ctx, tt.args.param)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestManageRepoImpl_SaveDraft(t *testing.T) {
	type fields struct {
		db                db.Provider
		promptBasicDAO    mysql.IPromptBasicDAO
		promptCommitDAO   mysql.IPromptCommitDAO
		promptDraftDAO    mysql.IPromptUserDraftDAO
		promptRelationDAO mysql.IPromptRelationDAO
		idgen             idgen.IIDGenerator
	}
	type args struct {
		ctx      context.Context
		promptDO *entity.Prompt
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      error
	}{
		{
			name: "nil prompt",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx:      context.Background(),
				promptDO: nil,
			},
			wantErr: errorx.New("promptDO or promptDO.PromptDraft is empty"),
		},
		{
			name: "nil prompt draft",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					ID: 1,
				},
			},
			wantErr: errorx.New("promptDO or promptDO.PromptDraft is empty"),
		},
		{
			name: "basic prompt not found",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(nil, nil)

				return fields{
					db:             mockDB,
					promptBasicDAO: mockBasicDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					ID: 1,
					PromptDraft: &entity.PromptDraft{
						DraftInfo: &entity.DraftInfo{
							UserID:      "test_user",
							BaseVersion: "1.0.0",
						},
					},
				},
			},
			wantErr: errorx.New("Prompt is not found, prompt id = 1"),
		},
		{
			name: "base commit not found",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:      1,
					SpaceID: 100,
				}, nil)

				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Get(gomock.Any(), int64(1), "1.0.0", gomock.Any()).Return(nil, nil)

				return fields{
					db:              mockDB,
					promptBasicDAO:  mockBasicDAO,
					promptCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					ID: 1,
					PromptDraft: &entity.PromptDraft{
						DraftInfo: &entity.DraftInfo{
							UserID:      "test_user",
							BaseVersion: "1.0.0",
						},
					},
				},
			},
			wantErr: errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("Draft's base prompt commit is not found, prompt id = 1, base commit version = 1.0.0")),
		},
		{
			name: "create new draft",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:      1,
					SpaceID: 100,
				}, nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(nil, nil)
				mockDraftDAO.EXPECT().GetByID(gomock.Any(), int64(1001), gomock.Any()).Return(&model.PromptUserDraft{
					ID:          1001,
					UserID:      "test_user",
					BaseVersion: "",
				}, nil)

				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)

				mockDraftDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, draft *model.PromptUserDraft, opts ...db.Option) error {
					assert.Equal(t, int64(1001), draft.ID)
					assert.Equal(t, int64(100), draft.SpaceID)
					assert.Equal(t, int32(1), draft.IsDraftEdited)
					return nil
				})

				return fields{
					db:             mockDB,
					promptBasicDAO: mockBasicDAO,
					promptDraftDAO: mockDraftDAO,
					idgen:          mockIDGen,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					ID: 1,
					PromptDraft: &entity.PromptDraft{
						DraftInfo: &entity.DraftInfo{
							UserID:      "test_user",
							BaseVersion: "",
						},
					},
				},
			},
			wantErr: nil,
		},
		//{
		//	name: "update draft with invalid base version",
		//	fieldsGetter: func(ctrl *gomock.Controller) fields {
		//		mockDB := dbmocks.NewMockProvider(ctrl)
		//		mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
		//			return fc(nil)
		//		})
		//
		//		mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
		//		mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
		//			ID:      1,
		//			SpaceID: 100,
		//		}, nil)
		//
		//		mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
		//		mockCommitDAO.EXPECT().Get(gomock.Any(), int64(1), "1.0.0", gomock.Any()).Return(&model.PromptCommit{
		//			PromptID: 1,
		//			Version:  "1.0.0",
		//		}, nil)
		//
		//		mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
		//		mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(&model.PromptUserDraft{
		//			ID:          1001,
		//			BaseVersion: "0.9.0",
		//		}, nil)
		//		mockDraftDAO.EXPECT().GetByID(gomock.Any(), int64(1001), gomock.Any()).Return(&model.PromptUserDraft{
		//			ID:          1001,
		//			UserID:      "test_user",
		//			BaseVersion: "0.9.0",
		//		}, nil)
		//
		//		return fields{
		//			db:              mockDB,
		//			promptBasicDAO:  mockBasicDAO,
		//			promptCommitDAO: mockCommitDAO,
		//			promptDraftDAO:  mockDraftDAO,
		//		}
		//	},
		//	args: args{
		//		ctx: context.Background(),
		//		promptDO: &entity.Prompt{
		//			ID: 1,
		//			PromptDraft: &entity.PromptDraft{
		//				DraftInfo: &entity.DraftInfo{
		//					UserID:      "test_user",
		//					BaseVersion: "1.0.0",
		//				},
		//				PromptDetail: &entity.PromptDetail{
		//					PromptTemplate: &entity.PromptTemplate{},
		//				},
		//			},
		//		},
		//	},
		//	wantErr: errorx.New("Draft's base version is invalid, saving draft's base version = 1.0.0, original draft's base version = 0.9.0 "),
		//},
		{
			name: "create draft with snippets",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:      1,
					SpaceID: 100,
				}, nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(nil, nil)

				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)

				mockDraftDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, draft *model.PromptUserDraft, opts ...db.Option) error {
					assert.Equal(t, int64(100), draft.SpaceID)
					assert.Equal(t, "test_user", draft.UserID)
					assert.True(t, draft.HasSnippets)
					return nil
				})

				mockDraftDAO.EXPECT().GetByID(gomock.Any(), int64(1001), gomock.Any()).Return(&model.PromptUserDraft{
					ID:     1001,
					UserID: "test_user",
				}, nil)

				mockRelationDAO := daomocks.NewMockIPromptRelationDAO(ctrl)
				mockRelationDAO.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*model.PromptRelation{}, nil)

				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{4001, 4002}, nil)
				mockRelationDAO.EXPECT().BatchCreate(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, relations []*model.PromptRelation, opts ...db.Option) error {
					assert.Len(t, relations, 2)
					for _, relation := range relations {
						assert.Equal(t, int64(1), relation.MainPromptID)
						assert.Equal(t, "", relation.MainPromptVersion)
						assert.Equal(t, "test_user", relation.MainDraftUserID)
						assert.Equal(t, int64(100), relation.SpaceID)
					}
					assert.Equal(t, int64(200), relations[0].SubPromptID)
					assert.Equal(t, "v1", relations[0].SubPromptVersion)
					assert.Equal(t, int64(201), relations[1].SubPromptID)
					assert.Equal(t, "", relations[1].SubPromptVersion)
					return nil
				})

				return fields{
					db:                mockDB,
					promptBasicDAO:    mockBasicDAO,
					promptDraftDAO:    mockDraftDAO,
					promptRelationDAO: mockRelationDAO,
					idgen:             mockIDGen,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					ID:      1,
					SpaceID: 100,
					PromptDraft: &entity.PromptDraft{
						DraftInfo: &entity.DraftInfo{
							UserID: "test_user",
						},
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								HasSnippets: true,
								Snippets: []*entity.Prompt{
									{
										ID: 200,
										PromptCommit: &entity.PromptCommit{
											CommitInfo: &entity.CommitInfo{
												Version: "v1",
											},
										},
									},
									{
										ID: 201,
									},
								},
							},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "create draft with snippets relation error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:      1,
					SpaceID: 100,
				}, nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(nil, nil)

				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)

				mockDraftDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockDraftDAO.EXPECT().GetByID(gomock.Any(), int64(1001), gomock.Any()).Return(&model.PromptUserDraft{
					ID:     1001,
					UserID: "test_user",
				}, nil)

				mockRelationDAO := daomocks.NewMockIPromptRelationDAO(ctrl)
				mockRelationDAO.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errorx.New("relation list error"))

				return fields{
					db:                mockDB,
					promptBasicDAO:    mockBasicDAO,
					promptDraftDAO:    mockDraftDAO,
					promptRelationDAO: mockRelationDAO,
					idgen:             mockIDGen,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					ID:      1,
					SpaceID: 100,
					PromptDraft: &entity.PromptDraft{
						DraftInfo: &entity.DraftInfo{
							UserID: "test_user",
						},
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								HasSnippets: true,
								Snippets: []*entity.Prompt{
									{ID: 200},
								},
							},
						},
					},
				},
			},
			wantErr: errorx.New("relation list error"),
		},
		{
			name: "update draft with no changes",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:      1,
					SpaceID: 100,
				}, nil)

				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Get(gomock.Any(), int64(1), "1.0.0", gomock.Any()).Return(&model.PromptCommit{
					PromptID: 1,
					Version:  "1.0.0",
				}, nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(&model.PromptUserDraft{
					ID:          1001,
					BaseVersion: "1.0.0",
				}, nil)

				return fields{
					db:                mockDB,
					promptBasicDAO:    mockBasicDAO,
					promptCommitDAO:   mockCommitDAO,
					promptDraftDAO:    mockDraftDAO,
					promptRelationDAO: nil,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					ID: 1,
					PromptDraft: &entity.PromptDraft{
						DraftInfo: &entity.DraftInfo{
							UserID:      "test_user",
							BaseVersion: "1.0.0",
						},
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "update draft with changes",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:      1,
					SpaceID: 100,
				}, nil)

				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Get(gomock.Any(), int64(1), "1.0.0", gomock.Any()).Return(&model.PromptCommit{
					Version: "1.0.0",
				}, nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(&model.PromptUserDraft{
					ID:          1001,
					BaseVersion: "1.0.0",
				}, nil)
				mockDraftDAO.EXPECT().GetByID(gomock.Any(), int64(1001), gomock.Any()).Return(&model.PromptUserDraft{
					ID:          1001,
					UserID:      "test_user",
					BaseVersion: "1.0.0",
				}, nil)

				mockDraftDAO.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, draft *model.PromptUserDraft, opts ...db.Option) error {
					assert.Equal(t, int64(1001), draft.ID)
					assert.Equal(t, int32(1), draft.IsDraftEdited)
					return nil
				})

				mockRelationDAO := daomocks.NewMockIPromptRelationDAO(ctrl)
				mockRelationDAO.EXPECT().DeleteByMainPrompt(gomock.Any(), int64(1), "", "test_user", gomock.Any()).Return(nil)

				return fields{
					db:                mockDB,
					promptBasicDAO:    mockBasicDAO,
					promptCommitDAO:   mockCommitDAO,
					promptDraftDAO:    mockDraftDAO,
					promptRelationDAO: mockRelationDAO,
					idgen:             nil,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					ID: 1,
					PromptDraft: &entity.PromptDraft{
						DraftInfo: &entity.DraftInfo{
							UserID:      "test_user",
							BaseVersion: "1.0.0",
						},
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								TemplateType: entity.TemplateTypeNormal,
								Messages: []*entity.Message{
									{
										Role:    entity.RoleUser,
										Content: ptr.Of("new content"),
									},
								},
							},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "update draft with snippets",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:      1,
					SpaceID: 100,
				}, nil)

				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Get(gomock.Any(), int64(1), "v1", gomock.Any()).Return(&model.PromptCommit{
					Version: "v1",
				}, nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(&model.PromptUserDraft{
					ID:          1001,
					BaseVersion: "v1",
				}, nil)
				mockDraftDAO.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, draft *model.PromptUserDraft, opts ...db.Option) error {
					assert.Equal(t, int64(1001), draft.ID)
					assert.True(t, draft.HasSnippets)
					return nil
				})
				mockDraftDAO.EXPECT().GetByID(gomock.Any(), int64(1001), gomock.Any()).Return(&model.PromptUserDraft{
					ID:     1001,
					UserID: "test_user",
				}, nil)

				mockRelationDAO := daomocks.NewMockIPromptRelationDAO(ctrl)
				mockRelationDAO.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*model.PromptRelation{
					{
						ID:                2001,
						SpaceID:           100,
						MainPromptID:      1,
						MainPromptVersion: "",
						MainDraftUserID:   "test_user",
						SubPromptID:       200,
						SubPromptVersion:  "v1",
					},
					{
						ID:                2002,
						SpaceID:           100,
						MainPromptID:      1,
						MainPromptVersion: "",
						MainDraftUserID:   "test_user",
						SubPromptID:       201,
						SubPromptVersion:  "old",
					},
				}, nil)
				mockRelationDAO.EXPECT().BatchDeleteByIDs(gomock.Any(), []int64{2002}, gomock.Any()).Return(nil)

				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{4001}, nil)
				mockRelationDAO.EXPECT().BatchCreate(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, relations []*model.PromptRelation, opts ...db.Option) error {
					assert.Len(t, relations, 1)
					assert.Equal(t, int64(1), relations[0].MainPromptID)
					assert.Equal(t, "test_user", relations[0].MainDraftUserID)
					assert.Equal(t, int64(202), relations[0].SubPromptID)
					assert.Equal(t, "v2", relations[0].SubPromptVersion)
					return nil
				})

				return fields{
					db:                mockDB,
					promptBasicDAO:    mockBasicDAO,
					promptCommitDAO:   mockCommitDAO,
					promptDraftDAO:    mockDraftDAO,
					promptRelationDAO: mockRelationDAO,
					idgen:             mockIDGen,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					ID:      1,
					SpaceID: 100,
					PromptDraft: &entity.PromptDraft{
						DraftInfo: &entity.DraftInfo{
							UserID:      "test_user",
							BaseVersion: "v1",
						},
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								HasSnippets: true,
								Snippets: []*entity.Prompt{
									{
										ID: 200,
										PromptCommit: &entity.PromptCommit{
											CommitInfo: &entity.CommitInfo{
												Version: "v1",
											},
										},
									},
									{
										ID: 202,
										PromptCommit: &entity.PromptCommit{
											CommitInfo: &entity.CommitInfo{
												Version: "v2",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "db error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).Return(errorx.New("db error"))

				return fields{
					db: mockDB,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					ID: 1,
					PromptDraft: &entity.PromptDraft{
						DraftInfo: &entity.DraftInfo{
							UserID:      "test_user",
							BaseVersion: "1.0.0",
						},
					},
				},
			},
			wantErr: errorx.New("db error"),
		},
		{
			name: "basic dao error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(nil, errorx.New("basic dao error"))

				return fields{
					db:             mockDB,
					promptBasicDAO: mockBasicDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					ID: 1,
					PromptDraft: &entity.PromptDraft{
						DraftInfo: &entity.DraftInfo{
							UserID:      "test_user",
							BaseVersion: "1.0.0",
						},
					},
				},
			},
			wantErr: errorx.New("basic dao error"),
		},
		{
			name: "commit dao error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:      1,
					SpaceID: 100,
				}, nil)

				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Get(gomock.Any(), int64(1), "1.0.0", gomock.Any()).Return(nil, errorx.New("commit dao error"))

				return fields{
					db:              mockDB,
					promptBasicDAO:  mockBasicDAO,
					promptCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					ID: 1,
					PromptDraft: &entity.PromptDraft{
						DraftInfo: &entity.DraftInfo{
							UserID:      "test_user",
							BaseVersion: "1.0.0",
						},
					},
				},
			},
			wantErr: errorx.New("commit dao error"),
		},
		{
			name: "draft dao error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:        1,
					SpaceID:   100,
					PromptKey: "test_key",
				}, nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(nil, errorx.New("draft dao error"))

				return fields{
					db:             mockDB,
					promptBasicDAO: mockBasicDAO,
					promptDraftDAO: mockDraftDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					ID: 1,
					PromptDraft: &entity.PromptDraft{
						DraftInfo: &entity.DraftInfo{
							UserID:      "test_user",
							BaseVersion: "",
						},
					},
				},
			},
			wantErr: errorx.New("draft dao error"),
		},
		{
			name: "idgen error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:      1,
					SpaceID: 100,
				}, nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(nil, nil)

				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(0), errorx.New("idgen error"))

				return fields{
					db:             mockDB,
					promptBasicDAO: mockBasicDAO,
					promptDraftDAO: mockDraftDAO,
					idgen:          mockIDGen,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					ID: 1,
					PromptDraft: &entity.PromptDraft{
						DraftInfo: &entity.DraftInfo{
							UserID:      "test_user",
							BaseVersion: "",
						},
					},
				},
			},
			wantErr: errorx.New("idgen error"),
		},
		{
			name: "create draft error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:      1,
					SpaceID: 100,
				}, nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(nil, nil)

				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)

				mockDraftDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).Return(errorx.New("create error"))

				return fields{
					db:             mockDB,
					promptBasicDAO: mockBasicDAO,
					promptDraftDAO: mockDraftDAO,
					idgen:          mockIDGen,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					ID: 1,
					PromptDraft: &entity.PromptDraft{
						DraftInfo: &entity.DraftInfo{
							UserID:      "test_user",
							BaseVersion: "",
						},
					},
				},
			},
			wantErr: errorx.New("create error"),
		},
		{
			name: "update draft error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:      1,
					SpaceID: 100,
				}, nil)

				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Get(gomock.Any(), int64(1), "1.0.0", gomock.Any()).Return(&model.PromptCommit{
					Version: "1.0.0",
				}, nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(&model.PromptUserDraft{
					ID:          1001,
					BaseVersion: "1.0.0",
				}, nil)

				mockDraftDAO.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(errorx.New("update error"))

				return fields{
					db:                mockDB,
					promptBasicDAO:    mockBasicDAO,
					promptCommitDAO:   mockCommitDAO,
					promptDraftDAO:    mockDraftDAO,
					promptRelationDAO: nil,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					ID: 1,
					PromptDraft: &entity.PromptDraft{
						DraftInfo: &entity.DraftInfo{
							UserID:      "test_user",
							BaseVersion: "1.0.0",
						},
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								TemplateType: entity.TemplateTypeNormal,
								Messages: []*entity.Message{
									{
										Role:    entity.RoleUser,
										Content: ptr.Of("new content"),
									},
								},
							},
						},
					},
				},
			},
			wantErr: errorx.New("update error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)

			d := &ManageRepoImpl{
				db:                ttFields.db,
				promptBasicDAO:    ttFields.promptBasicDAO,
				promptCommitDAO:   ttFields.promptCommitDAO,
				promptDraftDAO:    ttFields.promptDraftDAO,
				promptRelationDAO: ttFields.promptRelationDAO,
				idgen:             ttFields.idgen,
			}

			_, err := d.SaveDraft(tt.args.ctx, tt.args.promptDO)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
		})
	}
}

func TestManageRepoImpl_CreatePrompt(t *testing.T) {
	type fields struct {
		db                db.Provider
		promptBasicDAO    mysql.IPromptBasicDAO
		promptDraftDAO    mysql.IPromptUserDraftDAO
		promptRelationDAO mysql.IPromptRelationDAO
		idgen             idgen.IIDGenerator
	}
	type args struct {
		ctx      context.Context
		promptDO *entity.Prompt
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         int64
		wantErr      error
	}{
		{
			name: "nil prompt",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx:      context.Background(),
				promptDO: nil,
			},
			want:    0,
			wantErr: errorx.New("promptDO or promptDO.PromptBasic is empty"),
		},
		{
			name: "nil prompt basic",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					ID: 1,
				},
			},
			want:    0,
			wantErr: errorx.New("promptDO or promptDO.PromptBasic is empty"),
		},
		{
			name: "idgen error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(0), errorx.New("idgen error"))

				return fields{
					idgen: mockIDGen,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
				},
			},
			want:    0,
			wantErr: errorx.New("idgen error"),
		},
		{
			name: "draft idgen error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(0), errorx.New("draft idgen error"))

				return fields{
					idgen: mockIDGen,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
					PromptDraft: &entity.PromptDraft{
						DraftInfo: &entity.DraftInfo{
							UserID: "test_user",
						},
					},
				},
			},
			want:    0,
			wantErr: errorx.New("draft idgen error"),
		},
		{
			name: "db error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).Return(errorx.New("db error"))

				return fields{
					db:    mockDB,
					idgen: mockIDGen,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
				},
			},
			want:    0,
			wantErr: errorx.New("db error"),
		},
		{
			name: "basic dao error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, basic *model.PromptBasic, opts ...db.Option) error {
					assert.Equal(t, int64(1001), basic.ID)
					return errorx.New("basic dao error")
				})

				return fields{
					db:             mockDB,
					promptBasicDAO: mockBasicDAO,
					idgen:          mockIDGen,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
				},
			},
			want:    0,
			wantErr: errorx.New("basic dao error"),
		},
		{
			name: "draft dao error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(2001), nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, basic *model.PromptBasic, opts ...db.Option) error {
					assert.Equal(t, int64(1001), basic.ID)
					return nil
				})

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, draft *model.PromptUserDraft, opts ...db.Option) error {
					assert.Equal(t, int64(2001), draft.ID)
					assert.Equal(t, int64(1001), draft.PromptID)
					return errorx.New("draft dao error")
				})

				return fields{
					db:             mockDB,
					promptBasicDAO: mockBasicDAO,
					promptDraftDAO: mockDraftDAO,
					idgen:          mockIDGen,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
					PromptDraft: &entity.PromptDraft{
						DraftInfo: &entity.DraftInfo{
							UserID: "test_user",
						},
					},
				},
			},
			want:    0,
			wantErr: errorx.New("draft dao error"),
		},
		{
			name: "create basic prompt success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, basic *model.PromptBasic, opts ...db.Option) error {
					assert.Equal(t, int64(1001), basic.ID)
					return nil
				})

				return fields{
					db:             mockDB,
					promptBasicDAO: mockBasicDAO,
					idgen:          mockIDGen,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
				},
			},
			want:    1001,
			wantErr: nil,
		},
		{
			name: "create prompt with draft success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(2001), nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, basic *model.PromptBasic, opts ...db.Option) error {
					assert.Equal(t, int64(1001), basic.ID)
					return nil
				})

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, draft *model.PromptUserDraft, opts ...db.Option) error {
					assert.Equal(t, int64(2001), draft.ID)
					assert.Equal(t, int64(1001), draft.PromptID)
					return nil
				})

				return fields{
					db:             mockDB,
					promptBasicDAO: mockBasicDAO,
					promptDraftDAO: mockDraftDAO,
					idgen:          mockIDGen,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
					PromptDraft: &entity.PromptDraft{
						DraftInfo: &entity.DraftInfo{
							UserID: "test_user",
						},
					},
				},
			},
			want:    1001,
			wantErr: nil,
		},
		{
			name: "create prompt with snippets success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(2001), nil)
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{3001, 3002}, nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, basic *model.PromptBasic, opts ...db.Option) error {
					assert.Equal(t, int64(1001), basic.ID)
					return nil
				})

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, draft *model.PromptUserDraft, opts ...db.Option) error {
					assert.Equal(t, int64(2001), draft.ID)
					assert.Equal(t, int64(1001), draft.PromptID)
					return nil
				})

				mockRelationDAO := daomocks.NewMockIPromptRelationDAO(ctrl)
				mockRelationDAO.EXPECT().BatchCreate(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, relations []*model.PromptRelation, opts ...db.Option) error {
					assert.Len(t, relations, 2)
					assert.Equal(t, int64(1001), relations[0].MainPromptID)
					assert.Equal(t, int64(200), relations[0].SubPromptID)
					assert.Equal(t, int64(1001), relations[1].MainPromptID)
					assert.Equal(t, int64(201), relations[1].SubPromptID)
					assert.Equal(t, "test_user", relations[0].MainDraftUserID)
					assert.Equal(t, "test_user", relations[1].MainDraftUserID)
					return nil
				})

				return fields{
					db:                mockDB,
					promptBasicDAO:    mockBasicDAO,
					promptDraftDAO:    mockDraftDAO,
					promptRelationDAO: mockRelationDAO,
					idgen:             mockIDGen,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					SpaceID: 123,
					PromptBasic: &entity.PromptBasic{
						DisplayName: "Test Prompt",
						Description: "Test Description",
					},
					PromptDraft: &entity.PromptDraft{
						DraftInfo: &entity.DraftInfo{
							UserID: "test_user",
						},
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								HasSnippets: true,
								Snippets: []*entity.Prompt{
									{ID: 200},
									{ID: 201},
								},
							},
						},
					},
				},
			},
			want:    1001,
			wantErr: nil,
		},
		{
			name: "create prompt with snippets - skip nil snippets",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(2001), nil)
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 3).Return([]int64{3001, 3002, 3003}, nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				// No relation creation expected since valid snippets are 0
				mockRelationDAO := daomocks.NewMockIPromptRelationDAO(ctrl)
				mockRelationDAO.EXPECT().BatchCreate(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, relations []*model.PromptRelation, opts ...db.Option) error {
					assert.Len(t, relations, 2) // 2 valid snippets (ID: 0 and ID: 202)
					return nil
				})

				return fields{
					db:                mockDB,
					promptBasicDAO:    mockBasicDAO,
					promptDraftDAO:    mockDraftDAO,
					promptRelationDAO: mockRelationDAO,
					idgen:             mockIDGen,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					SpaceID: 123,
					PromptBasic: &entity.PromptBasic{
						DisplayName: "Test Prompt",
						Description: "Test Description",
					},
					PromptDraft: &entity.PromptDraft{
						DraftInfo: &entity.DraftInfo{
							UserID: "test_user",
						},
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								HasSnippets: true,
								Snippets: []*entity.Prompt{
									nil,
									{ID: 0},   // Invalid ID
									{ID: 202}, // Valid
								},
							},
						},
					},
				},
			},
			want:    1001,
			wantErr: nil,
		},
		{
			name: "create prompt with snippets - relation dao error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(2001), nil)
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{3001}, nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockRelationDAO := daomocks.NewMockIPromptRelationDAO(ctrl)
				mockRelationDAO.EXPECT().BatchCreate(gomock.Any(), gomock.Any(), gomock.Any()).Return(errorx.New("relation dao error"))

				return fields{
					db:                mockDB,
					promptBasicDAO:    mockBasicDAO,
					promptDraftDAO:    mockDraftDAO,
					promptRelationDAO: mockRelationDAO,
					idgen:             mockIDGen,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					SpaceID: 123,
					PromptBasic: &entity.PromptBasic{
						DisplayName: "Test Prompt",
						Description: "Test Description",
					},
					PromptDraft: &entity.PromptDraft{
						DraftInfo: &entity.DraftInfo{
							UserID: "test_user",
						},
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								HasSnippets: true,
								Snippets: []*entity.Prompt{
									{ID: 200},
								},
							},
						},
					},
				},
			},
			want:    0,
			wantErr: errorx.New("relation dao error"),
		},
		{
			name: "create prompt without snippets - no relation creation",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(2001), nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				// No relation creation expected
				mockRelationDAO := daomocks.NewMockIPromptRelationDAO(ctrl)

				return fields{
					db:                mockDB,
					promptBasicDAO:    mockBasicDAO,
					promptDraftDAO:    mockDraftDAO,
					promptRelationDAO: mockRelationDAO,
					idgen:             mockIDGen,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					SpaceID: 123,
					PromptBasic: &entity.PromptBasic{
						DisplayName: "Test Prompt",
						Description: "Test Description",
					},
					PromptDraft: &entity.PromptDraft{
						DraftInfo: &entity.DraftInfo{
							UserID: "test_user",
						},
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								HasSnippets: false,
								Snippets:    []*entity.Prompt{},
							},
						},
					},
				},
			},
			want:    1001,
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)

			d := &ManageRepoImpl{
				db:                ttFields.db,
				promptBasicDAO:    ttFields.promptBasicDAO,
				promptDraftDAO:    ttFields.promptDraftDAO,
				promptRelationDAO: ttFields.promptRelationDAO,
				idgen:             ttFields.idgen,
			}

			got, err := d.CreatePrompt(tt.args.ctx, tt.args.promptDO)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestManageRepoImpl_CommitDraft(t *testing.T) {
	type fields struct {
		db                    db.Provider
		idgen                 idgen.IIDGenerator
		promptBasicDAO        mysql.IPromptBasicDAO
		promptCommitDAO       mysql.IPromptCommitDAO
		promptDraftDAO        mysql.IPromptUserDraftDAO
		commitLabelMappingDAO mysql.ICommitLabelMappingDAO
		promptBasicCacheDAO   redis.IPromptBasicDAO
		promptCacheDAO        redis.IPromptDAO
		promptRelationDAO     mysql.IPromptRelationDAO
	}
	type args struct {
		ctx   context.Context
		param repo.CommitDraftParam
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      error
	}{
		{
			name: "invalid prompt id",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{
					commitLabelMappingDAO: daomocks.NewMockICommitLabelMappingDAO(ctrl),
					promptRelationDAO:     daomocks.NewMockIPromptRelationDAO(ctrl),
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitDraftParam{
					PromptID: 0,
					UserID:   "test_user",
				},
			},
			wantErr: errorx.New("param(PromptID or UserID or CommitVersion) is invalid, param = {\"PromptID\":0,\"UserID\":\"test_user\",\"CommitVersion\":\"\",\"CommitDescription\":\"\",\"LabelKeys\":null}"),
		},
		{
			name: "invalid user id",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{
					commitLabelMappingDAO: daomocks.NewMockICommitLabelMappingDAO(ctrl),
					promptRelationDAO:     daomocks.NewMockIPromptRelationDAO(ctrl),
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitDraftParam{
					PromptID: 1,
					UserID:   "",
				},
			},
			wantErr: errorx.New("param(PromptID or UserID or CommitVersion) is invalid, param = {\"PromptID\":1,\"UserID\":\"\",\"CommitVersion\":\"\",\"CommitDescription\":\"\",\"LabelKeys\":null}"),
		},
		{
			name: "invalid commit version",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{
					commitLabelMappingDAO: daomocks.NewMockICommitLabelMappingDAO(ctrl),
					promptRelationDAO:     daomocks.NewMockIPromptRelationDAO(ctrl),
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitDraftParam{
					PromptID:      1,
					UserID:        "test_user",
					CommitVersion: "",
				},
			},
			wantErr: errorx.New("param(PromptID or UserID or CommitVersion) is invalid, param = {\"PromptID\":1,\"UserID\":\"test_user\",\"CommitVersion\":\"\",\"CommitDescription\":\"\",\"LabelKeys\":null}"),
		},
		{
			name: "db error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).Return(errorx.New("db error"))

				return fields{
					db:                    mockDB,
					idgen:                 mockIDGen,
					commitLabelMappingDAO: daomocks.NewMockICommitLabelMappingDAO(ctrl),
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitDraftParam{
					PromptID:      1,
					UserID:        "test_user",
					CommitVersion: "1.0.0",
				},
			},
			wantErr: errorx.New("db error"),
		},
		{
			name: "basic prompt not found",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(nil, nil)

				return fields{
					db:             mockDB,
					promptBasicDAO: mockBasicDAO,
					idgen:          mockIDGen,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitDraftParam{
					PromptID:      1,
					UserID:        "test_user",
					CommitVersion: "1.0.0",
				},
			},
			wantErr: errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("Prompt is not found, prompt id = 1")),
		},
		{
			name: "draft not found",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:        1,
					SpaceID:   100,
					PromptKey: "test_key",
				}, nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(nil, nil)

				return fields{
					db:             mockDB,
					promptBasicDAO: mockBasicDAO,
					promptDraftDAO: mockDraftDAO,
					idgen:          mockIDGen,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitDraftParam{
					PromptID:      1,
					UserID:        "test_user",
					CommitVersion: "1.0.0",
				},
			},
			wantErr: errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("Prompt draft is not found, prompt id = 1, user id = test_user")),
		},
		{
			name: "commit dao error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:            1,
					SpaceID:       100,
					PromptKey:     "test_key",
					LatestVersion: "1.0.0",
				}, nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(&model.PromptUserDraft{
					ID:          1001,
					BaseVersion: "1.0.0",
				}, nil)

				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errorx.New("commit dao error"))

				return fields{
					db:              mockDB,
					promptBasicDAO:  mockBasicDAO,
					promptDraftDAO:  mockDraftDAO,
					promptCommitDAO: mockCommitDAO,
					idgen:           mockIDGen,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitDraftParam{
					PromptID:      1,
					UserID:        "test_user",
					CommitVersion: "2.0.0",
				},
			},
			wantErr: errorx.New("commit dao error"),
		},
		{
			name: "delete draft error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:            1,
					SpaceID:       100,
					PromptKey:     "test_key",
					LatestVersion: "1.0.0",
				}, nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(&model.PromptUserDraft{
					ID:          1001,
					BaseVersion: "1.0.0",
				}, nil)

				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockDraftDAO.EXPECT().Delete(gomock.Any(), int64(1001), gomock.Any()).Return(errorx.New("delete draft error"))

				return fields{
					db:              mockDB,
					promptBasicDAO:  mockBasicDAO,
					promptDraftDAO:  mockDraftDAO,
					promptCommitDAO: mockCommitDAO,
					idgen:           mockIDGen,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitDraftParam{
					PromptID:      1,
					UserID:        "test_user",
					CommitVersion: "2.0.0",
				},
			},
			wantErr: errorx.New("delete draft error"),
		},
		{
			name: "update basic error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})
				nilDB, _ := gorm.Open(nil)
				mockDB.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(nilDB)

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:            1,
					SpaceID:       100,
					PromptKey:     "test_key",
					LatestVersion: "1.0.0",
				}, nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(&model.PromptUserDraft{
					ID:          1001,
					BaseVersion: "1.0.0",
				}, nil)

				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockDraftDAO.EXPECT().Delete(gomock.Any(), int64(1001), gomock.Any()).Return(nil)

				mockBasicDAO.EXPECT().Update(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(errorx.New("update basic error"))

				return fields{
					db:              mockDB,
					promptBasicDAO:  mockBasicDAO,
					promptDraftDAO:  mockDraftDAO,
					promptCommitDAO: mockCommitDAO,
					idgen:           mockIDGen,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitDraftParam{
					PromptID:      1,
					UserID:        "test_user",
					CommitVersion: "2.0.0",
				},
			},
			wantErr: errorx.New("update basic error"),
		},
		{
			name: "cache delete error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 0).Return([]int64{}, nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				nilDB, _ := gorm.Open(nil)
				mockDB.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(nilDB)

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:            1,
					SpaceID:       100,
					PromptKey:     "test_key",
					LatestVersion: "1.0.0",
				}, nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(&model.PromptUserDraft{
					ID:          1001,
					BaseVersion: "1.0.0",
				}, nil)

				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockDraftDAO.EXPECT().Delete(gomock.Any(), int64(1001), gomock.Any()).Return(nil)

				mockBasicDAO.EXPECT().Update(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil)

				mockPromptBasicCacheDAO := redismocks.NewMockIPromptBasicDAO(ctrl)
				mockPromptBasicCacheDAO.EXPECT().DelByPromptKey(gomock.Any(), int64(100), "test_key").Return(errorx.New("cache delete error"))

				mockCommitLabelMappingDAO := daomocks.NewMockICommitLabelMappingDAO(ctrl)
				mockCommitLabelMappingDAO.EXPECT().ListByPromptIDAndLabelKeys(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil, nil)

				return fields{
					db:                    mockDB,
					idgen:                 mockIDGen,
					promptBasicDAO:        mockBasicDAO,
					promptDraftDAO:        mockDraftDAO,
					promptCommitDAO:       mockCommitDAO,
					commitLabelMappingDAO: mockCommitLabelMappingDAO,
					promptBasicCacheDAO:   mockPromptBasicCacheDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitDraftParam{
					PromptID:      1,
					UserID:        "test_user",
					CommitVersion: "2.0.0",
				},
			},
			wantErr: nil,
		},
		{
			name: "label binding - query existing mappings error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				nilDB, _ := gorm.Open(nil)
				mockDB.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(nilDB)

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:            1,
					SpaceID:       100,
					PromptKey:     "test_key",
					LatestVersion: "1.0.0",
				}, nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(&model.PromptUserDraft{
					ID:          1001,
					BaseVersion: "1.0.0",
				}, nil)

				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockDraftDAO.EXPECT().Delete(gomock.Any(), int64(1001), gomock.Any()).Return(nil)

				mockBasicDAO.EXPECT().Update(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil)

				mockCommitLabelMappingDAO := daomocks.NewMockICommitLabelMappingDAO(ctrl)
				mockCommitLabelMappingDAO.EXPECT().ListByPromptIDAndLabelKeys(gomock.Any(), int64(1), []string{"label1", "label2"}, gomock.Any()).Return(nil, errorx.New("query mapping error"))

				return fields{
					db:                    mockDB,
					idgen:                 mockIDGen,
					promptBasicDAO:        mockBasicDAO,
					promptDraftDAO:        mockDraftDAO,
					promptCommitDAO:       mockCommitDAO,
					commitLabelMappingDAO: mockCommitLabelMappingDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitDraftParam{
					PromptID:      1,
					UserID:        "test_user",
					CommitVersion: "2.0.0",
					LabelKeys:     []string{"label1", "label2"},
				},
			},
			wantErr: errorx.New("query mapping error"),
		},
		{
			name: "label binding - id generation error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 2).Return(nil, errorx.New("id gen error"))

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				nilDB, _ := gorm.Open(nil)
				mockDB.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(nilDB)

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:            1,
					SpaceID:       100,
					PromptKey:     "test_key",
					LatestVersion: "1.0.0",
				}, nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(&model.PromptUserDraft{
					ID:          1001,
					BaseVersion: "1.0.0",
				}, nil)

				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockDraftDAO.EXPECT().Delete(gomock.Any(), int64(1001), gomock.Any()).Return(nil)

				mockBasicDAO.EXPECT().Update(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil)

				mockCommitLabelMappingDAO := daomocks.NewMockICommitLabelMappingDAO(ctrl)
				mockCommitLabelMappingDAO.EXPECT().ListByPromptIDAndLabelKeys(gomock.Any(), int64(1), []string{"label1", "label2"}, gomock.Any()).Return(nil, nil)

				return fields{
					db:                    mockDB,
					idgen:                 mockIDGen,
					promptBasicDAO:        mockBasicDAO,
					promptDraftDAO:        mockDraftDAO,
					promptCommitDAO:       mockCommitDAO,
					commitLabelMappingDAO: mockCommitLabelMappingDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitDraftParam{
					PromptID:      1,
					UserID:        "test_user",
					CommitVersion: "2.0.0",
					LabelKeys:     []string{"label1", "label2"},
				},
			},
			wantErr: errorx.New("id gen error"),
		},
		{
			name: "label binding - batch create error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{2001, 2002}, nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				nilDB, _ := gorm.Open(nil)
				mockDB.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(nilDB)

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:            1,
					SpaceID:       100,
					PromptKey:     "test_key",
					LatestVersion: "1.0.0",
				}, nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(&model.PromptUserDraft{
					ID:          1001,
					BaseVersion: "1.0.0",
				}, nil)

				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockDraftDAO.EXPECT().Delete(gomock.Any(), int64(1001), gomock.Any()).Return(nil)

				mockBasicDAO.EXPECT().Update(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil)

				mockCommitLabelMappingDAO := daomocks.NewMockICommitLabelMappingDAO(ctrl)
				mockCommitLabelMappingDAO.EXPECT().ListByPromptIDAndLabelKeys(gomock.Any(), int64(1), []string{"label1", "label2"}, gomock.Any()).Return(nil, nil)
				mockCommitLabelMappingDAO.EXPECT().BatchCreate(gomock.Any(), gomock.Any(), gomock.Any()).Return(errorx.New("batch create error"))

				return fields{
					db:                    mockDB,
					idgen:                 mockIDGen,
					promptBasicDAO:        mockBasicDAO,
					promptDraftDAO:        mockDraftDAO,
					promptCommitDAO:       mockCommitDAO,
					commitLabelMappingDAO: mockCommitLabelMappingDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitDraftParam{
					PromptID:      1,
					UserID:        "test_user",
					CommitVersion: "2.0.0",
					LabelKeys:     []string{"label1", "label2"},
				},
			},
			wantErr: errorx.New("batch create error"),
		},
		{
			name: "label binding - batch update error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{2001, 2002}, nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				nilDB, _ := gorm.Open(nil)
				mockDB.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(nilDB)

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:            1,
					SpaceID:       100,
					PromptKey:     "test_key",
					LatestVersion: "1.0.0",
				}, nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(&model.PromptUserDraft{
					ID:          1001,
					BaseVersion: "1.0.0",
				}, nil)

				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockDraftDAO.EXPECT().Delete(gomock.Any(), int64(1001), gomock.Any()).Return(nil)

				mockBasicDAO.EXPECT().Update(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil)

				mockCommitLabelMappingDAO := daomocks.NewMockICommitLabelMappingDAO(ctrl)
				// 返回一个已存在的映射
				mockCommitLabelMappingDAO.EXPECT().ListByPromptIDAndLabelKeys(gomock.Any(), int64(1), []string{"label1", "label2"}, gomock.Any()).Return([]*model.PromptCommitLabelMapping{
					{
						ID:            3001,
						SpaceID:       100,
						PromptID:      1,
						LabelKey:      "label1",
						PromptVersion: "1.0.0",
					},
				}, nil)
				// 创建一个新的映射
				mockCommitLabelMappingDAO.EXPECT().BatchCreate(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, mappings []*model.PromptCommitLabelMapping, opts ...db.Option) error {
					assert.Equal(t, 1, len(mappings))
					assert.Equal(t, "label2", mappings[0].LabelKey)
					assert.Equal(t, "2.0.0", mappings[0].PromptVersion)
					return nil
				})
				// 更新已存在的映射失败
				mockCommitLabelMappingDAO.EXPECT().BatchUpdate(gomock.Any(), gomock.Any(), gomock.Any()).Return(errorx.New("batch update error"))

				return fields{
					db:                    mockDB,
					idgen:                 mockIDGen,
					promptBasicDAO:        mockBasicDAO,
					promptDraftDAO:        mockDraftDAO,
					promptCommitDAO:       mockCommitDAO,
					commitLabelMappingDAO: mockCommitLabelMappingDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitDraftParam{
					PromptID:      1,
					UserID:        "test_user",
					CommitVersion: "2.0.0",
					LabelKeys:     []string{"label1", "label2"},
				},
			},
			wantErr: errorx.New("batch update error"),
		},
		{
			name: "label binding - no labels",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 0).Return([]int64{}, nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				nilDB, _ := gorm.Open(nil)
				mockDB.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(nilDB)

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:            1,
					SpaceID:       100,
					PromptKey:     "test_key",
					LatestVersion: "1.0.0",
				}, nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(&model.PromptUserDraft{
					ID:          1001,
					BaseVersion: "1.0.0",
				}, nil)

				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockDraftDAO.EXPECT().Delete(gomock.Any(), int64(1001), gomock.Any()).Return(nil)

				mockBasicDAO.EXPECT().Update(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil)

				mockPromptBasicCacheDAO := redismocks.NewMockIPromptBasicDAO(ctrl)
				mockPromptBasicCacheDAO.EXPECT().DelByPromptKey(gomock.Any(), int64(100), "test_key").Return(nil)

				mockCommitLabelMappingDAO := daomocks.NewMockICommitLabelMappingDAO(ctrl)
				mockCommitLabelMappingDAO.EXPECT().ListByPromptIDAndLabelKeys(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil, nil)

				return fields{
					db:                    mockDB,
					idgen:                 mockIDGen,
					promptBasicDAO:        mockBasicDAO,
					promptDraftDAO:        mockDraftDAO,
					promptCommitDAO:       mockCommitDAO,
					commitLabelMappingDAO: mockCommitLabelMappingDAO,
					promptBasicCacheDAO:   mockPromptBasicCacheDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitDraftParam{
					PromptID:      1,
					UserID:        "test_user",
					CommitVersion: "2.0.0",
					LabelKeys:     []string{},
				},
			},
			wantErr: nil,
		},
		{
			name: "label binding - new labels creation",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{2001, 2002}, nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				nilDB, _ := gorm.Open(nil)
				mockDB.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(nilDB)

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:            1,
					SpaceID:       100,
					PromptKey:     "test_key",
					LatestVersion: "1.0.0",
				}, nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(&model.PromptUserDraft{
					ID:          1001,
					BaseVersion: "1.0.0",
				}, nil)

				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockDraftDAO.EXPECT().Delete(gomock.Any(), int64(1001), gomock.Any()).Return(nil)

				mockBasicDAO.EXPECT().Update(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil)

				mockPromptBasicCacheDAO := redismocks.NewMockIPromptBasicDAO(ctrl)
				mockPromptBasicCacheDAO.EXPECT().DelByPromptKey(gomock.Any(), int64(100), "test_key").Return(nil)

				mockCommitLabelMappingDAO := daomocks.NewMockICommitLabelMappingDAO(ctrl)
				// 没有已存在的映射
				mockCommitLabelMappingDAO.EXPECT().ListByPromptIDAndLabelKeys(gomock.Any(), int64(1), []string{"label1", "label2"}, gomock.Any()).Return(nil, nil)
				// 创建新的映射
				mockCommitLabelMappingDAO.EXPECT().BatchCreate(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, mappings []*model.PromptCommitLabelMapping, opts ...db.Option) error {
					assert.Equal(t, 2, len(mappings))
					assert.Equal(t, int64(2001), mappings[0].ID)
					assert.Equal(t, "label1", mappings[0].LabelKey)
					assert.Equal(t, "2.0.0", mappings[0].PromptVersion)
					assert.Equal(t, "test_user", mappings[0].CreatedBy)
					assert.Equal(t, int64(2002), mappings[1].ID)
					assert.Equal(t, "label2", mappings[1].LabelKey)
					assert.Equal(t, "2.0.0", mappings[1].PromptVersion)
					return nil
				})

				return fields{
					db:                    mockDB,
					idgen:                 mockIDGen,
					promptBasicDAO:        mockBasicDAO,
					promptDraftDAO:        mockDraftDAO,
					promptCommitDAO:       mockCommitDAO,
					commitLabelMappingDAO: mockCommitLabelMappingDAO,
					promptBasicCacheDAO:   mockPromptBasicCacheDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitDraftParam{
					PromptID:      1,
					UserID:        "test_user",
					CommitVersion: "2.0.0",
					LabelKeys:     []string{"label1", "label2"},
				},
			},
			wantErr: nil,
		},
		{
			name: "label binding - existing labels update",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{2001, 2002}, nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				nilDB, _ := gorm.Open(nil)
				mockDB.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(nilDB)

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:            1,
					SpaceID:       100,
					PromptKey:     "test_key",
					LatestVersion: "1.0.0",
				}, nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(&model.PromptUserDraft{
					ID:          1001,
					BaseVersion: "1.0.0",
				}, nil)

				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockDraftDAO.EXPECT().Delete(gomock.Any(), int64(1001), gomock.Any()).Return(nil)

				mockBasicDAO.EXPECT().Update(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil)

				mockPromptBasicCacheDAO := redismocks.NewMockIPromptBasicDAO(ctrl)
				mockPromptBasicCacheDAO.EXPECT().DelByPromptKey(gomock.Any(), int64(100), "test_key").Return(nil)

				mockCommitLabelMappingDAO := daomocks.NewMockICommitLabelMappingDAO(ctrl)
				// 返回已存在的映射
				mockCommitLabelMappingDAO.EXPECT().ListByPromptIDAndLabelKeys(gomock.Any(), int64(1), []string{"label1", "label2"}, gomock.Any()).Return([]*model.PromptCommitLabelMapping{
					{
						ID:            3001,
						SpaceID:       100,
						PromptID:      1,
						LabelKey:      "label1",
						PromptVersion: "1.0.0",
						CreatedBy:     "old_user",
						UpdatedBy:     "old_user",
					},
					{
						ID:            3002,
						SpaceID:       100,
						PromptID:      1,
						LabelKey:      "label2",
						PromptVersion: "1.5.0",
						CreatedBy:     "old_user",
						UpdatedBy:     "old_user",
					},
				}, nil)
				// 更新已存在的映射
				mockCommitLabelMappingDAO.EXPECT().BatchUpdate(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, mappings []*model.PromptCommitLabelMapping, opts ...db.Option) error {
					assert.Equal(t, 2, len(mappings))
					for _, mapping := range mappings {
						assert.Equal(t, "2.0.0", mapping.PromptVersion)
						assert.Equal(t, "test_user", mapping.UpdatedBy)
					}
					return nil
				})

				return fields{
					db:                    mockDB,
					idgen:                 mockIDGen,
					promptBasicDAO:        mockBasicDAO,
					promptDraftDAO:        mockDraftDAO,
					promptCommitDAO:       mockCommitDAO,
					commitLabelMappingDAO: mockCommitLabelMappingDAO,
					promptBasicCacheDAO:   mockPromptBasicCacheDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitDraftParam{
					PromptID:      1,
					UserID:        "test_user",
					CommitVersion: "2.0.0",
					LabelKeys:     []string{"label1", "label2"},
				},
			},
			wantErr: nil,
		},
		{
			name: "label binding - mixed scenario",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 3).Return([]int64{2001, 2002, 2003}, nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				nilDB, _ := gorm.Open(nil)
				mockDB.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(nilDB)

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:            1,
					SpaceID:       100,
					PromptKey:     "test_key",
					LatestVersion: "1.0.0",
				}, nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(&model.PromptUserDraft{
					ID:          1001,
					BaseVersion: "1.0.0",
				}, nil)

				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockDraftDAO.EXPECT().Delete(gomock.Any(), int64(1001), gomock.Any()).Return(nil)

				mockBasicDAO.EXPECT().Update(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil)

				mockPromptBasicCacheDAO := redismocks.NewMockIPromptBasicDAO(ctrl)
				mockPromptBasicCacheDAO.EXPECT().DelByPromptKey(gomock.Any(), int64(100), "test_key").Return(nil)

				mockCommitLabelMappingDAO := daomocks.NewMockICommitLabelMappingDAO(ctrl)
				// 返回部分已存在的映射
				mockCommitLabelMappingDAO.EXPECT().ListByPromptIDAndLabelKeys(gomock.Any(), int64(1), []string{"label1", "label2", "label3"}, gomock.Any()).Return([]*model.PromptCommitLabelMapping{
					{
						ID:            3001,
						SpaceID:       100,
						PromptID:      1,
						LabelKey:      "label1",
						PromptVersion: "1.0.0",
						CreatedBy:     "old_user",
						UpdatedBy:     "old_user",
					},
				}, nil)
				// 创建新的映射 (label2, label3)
				mockCommitLabelMappingDAO.EXPECT().BatchCreate(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, mappings []*model.PromptCommitLabelMapping, opts ...db.Option) error {
					assert.Equal(t, 2, len(mappings))
					labelKeys := make([]string, 0, len(mappings))
					for _, mapping := range mappings {
						labelKeys = append(labelKeys, mapping.LabelKey)
						assert.Equal(t, "2.0.0", mapping.PromptVersion)
						assert.Equal(t, "test_user", mapping.CreatedBy)
					}
					assert.Contains(t, labelKeys, "label2")
					assert.Contains(t, labelKeys, "label3")
					return nil
				})
				// 更新已存在的映射 (label1)
				mockCommitLabelMappingDAO.EXPECT().BatchUpdate(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, mappings []*model.PromptCommitLabelMapping, opts ...db.Option) error {
					assert.Equal(t, 1, len(mappings))
					assert.Equal(t, "label1", mappings[0].LabelKey)
					assert.Equal(t, "2.0.0", mappings[0].PromptVersion)
					assert.Equal(t, "test_user", mappings[0].UpdatedBy)
					return nil
				})

				return fields{
					db:                    mockDB,
					idgen:                 mockIDGen,
					promptBasicDAO:        mockBasicDAO,
					promptDraftDAO:        mockDraftDAO,
					promptCommitDAO:       mockCommitDAO,
					commitLabelMappingDAO: mockCommitLabelMappingDAO,
					promptBasicCacheDAO:   mockPromptBasicCacheDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitDraftParam{
					PromptID:      1,
					UserID:        "test_user",
					CommitVersion: "2.0.0",
					LabelKeys:     []string{"label1", "label2", "label3"},
				},
			},
			wantErr: nil,
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 0).Return([]int64{}, nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				nilDB, _ := gorm.Open(nil)
				mockDB.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(nilDB)

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:            1,
					SpaceID:       100,
					PromptKey:     "test_key",
					LatestVersion: "1.0.0",
				}, nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(&model.PromptUserDraft{
					ID:          1001,
					BaseVersion: "1.0.0",
				}, nil)

				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, commit *model.PromptCommit, timeNow time.Time, opts ...db.Option) error {
					assert.Equal(t, int64(1001), commit.ID)
					assert.Equal(t, "2.0.0", commit.Version)
					assert.Equal(t, "1.0.0", commit.BaseVersion)
					assert.Equal(t, "test_user", commit.CommittedBy)
					return nil
				})

				mockDraftDAO.EXPECT().Delete(gomock.Any(), int64(1001), gomock.Any()).Return(nil)

				mockBasicDAO.EXPECT().Update(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, id int64, updates map[string]interface{}, opts ...db.Option) error {
					assert.Equal(t, int64(1), id)
					assert.Equal(t, "2.0.0", updates["latest_version"])
					return nil
				})

				mockPromptBasicCacheDAO := redismocks.NewMockIPromptBasicDAO(ctrl)
				mockPromptBasicCacheDAO.EXPECT().DelByPromptKey(gomock.Any(), int64(100), "test_key").Return(nil)

				mockCommitLabelMappingDAO := daomocks.NewMockICommitLabelMappingDAO(ctrl)
				mockCommitLabelMappingDAO.EXPECT().ListByPromptIDAndLabelKeys(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil, nil)

				return fields{
					db:                    mockDB,
					idgen:                 mockIDGen,
					promptBasicDAO:        mockBasicDAO,
					promptDraftDAO:        mockDraftDAO,
					promptCommitDAO:       mockCommitDAO,
					commitLabelMappingDAO: mockCommitLabelMappingDAO,
					promptBasicCacheDAO:   mockPromptBasicCacheDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitDraftParam{
					PromptID:          1,
					UserID:            "test_user",
					CommitVersion:     "2.0.0",
					CommitDescription: "test commit",
				},
			},
			wantErr: nil,
		},
		{
			name: "commit with snippets - hasSnippets true",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 0).Return([]int64{}, nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				nilDB, _ := gorm.Open(nil)
				mockDB.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(nilDB)

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:            1,
					SpaceID:       100,
					PromptKey:     "test_key",
					LatestVersion: "1.0.0",
				}, nil)

				// 创建包含snippet的草稿
				draftWithSnippets := &model.PromptUserDraft{
					ID:          1001,
					BaseVersion: "1.0.0",
					HasSnippets: true,
				}

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(draftWithSnippets, nil)

				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, commit *model.PromptCommit, timeNow time.Time, opts ...db.Option) error {
					assert.Equal(t, int64(1001), commit.ID)
					assert.Equal(t, "2.0.0", commit.Version)
					assert.Equal(t, "1.0.0", commit.BaseVersion)
					assert.Equal(t, "test_user", commit.CommittedBy)
					assert.True(t, commit.HasSnippets, "HasSnippets should be true for drafts with snippets")
					return nil
				})

				mockDraftDAO.EXPECT().Delete(gomock.Any(), int64(1001), gomock.Any()).Return(nil)

				mockBasicDAO.EXPECT().Update(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil)

				mockPromptBasicCacheDAO := redismocks.NewMockIPromptBasicDAO(ctrl)
				mockPromptBasicCacheDAO.EXPECT().DelByPromptKey(gomock.Any(), int64(100), "test_key").Return(nil)

				mockCommitLabelMappingDAO := daomocks.NewMockICommitLabelMappingDAO(ctrl)
				mockCommitLabelMappingDAO.EXPECT().ListByPromptIDAndLabelKeys(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil, nil)

				// Mock snippet relation operations
				mockRelationDAO := daomocks.NewMockIPromptRelationDAO(ctrl)
				mockRelationDAO.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*model.PromptRelation{
					{
						ID:                2001,
						SpaceID:           100,
						MainPromptID:      1,
						MainPromptVersion: "",
						MainDraftUserID:   "test_user",
						SubPromptID:       2,
						SubPromptVersion:  "1.0.0",
					},
				}, nil)
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{3001}, nil)
				mockRelationDAO.EXPECT().BatchCreate(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockRelationDAO.EXPECT().BatchDeleteByIDs(gomock.Any(), []int64{2001}, gomock.Any()).Return(nil)

				return fields{
					db:                    mockDB,
					idgen:                 mockIDGen,
					promptBasicDAO:        mockBasicDAO,
					promptCommitDAO:       mockCommitDAO,
					promptDraftDAO:        mockDraftDAO,
					commitLabelMappingDAO: mockCommitLabelMappingDAO,
					promptBasicCacheDAO:   mockPromptBasicCacheDAO,
					promptCacheDAO:        redismocks.NewMockIPromptDAO(ctrl),
					promptRelationDAO:     mockRelationDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitDraftParam{
					PromptID:          1,
					UserID:            "test_user",
					CommitVersion:     "2.0.0",
					CommitDescription: "commit with snippets",
				},
			},
			wantErr: nil,
		},
		{
			name: "commit without snippets - hasSnippets false",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), 0).Return([]int64{}, nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				nilDB, _ := gorm.Open(nil)
				mockDB.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(nilDB)

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&model.PromptBasic{
					ID:            1,
					SpaceID:       100,
					PromptKey:     "test_key",
					LatestVersion: "1.0.0",
				}, nil)

				// 创建不包含snippet的草稿
				draftWithoutSnippets := &model.PromptUserDraft{
					ID:          1001,
					BaseVersion: "1.0.0",
					HasSnippets: false,
				}

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().Get(gomock.Any(), int64(1), "test_user", gomock.Any()).Return(draftWithoutSnippets, nil)

				mockCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, commit *model.PromptCommit, timeNow time.Time, opts ...db.Option) error {
					assert.Equal(t, int64(1001), commit.ID)
					assert.Equal(t, "2.0.0", commit.Version)
					assert.Equal(t, "1.0.0", commit.BaseVersion)
					assert.Equal(t, "test_user", commit.CommittedBy)
					assert.False(t, commit.HasSnippets, "HasSnippets should be false for drafts without snippets")
					return nil
				})

				mockDraftDAO.EXPECT().Delete(gomock.Any(), int64(1001), gomock.Any()).Return(nil)

				mockBasicDAO.EXPECT().Update(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil)

				mockPromptBasicCacheDAO := redismocks.NewMockIPromptBasicDAO(ctrl)
				mockPromptBasicCacheDAO.EXPECT().DelByPromptKey(gomock.Any(), int64(100), "test_key").Return(nil)

				mockRelationDAO := daomocks.NewMockIPromptRelationDAO(ctrl)

				mockCommitLabelMappingDAO := daomocks.NewMockICommitLabelMappingDAO(ctrl)
				mockCommitLabelMappingDAO.EXPECT().ListByPromptIDAndLabelKeys(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil, nil)

				return fields{
					db:                    mockDB,
					idgen:                 mockIDGen,
					promptBasicDAO:        mockBasicDAO,
					promptCommitDAO:       mockCommitDAO,
					promptDraftDAO:        mockDraftDAO,
					commitLabelMappingDAO: mockCommitLabelMappingDAO,
					promptBasicCacheDAO:   mockPromptBasicCacheDAO,
					promptCacheDAO:        redismocks.NewMockIPromptDAO(ctrl),
					promptRelationDAO:     mockRelationDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitDraftParam{
					PromptID:          1,
					UserID:            "test_user",
					CommitVersion:     "2.0.0",
					CommitDescription: "commit without snippets",
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)

			d := &ManageRepoImpl{
				db:                    ttFields.db,
				idgen:                 ttFields.idgen,
				promptBasicDAO:        ttFields.promptBasicDAO,
				promptCommitDAO:       ttFields.promptCommitDAO,
				promptDraftDAO:        ttFields.promptDraftDAO,
				commitLabelMappingDAO: ttFields.commitLabelMappingDAO,
				promptBasicCacheDAO:   ttFields.promptBasicCacheDAO,
				promptCacheDAO:        ttFields.promptCacheDAO,
				promptRelationDAO:     ttFields.promptRelationDAO,
			}

			err := d.CommitDraft(tt.args.ctx, tt.args.param)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
		})
	}
}

func TestNewManageRepo(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建mock依赖
	mockDB := dbmocks.NewMockProvider(ctrl)
	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockPromptBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
	mockPromptCommitDAO := daomocks.NewMockIPromptCommitDAO(ctrl)
	mockPromptDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
	mockCommitLabelMappingDAO := daomocks.NewMockICommitLabelMappingDAO(ctrl)
	mockPromptBasicCacheDAO := redismocks.NewMockIPromptBasicDAO(ctrl)
	mockPromptCacheDAO := redismocks.NewMockIPromptDAO(ctrl)
	mockPromptRelationDAO := daomocks.NewMockIPromptRelationDAO(ctrl)

	// 调用构造函数
	// 调用构造函数
	repo := NewManageRepo(
		mockDB,
		mockIDGen,
		nil, // meter可以为nil，因为我们只测试构造函数
		mockPromptBasicDAO,
		mockPromptCommitDAO,
		mockPromptDraftDAO,
		mockCommitLabelMappingDAO,
		mockPromptRelationDAO,
		mockPromptBasicCacheDAO,
		mockPromptCacheDAO,
	)
	// 验证返回的实例
	assert.NotNil(t, repo)
	// 类型断言以访问内部字段
	manageRepo, ok := repo.(*ManageRepoImpl)
	assert.True(t, ok)
	// 验证所有字段都正确设置
	assert.Equal(t, mockDB, manageRepo.db)
	assert.Equal(t, mockIDGen, manageRepo.idgen)
	assert.Equal(t, mockPromptBasicDAO, manageRepo.promptBasicDAO)
	assert.Equal(t, mockPromptCommitDAO, manageRepo.promptCommitDAO)
	assert.Equal(t, mockPromptDraftDAO, manageRepo.promptDraftDAO)
	assert.Equal(t, mockCommitLabelMappingDAO, manageRepo.commitLabelMappingDAO)
	assert.Equal(t, mockPromptBasicCacheDAO, manageRepo.promptBasicCacheDAO)
	assert.Equal(t, mockPromptCacheDAO, manageRepo.promptCacheDAO)
	// meter为nil时，promptCacheMetrics也会是nil，这是正常的
	assert.Nil(t, manageRepo.promptCacheMetrics)
}

func TestManageRepoImpl_ListPrompt(t *testing.T) {
	type fields struct {
		promptBasicDAO mysql.IPromptBasicDAO
		promptDraftDAO mysql.IPromptUserDraftDAO
	}
	type args struct {
		ctx   context.Context
		param repo.ListPromptParam
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *repo.ListPromptResult
		wantErr      error
	}{
		{
			name: "invalid space id",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListPromptParam{
					SpaceID:  0,
					PageNum:  1,
					PageSize: 10,
				},
			},
			want:    nil,
			wantErr: errorx.New("param(SpaceID or PageNum or PageSize) is invalid, param = {\"SpaceID\":0,\"KeyWord\":\"\",\"CreatedBys\":null,\"UserID\":\"\",\"CommittedOnly\":false,\"FilterPromptTypes\":null,\"PageNum\":1,\"PageSize\":10,\"OrderBy\":0,\"Asc\":false}"),
		},
		{
			name: "invalid page num",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListPromptParam{
					SpaceID:  123,
					PageNum:  0,
					PageSize: 10,
				},
			},
			want:    nil,
			wantErr: errorx.New("param(SpaceID or PageNum or PageSize) is invalid, param = {\"SpaceID\":123,\"KeyWord\":\"\",\"CreatedBys\":null,\"UserID\":\"\",\"CommittedOnly\":false,\"FilterPromptTypes\":null,\"PageNum\":0,\"PageSize\":10,\"OrderBy\":0,\"Asc\":false}"),
		},
		{
			name: "invalid page size",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListPromptParam{
					SpaceID:  123,
					PageNum:  1,
					PageSize: 0,
				},
			},
			want:    nil,
			wantErr: errorx.New("param(SpaceID or PageNum or PageSize) is invalid, param = {\"SpaceID\":123,\"KeyWord\":\"\",\"CreatedBys\":null,\"UserID\":\"\",\"CommittedOnly\":false,\"FilterPromptTypes\":null,\"PageNum\":1,\"PageSize\":0,\"OrderBy\":0,\"Asc\":false}"),
		},
		{
			name: "empty result",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().List(gomock.Any(), mysql.ListPromptBasicParam{
					SpaceID: 123,
					Offset:  0,
					Limit:   10,
				}).Return([]*model.PromptBasic{}, int64(0), nil)

				return fields{
					promptBasicDAO: mockBasicDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListPromptParam{
					SpaceID:  123,
					PageNum:  1,
					PageSize: 10,
				},
			},
			want: &repo.ListPromptResult{
				Total:     0,
				PromptDOs: nil,
			},
			wantErr: nil,
		},
		{
			name: "list with user draft association",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().List(gomock.Any(), mysql.ListPromptBasicParam{
					SpaceID: 123,
					Offset:  0,
					Limit:   10,
				}).Return([]*model.PromptBasic{
					{
						ID:        1001,
						SpaceID:   123,
						PromptKey: "test_key_1",
						Name:      "Test Prompt 1",
					},
					{
						ID:        1002,
						SpaceID:   123,
						PromptKey: "test_key_2",
						Name:      "Test Prompt 2",
					},
				}, int64(2), nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().MGet(gomock.Any(), []mysql.PromptIDUserIDPair{
					{
						PromptID: 1001,
						UserID:   "test_user",
					},
					{
						PromptID: 1002,
						UserID:   "test_user",
					},
				}).Return(map[mysql.PromptIDUserIDPair]*model.PromptUserDraft{
					{
						PromptID: 1001,
						UserID:   "test_user",
					}: {
						ID:       2001,
						PromptID: 1001,
						UserID:   "test_user",
					},
				}, nil)

				return fields{
					promptBasicDAO: mockBasicDAO,
					promptDraftDAO: mockDraftDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListPromptParam{
					SpaceID:  123,
					PageNum:  1,
					PageSize: 10,
					UserID:   "test_user",
				},
			},
			want: &repo.ListPromptResult{
				Total: 2,
				PromptDOs: []*entity.Prompt{
					{
						ID:          1001,
						SpaceID:     123,
						PromptKey:   "test_key_1",
						PromptBasic: &entity.PromptBasic{PromptType: entity.PromptTypeNormal, DisplayName: "Test Prompt 1"},
						PromptDraft: &entity.PromptDraft{
							PromptDetail: &entity.PromptDetail{
								PromptTemplate: &entity.PromptTemplate{},
							},
							DraftInfo: &entity.DraftInfo{
								UserID: "test_user",
							},
						},
					},
					{
						ID:          1002,
						SpaceID:     123,
						PromptKey:   "test_key_2",
						PromptBasic: &entity.PromptBasic{PromptType: entity.PromptTypeNormal, DisplayName: "Test Prompt 2"},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "list with keyword filter",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().List(gomock.Any(), mysql.ListPromptBasicParam{
					SpaceID: 123,
					KeyWord: "search_term",
					Offset:  0,
					Limit:   10,
				}).Return([]*model.PromptBasic{
					{
						ID:        1001,
						SpaceID:   123,
						PromptKey: "test_key_1",
						Name:      "Test search_term Prompt",
					},
				}, int64(1), nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().MGet(gomock.Any(), []mysql.PromptIDUserIDPair{
					{
						PromptID: 1001,
						UserID:   "test_user",
					},
				}).Return(map[mysql.PromptIDUserIDPair]*model.PromptUserDraft{}, nil)

				return fields{
					promptBasicDAO: mockBasicDAO,
					promptDraftDAO: mockDraftDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListPromptParam{
					SpaceID:  123,
					PageNum:  1,
					PageSize: 10,
					KeyWord:  "search_term",
					UserID:   "test_user",
				},
			},
			want: &repo.ListPromptResult{
				Total: 1,
				PromptDOs: []*entity.Prompt{
					{
						ID:          1001,
						SpaceID:     123,
						PromptKey:   "test_key_1",
						PromptBasic: &entity.PromptBasic{PromptType: entity.PromptTypeNormal, DisplayName: "Test search_term Prompt"},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "list with created by filter",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().List(gomock.Any(), mysql.ListPromptBasicParam{
					SpaceID:    123,
					CreatedBys: []string{"user1", "user2"},
					Offset:     0,
					Limit:      10,
				}).Return([]*model.PromptBasic{
					{
						ID:        1001,
						SpaceID:   123,
						PromptKey: "test_key_1",
						Name:      "Test Prompt 1",
						CreatedBy: "user1",
					},
				}, int64(1), nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().MGet(gomock.Any(), []mysql.PromptIDUserIDPair{
					{
						PromptID: 1001,
						UserID:   "test_user",
					},
				}).Return(map[mysql.PromptIDUserIDPair]*model.PromptUserDraft{}, nil)

				return fields{
					promptBasicDAO: mockBasicDAO,
					promptDraftDAO: mockDraftDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListPromptParam{
					SpaceID:    123,
					PageNum:    1,
					PageSize:   10,
					CreatedBys: []string{"user1", "user2"},
					UserID:     "test_user",
				},
			},
			want: &repo.ListPromptResult{
				Total: 1,
				PromptDOs: []*entity.Prompt{
					{
						ID:          1001,
						SpaceID:     123,
						PromptKey:   "test_key_1",
						PromptBasic: &entity.PromptBasic{PromptType: entity.PromptTypeNormal, DisplayName: "Test Prompt 1", CreatedBy: "user1"},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "list with order by and asc",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().List(gomock.Any(), mysql.ListPromptBasicParam{
					SpaceID: 123,
					Offset:  10,
					Limit:   5,
					OrderBy: 1,
					Asc:     true,
				}).Return([]*model.PromptBasic{
					{
						ID:        1001,
						SpaceID:   123,
						PromptKey: "test_key_1",
						Name:      "Test Prompt 1",
					},
				}, int64(15), nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().MGet(gomock.Any(), []mysql.PromptIDUserIDPair{
					{
						PromptID: 1001,
						UserID:   "test_user",
					},
				}).Return(map[mysql.PromptIDUserIDPair]*model.PromptUserDraft{}, nil)

				return fields{
					promptBasicDAO: mockBasicDAO,
					promptDraftDAO: mockDraftDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListPromptParam{
					SpaceID:  123,
					PageNum:  3,
					PageSize: 5,
					OrderBy:  1,
					Asc:      true,
					UserID:   "test_user",
				},
			},
			want: &repo.ListPromptResult{
				Total: 15,
				PromptDOs: []*entity.Prompt{
					{
						ID:          1001,
						SpaceID:     123,
						PromptKey:   "test_key_1",
						PromptBasic: &entity.PromptBasic{PromptType: entity.PromptTypeNormal, DisplayName: "Test Prompt 1"},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "basic dao error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().List(gomock.Any(), mysql.ListPromptBasicParam{
					SpaceID: 123,
					Offset:  0,
					Limit:   10,
				}).Return(nil, int64(0), errorx.New("basic dao error"))

				return fields{
					promptBasicDAO: mockBasicDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListPromptParam{
					SpaceID:  123,
					PageNum:  1,
					PageSize: 10,
				},
			},
			want:    nil,
			wantErr: errorx.New("basic dao error"),
		},
		{
			name: "draft dao error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().List(gomock.Any(), mysql.ListPromptBasicParam{
					SpaceID: 123,
					Offset:  0,
					Limit:   10,
				}).Return([]*model.PromptBasic{
					{
						ID:        1001,
						SpaceID:   123,
						PromptKey: "test_key_1",
						Name:      "Test Prompt 1",
					},
				}, int64(1), nil)

				mockDraftDAO := daomocks.NewMockIPromptUserDraftDAO(ctrl)
				mockDraftDAO.EXPECT().MGet(gomock.Any(), []mysql.PromptIDUserIDPair{
					{
						PromptID: 1001,
						UserID:   "test_user",
					},
				}).Return(nil, errorx.New("draft dao error"))

				return fields{
					promptBasicDAO: mockBasicDAO,
					promptDraftDAO: mockDraftDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListPromptParam{
					SpaceID:  123,
					PageNum:  1,
					PageSize: 10,
					UserID:   "test_user",
				},
			},
			want:    nil,
			wantErr: errorx.New("draft dao error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)

			d := &ManageRepoImpl{
				promptBasicDAO: ttFields.promptBasicDAO,
				promptDraftDAO: ttFields.promptDraftDAO,
			}

			got, err := d.ListPrompt(tt.args.ctx, tt.args.param)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestManageRepoImpl_ListParentPrompt(t *testing.T) {
	t.Parallel()
	type fields struct {
		promptRelationDAO mysql.IPromptRelationDAO
		promptBasicDAO    mysql.IPromptBasicDAO
	}
	type args struct {
		ctx   context.Context
		param repo.ListParentPromptParam
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      error
		wantErrMsg   string
		check        func(t *testing.T, got map[string]*repo.PromptCommitVersions)
	}{
		{
			name: "invalid sub prompt id",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListParentPromptParam{
					SubPromptID: 0,
				},
			},
			wantErrMsg: "param(SubPromptID) is invalid",
		},
		{
			name: "relation dao error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRelationDAO := daomocks.NewMockIPromptRelationDAO(ctrl)
				mockRelationDAO.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, errorx.New("list error"))
				return fields{
					promptRelationDAO: mockRelationDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListParentPromptParam{
					SubPromptID: 200,
				},
			},
			wantErr: errorx.New("list error"),
		},
		{
			name: "no relations",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRelationDAO := daomocks.NewMockIPromptRelationDAO(ctrl)
				mockRelationDAO.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*model.PromptRelation{}, nil)
				return fields{
					promptRelationDAO: mockRelationDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListParentPromptParam{
					SubPromptID: 200,
				},
			},
			check: func(t *testing.T, got map[string]*repo.PromptCommitVersions) {
				assert.Nil(t, got)
			},
		},
		{
			name: "list success with versions",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				relations := []*model.PromptRelation{
					{
						ID:                1,
						SpaceID:           1,
						MainPromptID:      101,
						MainPromptVersion: "1.0.0",
						SubPromptID:       200,
						SubPromptVersion:  "v1",
					},
					{
						ID:                2,
						SpaceID:           1,
						MainPromptID:      101,
						MainPromptVersion: "",
						SubPromptID:       200,
						SubPromptVersion:  "v1",
					},
					{
						ID:                3,
						SpaceID:           1,
						MainPromptID:      102,
						MainPromptVersion: "2.0.0",
						SubPromptID:       200,
						SubPromptVersion:  "v2",
					},
					{
						ID:                4,
						SpaceID:           1,
						MainPromptID:      102,
						MainPromptVersion: "2.0.1",
						SubPromptID:       200,
						SubPromptVersion:  "v2",
					},
				}
				mockRelationDAO := daomocks.NewMockIPromptRelationDAO(ctrl)
				mockRelationDAO.EXPECT().List(gomock.Any(), gomock.Any()).Return(relations, nil)

				mockBasicDAO := daomocks.NewMockIPromptBasicDAO(ctrl)
				mockBasicDAO.EXPECT().MGet(gomock.Any(), gomock.Any()).Return(map[int64]*model.PromptBasic{
					101: {
						ID:         101,
						SpaceID:    1,
						PromptKey:  "parent_a",
						PromptType: string(entity.PromptTypeNormal),
					},
					102: {
						ID:         102,
						SpaceID:    1,
						PromptKey:  "parent_b",
						PromptType: string(entity.PromptTypeSnippet),
					},
				}, nil)

				return fields{
					promptRelationDAO: mockRelationDAO,
					promptBasicDAO:    mockBasicDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListParentPromptParam{
					SubPromptID:       200,
					SubPromptVersions: []string{"v1", "v2"},
				},
			},
			check: func(t *testing.T, got map[string]*repo.PromptCommitVersions) {
				assert.Len(t, got, 2)
				v1, ok := got["v1"]
				assert.True(t, ok)
				assert.Equal(t, int64(101), v1.PromptID)
				assert.Equal(t, int64(1), v1.SpaceID)
				assert.Equal(t, "parent_a", v1.PromptKey)
				if assert.NotNil(t, v1.PromptBasic) {
					assert.Equal(t, entity.PromptTypeNormal, v1.PromptBasic.PromptType)
				}
				assert.Equal(t, []string{"1.0.0"}, v1.CommitVersions)

				v2, ok := got["v2"]
				assert.True(t, ok)
				assert.Equal(t, int64(102), v2.PromptID)
				assert.Equal(t, "parent_b", v2.PromptKey)
				if assert.NotNil(t, v2.PromptBasic) {
					assert.Equal(t, entity.PromptTypeSnippet, v2.PromptBasic.PromptType)
				}
				assert.ElementsMatch(t, []string{"2.0.0", "2.0.1"}, v2.CommitVersions)
			},
		},
	}

	for _, tt := range tests {
		ttt := tt
		t.Run(ttt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			fields := ttt.fieldsGetter(ctrl)
			repoImpl := &ManageRepoImpl{
				promptRelationDAO: fields.promptRelationDAO,
				promptBasicDAO:    fields.promptBasicDAO,
			}

			got, err := repoImpl.ListParentPrompt(ttt.args.ctx, ttt.args.param)
			if ttt.wantErr != nil {
				unittest.AssertErrorEqual(t, ttt.wantErr, err)
				return
			}
			if ttt.wantErrMsg != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), ttt.wantErrMsg)
				return
			}
			assert.NoError(t, err)
			if ttt.check != nil {
				ttt.check(t, got)
			}
		})
	}
}
