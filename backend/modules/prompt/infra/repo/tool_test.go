// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	dbmocks "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity/toolmgmt"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql"
	mysqlmodel "github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/model"
	daomocks "github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/mocks"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/unittest"
)

type toolRepoFields struct {
	db            db.Provider
	idgen         idgen.IIDGenerator
	toolBasicDAO  mysql.IToolBasicDAO
	toolCommitDAO mysql.IToolCommitDAO
}

func TestNewToolRepo(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := dbmocks.NewMockProvider(ctrl)
	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
	mockCommitDAO := daomocks.NewMockIToolCommitDAO(ctrl)

	r := NewToolRepo(mockDB, mockIDGen, mockBasicDAO, mockCommitDAO)
	assert.NotNil(t, r)
}

func TestToolRepoImpl_CreateTool(t *testing.T) {
	t.Parallel()
	type args struct {
		ctx  context.Context
		tool *toolmgmt.Tool
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) toolRepoFields
		args         args
		wantToolID   int64
		wantErr      error
	}{
		{
			name: "nil tool",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				return toolRepoFields{}
			},
			args: args{
				ctx:  context.Background(),
				tool: nil,
			},
			wantToolID: 0,
			wantErr:    errorx.New("tool or tool.ToolBasic is empty"),
		},
		{
			name: "nil ToolBasic",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				return toolRepoFields{}
			},
			args: args{
				ctx:  context.Background(),
				tool: &toolmgmt.Tool{ID: 1, SpaceID: 100},
			},
			wantToolID: 0,
			wantErr:    errorx.New("tool or tool.ToolBasic is empty"),
		},
		{
			name: "idgen error",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(0), errorx.New("idgen error"))
				return toolRepoFields{
					idgen: mockIDGen,
				}
			},
			args: args{
				ctx: context.Background(),
				tool: &toolmgmt.Tool{
					SpaceID: 100,
					ToolBasic: &toolmgmt.ToolBasic{
						Name:      "test_tool",
						CreatedBy: "user1",
					},
				},
			},
			wantToolID: 0,
			wantErr:    errorx.New("idgen error"),
		},
		{
			name: "toolBasicDAO.Create error",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).Return(errorx.New("create basic error"))

				return toolRepoFields{
					db:           mockDB,
					idgen:        mockIDGen,
					toolBasicDAO: mockBasicDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				tool: &toolmgmt.Tool{
					SpaceID: 100,
					ToolBasic: &toolmgmt.ToolBasic{
						Name:      "test_tool",
						CreatedBy: "user1",
					},
				},
			},
			wantToolID: 0,
			wantErr:    errorx.New("create basic error"),
		},
		{
			name: "toolCommitDAO.UpsertDraft error",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockCommitDAO := daomocks.NewMockIToolCommitDAO(ctrl)
				mockCommitDAO.EXPECT().UpsertDraft(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errorx.New("upsert draft error"))

				return toolRepoFields{
					db:            mockDB,
					idgen:         mockIDGen,
					toolBasicDAO:  mockBasicDAO,
					toolCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				tool: &toolmgmt.Tool{
					SpaceID: 100,
					ToolBasic: &toolmgmt.ToolBasic{
						Name:      "test_tool",
						CreatedBy: "user1",
					},
				},
			},
			wantToolID: 0,
			wantErr:    errorx.New("upsert draft error"),
		},
		{
			name: "success without draft content",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockCommitDAO := daomocks.NewMockIToolCommitDAO(ctrl)
				mockCommitDAO.EXPECT().UpsertDraft(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, po *mysqlmodel.ToolCommit, timeNow time.Time, opts ...db.Option) error {
					assert.Equal(t, int64(100), po.SpaceID)
					assert.Equal(t, int64(1001), po.ToolID)
					assert.Equal(t, toolmgmt.PublicDraftVersion, po.Version)
					assert.Equal(t, "", po.BaseVersion)
					assert.Equal(t, "user1", po.CommittedBy)
					assert.Equal(t, lo.ToPtr(""), po.Content)
					return nil
				})

				return toolRepoFields{
					db:            mockDB,
					idgen:         mockIDGen,
					toolBasicDAO:  mockBasicDAO,
					toolCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				tool: &toolmgmt.Tool{
					SpaceID: 100,
					ToolBasic: &toolmgmt.ToolBasic{
						Name:      "test_tool",
						CreatedBy: "user1",
					},
				},
			},
			wantToolID: 1001,
			wantErr:    nil,
		},
		{
			name: "success with draft content",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(2001), nil)

				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockCommitDAO := daomocks.NewMockIToolCommitDAO(ctrl)
				mockCommitDAO.EXPECT().UpsertDraft(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, po *mysqlmodel.ToolCommit, timeNow time.Time, opts ...db.Option) error {
					assert.Equal(t, lo.ToPtr("my tool content"), po.Content)
					return nil
				})

				return toolRepoFields{
					db:            mockDB,
					idgen:         mockIDGen,
					toolBasicDAO:  mockBasicDAO,
					toolCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				tool: &toolmgmt.Tool{
					SpaceID: 100,
					ToolBasic: &toolmgmt.ToolBasic{
						Name:      "test_tool",
						CreatedBy: "user1",
					},
					ToolCommit: &toolmgmt.ToolCommit{
						ToolDetail: &toolmgmt.ToolDetail{
							Content: "my tool content",
						},
					},
				},
			},
			wantToolID: 2001,
			wantErr:    nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)

			d := &ToolRepoImpl{db: f.db, idgen: f.idgen, toolBasicDAO: f.toolBasicDAO, toolCommitDAO: f.toolCommitDAO}

			got, err := d.CreateTool(tt.args.ctx, tt.args.tool)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.wantToolID, got)
			}
		})
	}
}

func TestToolRepoImpl_DeleteTool(t *testing.T) {
	t.Parallel()
	type args struct {
		ctx    context.Context
		toolID int64
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) toolRepoFields
		args         args
		wantErr      error
	}{
		{
			name: "invalid toolID",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				return toolRepoFields{}
			},
			args: args{
				ctx:    context.Background(),
				toolID: 0,
			},
			wantErr: errorx.New("toolID is invalid, toolID = 0"),
		},
		{
			name: "toolBasicDAO.Get error",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1)).Return(nil, errorx.New("get error"))
				return toolRepoFields{
					toolBasicDAO: mockBasicDAO,
				}
			},
			args: args{
				ctx:    context.Background(),
				toolID: 1,
			},
			wantErr: errorx.New("get error"),
		},
		{
			name: "tool not found",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1)).Return(nil, nil)
				return toolRepoFields{
					toolBasicDAO: mockBasicDAO,
				}
			},
			args: args{
				ctx:    context.Background(),
				toolID: 1,
			},
			wantErr: errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg("tool is not found, tool id = 1")),
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1)).Return(&mysqlmodel.ToolBasic{
					ID:      1,
					SpaceID: 100,
				}, nil)
				mockBasicDAO.EXPECT().Delete(gomock.Any(), int64(1), int64(100)).Return(nil)
				return toolRepoFields{
					toolBasicDAO: mockBasicDAO,
				}
			},
			args: args{
				ctx:    context.Background(),
				toolID: 1,
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)

			d := &ToolRepoImpl{db: f.db, idgen: f.idgen, toolBasicDAO: f.toolBasicDAO, toolCommitDAO: f.toolCommitDAO}

			err := d.DeleteTool(tt.args.ctx, tt.args.toolID)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
		})
	}
}

func TestToolRepoImpl_GetTool(t *testing.T) {
	t.Parallel()
	type args struct {
		ctx   context.Context
		param repo.GetToolParam
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) toolRepoFields
		args         args
		want         *toolmgmt.Tool
		wantErr      error
	}{
		{
			name: "invalid toolID",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				return toolRepoFields{}
			},
			args: args{
				ctx:   context.Background(),
				param: repo.GetToolParam{ToolID: 0},
			},
			want:    nil,
			wantErr: errorx.New(`param.ToolID is invalid, param = {"ToolID":0,"WithCommit":false,"CommitVersion":"","WithDraft":false}`),
		},
		{
			name: "WithCommit but empty version",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				return toolRepoFields{}
			},
			args: args{
				ctx: context.Background(),
				param: repo.GetToolParam{
					ToolID:     1,
					WithCommit: true,
				},
			},
			want:    nil,
			wantErr: errorx.New(`Get with commit, but param.CommitVersion is empty, param = {"ToolID":1,"WithCommit":true,"CommitVersion":"","WithDraft":false}`),
		},
		{
			name: "toolBasicDAO.Get error",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(nil, errorx.New("get basic error"))
				return toolRepoFields{
					db:           mockDB,
					toolBasicDAO: mockBasicDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.GetToolParam{
					ToolID: 1,
				},
			},
			want:    nil,
			wantErr: errorx.New("get basic error"),
		},
		{
			name: "tool not found",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(nil, nil)
				return toolRepoFields{
					db:           mockDB,
					toolBasicDAO: mockBasicDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.GetToolParam{
					ToolID: 1,
				},
			},
			want:    nil,
			wantErr: errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg("tool id = 1")),
		},
		{
			name: "WithCommit success",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&mysqlmodel.ToolBasic{
					ID:      1,
					SpaceID: 100,
					Name:    "test_tool",
				}, nil)
				mockCommitDAO := daomocks.NewMockIToolCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Get(gomock.Any(), int64(1), "1.0.0", gomock.Any()).Return(&mysqlmodel.ToolCommit{
					ToolID:      1,
					Version:     "1.0.0",
					BaseVersion: "0.9.0",
					Content:     ptr.Of("content_v1"),
					CommittedBy: "user1",
					Description: ptr.Of("first release"),
					CreatedAt:   time.Unix(1000, 0),
				}, nil)
				return toolRepoFields{
					db:            mockDB,
					toolBasicDAO:  mockBasicDAO,
					toolCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.GetToolParam{
					ToolID:        1,
					WithCommit:    true,
					CommitVersion: "1.0.0",
				},
			},
			want: &toolmgmt.Tool{
				ID:      1,
				SpaceID: 100,
				ToolBasic: &toolmgmt.ToolBasic{
					Name: "test_tool",
				},
				ToolCommit: &toolmgmt.ToolCommit{
					CommitInfo: &toolmgmt.CommitInfo{
						Version:     "1.0.0",
						BaseVersion: "0.9.0",
						Description: "first release",
						CommittedBy: "user1",
						CommittedAt: time.Unix(1000, 0),
					},
					ToolDetail: &toolmgmt.ToolDetail{
						Content: "content_v1",
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "WithCommit commit not found",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&mysqlmodel.ToolBasic{
					ID:      1,
					SpaceID: 100,
				}, nil)
				mockCommitDAO := daomocks.NewMockIToolCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Get(gomock.Any(), int64(1), "2.0.0", gomock.Any()).Return(nil, nil)
				return toolRepoFields{
					db:            mockDB,
					toolBasicDAO:  mockBasicDAO,
					toolCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.GetToolParam{
					ToolID:        1,
					WithCommit:    true,
					CommitVersion: "2.0.0",
				},
			},
			want:    nil,
			wantErr: errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg("tool commit is not found, tool id = 1, version = 2.0.0")),
		},
		{
			name: "WithDraft success",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&mysqlmodel.ToolBasic{
					ID:      1,
					SpaceID: 100,
					Name:    "test_tool",
				}, nil)
				mockCommitDAO := daomocks.NewMockIToolCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Get(gomock.Any(), int64(1), toolmgmt.PublicDraftVersion, gomock.Any()).Return(&mysqlmodel.ToolCommit{
					ToolID:  1,
					Version: toolmgmt.PublicDraftVersion,
					Content: ptr.Of("draft content"),
				}, nil)
				return toolRepoFields{
					db:            mockDB,
					toolBasicDAO:  mockBasicDAO,
					toolCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.GetToolParam{
					ToolID:    1,
					WithDraft: true,
				},
			},
			want: &toolmgmt.Tool{
				ID:      1,
				SpaceID: 100,
				ToolBasic: &toolmgmt.ToolBasic{
					Name: "test_tool",
				},
				ToolCommit: &toolmgmt.ToolCommit{
					CommitInfo: &toolmgmt.CommitInfo{
						Version: toolmgmt.PublicDraftVersion,
					},
					ToolDetail: &toolmgmt.ToolDetail{
						Content: "draft content",
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "basic only",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&mysqlmodel.ToolBasic{
					ID:      1,
					SpaceID: 100,
					Name:    "test_tool",
				}, nil)
				return toolRepoFields{
					db:           mockDB,
					toolBasicDAO: mockBasicDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.GetToolParam{
					ToolID: 1,
				},
			},
			want: &toolmgmt.Tool{
				ID:      1,
				SpaceID: 100,
				ToolBasic: &toolmgmt.ToolBasic{
					Name: "test_tool",
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)

			d := &ToolRepoImpl{db: f.db, idgen: f.idgen, toolBasicDAO: f.toolBasicDAO, toolCommitDAO: f.toolCommitDAO}

			got, err := d.GetTool(tt.args.ctx, tt.args.param)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestToolRepoImpl_ListTool(t *testing.T) {
	t.Parallel()
	type args struct {
		ctx   context.Context
		param repo.ListToolParam
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) toolRepoFields
		args         args
		want         *repo.ListToolResult
		wantErr      error
	}{
		{
			name: "invalid params SpaceID=0",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				return toolRepoFields{}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListToolParam{
					SpaceID:  0,
					PageNum:  1,
					PageSize: 10,
				},
			},
			want:    nil,
			wantErr: errorx.New(`param is invalid, param = {"SpaceID":0,"KeyWord":"","CreatedBys":null,"CommittedOnly":false,"PageNum":1,"PageSize":10,"OrderBy":0,"Asc":false}`),
		},
		{
			name: "toolBasicDAO.List error",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().List(gomock.Any(), mysql.ListToolBasicParam{
					SpaceID: 100,
					Offset:  0,
					Limit:   10,
				}).Return(nil, int64(0), errorx.New("list error"))
				return toolRepoFields{
					toolBasicDAO: mockBasicDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListToolParam{
					SpaceID:  100,
					PageNum:  1,
					PageSize: 10,
				},
			},
			want:    nil,
			wantErr: errorx.New("list error"),
		},
		{
			name: "success empty",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*mysqlmodel.ToolBasic{}, int64(0), nil)
				return toolRepoFields{
					toolBasicDAO: mockBasicDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListToolParam{
					SpaceID:  100,
					PageNum:  1,
					PageSize: 10,
				},
			},
			want: &repo.ListToolResult{
				Total: 0,
				Tools: []*toolmgmt.Tool{},
			},
			wantErr: nil,
		},
		{
			name: "success with results",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*mysqlmodel.ToolBasic{
					{
						ID:      1,
						SpaceID: 100,
						Name:    "tool1",
					},
					{
						ID:      2,
						SpaceID: 100,
						Name:    "tool2",
					},
				}, int64(2), nil)
				return toolRepoFields{
					toolBasicDAO: mockBasicDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListToolParam{
					SpaceID:  100,
					PageNum:  1,
					PageSize: 10,
				},
			},
			want: &repo.ListToolResult{
				Total: 2,
				Tools: []*toolmgmt.Tool{
					{
						ID:      1,
						SpaceID: 100,
						ToolBasic: &toolmgmt.ToolBasic{
							Name: "tool1",
						},
					},
					{
						ID:      2,
						SpaceID: 100,
						ToolBasic: &toolmgmt.ToolBasic{
							Name: "tool2",
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "nil PO filtered",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*mysqlmodel.ToolBasic{
					{
						ID:      1,
						SpaceID: 100,
						Name:    "tool1",
					},
					nil,
					{
						ID:      3,
						SpaceID: 100,
						Name:    "tool3",
					},
				}, int64(3), nil)
				return toolRepoFields{
					toolBasicDAO: mockBasicDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListToolParam{
					SpaceID:  100,
					PageNum:  1,
					PageSize: 10,
				},
			},
			want: &repo.ListToolResult{
				Total: 3,
				Tools: []*toolmgmt.Tool{
					{
						ID:      1,
						SpaceID: 100,
						ToolBasic: &toolmgmt.ToolBasic{
							Name: "tool1",
						},
					},
					{
						ID:      3,
						SpaceID: 100,
						ToolBasic: &toolmgmt.ToolBasic{
							Name: "tool3",
						},
					},
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)

			d := &ToolRepoImpl{db: f.db, idgen: f.idgen, toolBasicDAO: f.toolBasicDAO, toolCommitDAO: f.toolCommitDAO}

			got, err := d.ListTool(tt.args.ctx, tt.args.param)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestToolRepoImpl_SaveToolDetail(t *testing.T) {
	t.Parallel()
	type args struct {
		ctx   context.Context
		param repo.SaveToolDetailParam
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) toolRepoFields
		args         args
		wantErr      error
	}{
		{
			name: "invalid toolID",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				return toolRepoFields{}
			},
			args: args{
				ctx:   context.Background(),
				param: repo.SaveToolDetailParam{ToolID: 0},
			},
			wantErr: errorx.New(`param.ToolID is invalid, param = {"ToolID":0,"BaseVersion":"","Content":"","UpdatedBy":""}`),
		},
		{
			name: "toolBasicDAO.Get error",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(nil, errorx.New("get error"))
				return toolRepoFields{
					db:           mockDB,
					toolBasicDAO: mockBasicDAO,
				}
			},
			args: args{
				ctx:   context.Background(),
				param: repo.SaveToolDetailParam{ToolID: 1},
			},
			wantErr: errorx.New("get error"),
		},
		{
			name: "tool not found",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(nil, nil)
				return toolRepoFields{
					db:           mockDB,
					toolBasicDAO: mockBasicDAO,
				}
			},
			args: args{
				ctx:   context.Background(),
				param: repo.SaveToolDetailParam{ToolID: 1},
			},
			wantErr: errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg("tool id = 1")),
		},
		{
			name: "toolCommitDAO.UpsertDraft error",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&mysqlmodel.ToolBasic{
					ID:      1,
					SpaceID: 100,
				}, nil)
				mockCommitDAO := daomocks.NewMockIToolCommitDAO(ctrl)
				mockCommitDAO.EXPECT().UpsertDraft(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errorx.New("upsert error"))
				return toolRepoFields{
					db:            mockDB,
					toolBasicDAO:  mockBasicDAO,
					toolCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.SaveToolDetailParam{
					ToolID:    1,
					Content:   "new content",
					UpdatedBy: "user1",
				},
			},
			wantErr: errorx.New("upsert error"),
		},
		{
			name: "toolBasicDAO.Update error",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&mysqlmodel.ToolBasic{
					ID:      1,
					SpaceID: 100,
				}, nil)
				mockCommitDAO := daomocks.NewMockIToolCommitDAO(ctrl)
				mockCommitDAO.EXPECT().UpsertDraft(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockBasicDAO.EXPECT().Update(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(errorx.New("update error"))
				return toolRepoFields{
					db:            mockDB,
					toolBasicDAO:  mockBasicDAO,
					toolCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.SaveToolDetailParam{
					ToolID:    1,
					Content:   "new content",
					UpdatedBy: "user1",
				},
			},
			wantErr: errorx.New("update error"),
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&mysqlmodel.ToolBasic{
					ID:      1,
					SpaceID: 100,
				}, nil)
				mockCommitDAO := daomocks.NewMockIToolCommitDAO(ctrl)
				mockCommitDAO.EXPECT().UpsertDraft(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockBasicDAO.EXPECT().Update(gomock.Any(), int64(1), map[string]interface{}{
					"updated_by": "user1",
				}, gomock.Any()).Return(nil)
				return toolRepoFields{
					db:            mockDB,
					toolBasicDAO:  mockBasicDAO,
					toolCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.SaveToolDetailParam{
					ToolID:      1,
					BaseVersion: "0.9.0",
					Content:     "new content",
					UpdatedBy:   "user1",
				},
			},
			wantErr: nil,
		},
		{
			name: "success with empty UpdatedBy skips Update",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&mysqlmodel.ToolBasic{
					ID:      1,
					SpaceID: 100,
				}, nil)
				mockCommitDAO := daomocks.NewMockIToolCommitDAO(ctrl)
				mockCommitDAO.EXPECT().UpsertDraft(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				return toolRepoFields{
					db:            mockDB,
					toolBasicDAO:  mockBasicDAO,
					toolCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.SaveToolDetailParam{
					ToolID:    1,
					Content:   "new content",
					UpdatedBy: "",
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)

			d := &ToolRepoImpl{db: f.db, idgen: f.idgen, toolBasicDAO: f.toolBasicDAO, toolCommitDAO: f.toolCommitDAO}

			err := d.SaveToolDetail(tt.args.ctx, tt.args.param)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
		})
	}
}

func TestToolRepoImpl_CommitToolDraft(t *testing.T) {
	t.Parallel()
	type args struct {
		ctx   context.Context
		param repo.CommitToolDraftParam
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) toolRepoFields
		args         args
		wantErr      error
	}{
		{
			name: "invalid params toolID=0",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				return toolRepoFields{}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitToolDraftParam{
					ToolID:        0,
					CommitVersion: "1.0.0",
				},
			},
			wantErr: errorx.New(`param is invalid, param = {"ToolID":0,"CommitVersion":"1.0.0","CommitDescription":"","BaseVersion":"","CommittedBy":""}`),
		},
		{
			name: "invalid params empty version",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				return toolRepoFields{}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitToolDraftParam{
					ToolID:        1,
					CommitVersion: "",
				},
			},
			wantErr: errorx.New(`param is invalid, param = {"ToolID":1,"CommitVersion":"","CommitDescription":"","BaseVersion":"","CommittedBy":""}`),
		},
		{
			name: "version equals PublicDraftVersion",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				return toolRepoFields{}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitToolDraftParam{
					ToolID:        1,
					CommitVersion: toolmgmt.PublicDraftVersion,
				},
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("commit_version is invalid")),
		},
		{
			name: "toolBasicDAO.Get error",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(nil, errorx.New("get error"))
				return toolRepoFields{
					db:           mockDB,
					toolBasicDAO: mockBasicDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitToolDraftParam{
					ToolID:        1,
					CommitVersion: "1.0.0",
				},
			},
			wantErr: errorx.New("get error"),
		},
		{
			name: "tool not found",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(nil, nil)
				return toolRepoFields{
					db:           mockDB,
					toolBasicDAO: mockBasicDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitToolDraftParam{
					ToolID:        1,
					CommitVersion: "1.0.0",
				},
			},
			wantErr: errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg("tool id = 1")),
		},
		{
			name: "draft not found",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&mysqlmodel.ToolBasic{
					ID:      1,
					SpaceID: 100,
				}, nil)
				mockCommitDAO := daomocks.NewMockIToolCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Get(gomock.Any(), int64(1), toolmgmt.PublicDraftVersion, gomock.Any()).Return(nil, nil)
				return toolRepoFields{
					db:            mockDB,
					toolBasicDAO:  mockBasicDAO,
					toolCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitToolDraftParam{
					ToolID:        1,
					CommitVersion: "1.0.0",
				},
			},
			wantErr: errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg("tool draft is not found, tool id = 1")),
		},
		{
			name: "toolCommitDAO.Create error",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&mysqlmodel.ToolBasic{
					ID:      1,
					SpaceID: 100,
				}, nil)
				mockCommitDAO := daomocks.NewMockIToolCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Get(gomock.Any(), int64(1), toolmgmt.PublicDraftVersion, gomock.Any()).Return(&mysqlmodel.ToolCommit{
					ToolID:  1,
					Content: ptr.Of("draft content"),
				}, nil)
				mockCommitDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errorx.New("create commit error"))
				return toolRepoFields{
					db:            mockDB,
					toolBasicDAO:  mockBasicDAO,
					toolCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitToolDraftParam{
					ToolID:        1,
					CommitVersion: "1.0.0",
					CommittedBy:   "user1",
				},
			},
			wantErr: errorx.New("create commit error"),
		},
		{
			name: "toolBasicDAO.Update error",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&mysqlmodel.ToolBasic{
					ID:      1,
					SpaceID: 100,
				}, nil)
				mockCommitDAO := daomocks.NewMockIToolCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Get(gomock.Any(), int64(1), toolmgmt.PublicDraftVersion, gomock.Any()).Return(&mysqlmodel.ToolCommit{
					ToolID:  1,
					Content: ptr.Of("draft content"),
				}, nil)
				mockCommitDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockBasicDAO.EXPECT().Update(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(errorx.New("update error"))
				return toolRepoFields{
					db:            mockDB,
					toolBasicDAO:  mockBasicDAO,
					toolCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitToolDraftParam{
					ToolID:        1,
					CommitVersion: "1.0.0",
					CommittedBy:   "user1",
				},
			},
			wantErr: errorx.New("update error"),
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockDB := dbmocks.NewMockProvider(ctrl)
				mockDB.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().Get(gomock.Any(), int64(1), gomock.Any()).Return(&mysqlmodel.ToolBasic{
					ID:      1,
					SpaceID: 100,
				}, nil)
				mockCommitDAO := daomocks.NewMockIToolCommitDAO(ctrl)
				mockCommitDAO.EXPECT().Get(gomock.Any(), int64(1), toolmgmt.PublicDraftVersion, gomock.Any()).Return(&mysqlmodel.ToolCommit{
					ToolID:  1,
					Content: ptr.Of("draft content"),
				}, nil)
				mockCommitDAO.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockBasicDAO.EXPECT().Update(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil)
				return toolRepoFields{
					db:            mockDB,
					toolBasicDAO:  mockBasicDAO,
					toolCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.CommitToolDraftParam{
					ToolID:            1,
					CommitVersion:     "1.0.0",
					CommitDescription: "first release",
					BaseVersion:       "0.9.0",
					CommittedBy:       "user1",
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)

			d := &ToolRepoImpl{db: f.db, idgen: f.idgen, toolBasicDAO: f.toolBasicDAO, toolCommitDAO: f.toolCommitDAO}

			err := d.CommitToolDraft(tt.args.ctx, tt.args.param)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
		})
	}
}

func TestToolRepoImpl_BatchGetTools(t *testing.T) {
	t.Parallel()
	type args struct {
		ctx   context.Context
		param repo.BatchGetToolsParam
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) toolRepoFields
		args         args
		want         []*repo.BatchGetToolsResult
		wantErr      error
	}{
		{
			name: "empty queries",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				return toolRepoFields{}
			},
			args: args{
				ctx: context.Background(),
				param: repo.BatchGetToolsParam{
					Queries: []repo.BatchGetToolsQuery{},
				},
			},
			want:    nil,
			wantErr: nil,
		},
		{
			name: "toolBasicDAO.BatchGet error",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().BatchGet(gomock.Any(), gomock.Any()).Return(nil, errorx.New("batch get error"))
				return toolRepoFields{
					toolBasicDAO: mockBasicDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.BatchGetToolsParam{
					Queries: []repo.BatchGetToolsQuery{
						{ToolID: 1, Version: "1.0.0"},
					},
				},
			},
			want:    nil,
			wantErr: errorx.New("batch get error"),
		},
		{
			name: "toolCommitDAO.BatchGet error",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().BatchGet(gomock.Any(), gomock.Any()).Return([]*mysqlmodel.ToolBasic{
					{ID: 1, SpaceID: 100, LatestCommittedVersion: "1.0.0"},
				}, nil)
				mockCommitDAO := daomocks.NewMockIToolCommitDAO(ctrl)
				mockCommitDAO.EXPECT().BatchGet(gomock.Any(), gomock.Any()).Return(nil, errorx.New("batch get commit error"))
				return toolRepoFields{
					toolBasicDAO:  mockBasicDAO,
					toolCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.BatchGetToolsParam{
					Queries: []repo.BatchGetToolsQuery{
						{ToolID: 1, Version: "1.0.0"},
					},
				},
			},
			want:    nil,
			wantErr: errorx.New("batch get commit error"),
		},
		{
			name: "success with version specified",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().BatchGet(gomock.Any(), gomock.Any()).Return([]*mysqlmodel.ToolBasic{
					{ID: 1, SpaceID: 100, Name: "tool1", LatestCommittedVersion: "2.0.0"},
				}, nil)
				mockCommitDAO := daomocks.NewMockIToolCommitDAO(ctrl)
				mockCommitDAO.EXPECT().BatchGet(gomock.Any(), gomock.Any()).Return([]*mysqlmodel.ToolCommit{
					{ToolID: 1, Version: "1.0.0", Content: ptr.Of("content_v1")},
				}, nil)
				return toolRepoFields{
					toolBasicDAO:  mockBasicDAO,
					toolCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.BatchGetToolsParam{
					Queries: []repo.BatchGetToolsQuery{
						{ToolID: 1, Version: "1.0.0"},
					},
				},
			},
			want: []*repo.BatchGetToolsResult{
				{
					Query: repo.BatchGetToolsQuery{ToolID: 1, Version: "1.0.0"},
					Tool: &toolmgmt.Tool{
						ID:      1,
						SpaceID: 100,
						ToolBasic: &toolmgmt.ToolBasic{
							Name:                   "tool1",
							LatestCommittedVersion: "2.0.0",
						},
						ToolCommit: &toolmgmt.ToolCommit{
							CommitInfo: &toolmgmt.CommitInfo{
								Version: "1.0.0",
							},
							ToolDetail: &toolmgmt.ToolDetail{
								Content: "content_v1",
							},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "success with empty version uses LatestCommittedVersion",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().BatchGet(gomock.Any(), gomock.Any()).Return([]*mysqlmodel.ToolBasic{
					{ID: 1, SpaceID: 100, Name: "tool1", LatestCommittedVersion: "2.0.0"},
				}, nil)
				mockCommitDAO := daomocks.NewMockIToolCommitDAO(ctrl)
				mockCommitDAO.EXPECT().BatchGet(gomock.Any(), []mysql.ToolIDVersionPair{
					{ToolID: 1, Version: "2.0.0"},
				}).Return([]*mysqlmodel.ToolCommit{
					{ToolID: 1, Version: "2.0.0", Content: ptr.Of("content_v2")},
				}, nil)
				return toolRepoFields{
					toolBasicDAO:  mockBasicDAO,
					toolCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.BatchGetToolsParam{
					Queries: []repo.BatchGetToolsQuery{
						{ToolID: 1, Version: ""},
					},
				},
			},
			want: []*repo.BatchGetToolsResult{
				{
					Query: repo.BatchGetToolsQuery{ToolID: 1, Version: ""},
					Tool: &toolmgmt.Tool{
						ID:      1,
						SpaceID: 100,
						ToolBasic: &toolmgmt.ToolBasic{
							Name:                   "tool1",
							LatestCommittedVersion: "2.0.0",
						},
						ToolCommit: &toolmgmt.ToolCommit{
							CommitInfo: &toolmgmt.CommitInfo{
								Version: "2.0.0",
							},
							ToolDetail: &toolmgmt.ToolDetail{
								Content: "content_v2",
							},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "success with SpaceID filter",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().BatchGet(gomock.Any(), gomock.Any()).Return([]*mysqlmodel.ToolBasic{
					{ID: 1, SpaceID: 100, Name: "tool1", LatestCommittedVersion: "1.0.0"},
					{ID: 2, SpaceID: 200, Name: "tool2", LatestCommittedVersion: "1.0.0"},
				}, nil)
				mockCommitDAO := daomocks.NewMockIToolCommitDAO(ctrl)
				mockCommitDAO.EXPECT().BatchGet(gomock.Any(), gomock.Any()).Return([]*mysqlmodel.ToolCommit{
					{ToolID: 1, Version: "1.0.0", Content: ptr.Of("c1")},
					{ToolID: 2, Version: "1.0.0", Content: ptr.Of("c2")},
				}, nil)
				return toolRepoFields{
					toolBasicDAO:  mockBasicDAO,
					toolCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.BatchGetToolsParam{
					SpaceID: 100,
					Queries: []repo.BatchGetToolsQuery{
						{ToolID: 1, Version: "1.0.0"},
						{ToolID: 2, Version: "1.0.0"},
					},
				},
			},
			want: []*repo.BatchGetToolsResult{
				{
					Query: repo.BatchGetToolsQuery{ToolID: 1, Version: "1.0.0"},
					Tool: &toolmgmt.Tool{
						ID:      1,
						SpaceID: 100,
						ToolBasic: &toolmgmt.ToolBasic{
							Name:                   "tool1",
							LatestCommittedVersion: "1.0.0",
						},
						ToolCommit: &toolmgmt.ToolCommit{
							CommitInfo: &toolmgmt.CommitInfo{
								Version: "1.0.0",
							},
							ToolDetail: &toolmgmt.ToolDetail{
								Content: "c1",
							},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "nil PO filtered",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockBasicDAO := daomocks.NewMockIToolBasicDAO(ctrl)
				mockBasicDAO.EXPECT().BatchGet(gomock.Any(), gomock.Any()).Return([]*mysqlmodel.ToolBasic{
					{ID: 1, SpaceID: 100, Name: "tool1", LatestCommittedVersion: "1.0.0"},
					nil,
				}, nil)
				mockCommitDAO := daomocks.NewMockIToolCommitDAO(ctrl)
				mockCommitDAO.EXPECT().BatchGet(gomock.Any(), gomock.Any()).Return([]*mysqlmodel.ToolCommit{
					{ToolID: 1, Version: "1.0.0", Content: ptr.Of("c1")},
					nil,
				}, nil)
				return toolRepoFields{
					toolBasicDAO:  mockBasicDAO,
					toolCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.BatchGetToolsParam{
					Queries: []repo.BatchGetToolsQuery{
						{ToolID: 1, Version: "1.0.0"},
						{ToolID: 2, Version: "1.0.0"},
					},
				},
			},
			want: []*repo.BatchGetToolsResult{
				{
					Query: repo.BatchGetToolsQuery{ToolID: 1, Version: "1.0.0"},
					Tool: &toolmgmt.Tool{
						ID:      1,
						SpaceID: 100,
						ToolBasic: &toolmgmt.ToolBasic{
							Name:                   "tool1",
							LatestCommittedVersion: "1.0.0",
						},
						ToolCommit: &toolmgmt.ToolCommit{
							CommitInfo: &toolmgmt.CommitInfo{
								Version: "1.0.0",
							},
							ToolDetail: &toolmgmt.ToolDetail{
								Content: "c1",
							},
						},
					},
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)

			d := &ToolRepoImpl{db: f.db, idgen: f.idgen, toolBasicDAO: f.toolBasicDAO, toolCommitDAO: f.toolCommitDAO}

			got, err := d.BatchGetTools(tt.args.ctx, tt.args.param)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestToolRepoImpl_ListToolCommit(t *testing.T) {
	t.Parallel()
	type args struct {
		ctx   context.Context
		param repo.ListToolCommitParam
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) toolRepoFields
		args         args
		want         *repo.ListToolCommitResult
		wantErr      error
	}{
		{
			name: "invalid params",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				return toolRepoFields{}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListToolCommitParam{
					ToolID:   0,
					PageSize: 10,
				},
			},
			want:    nil,
			wantErr: errorx.New(`param is invalid, param = {"ToolID":0,"PageSize":10,"PageToken":null,"Asc":false,"WithCommitDetail":false}`),
		},
		{
			name: "invalid params PageSize=0",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				return toolRepoFields{}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListToolCommitParam{
					ToolID:   1,
					PageSize: 0,
				},
			},
			want:    nil,
			wantErr: errorx.New(`param is invalid, param = {"ToolID":1,"PageSize":0,"PageToken":null,"Asc":false,"WithCommitDetail":false}`),
		},
		{
			name: "toolCommitDAO.List error",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockCommitDAO := daomocks.NewMockIToolCommitDAO(ctrl)
				mockCommitDAO.EXPECT().List(gomock.Any(), mysql.ListToolCommitParam{
					ToolID:         1,
					Limit:          10,
					ExcludeVersion: toolmgmt.PublicDraftVersion,
				}).Return(nil, errorx.New("list error"))
				return toolRepoFields{
					toolCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListToolCommitParam{
					ToolID:   1,
					PageSize: 10,
				},
			},
			want:    nil,
			wantErr: errorx.New("list error"),
		},
		{
			name: "success empty",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockCommitDAO := daomocks.NewMockIToolCommitDAO(ctrl)
				mockCommitDAO.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*mysqlmodel.ToolCommit{}, nil)
				return toolRepoFields{
					toolCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListToolCommitParam{
					ToolID:   1,
					PageSize: 10,
				},
			},
			want: &repo.ListToolCommitResult{
				CommitInfos:   []*toolmgmt.CommitInfo{},
				CommitDetails: nil,
				NextPageToken: 0,
			},
			wantErr: nil,
		},
		{
			name: "success with results",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockCommitDAO := daomocks.NewMockIToolCommitDAO(ctrl)
				mockCommitDAO.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*mysqlmodel.ToolCommit{
					{
						ToolID:      1,
						Version:     "1.0.0",
						BaseVersion: "0.9.0",
						CommittedBy: "user1",
						Description: ptr.Of("v1 release"),
						Content:     ptr.Of("content_v1"),
						CreatedAt:   time.Unix(1000, 0),
					},
					{
						ToolID:      1,
						Version:     "2.0.0",
						BaseVersion: "1.0.0",
						CommittedBy: "user2",
						Description: ptr.Of("v2 release"),
						Content:     ptr.Of("content_v2"),
						CreatedAt:   time.Unix(2000, 0),
					},
				}, nil)
				return toolRepoFields{
					toolCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListToolCommitParam{
					ToolID:   1,
					PageSize: 10,
				},
			},
			want: &repo.ListToolCommitResult{
				CommitInfos: []*toolmgmt.CommitInfo{
					{
						Version:     "1.0.0",
						BaseVersion: "0.9.0",
						Description: "v1 release",
						CommittedBy: "user1",
						CommittedAt: time.Unix(1000, 0),
					},
					{
						Version:     "2.0.0",
						BaseVersion: "1.0.0",
						Description: "v2 release",
						CommittedBy: "user2",
						CommittedAt: time.Unix(2000, 0),
					},
				},
				CommitDetails: nil,
				NextPageToken: 2000,
			},
			wantErr: nil,
		},
		{
			name: "WithCommitDetail",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockCommitDAO := daomocks.NewMockIToolCommitDAO(ctrl)
				mockCommitDAO.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*mysqlmodel.ToolCommit{
					{
						ToolID:      1,
						Version:     "1.0.0",
						BaseVersion: "",
						CommittedBy: "user1",
						Description: ptr.Of("initial"),
						Content:     ptr.Of("content_v1"),
						CreatedAt:   time.Unix(5000, 0),
					},
				}, nil)
				return toolRepoFields{
					toolCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListToolCommitParam{
					ToolID:           1,
					PageSize:         10,
					WithCommitDetail: true,
				},
			},
			want: &repo.ListToolCommitResult{
				CommitInfos: []*toolmgmt.CommitInfo{
					{
						Version:     "1.0.0",
						Description: "initial",
						CommittedBy: "user1",
						CommittedAt: time.Unix(5000, 0),
					},
				},
				CommitDetails: map[string]*toolmgmt.ToolDetail{
					"1.0.0": {Content: "content_v1"},
				},
				NextPageToken: 5000,
			},
			wantErr: nil,
		},
		{
			name: "nil PO filtered",
			fieldsGetter: func(ctrl *gomock.Controller) toolRepoFields {
				mockCommitDAO := daomocks.NewMockIToolCommitDAO(ctrl)
				mockCommitDAO.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*mysqlmodel.ToolCommit{
					{
						ToolID:      1,
						Version:     "1.0.0",
						CommittedBy: "user1",
						Description: ptr.Of("v1"),
						Content:     ptr.Of("c1"),
						CreatedAt:   time.Unix(1000, 0),
					},
					nil,
					{
						ToolID:      1,
						Version:     "3.0.0",
						CommittedBy: "user3",
						Description: ptr.Of("v3"),
						Content:     ptr.Of("c3"),
						CreatedAt:   time.Unix(3000, 0),
					},
				}, nil)
				return toolRepoFields{
					toolCommitDAO: mockCommitDAO,
				}
			},
			args: args{
				ctx: context.Background(),
				param: repo.ListToolCommitParam{
					ToolID:   1,
					PageSize: 10,
				},
			},
			want: &repo.ListToolCommitResult{
				CommitInfos: []*toolmgmt.CommitInfo{
					{
						Version:     "1.0.0",
						Description: "v1",
						CommittedBy: "user1",
						CommittedAt: time.Unix(1000, 0),
					},
					{
						Version:     "3.0.0",
						Description: "v3",
						CommittedBy: "user3",
						CommittedAt: time.Unix(3000, 0),
					},
				},
				CommitDetails: nil,
				NextPageToken: 3000,
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)

			d := &ToolRepoImpl{db: f.db, idgen: f.idgen, toolBasicDAO: f.toolBasicDAO, toolCommitDAO: f.toolCommitDAO}

			got, err := d.ListToolCommit(tt.args.ctx, tt.args.param)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
