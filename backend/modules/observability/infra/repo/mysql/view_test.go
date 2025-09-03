// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestViewRepoImpl_ListViews(t *testing.T) {
	type fields struct {
		dbMgr db.Provider
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
				d, mock, _ := sqlmock.New()
				viewRows := sqlmock.NewRows([]string{"id"}).AddRow(1)
				mock.ExpectQuery("^SELECT").WillReturnRows(viewRows)
				gormDb, _ := gorm.Open(mysql.New(mysql.Config{Conn: d, SkipInitializeWithVersion: true}), &gorm.Config{})
				mockdb := dbmock.NewMockProvider(ctrl)
				mockdb.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(gormDb)
				return fields{
					dbMgr: mockdb,
				}
			},
			args: args{
				ctx:         context.Background(),
				workspaceID: 1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			v := &ViewDaoImpl{
				dbMgr: fields.dbMgr,
			}
			_, err := v.ListViews(tt.args.ctx, tt.args.workspaceID, tt.args.userID)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestViewRepoImpl_GetView(t *testing.T) {
	type fields struct {
		dbMgr db.Provider
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
				d, mock, _ := sqlmock.New()
				viewRows := sqlmock.NewRows([]string{"id"}).AddRow(1)
				mock.ExpectQuery("^SELECT").WillReturnRows(viewRows)
				gormDb, _ := gorm.Open(mysql.New(mysql.Config{Conn: d, SkipInitializeWithVersion: true}), &gorm.Config{})
				mockdb := dbmock.NewMockProvider(ctrl)
				mockdb.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(gormDb)
				return fields{
					dbMgr: mockdb,
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
			v := &ViewDaoImpl{
				dbMgr: fields.dbMgr,
			}
			_, err := v.GetView(tt.args.ctx, tt.args.id, ptr.Of(tt.args.workspaceID), ptr.Of(tt.args.userID))
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestViewRepoImpl_UpdateView(t *testing.T) {
	type fields struct {
		dbMgr db.Provider
	}
	type args struct {
		ctx  context.Context
		view *model.ObservabilityView
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
				d, mock, _ := sqlmock.New()
				mock.ExpectBegin()
				mock.ExpectExec(".*").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
				gormDb, _ := gorm.Open(mysql.New(mysql.Config{Conn: d, SkipInitializeWithVersion: true}), &gorm.Config{})
				mockdb := dbmock.NewMockProvider(ctrl)
				mockdb.EXPECT().NewSession(gomock.Any()).Return(gormDb)
				return fields{
					dbMgr: mockdb,
				}
			},
			args: args{
				ctx:  context.Background(),
				view: &model.ObservabilityView{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			v := &ViewDaoImpl{
				dbMgr: fields.dbMgr,
			}
			err := v.UpdateView(tt.args.ctx, tt.args.view)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestViewRepoImpl_DeleteView(t *testing.T) {
	type fields struct {
		dbMgr db.Provider
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
				d, mock, _ := sqlmock.New()
				mock.ExpectBegin()
				mock.ExpectExec(".*").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
				gormDb, _ := gorm.Open(mysql.New(mysql.Config{Conn: d, SkipInitializeWithVersion: true}), &gorm.Config{})
				mockdb := dbmock.NewMockProvider(ctrl)
				mockdb.EXPECT().NewSession(gomock.Any()).Return(gormDb)
				return fields{
					dbMgr: mockdb,
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
			v := &ViewDaoImpl{
				dbMgr: fields.dbMgr,
			}
			err := v.DeleteView(tt.args.ctx, tt.args.id, tt.args.workspaceID, tt.args.userID)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestViewRepoImpl_CreateView(t *testing.T) {
	type fields struct {
		dbMgr db.Provider
	}
	type args struct {
		ctx  context.Context
		view *model.ObservabilityView
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
				d, mock, _ := sqlmock.New()
				mock.ExpectBegin()
				mock.ExpectExec("^INSERT").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
				gormDb, _ := gorm.Open(mysql.New(mysql.Config{Conn: d, SkipInitializeWithVersion: true}), &gorm.Config{})
				mockdb := dbmock.NewMockProvider(ctrl)
				mockdb.EXPECT().NewSession(gomock.Any()).Return(gormDb)
				return fields{
					dbMgr: mockdb,
				}
			},
			args: args{
				ctx:  context.Background(),
				view: &model.ObservabilityView{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			v := &ViewDaoImpl{
				dbMgr: fields.dbMgr,
			}
			_, err := v.CreateView(tt.args.ctx, tt.args.view)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}
