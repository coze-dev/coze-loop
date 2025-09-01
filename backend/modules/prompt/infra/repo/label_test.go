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
	mysqlmocks "github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/redis"
	redismocks "github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/redis/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/unittest"
)

func TestLabelRepoImpl_CreateLabel(t *testing.T) {
	type fields struct {
		db       db.Provider
		idgen    idgen.IIDGenerator
		labelDAO mysql.ILabelDAO
	}
	type args struct {
		ctx     context.Context
		labelDO *entity.PromptLabel
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      error
	}{
		{
			name: "nil labelDO",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{
					db:       dbmocks.NewMockProvider(ctrl),
					idgen:    idgenmocks.NewMockIIDGenerator(ctrl),
					labelDAO: mysqlmocks.NewMockILabelDAO(ctrl),
				}
			},
			args: args{
				ctx:     context.Background(),
				labelDO: nil,
			},
			wantErr: nil,
		},
		{
			name: "idgen error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(0), errorx.New("id generation failed"))

				return fields{
					db:       dbmocks.NewMockProvider(ctrl),
					idgen:    mockIDGen,
					labelDAO: mysqlmocks.NewMockILabelDAO(ctrl),
				}
			},
			args: args{
				ctx: context.Background(),
				labelDO: &entity.PromptLabel{
					SpaceID:   1,
					LabelKey:  "test-label",
					CreatedBy: "test-user",
				},
			},
			wantErr: errorx.New("id generation failed"),
		},
		{
			name: "labelDAO create error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(123), nil)

				mockLabelDAO := mysqlmocks.NewMockILabelDAO(ctrl)
				mockLabelDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errorx.New("create failed"))

				return fields{
					db:       dbmocks.NewMockProvider(ctrl),
					idgen:    mockIDGen,
					labelDAO: mockLabelDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				labelDO: &entity.PromptLabel{
					SpaceID:   1,
					LabelKey:  "test-label",
					CreatedBy: "test-user",
				},
			},
			wantErr: errorx.New("create failed"),
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(123), nil)

				mockLabelDAO := mysqlmocks.NewMockILabelDAO(ctrl)
				mockLabelDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

				return fields{
					db:       dbmocks.NewMockProvider(ctrl),
					idgen:    mockIDGen,
					labelDAO: mockLabelDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				labelDO: &entity.PromptLabel{
					SpaceID:   1,
					LabelKey:  "test-label",
					CreatedBy: "test-user",
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

			r := &LabelRepoImpl{
				db:       ttFields.db,
				idgen:    ttFields.idgen,
				labelDAO: ttFields.labelDAO,
			}

			err := r.CreateLabel(tt.args.ctx, tt.args.labelDO)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
		})
	}
}

func TestLabelRepoImpl_ListLabel(t *testing.T) {
	type fields struct {
		labelDAO mysql.ILabelDAO
	}
	type args struct {
		ctx   context.Context
		param repo.ListLabelParam
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         []*entity.PromptLabel
		wantNextPage *int64
		wantErr      error
	}{
		{
			name: "DAO error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLabelDAO := mysqlmocks.NewMockILabelDAO(ctrl)
				mockLabelDAO.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, errorx.New("dao error"))

				return fields{
					labelDAO: mockLabelDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListLabelParam{
					SpaceID:      1,
					LabelKeyLike: "test",
					PageSize:     10,
				},
			},
			want:         nil,
			wantNextPage: nil,
			wantErr:      errorx.New("dao error"),
		},
		{
			name: "no data",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLabelDAO := mysqlmocks.NewMockILabelDAO(ctrl)
				mockLabelDAO.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*model.PromptLabel{}, nil)

				return fields{
					labelDAO: mockLabelDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListLabelParam{
					SpaceID:  1,
					PageSize: 10,
				},
			},
			want:         []*entity.PromptLabel{},
			wantNextPage: nil,
			wantErr:      nil,
		},
		{
			name: "with next page",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				now := time.Now()
				labelPOs := []*model.PromptLabel{
					{
						ID:        100,
						SpaceID:   1,
						LabelKey:  "label1",
						CreatedBy: "user1",
						CreatedAt: now,
						UpdatedBy: "user1",
						UpdatedAt: now,
					},
					{
						ID:        101,
						SpaceID:   1,
						LabelKey:  "label2",
						CreatedBy: "user1",
						CreatedAt: now,
						UpdatedBy: "user1",
						UpdatedAt: now,
					},
					{
						ID:        102,
						SpaceID:   1,
						LabelKey:  "label3",
						CreatedBy: "user1",
						CreatedAt: now,
						UpdatedBy: "user1",
						UpdatedAt: now,
					},
				}

				mockLabelDAO := mysqlmocks.NewMockILabelDAO(ctrl)
				mockLabelDAO.EXPECT().List(gomock.Any(), gomock.Any()).Return(labelPOs, nil)

				return fields{
					labelDAO: mockLabelDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListLabelParam{
					SpaceID:  1,
					PageSize: 2,
				},
			},
			want: []*entity.PromptLabel{
				{
					ID:       100,
					SpaceID:  1,
					LabelKey: "label1",
				},
				{
					ID:       101,
					SpaceID:  1,
					LabelKey: "label2",
				},
			},
			wantNextPage: ptr.Of(int64(102)),
			wantErr:      nil,
		},
		{
			name: "no next page",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				now := time.Now()
				labelPOs := []*model.PromptLabel{
					{
						ID:        100,
						SpaceID:   1,
						LabelKey:  "label1",
						CreatedBy: "user1",
						CreatedAt: now,
						UpdatedBy: "user1",
						UpdatedAt: now,
					},
				}

				mockLabelDAO := mysqlmocks.NewMockILabelDAO(ctrl)
				mockLabelDAO.EXPECT().List(gomock.Any(), gomock.Any()).Return(labelPOs, nil)

				return fields{
					labelDAO: mockLabelDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListLabelParam{
					SpaceID:  1,
					PageSize: 2,
				},
			},
			want: []*entity.PromptLabel{
				{
					ID:       100,
					SpaceID:  1,
					LabelKey: "label1",
				},
			},
			wantNextPage: nil,
			wantErr:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)

			r := &LabelRepoImpl{
				labelDAO: ttFields.labelDAO,
			}

			got, nextPage, err := r.ListLabel(tt.args.ctx, tt.args.param)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, len(tt.want), len(got))
				for i, want := range tt.want {
					assert.Equal(t, want.ID, got[i].ID)
					assert.Equal(t, want.SpaceID, got[i].SpaceID)
					assert.Equal(t, want.LabelKey, got[i].LabelKey)
				}
				assert.Equal(t, tt.wantNextPage, nextPage)
			}
		})
	}
}

func TestLabelRepoImpl_BatchGetLabel(t *testing.T) {
	type fields struct {
		labelDAO mysql.ILabelDAO
	}
	type args struct {
		ctx       context.Context
		spaceID   int64
		labelKeys []string
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         []*entity.PromptLabel
		wantErr      error
	}{
		{
			name: "empty labelKeys",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{
					labelDAO: mysqlmocks.NewMockILabelDAO(ctrl),
				}
			},
			args: args{
				ctx:       context.Background(),
				spaceID:   1,
				labelKeys: []string{},
			},
			want:    nil,
			wantErr: nil,
		},
		{
			name: "DAO error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLabelDAO := mysqlmocks.NewMockILabelDAO(ctrl)
				mockLabelDAO.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errorx.New("dao error"))

				return fields{
					labelDAO: mockLabelDAO,
				}
			},
			args: args{
				ctx:       context.Background(),
				spaceID:   1,
				labelKeys: []string{"label1", "label2"},
			},
			want:    nil,
			wantErr: errorx.New("dao error"),
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				now := time.Now()
				labelPOs := []*model.PromptLabel{
					{
						ID:        100,
						SpaceID:   1,
						LabelKey:  "label1",
						CreatedBy: "user1",
						CreatedAt: now,
						UpdatedBy: "user1",
						UpdatedAt: now,
					},
					{
						ID:        101,
						SpaceID:   1,
						LabelKey:  "label2",
						CreatedBy: "user1",
						CreatedAt: now,
						UpdatedBy: "user1",
						UpdatedAt: now,
					},
				}

				mockLabelDAO := mysqlmocks.NewMockILabelDAO(ctrl)
				mockLabelDAO.EXPECT().BatchGet(gomock.Any(), int64(1), []string{"label1", "label2"}).Return(labelPOs, nil)

				return fields{
					labelDAO: mockLabelDAO,
				}
			},
			args: args{
				ctx:       context.Background(),
				spaceID:   1,
				labelKeys: []string{"label1", "label2"},
			},
			want: []*entity.PromptLabel{
				{
					ID:       100,
					SpaceID:  1,
					LabelKey: "label1",
				},
				{
					ID:       101,
					SpaceID:  1,
					LabelKey: "label2",
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

			r := &LabelRepoImpl{
				labelDAO: ttFields.labelDAO,
			}

			got, err := r.BatchGetLabel(tt.args.ctx, tt.args.spaceID, tt.args.labelKeys)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, len(tt.want), len(got))
				for i, want := range tt.want {
					assert.Equal(t, want.ID, got[i].ID)
					assert.Equal(t, want.SpaceID, got[i].SpaceID)
					assert.Equal(t, want.LabelKey, got[i].LabelKey)
				}
			}
		})
	}
}

func TestLabelRepoImpl_UpdateCommitLabels(t *testing.T) {
	type fields struct {
		db                    db.Provider
		idgen                 idgen.IIDGenerator
		commitLabelMappingDAO mysql.ICommitLabelMappingDAO
		promptBasicDAO        mysql.IPromptBasicDAO
		promptLabelVersionDAO redis.IPromptLabelVersionDAO
	}
	type args struct {
		ctx   context.Context
		param repo.UpdateCommitLabelsParam
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      error
	}{
		{
			name: "prompt basic get error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockTx := &gorm.DB{}
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(*gorm.DB) error, opts ...db.Option) error {
					return fn(mockTx)
				})

				mockPromptBasicDAO := mysqlmocks.NewMockIPromptBasicDAO(ctrl)
				mockPromptBasicDAO.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errorx.New("prompt not found"))

				mockCacheDAO := redismocks.NewMockIPromptLabelVersionDAO(ctrl)

				return fields{
					db:                    mockDB,
					promptBasicDAO:        mockPromptBasicDAO,
					promptLabelVersionDAO: mockCacheDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.UpdateCommitLabelsParam{
					SpaceID:       1,
					PromptID:      100,
					LabelKeys:     []string{"label1"},
					CommitVersion: "v1.0.0",
					UpdatedBy:     "test-user",
				},
			},
			wantErr: errorx.New("prompt not found"),
		},
		{
			name: "success create new mappings",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockTx := &gorm.DB{}
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(*gorm.DB) error, opts ...db.Option) error {
					return fn(mockTx)
				})

				mockPromptBasicDAO := mysqlmocks.NewMockIPromptBasicDAO(ctrl)
				mockPromptBasicDAO.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&model.PromptBasic{ID: 100}, nil)

				mockCommitLabelMappingDAO := mysqlmocks.NewMockICommitLabelMappingDAO(ctrl)
				mockCommitLabelMappingDAO.EXPECT().ListByPromptIDAndLabelKeys(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*model.PromptCommitLabelMapping{}, nil)
				mockCommitLabelMappingDAO.EXPECT().ListByPromptIDAndVersions(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*model.PromptCommitLabelMapping{}, nil)
				mockCommitLabelMappingDAO.EXPECT().BatchCreate(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).Return([]int64{201, 202}, nil)

				mockCacheDAO := redismocks.NewMockIPromptLabelVersionDAO(ctrl)
				mockCacheDAO.EXPECT().MDel(gomock.Any(), gomock.Any()).Return(nil)

				return fields{
					db:                    mockDB,
					idgen:                 mockIDGen,
					commitLabelMappingDAO: mockCommitLabelMappingDAO,
					promptBasicDAO:        mockPromptBasicDAO,
					promptLabelVersionDAO: mockCacheDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.UpdateCommitLabelsParam{
					SpaceID:       1,
					PromptID:      100,
					LabelKeys:     []string{"label1", "label2"},
					CommitVersion: "v1.0.0",
					UpdatedBy:     "test-user",
				},
			},
			wantErr: nil,
		},
		{
			name: "success update existing mappings",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockTx := &gorm.DB{}
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(*gorm.DB) error, opts ...db.Option) error {
					return fn(mockTx)
				})

				mockPromptBasicDAO := mysqlmocks.NewMockIPromptBasicDAO(ctrl)
				mockPromptBasicDAO.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&model.PromptBasic{ID: 100}, nil)

				existingMapping := &model.PromptCommitLabelMapping{
					ID:            300,
					SpaceID:       1,
					PromptID:      100,
					LabelKey:      "label1",
					PromptVersion: "v0.9.0",
					CreatedBy:     "old-user",
					UpdatedBy:     "old-user",
				}

				mockCommitLabelMappingDAO := mysqlmocks.NewMockICommitLabelMappingDAO(ctrl)
				mockCommitLabelMappingDAO.EXPECT().ListByPromptIDAndLabelKeys(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*model.PromptCommitLabelMapping{existingMapping}, nil)
				mockCommitLabelMappingDAO.EXPECT().ListByPromptIDAndVersions(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*model.PromptCommitLabelMapping{}, nil)
				mockCommitLabelMappingDAO.EXPECT().BatchUpdate(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).Return([]int64{201}, nil)

				mockCacheDAO := redismocks.NewMockIPromptLabelVersionDAO(ctrl)
				mockCacheDAO.EXPECT().MDel(gomock.Any(), gomock.Any()).Return(nil)

				return fields{
					db:                    mockDB,
					idgen:                 mockIDGen,
					commitLabelMappingDAO: mockCommitLabelMappingDAO,
					promptBasicDAO:        mockPromptBasicDAO,
					promptLabelVersionDAO: mockCacheDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.UpdateCommitLabelsParam{
					SpaceID:       1,
					PromptID:      100,
					LabelKeys:     []string{"label1"},
					CommitVersion: "v1.0.0",
					UpdatedBy:     "test-user",
				},
			},
			wantErr: nil,
		},
		{
			name: "success delete old mappings",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockTx := &gorm.DB{}
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(*gorm.DB) error, opts ...db.Option) error {
					return fn(mockTx)
				})

				mockPromptBasicDAO := mysqlmocks.NewMockIPromptBasicDAO(ctrl)
				mockPromptBasicDAO.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&model.PromptBasic{ID: 100}, nil)

				oldMapping := &model.PromptCommitLabelMapping{
					ID:            400,
					SpaceID:       1,
					PromptID:      100,
					LabelKey:      "old-label",
					PromptVersion: "v1.0.0",
					CreatedBy:     "old-user",
					UpdatedBy:     "old-user",
				}

				mockCommitLabelMappingDAO := mysqlmocks.NewMockICommitLabelMappingDAO(ctrl)
				mockCommitLabelMappingDAO.EXPECT().ListByPromptIDAndLabelKeys(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*model.PromptCommitLabelMapping{}, nil)
				mockCommitLabelMappingDAO.EXPECT().ListByPromptIDAndVersions(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*model.PromptCommitLabelMapping{oldMapping}, nil)
				mockCommitLabelMappingDAO.EXPECT().BatchCreate(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockCommitLabelMappingDAO.EXPECT().BatchDelete(gomock.Any(), []int64{400}, gomock.Any()).Return(nil)

				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).Return([]int64{401}, nil)

				mockCacheDAO := redismocks.NewMockIPromptLabelVersionDAO(ctrl)
				mockCacheDAO.EXPECT().MDel(gomock.Any(), gomock.Any()).Return(nil)

				return fields{
					db:                    mockDB,
					idgen:                 mockIDGen,
					commitLabelMappingDAO: mockCommitLabelMappingDAO,
					promptBasicDAO:        mockPromptBasicDAO,
					promptLabelVersionDAO: mockCacheDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.UpdateCommitLabelsParam{
					SpaceID:       1,
					PromptID:      100,
					LabelKeys:     []string{"new-label"},
					CommitVersion: "v1.0.0",
					UpdatedBy:     "test-user",
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

			r := &LabelRepoImpl{
				db:                    ttFields.db,
				idgen:                 ttFields.idgen,
				commitLabelMappingDAO: ttFields.commitLabelMappingDAO,
				promptBasicDAO:        ttFields.promptBasicDAO,
				promptLabelVersionDAO: ttFields.promptLabelVersionDAO,
			}

			err := r.UpdateCommitLabels(tt.args.ctx, tt.args.param)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
		})
	}
}

func TestLabelRepoImpl_GetCommitLabels(t *testing.T) {
	type fields struct {
		commitLabelMappingDAO mysql.ICommitLabelMappingDAO
	}
	type args struct {
		ctx            context.Context
		promptID       int64
		commitVersions []string
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         map[string][]*entity.PromptLabel
		wantErr      error
	}{
		{
			name: "DAO error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockCommitLabelMappingDAO := mysqlmocks.NewMockICommitLabelMappingDAO(ctrl)
				mockCommitLabelMappingDAO.EXPECT().ListByPromptIDAndVersions(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errorx.New("dao error"))

				return fields{
					commitLabelMappingDAO: mockCommitLabelMappingDAO,
				}
			},
			args: args{
				ctx:            context.Background(),
				promptID:       100,
				commitVersions: []string{"v1.0.0", "v1.1.0"},
			},
			want:    nil,
			wantErr: errorx.New("dao error"),
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mappingPOs := []*model.PromptCommitLabelMapping{
					{
						ID:            300,
						SpaceID:       1,
						PromptID:      100,
						LabelKey:      "label1",
						PromptVersion: "v1.0.0",
					},
					{
						ID:            301,
						SpaceID:       1,
						PromptID:      100,
						LabelKey:      "label2",
						PromptVersion: "v1.0.0",
					},
					{
						ID:            302,
						SpaceID:       1,
						PromptID:      100,
						LabelKey:      "label1",
						PromptVersion: "v1.1.0",
					},
				}

				mockCommitLabelMappingDAO := mysqlmocks.NewMockICommitLabelMappingDAO(ctrl)
				mockCommitLabelMappingDAO.EXPECT().ListByPromptIDAndVersions(gomock.Any(), int64(100), []string{"v1.0.0", "v1.1.0"}).Return(mappingPOs, nil)

				return fields{
					commitLabelMappingDAO: mockCommitLabelMappingDAO,
				}
			},
			args: args{
				ctx:            context.Background(),
				promptID:       100,
				commitVersions: []string{"v1.0.0", "v1.1.0"},
			},
			want: map[string][]*entity.PromptLabel{
				"v1.0.0": {
					{LabelKey: "label1"},
					{LabelKey: "label2"},
				},
				"v1.1.0": {
					{LabelKey: "label1"},
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

			r := &LabelRepoImpl{
				commitLabelMappingDAO: ttFields.commitLabelMappingDAO,
			}

			got, err := r.GetCommitLabels(tt.args.ctx, tt.args.promptID, tt.args.commitVersions)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, len(tt.want), len(got))
				for version, expectedLabels := range tt.want {
					actualLabels := got[version]
					assert.Equal(t, len(expectedLabels), len(actualLabels))
					for i, expectedLabel := range expectedLabels {
						assert.Equal(t, expectedLabel.LabelKey, actualLabels[i].LabelKey)
					}
				}
			}
		})
	}
}

func TestLabelRepoImpl_BatchGetPromptVersionByLabel(t *testing.T) {
	type fields struct {
		commitLabelMappingDAO          mysql.ICommitLabelMappingDAO
		promptLabelVersionDAO          redis.IPromptLabelVersionDAO
		promptLabelVersionCacheMetrics *metricsinfra.PromptLabelVersionCacheMetrics
	}
	type args struct {
		ctx     context.Context
		queries []repo.PromptLabelQuery
		opts    []repo.GetLabelMappingOptionFunc
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         map[repo.PromptLabelQuery]string
		wantErr      error
	}{
		{
			name: "cache disabled, DAO error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockCommitLabelMappingDAO := mysqlmocks.NewMockICommitLabelMappingDAO(ctrl)
				mockCommitLabelMappingDAO.EXPECT().MGetPromptVersionByLabelQuery(gomock.Any(), gomock.Any()).Return(nil, errorx.New("dao error"))

				return fields{
					commitLabelMappingDAO:          mockCommitLabelMappingDAO,
					promptLabelVersionCacheMetrics: (*metricsinfra.PromptLabelVersionCacheMetrics)(nil),
				}
			},
			args: args{
				ctx: context.Background(),
				queries: []repo.PromptLabelQuery{
					{PromptID: 100, LabelKey: "label1"},
				},
				opts: []repo.GetLabelMappingOptionFunc{},
			},
			want:    nil,
			wantErr: errorx.New("dao error"),
		},
		{
			name: "cache disabled, success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mappingPOs := []*model.PromptCommitLabelMapping{
					{
						PromptID:      100,
						LabelKey:      "label1",
						PromptVersion: "v1.0.0",
					},
					{
						PromptID:      101,
						LabelKey:      "label2",
						PromptVersion: "v2.0.0",
					},
				}

				mockCommitLabelMappingDAO := mysqlmocks.NewMockICommitLabelMappingDAO(ctrl)
				mockCommitLabelMappingDAO.EXPECT().MGetPromptVersionByLabelQuery(gomock.Any(), gomock.Any()).Return(mappingPOs, nil)

				return fields{
					commitLabelMappingDAO:          mockCommitLabelMappingDAO,
					promptLabelVersionCacheMetrics: (*metricsinfra.PromptLabelVersionCacheMetrics)(nil),
				}
			},
			args: args{
				ctx: context.Background(),
				queries: []repo.PromptLabelQuery{
					{PromptID: 100, LabelKey: "label1"},
					{PromptID: 101, LabelKey: "label2"},
				},
				opts: []repo.GetLabelMappingOptionFunc{},
			},
			want: map[repo.PromptLabelQuery]string{
				{PromptID: 100, LabelKey: "label1"}: "v1.0.0",
				{PromptID: 101, LabelKey: "label2"}: "v2.0.0",
			},
			wantErr: nil,
		},
		{
			name: "cache enabled, cache hit",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockCacheDAO := redismocks.NewMockIPromptLabelVersionDAO(ctrl)
				cacheResult := map[redis.PromptLabelVersionQuery]string{
					{PromptID: 100, LabelKey: "label1"}: "v1.0.0",
					{PromptID: 101, LabelKey: "label2"}: "v2.0.0",
				}
				mockCacheDAO.EXPECT().MGet(gomock.Any(), gomock.Any()).Return(cacheResult, nil)

				return fields{
					promptLabelVersionDAO:          mockCacheDAO,
					promptLabelVersionCacheMetrics: (*metricsinfra.PromptLabelVersionCacheMetrics)(nil),
				}
			},
			args: args{
				ctx: context.Background(),
				queries: []repo.PromptLabelQuery{
					{PromptID: 100, LabelKey: "label1"},
					{PromptID: 101, LabelKey: "label2"},
				},
				opts: []repo.GetLabelMappingOptionFunc{
					repo.WithLabelMappingCacheEnable(),
				},
			},
			want: map[repo.PromptLabelQuery]string{
				{PromptID: 100, LabelKey: "label1"}: "v1.0.0",
				{PromptID: 101, LabelKey: "label2"}: "v2.0.0",
			},
			wantErr: nil,
		},
		{
			name: "cache enabled, cache miss, fallback to DB",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockCacheDAO := redismocks.NewMockIPromptLabelVersionDAO(ctrl)
				// Cache miss - return empty map
				mockCacheDAO.EXPECT().MGet(gomock.Any(), gomock.Any()).Return(map[redis.PromptLabelVersionQuery]string{}, nil)
				// Set cache after DB query
				mockCacheDAO.EXPECT().MSet(gomock.Any(), gomock.Any()).Return(nil)

				mappingPOs := []*model.PromptCommitLabelMapping{
					{
						PromptID:      100,
						LabelKey:      "label1",
						PromptVersion: "v1.0.0",
					},
				}

				mockCommitLabelMappingDAO := mysqlmocks.NewMockICommitLabelMappingDAO(ctrl)
				mockCommitLabelMappingDAO.EXPECT().MGetPromptVersionByLabelQuery(gomock.Any(), gomock.Any()).Return(mappingPOs, nil)

				return fields{
					commitLabelMappingDAO:          mockCommitLabelMappingDAO,
					promptLabelVersionDAO:          mockCacheDAO,
					promptLabelVersionCacheMetrics: (*metricsinfra.PromptLabelVersionCacheMetrics)(nil),
				}
			},
			args: args{
				ctx: context.Background(),
				queries: []repo.PromptLabelQuery{
					{PromptID: 100, LabelKey: "label1"},
				},
				opts: []repo.GetLabelMappingOptionFunc{
					repo.WithLabelMappingCacheEnable(),
				},
			},
			want: map[repo.PromptLabelQuery]string{
				{PromptID: 100, LabelKey: "label1"}: "v1.0.0",
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

			r := &LabelRepoImpl{
				commitLabelMappingDAO:          ttFields.commitLabelMappingDAO,
				promptLabelVersionDAO:          ttFields.promptLabelVersionDAO,
				promptLabelVersionCacheMetrics: ttFields.promptLabelVersionCacheMetrics,
			}

			got, err := r.BatchGetPromptVersionByLabel(tt.args.ctx, tt.args.queries, tt.args.opts...)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
