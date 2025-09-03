// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"testing"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	idgenmock "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/model"
	mysqlmock "github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestViewRepoImpl_ListViews(t *testing.T) {
	type fields struct {
		viewDao mysql.IViewDao
	}
	type args struct {
		ctx         context.Context
		workspaceID int64
		userID      string
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      bool
	}{
		{
			name: "list view",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				viewDao := mysqlmock.NewMockIViewDao(ctrl)
				viewDao.EXPECT().ListViews(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
				return fields{
					viewDao: viewDao,
				}
			},
			args: args{
				ctx:         context.Background(),
				workspaceID: 1,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			v := &ViewRepoImpl{
				viewDao: fields.viewDao,
			}
			_, err := v.ListViews(tt.args.ctx, tt.args.workspaceID, tt.args.userID)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestViewRepoImpl_GetView(t *testing.T) {
	type fields struct {
		viewDao mysql.IViewDao
	}
	type args struct {
		ctx         context.Context
		id          int64
		workspaceID int64
		userID      string
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      bool
	}{
		{
			name: "get view",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				viewDao := mysqlmock.NewMockIViewDao(ctrl)
				viewDao.EXPECT().GetView(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&model.ObservabilityView{}, nil)
				return fields{
					viewDao: viewDao,
				}
			},
			args: args{
				ctx: context.Background(),
				id:  1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			v := &ViewRepoImpl{
				viewDao: fields.viewDao,
			}
			_, err := v.GetView(tt.args.ctx, tt.args.id, ptr.Of(tt.args.workspaceID), ptr.Of(tt.args.userID))
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestViewRepoImpl_UpdateView(t *testing.T) {
	type fields struct {
		viewDao mysql.IViewDao
	}
	type args struct {
		ctx  context.Context
		view *entity.ObservabilityView
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      bool
	}{
		{
			name: "update view",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				viewDao := mysqlmock.NewMockIViewDao(ctrl)
				viewDao.EXPECT().UpdateView(gomock.Any(), gomock.Any()).Return(nil)
				return fields{
					viewDao: viewDao,
				}
			},
			args: args{
				ctx:  context.Background(),
				view: &entity.ObservabilityView{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			v := &ViewRepoImpl{
				viewDao: fields.viewDao,
			}
			err := v.UpdateView(tt.args.ctx, tt.args.view)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestViewRepoImpl_DeleteView(t *testing.T) {
	type fields struct {
		viewDao mysql.IViewDao
	}
	type args struct {
		ctx         context.Context
		id          int64
		workspaceID int64
		userID      string
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      bool
	}{
		{
			name: "delete view",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				viewDao := mysqlmock.NewMockIViewDao(ctrl)
				viewDao.EXPECT().DeleteView(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				return fields{
					viewDao: viewDao,
				}
			},
			args: args{
				ctx: context.Background(),
				id:  1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			v := &ViewRepoImpl{
				viewDao: fields.viewDao,
			}
			err := v.DeleteView(tt.args.ctx, tt.args.id, tt.args.workspaceID, tt.args.userID)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestViewRepoImpl_CreateView(t *testing.T) {
	type fields struct {
		viewDao mysql.IViewDao
		idgen   idgen.IIDGenerator
	}
	type args struct {
		ctx  context.Context
		view *entity.ObservabilityView
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      bool
	}{
		{
			name: "create view",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				viewDao := mysqlmock.NewMockIViewDao(ctrl)
				viewDao.EXPECT().CreateView(gomock.Any(), gomock.Any()).Return(int64(0), nil)
				idgenMock := idgenmock.NewMockIIDGenerator(ctrl)
				idgenMock.EXPECT().GenID(gomock.Any()).Return(int64(123), nil)
				return fields{
					viewDao: viewDao,
					idgen:   idgenMock,
				}
			},
			args: args{
				ctx:  context.Background(),
				view: &entity.ObservabilityView{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			v := &ViewRepoImpl{
				viewDao:     fields.viewDao,
				idGenerator: fields.idgen,
			}
			_, err := v.CreateView(tt.args.ctx, tt.args.view)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}
