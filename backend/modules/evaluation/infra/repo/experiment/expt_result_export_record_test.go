// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	mockidgen "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	model "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	mocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/mocks"
)

func newExportRecordRepo(ctrl *gomock.Controller) (*ExptResultExportRecordRepoImpl, *mocks.MockExptResultExportRecordDAO, *mockidgen.MockIIDGenerator) {
	dao := mocks.NewMockExptResultExportRecordDAO(ctrl)
	idGen := mockidgen.NewMockIIDGenerator(ctrl)
	return &ExptResultExportRecordRepoImpl{
		exptResultExportRecordDAO: dao,
		idgenerator:               idGen,
	}, dao, idGen
}

func TestExptResultExportRecordRepoImpl_Create(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo, dao, idGen := newExportRecordRepo(ctrl)
	record := &entity.ExptResultExportRecord{}

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "success",
			mockSetup: func() {
				idGen.EXPECT().GenID(gomock.Any()).Return(int64(123), nil)
				dao.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "fail_idgen_error",
			mockSetup: func() {
				idGen.EXPECT().GenID(gomock.Any()).Return(int64(0), errors.New("idgen error"))
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			id, err := repo.Create(context.Background(), record)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, int64(0), id)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, int64(123), id)
			}
		})
	}
}

func TestExptResultExportRecordRepoImpl_Update(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo, dao, _ := newExportRecordRepo(ctrl)
	record := &entity.ExptResultExportRecord{}

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "success",
			mockSetup: func() {
				dao.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "fail_dao_error",
			mockSetup: func() {
				dao.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("dao error"))
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.Update(context.Background(), record)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptResultExportRecordRepoImpl_List(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo, dao, _ := newExportRecordRepo(ctrl)
	page := entity.Page{}
	var status int32 = 1

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "success",
			mockSetup: func() {
				dao.EXPECT().List(gomock.Any(), int64(1), int64(2), page, &status).Return([]*model.ExptResultExportRecord{{}}, int64(1), nil)
			},
			wantErr: false,
		},
		{
			name: "fail_dao_error",
			mockSetup: func() {
				dao.EXPECT().List(gomock.Any(), int64(1), int64(2), page, &status).Return(nil, int64(0), errors.New("dao error"))
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, total, err := repo.List(context.Background(), 1, 2, page, &status)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
				assert.Equal(t, int64(0), total)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, int64(1), total)
			}
		})
	}
}

func TestExptResultExportRecordRepoImpl_Get(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo, dao, _ := newExportRecordRepo(ctrl)

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "success",
			mockSetup: func() {
				dao.EXPECT().Get(gomock.Any(), int64(1), int64(2), gomock.Any()).Return(&model.ExptResultExportRecord{}, nil)
			},
			wantErr: false,
		},
		{
			name: "fail_dao_error",
			mockSetup: func() {
				dao.EXPECT().Get(gomock.Any(), int64(1), int64(2), gomock.Any()).Return(nil, errors.New("dao error"))
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := repo.Get(context.Background(), 1, 2)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
			}
		})
	}
}
