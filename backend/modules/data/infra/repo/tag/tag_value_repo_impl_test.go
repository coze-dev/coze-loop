// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tag

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	mocks2 "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/tag/entity"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/pagination"
)

func TestTagRepoImpl_MCreateTagValues(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()
	rows := sqlmock.NewRows([]string{"version"}).
		AddRow("8.0.26") // 根据实际情况填写数据库版本
	mock.ExpectQuery("SELECT VERSION()").
		WillReturnRows(rows)
	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn: db,
	}), &gorm.Config{})
	assert.NoError(t, err)

	dbMock := mocks.NewMockProvider(ctrl)
	idGenMock := mocks2.NewMockIIDGenerator(ctrl)
	tagRepo := NewTagRepoImpl(dbMock, idGenMock)

	type args struct {
		val []*entity.TagValue
	}

	tests := []struct {
		args      args
		name      string
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "input val is empty",
			args: args{
				val: []*entity.TagValue{},
			},
			wantErr: false,
		},
		{
			name: "gen id failed",
			args: args{
				val: []*entity.TagValue{
					{
						TagValueName: "test",
					},
				},
			},
			mockSetup: func() {
				idGenMock.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).Return(nil, errors.New("123"))
			},
			wantErr: true,
		},
		{
			name: "create failed",
			args: args{
				val: []*entity.TagValue{
					{
						TagValueName: "test",
					},
				},
			},
			wantErr: true,
			mockSetup: func() {
				idGenMock.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).Return([]int64{123, 345}, nil)
				dbMock.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(gormDB)
				mock.ExpectBegin()
				mock.ExpectExec("^INSERT").WillReturnError(errors.New("123"))
				mock.ExpectRollback()
			},
		},
		{
			name: "normal case",
			args: args{
				val: []*entity.TagValue{
					{
						TagValueName: "test",
					},
				},
			},
			wantErr: false,
			mockSetup: func() {
				idGenMock.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).Return([]int64{123, 345}, nil)
				dbMock.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(gormDB)
				mock.ExpectBegin()
				mock.ExpectExec("^INSERT").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			},
		},
	}

	for _, tt := range tests {
		if tt.mockSetup != nil {
			tt.mockSetup()
		}
		err := tagRepo.MCreateTagValues(ctx, tt.args.val)
		if (err != nil) != tt.wantErr {
			t.Errorf("MCreateTagValues() test case: %s, error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func TestTagRepoImpl_GetTagValue(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()
	rows := sqlmock.NewRows([]string{"version"}).
		AddRow("8.0.26") // 根据实际情况填写数据库版本
	mock.ExpectQuery("SELECT VERSION()").
		WillReturnRows(rows)
	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn: db,
	}), &gorm.Config{})
	assert.NoError(t, err)

	dbMock := mocks.NewMockProvider(ctrl)
	idGenMock := mocks2.NewMockIIDGenerator(ctrl)
	tagRepo := NewTagRepoImpl(dbMock, idGenMock)

	type args struct {
		spaceID int64
		id      int64
	}
	tests := []struct {
		args      args
		name      string
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "spaceID is empty",
			args: args{
				spaceID: 0,
				id:      0,
			},
			wantErr:   true,
			mockSetup: func() {},
		},
		{
			name: "query failed",
			args: args{
				spaceID: 123,
				id:      123,
			},
			wantErr: true,
			mockSetup: func() {
				dbMock.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(gormDB)
				mock.ExpectQuery("^SELECT").
					WillReturnError(errors.New("123"))
			},
		},
	}
	for _, tt := range tests {
		if tt.mockSetup != nil {
			tt.mockSetup()
		}
		_, err := tagRepo.GetTagValue(ctx, tt.args.spaceID, tt.args.id)
		if (err != nil) != tt.wantErr {
			t.Errorf("GetTagKey() test case: %s, error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func TestTagRepoImpl_MGetTagValues(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()
	rows := sqlmock.NewRows([]string{"version"}).
		AddRow("8.0.26") // 根据实际情况填写数据库版本
	mock.ExpectQuery("SELECT VERSION()").
		WillReturnRows(rows)
	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn: db,
	}), &gorm.Config{})
	assert.NoError(t, err)

	dbMock := mocks.NewMockProvider(ctrl)
	idGenMock := mocks2.NewMockIIDGenerator(ctrl)
	tagRepo := NewTagRepoImpl(dbMock, idGenMock)

	tests := []struct {
		param     *entity.MGetTagValueParam
		name      string
		mockSetup func()
		wantErr   bool
	}{
		{
			name:    "param is empty",
			wantErr: true,
		},
		{
			name: "find failed",
			param: &entity.MGetTagValueParam{
				Paginator: pagination.New(),
				SpaceID:   int64(123),
				IDs:       []int64{123},
			},
			wantErr: true,
			mockSetup: func() {
				dbMock.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(gormDB)
				mock.ExpectQuery("^SELECT").WillReturnError(errors.New("123"))
			},
		},
		{
			name: "normal case",
			param: &entity.MGetTagValueParam{
				Paginator: pagination.New(),
				SpaceID:   int64(123),
				IDs:       []int64{123},
			},
			wantErr: false,
			mockSetup: func() {
				dbMock.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(gormDB)
				rows := sqlmock.NewRows([]string{"id"}).AddRow(123)
				mock.ExpectQuery("^SELECT").WillReturnRows(rows)
			},
		},
	}

	for _, tt := range tests {
		if tt.mockSetup != nil {
			tt.mockSetup()
		}
		_, _, err := tagRepo.MGetTagValue(ctx, tt.param)
		if (err != nil) != tt.wantErr {
			t.Errorf("MGetTagKeys() test case: %s, error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func TestTagRepoImpl_PatchTagValue(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()
	rows := sqlmock.NewRows([]string{"version"}).
		AddRow("8.0.26") // 根据实际情况填写数据库版本
	mock.ExpectQuery("SELECT VERSION()").
		WillReturnRows(rows)
	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn: db,
	}), &gorm.Config{})
	assert.NoError(t, err)

	dbMock := mocks.NewMockProvider(ctrl)
	idGenMock := mocks2.NewMockIIDGenerator(ctrl)
	tagRepo := NewTagRepoImpl(dbMock, idGenMock)
	type args struct {
		spaceID int64
		id      int64
		patch   *entity.TagValue
	}

	tests := []struct {
		args      args
		name      string
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "spaceid is empty",
			args: args{
				spaceID: 0,
				id:      123,
			},
			wantErr: true,
		},
		{
			name: "id is empty",
			args: args{
				spaceID: 123,
				id:      0,
			},
			wantErr: true,
		},
		{
			name: "patch is empty",
			args: args{
				spaceID: 123,
				id:      123,
				patch:   nil,
			},
			wantErr: true,
		},
		{
			name: "updates failed",
			args: args{
				spaceID: 123,
				id:      123,
				patch:   &entity.TagValue{},
			},
			wantErr: true,
			mockSetup: func() {
				dbMock.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(gormDB)
				mock.ExpectBegin()
				mock.ExpectExec("^UPDATE").WillReturnError(errors.New("123"))
				mock.ExpectRollback()
			},
		},
		{
			name: "tag key not found",
			args: args{
				spaceID: 123,
				id:      123,
				patch:   &entity.TagValue{},
			},
			wantErr: true,
			mockSetup: func() {
				dbMock.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(gormDB)
				mock.ExpectBegin()
				mock.ExpectExec("^UPDATE").WillReturnResult(sqlmock.NewResult(1, 0))
				mock.ExpectCommit()
			},
		},
		{
			name: "normal case",
			args: args{
				spaceID: 123,
				id:      123,
				patch:   &entity.TagValue{},
			},
			wantErr: false,
			mockSetup: func() {
				dbMock.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(gormDB)
				mock.ExpectBegin()
				mock.ExpectExec("^UPDATE").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			},
		},
	}
	for _, tt := range tests {
		if tt.mockSetup != nil {
			tt.mockSetup()
		}
		err := tagRepo.PatchTagValue(ctx, tt.args.spaceID, tt.args.id, tt.args.patch)
		if (err != nil) != tt.wantErr {
			t.Errorf("PatchTagKey() test case: %s, error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func TestTagRepoImpl_DeleteTagValue(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()
	rows := sqlmock.NewRows([]string{"version"}).
		AddRow("8.0.26") // 根据实际情况填写数据库版本
	mock.ExpectQuery("SELECT VERSION()").
		WillReturnRows(rows)
	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn: db,
	}), &gorm.Config{})
	assert.NoError(t, err)

	dbMock := mocks.NewMockProvider(ctrl)
	idGenMock := mocks2.NewMockIIDGenerator(ctrl)
	tagRepo := NewTagRepoImpl(dbMock, idGenMock)
	type args struct {
		spaceID int64
		id      int64
	}
	tests := []struct {
		args      args
		name      string
		wantErr   bool
		mockSetup func()
	}{
		{
			name: "spaceID is empty",
			args: args{
				spaceID: 0,
				id:      123,
			},
			wantErr: true,
		},
		{
			name: "delete failed",
			args: args{
				spaceID: 123,
				id:      123,
			},
			wantErr: true,
			mockSetup: func() {
				dbMock.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(gormDB)
				mock.ExpectBegin()
				mock.ExpectExec("^DELETE").WillReturnError(errors.New("123"))
				mock.ExpectRollback()
			},
		},
		{
			name: "normal case",
			args: args{
				spaceID: 123,
				id:      123,
			},
			wantErr: false,
			mockSetup: func() {
				dbMock.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(gormDB)
				mock.ExpectBegin()
				mock.ExpectExec("^DELETE").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			},
		},
	}

	for _, tt := range tests {
		if tt.mockSetup != nil {
			tt.mockSetup()
		}
		err := tagRepo.DeleteTagValue(ctx, tt.args.spaceID, tt.args.id)
		if (err != nil) != tt.wantErr {
			t.Errorf("DeleteTagKey() test case: %s, error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func TestTagRepoImpl_UpdateTagValuesStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()
	rows := sqlmock.NewRows([]string{"version"}).
		AddRow("8.0.26") // 根据实际情况填写数据库版本
	mock.ExpectQuery("SELECT VERSION()").
		WillReturnRows(rows)
	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn: db,
	}), &gorm.Config{})
	assert.NoError(t, err)

	dbMock := mocks.NewMockProvider(ctrl)
	idGenMock := mocks2.NewMockIIDGenerator(ctrl)
	tagRepo := NewTagRepoImpl(dbMock, idGenMock)
	type args struct {
		spaceID    int64
		tagKeyID   int64
		versionNum int32
		toStatus   entity.TagStatus
		updateInfo bool
	}
	tests := []struct {
		args      args
		name      string
		wantErr   bool
		mockSetup func()
	}{
		{
			name: "spaceID is empty",
			args: args{
				spaceID: 0,
			},
			wantErr: true,
		},
		{
			name: "update failed",
			args: args{
				spaceID:    123,
				tagKeyID:   123,
				versionNum: 1,
				toStatus:   entity.TagStatusActive,
				updateInfo: true,
			},
			wantErr: true,
			mockSetup: func() {
				dbMock.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(gormDB)
				mock.ExpectBegin()
				mock.ExpectExec("^UPDATE").WillReturnError(errors.New("123"))
				mock.ExpectRollback()
			},
		},
		{
			name: "normal case",
			args: args{
				spaceID:    123,
				tagKeyID:   123,
				versionNum: 1,
				toStatus:   entity.TagStatusActive,
				updateInfo: false,
			},
			wantErr: false,
			mockSetup: func() {
				dbMock.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(gormDB)
				mock.ExpectBegin()
				mock.ExpectExec("^UPDATE").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			},
		},
	}
	for _, tt := range tests {
		if tt.mockSetup != nil {
			tt.mockSetup()
		}
		err := tagRepo.UpdateTagValuesStatus(ctx, tt.args.spaceID, tt.args.tagKeyID, tt.args.versionNum, tt.args.toStatus, tt.args.updateInfo)
		if (err != nil) != tt.wantErr {
			t.Errorf("UpdateTagKeysStatus() test case: %s, error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}
