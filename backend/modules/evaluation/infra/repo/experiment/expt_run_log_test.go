// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/mocks"
)

func TestExptRunLogImpl_Get(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDAO := mocks.NewMockIExptRunLogDAO(ctrl)
	repo := &ExptRunLogImpl{exptRunLogDAO: mockDAO}

	tests := []struct {
		name      string
		exptID    int64
		exptRunID int64
		mockSetup func()
		wantErr   bool
	}{
		{
			name:      "success",
			exptID:    1,
			exptRunID: 2,
			mockSetup: func() {
				mockDAO.EXPECT().Get(gomock.Any(), int64(1), int64(2)).Return(&model.ExptRunLog{}, nil)
			},
			wantErr: false,
		},
		{
			name:      "fail_dao_error",
			exptID:    3,
			exptRunID: 4,
			mockSetup: func() {
				mockDAO.EXPECT().Get(gomock.Any(), int64(3), int64(4)).Return(nil, errors.New("dao error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := repo.Get(context.Background(), tt.exptID, tt.exptRunID)
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

func TestExptRunLogImpl_Create(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDAO := mocks.NewMockIExptRunLogDAO(ctrl)
	repo := &ExptRunLogImpl{exptRunLogDAO: mockDAO}
	input := &entity.ExptRunLog{}

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "success",
			mockSetup: func() {
				mockDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "fail_dao_error",
			mockSetup: func() {
				mockDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("dao error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.Create(context.Background(), input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptRunLogImpl_Save(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDAO := mocks.NewMockIExptRunLogDAO(ctrl)
	repo := &ExptRunLogImpl{exptRunLogDAO: mockDAO}
	input := &entity.ExptRunLog{}

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "success",
			mockSetup: func() {
				mockDAO.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "fail_dao_error",
			mockSetup: func() {
				mockDAO.EXPECT().Save(gomock.Any(), gomock.Any()).Return(errors.New("dao error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.Save(context.Background(), input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptRunLogImpl_Update(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDAO := mocks.NewMockIExptRunLogDAO(ctrl)
	repo := &ExptRunLogImpl{exptRunLogDAO: mockDAO}

	tests := []struct {
		name      string
		exptID    int64
		exptRunID int64
		spaceID   int64
		ufields   map[string]any
		mockSetup func()
		wantErr   bool
	}{
		{
			name:      "success",
			exptID:    1,
			exptRunID: 2,
			spaceID:   789,
			ufields:   map[string]any{"status": 1},
			mockSetup: func() {
				mockDAO.EXPECT().Update(gomock.Any(), int64(1), int64(2), int64(789), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name:      "fail_dao_error",
			exptID:    3,
			exptRunID: 4,
			spaceID:   790,
			ufields:   map[string]any{"status": 2},
			mockSetup: func() {
				mockDAO.EXPECT().Update(gomock.Any(), int64(3), int64(4), int64(790), gomock.Any()).Return(errors.New("dao error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.Update(context.Background(), tt.exptID, tt.exptRunID, tt.spaceID, tt.ufields)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
