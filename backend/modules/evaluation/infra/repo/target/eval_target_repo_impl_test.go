// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package target

import (
	"context"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	idgen "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	platestwrite_mocks "github.com/coze-dev/coze-loop/backend/infra/platestwrite/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repointerface "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/model"
	mysqlmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestEvalTargetRepoImpl_CreateEvalTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockEvalTargetDao := mysqlmocks.NewMockEvalTargetDAO(ctrl)
	mockEvalTargetVersionDao := mysqlmocks.NewMockEvalTargetVersionDAO(ctrl)
	mockEvalTargetRecordDao := mysqlmocks.NewMockEvalTargetRecordDAO(ctrl)
	mockIDGen := idgen.NewMockIIDGenerator(ctrl)
	mockDBProvider := dbmock.NewMockProvider(ctrl)
	mockLWT := platestwrite_mocks.NewMockILatestWriteTracker(ctrl)

	repo := &EvalTargetRepoImpl{
		evalTargetDao:        mockEvalTargetDao,
		evalTargetVersionDao: mockEvalTargetVersionDao,
		evalTargetRecordDao:  mockEvalTargetRecordDao,
		idgen:                mockIDGen,
		dbProvider:           mockDBProvider,
		lwt:                  mockLWT,
	}

	// Test data
	validSpaceID := int64(123)
	validSourceTargetID := "source-123"
	validSourceTargetVersion := "v1.0"
	validEvalTargetType := int32(1)
	validTargetID := int64(456)
	validVersionID := int64(789)

	tests := []struct {
		name        string
		do          *entity.EvalTarget
		mockSetup   func()
		wantID      int64
		wantVersion int64
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - create new target and version",
			do: &entity.EvalTarget{
				SpaceID:        validSpaceID,
				SourceTargetID: validSourceTargetID,
				EvalTargetType: entity.EvalTargetType(validEvalTargetType),
				EvalTargetVersion: &entity.EvalTargetVersion{
					SpaceID:             validSpaceID,
					SourceTargetVersion: validSourceTargetVersion,
					BaseInfo: &entity.BaseInfo{
						CreatedBy: &entity.UserInfo{},
						UpdatedBy: &entity.UserInfo{},
						CreatedAt: gptr.Of(time.Now().UnixMilli()),
						UpdatedAt: gptr.Of(time.Now().UnixMilli()),
					},
				},
				BaseInfo: &entity.BaseInfo{
					CreatedBy: &entity.UserInfo{},
					UpdatedBy: &entity.UserInfo{},
					CreatedAt: gptr.Of(time.Now().UnixMilli()),
					UpdatedAt: gptr.Of(time.Now().UnixMilli()),
				},
			},
			mockSetup: func() {
				// Mock ID generation
				mockIDGen.EXPECT().
					GenMultiIDs(gomock.Any(), 2).
					Return([]int64{validTargetID, validVersionID}, nil)

				// Mock transaction

				mockDBProvider.EXPECT().Transaction(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				// Mock get target by source
				mockEvalTargetDao.EXPECT().
					GetEvalTargetBySourceID(gomock.Any(), validSpaceID, validSourceTargetID, validEvalTargetType, gomock.Any(), gomock.Any()).
					Return(nil, nil)

				// Mock create target
				mockEvalTargetDao.EXPECT().
					CreateEvalTarget(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)

				// Mock get version by target
				mockEvalTargetVersionDao.EXPECT().
					GetEvalTargetVersionByTarget(gomock.Any(), validSpaceID, validTargetID, validSourceTargetVersion, gomock.Any(), gomock.Any()).
					Return(nil, nil)

				// Mock create version
				mockEvalTargetVersionDao.EXPECT().
					CreateEvalTargetVersion(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)

				// Mock latest write tracker
				mockLWT.EXPECT().
					SetWriteFlag(gomock.Any(), platestwrite.ResourceTypeTarget, gomock.Any())
				mockLWT.EXPECT().
					SetWriteFlag(gomock.Any(), platestwrite.ResourceTypeTargetVersion, gomock.Any())
			},
			wantID:      validTargetID,
			wantVersion: validVersionID,
			wantErr:     false,
		},
		{
			name:        "error - nil target",
			do:          nil,
			mockSetup:   func() {},
			wantID:      0,
			wantVersion: 0,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - nil version",
			do: &entity.EvalTarget{
				SpaceID:        validSpaceID,
				SourceTargetID: validSourceTargetID,
				EvalTargetType: entity.EvalTargetType(validEvalTargetType),
			},
			mockSetup:   func() {},
			wantID:      0,
			wantVersion: 0,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "error - ID generation failed",
			do: &entity.EvalTarget{
				SpaceID:        validSpaceID,
				SourceTargetID: validSourceTargetID,
				EvalTargetType: entity.EvalTargetType(validEvalTargetType),
				EvalTargetVersion: &entity.EvalTargetVersion{
					SpaceID:             validSpaceID,
					SourceTargetVersion: validSourceTargetVersion,
				},
			},
			mockSetup: func() {
				mockIDGen.EXPECT().
					GenMultiIDs(gomock.Any(), 2).
					Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantID:      0,
			wantVersion: 0,
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name: "error - target already exists",
			do: &entity.EvalTarget{
				SpaceID:        validSpaceID,
				SourceTargetID: validSourceTargetID,
				EvalTargetType: entity.EvalTargetType(validEvalTargetType),
				EvalTargetVersion: &entity.EvalTargetVersion{
					SpaceID:             validSpaceID,
					SourceTargetVersion: validSourceTargetVersion,
				},
			},
			mockSetup: func() {
				mockIDGen.EXPECT().
					GenMultiIDs(gomock.Any(), 2).
					Return([]int64{validTargetID, validVersionID}, nil)

				mockDBProvider.EXPECT().Transaction(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockEvalTargetDao.EXPECT().
					GetEvalTargetBySourceID(gomock.Any(), validSpaceID, validSourceTargetID, validEvalTargetType, gomock.Any(), gomock.Any()).
					Return(&model.Target{ID: validTargetID}, nil)

				mockEvalTargetVersionDao.EXPECT().
					GetEvalTargetVersionByTarget(gomock.Any(), validSpaceID, validTargetID, validSourceTargetVersion, gomock.Any(), gomock.Any()).
					Return(nil, nil)

				mockEvalTargetVersionDao.EXPECT().
					CreateEvalTargetVersion(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)

				mockLWT.EXPECT().
					SetWriteFlag(gomock.Any(), platestwrite.ResourceTypeTarget, gomock.Any())
				mockLWT.EXPECT().
					SetWriteFlag(gomock.Any(), platestwrite.ResourceTypeTargetVersion, gomock.Any())
			},
			wantID:      validTargetID,
			wantVersion: validVersionID,
			wantErr:     false,
		},
		{
			name: "error - version already exists",
			do: &entity.EvalTarget{
				SpaceID:        validSpaceID,
				SourceTargetID: validSourceTargetID,
				EvalTargetType: entity.EvalTargetType(validEvalTargetType),
				EvalTargetVersion: &entity.EvalTargetVersion{
					SpaceID:             validSpaceID,
					SourceTargetVersion: validSourceTargetVersion,
				},
			},
			mockSetup: func() {
				mockIDGen.EXPECT().
					GenMultiIDs(gomock.Any(), 2).
					Return([]int64{validTargetID, validVersionID}, nil)

				mockDBProvider.EXPECT().Transaction(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})

				mockEvalTargetDao.EXPECT().
					GetEvalTargetBySourceID(gomock.Any(), validSpaceID, validSourceTargetID, validEvalTargetType, gomock.Any(), gomock.Any()).
					Return(&model.Target{ID: validTargetID}, nil)

				mockEvalTargetVersionDao.EXPECT().
					GetEvalTargetVersionByTarget(gomock.Any(), validSpaceID, validTargetID, validSourceTargetVersion, gomock.Any(), gomock.Any()).
					Return(&model.TargetVersion{ID: validVersionID}, nil)

				mockLWT.EXPECT().
					SetWriteFlag(gomock.Any(), platestwrite.ResourceTypeTarget, gomock.Any())
				mockLWT.EXPECT().
					SetWriteFlag(gomock.Any(), platestwrite.ResourceTypeTargetVersion, gomock.Any())
			},
			wantID:      validTargetID,
			wantVersion: validVersionID,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			id, versionID, err := repo.CreateEvalTarget(context.Background(), tt.do)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					if ok {
						assert.Equal(t, tt.wantErrCode, statusErr.Code())
					}
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantID, id)
				assert.Equal(t, tt.wantVersion, versionID)
			}
		})
	}
}

func TestEvalTargetRepoImpl_GetEvalTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockEvalTargetDao := mysqlmocks.NewMockEvalTargetDAO(ctrl)
	mockEvalTargetVersionDao := mysqlmocks.NewMockEvalTargetVersionDAO(ctrl)
	mockEvalTargetRecordDao := mysqlmocks.NewMockEvalTargetRecordDAO(ctrl)
	mockIDGen := idgen.NewMockIIDGenerator(ctrl)
	mockDBProvider := dbmock.NewMockProvider(ctrl)
	mockLWT := platestwrite_mocks.NewMockILatestWriteTracker(ctrl)

	repo := &EvalTargetRepoImpl{
		evalTargetDao:        mockEvalTargetDao,
		evalTargetVersionDao: mockEvalTargetVersionDao,
		evalTargetRecordDao:  mockEvalTargetRecordDao,
		idgen:                mockIDGen,
		dbProvider:           mockDBProvider,
		lwt:                  mockLWT,
	}

	// Test data
	validTargetID := int64(123)
	validSpaceID := int64(456)
	validSourceTargetID := "source-123"
	validEvalTargetType := int32(1)

	tests := []struct {
		name      string
		targetID  int64
		mockSetup func()
		want      *entity.EvalTarget
		wantErr   bool
	}{
		{
			name:     "success - target exists",
			targetID: validTargetID,
			mockSetup: func() {
				// Mock latest write tracker check
				mockLWT.EXPECT().
					CheckWriteFlagByID(gomock.Any(), platestwrite.ResourceTypeTarget, validTargetID).
					Return(false)

				// Mock get target
				mockEvalTargetDao.EXPECT().
					GetEvalTarget(gomock.Any(), validTargetID, gomock.Any()).
					Return(&model.Target{
						ID:             validTargetID,
						SpaceID:        validSpaceID,
						SourceTargetID: validSourceTargetID,
						TargetType:     validEvalTargetType,
					}, nil)
			},
			want: &entity.EvalTarget{
				ID:             validTargetID,
				SpaceID:        validSpaceID,
				SourceTargetID: validSourceTargetID,
				EvalTargetType: entity.EvalTargetType(validEvalTargetType),
			},
			wantErr: false,
		},
		{
			name:     "success - target exists with latest write",
			targetID: validTargetID,
			mockSetup: func() {
				// Mock latest write tracker check
				mockLWT.EXPECT().
					CheckWriteFlagByID(gomock.Any(), platestwrite.ResourceTypeTarget, validTargetID).
					Return(true)

				// Mock get target with master option
				mockEvalTargetDao.EXPECT().
					GetEvalTarget(gomock.Any(), validTargetID, gomock.Any()).
					Return(&model.Target{
						ID:             validTargetID,
						SpaceID:        validSpaceID,
						SourceTargetID: validSourceTargetID,
						TargetType:     validEvalTargetType,
					}, nil)
			},
			want: &entity.EvalTarget{
				ID:             validTargetID,
				SpaceID:        validSpaceID,
				SourceTargetID: validSourceTargetID,
				EvalTargetType: entity.EvalTargetType(validEvalTargetType),
			},
			wantErr: false,
		},
		{
			name:     "error - dao error",
			targetID: validTargetID,
			mockSetup: func() {
				// Mock latest write tracker check
				mockLWT.EXPECT().
					CheckWriteFlagByID(gomock.Any(), platestwrite.ResourceTypeTarget, validTargetID).
					Return(false)

				// Mock get target returns error
				mockEvalTargetDao.EXPECT().
					GetEvalTarget(gomock.Any(), validTargetID).
					Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			got, err := repo.GetEvalTarget(context.Background(), tt.targetID)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want.ID, got.ID)
			}
		})
	}
}

func TestEvalTargetRepoImpl_GetEvalTargetVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockEvalTargetDao := mysqlmocks.NewMockEvalTargetDAO(ctrl)
	mockEvalTargetVersionDao := mysqlmocks.NewMockEvalTargetVersionDAO(ctrl)
	mockEvalTargetRecordDao := mysqlmocks.NewMockEvalTargetRecordDAO(ctrl)
	mockIDGen := idgen.NewMockIIDGenerator(ctrl)
	mockDBProvider := dbmock.NewMockProvider(ctrl)
	mockLWT := platestwrite_mocks.NewMockILatestWriteTracker(ctrl)

	repo := &EvalTargetRepoImpl{
		evalTargetDao:        mockEvalTargetDao,
		evalTargetVersionDao: mockEvalTargetVersionDao,
		evalTargetRecordDao:  mockEvalTargetRecordDao,
		idgen:                mockIDGen,
		dbProvider:           mockDBProvider,
		lwt:                  mockLWT,
	}

	// Test data
	validSpaceID := int64(123)
	validVersionID := int64(456)
	validTargetID := int64(789)
	validSourceTargetID := "source-123"
	validEvalTargetType := int32(1)
	validSourceTargetVersion := "v1.0"

	tests := []struct {
		name        string
		spaceID     int64
		versionID   int64
		mockSetup   func()
		want        *entity.EvalTarget
		wantErr     bool
		wantErrCode int32
	}{
		{
			name:      "success - version exists",
			spaceID:   validSpaceID,
			versionID: validVersionID,
			mockSetup: func() {
				// Mock latest write tracker check for version
				mockLWT.EXPECT().
					CheckWriteFlagByID(gomock.Any(), platestwrite.ResourceTypeTargetVersion, validVersionID).
					Return(false)

				// Mock get version
				mockEvalTargetVersionDao.EXPECT().
					GetEvalTargetVersion(gomock.Any(), validSpaceID, validVersionID, gomock.Any()).
					Return(&model.TargetVersion{
						ID:                  validVersionID,
						SpaceID:             validSpaceID,
						TargetID:            validTargetID,
						SourceTargetVersion: validSourceTargetVersion,
						InputSchema:         gptr.Of([]byte("[]")),
						OutputSchema:        gptr.Of([]byte("[]")),
						TargetMeta:          gptr.Of([]byte("{}")),
					}, nil)

				// Mock latest write tracker check for target
				mockLWT.EXPECT().
					CheckWriteFlagByID(gomock.Any(), platestwrite.ResourceTypeTarget, validTargetID).
					Return(false)

				// Mock get target
				mockEvalTargetDao.EXPECT().
					GetEvalTarget(gomock.Any(), validTargetID, gomock.Any()).
					Return(&model.Target{
						ID:             validTargetID,
						SpaceID:        validSpaceID,
						SourceTargetID: validSourceTargetID,
						TargetType:     validEvalTargetType,
					}, nil)
			},
			want: &entity.EvalTarget{
				ID:             validTargetID,
				SpaceID:        validSpaceID,
				SourceTargetID: validSourceTargetID,
				EvalTargetType: entity.EvalTargetType(validEvalTargetType),
				EvalTargetVersion: &entity.EvalTargetVersion{
					ID:                  validVersionID,
					SpaceID:             validSpaceID,
					TargetID:            validTargetID,
					SourceTargetVersion: validSourceTargetVersion,
				},
			},
			wantErr: false,
		},
		{
			name:      "error - version not found",
			spaceID:   validSpaceID,
			versionID: validVersionID,
			mockSetup: func() {
				// Mock latest write tracker check for version
				mockLWT.EXPECT().
					CheckWriteFlagByID(gomock.Any(), platestwrite.ResourceTypeTargetVersion, validVersionID).
					Return(false)

				// Mock get version returns nil
				mockEvalTargetVersionDao.EXPECT().
					GetEvalTargetVersion(gomock.Any(), validSpaceID, validVersionID, gomock.Any()).
					Return(nil, nil)
			},
			want:        nil,
			wantErr:     true,
			wantErrCode: errno.ResourceNotFoundCode,
		},
		{
			name:      "error - target not found",
			spaceID:   validSpaceID,
			versionID: validVersionID,
			mockSetup: func() {
				// Mock latest write tracker check for version
				mockLWT.EXPECT().
					CheckWriteFlagByID(gomock.Any(), platestwrite.ResourceTypeTargetVersion, validVersionID).
					Return(false)

				// Mock get version
				mockEvalTargetVersionDao.EXPECT().
					GetEvalTargetVersion(gomock.Any(), validSpaceID, validVersionID, gomock.Any()).
					Return(&model.TargetVersion{
						ID:                  validVersionID,
						SpaceID:             validSpaceID,
						TargetID:            validTargetID,
						SourceTargetVersion: validSourceTargetVersion,
					}, nil)

				// Mock latest write tracker check for target
				mockLWT.EXPECT().
					CheckWriteFlagByID(gomock.Any(), platestwrite.ResourceTypeTarget, validTargetID).
					Return(false)

				// Mock get target returns nil
				mockEvalTargetDao.EXPECT().
					GetEvalTarget(gomock.Any(), validTargetID, gomock.Any()).
					Return(nil, nil)
			},
			want:        nil,
			wantErr:     true,
			wantErrCode: errno.ResourceNotFoundCode,
		},
		{
			name:      "error - version dao error",
			spaceID:   validSpaceID,
			versionID: validVersionID,
			mockSetup: func() {
				// Mock latest write tracker check for version
				mockLWT.EXPECT().
					CheckWriteFlagByID(gomock.Any(), platestwrite.ResourceTypeTargetVersion, validVersionID).
					Return(false)

				// Mock get version returns error
				mockEvalTargetVersionDao.EXPECT().
					GetEvalTargetVersion(gomock.Any(), validSpaceID, validVersionID, gomock.Any()).
					Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			want:        nil,
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name:      "error - target dao error",
			spaceID:   validSpaceID,
			versionID: validVersionID,
			mockSetup: func() {
				// Mock latest write tracker check for version
				mockLWT.EXPECT().
					CheckWriteFlagByID(gomock.Any(), platestwrite.ResourceTypeTargetVersion, validVersionID).
					Return(false)

				// Mock get version
				mockEvalTargetVersionDao.EXPECT().
					GetEvalTargetVersion(gomock.Any(), validSpaceID, validVersionID, gomock.Any()).
					Return(&model.TargetVersion{
						ID:                  validVersionID,
						SpaceID:             validSpaceID,
						TargetID:            validTargetID,
						SourceTargetVersion: validSourceTargetVersion,
					}, nil)

				// Mock latest write tracker check for target
				mockLWT.EXPECT().
					CheckWriteFlagByID(gomock.Any(), platestwrite.ResourceTypeTarget, validTargetID).
					Return(false)

				// Mock get target returns error
				mockEvalTargetDao.EXPECT().
					GetEvalTarget(gomock.Any(), validTargetID, gomock.Any()).
					Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			want:        nil,
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			got, err := repo.GetEvalTargetVersion(context.Background(), tt.spaceID, tt.versionID)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					if ok {
						assert.Equal(t, tt.wantErrCode, statusErr.Code())
					}
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want.ID, got.ID)
				assert.Equal(t, tt.want.EvalTargetVersion.ID, got.EvalTargetVersion.ID)
			}
		})
	}
}

func TestEvalTargetRepoImpl_CreateEvalTargetRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockEvalTargetDao := mysqlmocks.NewMockEvalTargetDAO(ctrl)
	mockEvalTargetVersionDao := mysqlmocks.NewMockEvalTargetVersionDAO(ctrl)
	mockEvalTargetRecordDao := mysqlmocks.NewMockEvalTargetRecordDAO(ctrl)
	mockIDGen := idgen.NewMockIIDGenerator(ctrl)
	mockDBProvider := dbmock.NewMockProvider(ctrl)
	mockLWT := platestwrite_mocks.NewMockILatestWriteTracker(ctrl)

	repo := &EvalTargetRepoImpl{
		evalTargetDao:        mockEvalTargetDao,
		evalTargetVersionDao: mockEvalTargetVersionDao,
		evalTargetRecordDao:  mockEvalTargetRecordDao,
		idgen:                mockIDGen,
		dbProvider:           mockDBProvider,
		lwt:                  mockLWT,
	}

	// Test data
	validSpaceID := int64(123)
	validTargetID := int64(456)
	validVersionID := int64(789)
	validRecordID := int64(101)

	tests := []struct {
		name        string
		record      *entity.EvalTargetRecord
		mockSetup   func()
		wantID      int64
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - create record",
			record: &entity.EvalTargetRecord{
				SpaceID:              validSpaceID,
				TargetID:             validTargetID,
				TargetVersionID:      validVersionID,
				EvalTargetInputData:  &entity.EvalTargetInputData{},
				EvalTargetOutputData: &entity.EvalTargetOutputData{},
				BaseInfo: &entity.BaseInfo{
					CreatedBy: &entity.UserInfo{},
					UpdatedBy: &entity.UserInfo{},
					CreatedAt: gptr.Of(time.Now().UnixMilli()),
					UpdatedAt: gptr.Of(time.Now().UnixMilli()),
				},
			},
			mockSetup: func() {
				// Mock create record
				mockEvalTargetRecordDao.EXPECT().
					Create(gomock.Any(), gomock.Any()).
					Return(validRecordID, nil)
			},
			wantID:  validRecordID,
			wantErr: false,
		},
		{
			name: "error - create record failed",
			record: &entity.EvalTargetRecord{
				SpaceID:         validSpaceID,
				TargetID:        validTargetID,
				TargetVersionID: validVersionID,
				BaseInfo: &entity.BaseInfo{
					CreatedBy: &entity.UserInfo{},
					UpdatedBy: &entity.UserInfo{},
					CreatedAt: gptr.Of(time.Now().UnixMilli()),
					UpdatedAt: gptr.Of(time.Now().UnixMilli()),
				},
			},
			mockSetup: func() {
				// Mock create record returns error
				mockEvalTargetRecordDao.EXPECT().
					Create(gomock.Any(), gomock.Any()).
					Return(int64(0), errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantID:      0,
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			got, err := repo.CreateEvalTargetRecord(context.Background(), tt.record)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					if ok {
						assert.Equal(t, tt.wantErrCode, statusErr.Code())
					}
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantID, got)
			}
		})
	}
}

func TestEvalTargetRepoImpl_GetEvalTargetRecordByIDAndSpaceID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockEvalTargetDao := mysqlmocks.NewMockEvalTargetDAO(ctrl)
	mockEvalTargetVersionDao := mysqlmocks.NewMockEvalTargetVersionDAO(ctrl)
	mockEvalTargetRecordDao := mysqlmocks.NewMockEvalTargetRecordDAO(ctrl)
	mockIDGen := idgen.NewMockIIDGenerator(ctrl)
	mockDBProvider := dbmock.NewMockProvider(ctrl)
	mockLWT := platestwrite_mocks.NewMockILatestWriteTracker(ctrl)

	repo := &EvalTargetRepoImpl{
		evalTargetDao:        mockEvalTargetDao,
		evalTargetVersionDao: mockEvalTargetVersionDao,
		evalTargetRecordDao:  mockEvalTargetRecordDao,
		idgen:                mockIDGen,
		dbProvider:           mockDBProvider,
		lwt:                  mockLWT,
	}

	// Test data
	validSpaceID := int64(123)
	validRecordID := int64(456)
	validTargetID := int64(789)
	validVersionID := int64(101)

	tests := []struct {
		name        string
		spaceID     int64
		recordID    int64
		mockSetup   func()
		want        *entity.EvalTargetRecord
		wantErr     bool
		wantErrCode int32
	}{
		{
			name:     "success - record exists",
			spaceID:  validSpaceID,
			recordID: validRecordID,
			mockSetup: func() {
				// Mock get record
				mockEvalTargetRecordDao.EXPECT().
					GetByIDAndSpaceID(gomock.Any(), validRecordID, validSpaceID).
					Return(&model.TargetRecord{
						ID:              validRecordID,
						SpaceID:         validSpaceID,
						TargetID:        validTargetID,
						TargetVersionID: validVersionID,
						InputData:       gptr.Of([]byte("{}")),
						OutputData:      gptr.Of([]byte("{}")),
					}, nil)
			},
			want: &entity.EvalTargetRecord{
				ID:                   validRecordID,
				SpaceID:              validSpaceID,
				TargetID:             validTargetID,
				TargetVersionID:      validVersionID,
				EvalTargetInputData:  &entity.EvalTargetInputData{},
				EvalTargetOutputData: &entity.EvalTargetOutputData{},
			},
			wantErr: false,
		},
		{
			name:     "success - record not found",
			spaceID:  validSpaceID,
			recordID: validRecordID,
			mockSetup: func() {
				// Mock get record returns nil
				mockEvalTargetRecordDao.EXPECT().
					GetByIDAndSpaceID(gomock.Any(), validRecordID, validSpaceID).
					Return(nil, nil)
			},
			want:    nil,
			wantErr: false,
		},
		{
			name:     "error - dao error",
			spaceID:  validSpaceID,
			recordID: validRecordID,
			mockSetup: func() {
				// Mock get record returns error
				mockEvalTargetRecordDao.EXPECT().
					GetByIDAndSpaceID(gomock.Any(), validRecordID, validSpaceID).
					Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			want:        nil,
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name:     "error - convert to DO failed",
			spaceID:  validSpaceID,
			recordID: validRecordID,
			mockSetup: func() {
				// Mock get record returns invalid data
				mockEvalTargetRecordDao.EXPECT().
					GetByIDAndSpaceID(gomock.Any(), validRecordID, validSpaceID).
					Return(&model.TargetRecord{
						ID:              validRecordID,
						SpaceID:         validSpaceID,
						TargetID:        validTargetID,
						TargetVersionID: validVersionID,
						// Invalid data to trigger conversion error
						InputData:  gptr.Of([]byte("1")),
						OutputData: gptr.Of([]byte("1")),
					}, nil)
			},
			want:        nil,
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			got, err := repo.GetEvalTargetRecordByIDAndSpaceID(context.Background(), tt.spaceID, tt.recordID)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					if ok {
						assert.Equal(t, tt.wantErrCode, statusErr.Code())
					}
				}
			} else {
				assert.NoError(t, err)
				if tt.want == nil {
					assert.Nil(t, got)
				} else {
					assert.Equal(t, tt.want.ID, got.ID)
					assert.Equal(t, tt.want.SpaceID, got.SpaceID)
					assert.Equal(t, tt.want.TargetID, got.TargetID)
					assert.Equal(t, tt.want.TargetVersionID, got.TargetVersionID)
				}
			}
		})
	}
}

func TestEvalTargetRepoImpl_BatchGetEvalTargetBySource(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockEvalTargetDao := mysqlmocks.NewMockEvalTargetDAO(ctrl)
	mockEvalTargetVersionDao := mysqlmocks.NewMockEvalTargetVersionDAO(ctrl)
	mockEvalTargetRecordDao := mysqlmocks.NewMockEvalTargetRecordDAO(ctrl)
	mockIDGen := idgen.NewMockIIDGenerator(ctrl)
	mockDBProvider := dbmock.NewMockProvider(ctrl)
	mockLWT := platestwrite_mocks.NewMockILatestWriteTracker(ctrl)

	repo := &EvalTargetRepoImpl{
		evalTargetDao:        mockEvalTargetDao,
		evalTargetVersionDao: mockEvalTargetVersionDao,
		evalTargetRecordDao:  mockEvalTargetRecordDao,
		idgen:                mockIDGen,
		dbProvider:           mockDBProvider,
		lwt:                  mockLWT,
	}

	// Test data
	validSpaceID := int64(123)
	validSourceTargetIDs := []string{"source-1", "source-2"}
	validTargetType := entity.EvalTargetTypeLoopPrompt
	validTargets := []*model.Target{
		{
			ID:             456,
			SpaceID:        validSpaceID,
			SourceTargetID: "source-1",
			TargetType:     int32(validTargetType),
		},
		{
			ID:             789,
			SpaceID:        validSpaceID,
			SourceTargetID: "source-2",
			TargetType:     int32(validTargetType),
		},
	}

	tests := []struct {
		name      string
		param     *repointerface.BatchGetEvalTargetBySourceParam
		mockSetup func()
		wantLen   int
		wantErr   bool
	}{
		{
			name: "success - targets found",
			param: &repointerface.BatchGetEvalTargetBySourceParam{
				SpaceID:        validSpaceID,
				SourceTargetID: validSourceTargetIDs,
				TargetType:     validTargetType,
			},
			mockSetup: func() {
				mockEvalTargetDao.EXPECT().
					BatchGetEvalTargetBySource(gomock.Any(), validSpaceID, validSourceTargetIDs, int32(validTargetType)).
					Return(validTargets, nil)
			},
			wantLen: 2,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.mockSetup()

			got, err := repo.BatchGetEvalTargetBySource(context.Background(), tt.param)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.wantLen == 0 {
					assert.Nil(t, got)
				} else {
					assert.Len(t, got, tt.wantLen)
				}
			}
		})
	}
}

func TestEvalTargetRepoImpl_GetEvalTargetVersionByTarget(t *testing.T) {
	// Test data
	validSpaceID := int64(123)
	validTargetID := int64(456)
	validVersionID := int64(789)
	validSourceTargetVersion := "v1.0"
	validSourceTargetID := "source-123"
	validEvalTargetType := int32(1)

	tests := []struct {
		name                string
		spaceID             int64
		targetID            int64
		sourceTargetVersion string
		mockSetup           func(*gomock.Controller) *EvalTargetRepoImpl
		want                *entity.EvalTarget
		wantErr             bool
		wantErrCode         int32
	}{
		{
			name:                "success - version found",
			spaceID:             validSpaceID,
			targetID:            validTargetID,
			sourceTargetVersion: validSourceTargetVersion,
			mockSetup: func(ctrl *gomock.Controller) *EvalTargetRepoImpl {
				mockEvalTargetDao := mysqlmocks.NewMockEvalTargetDAO(ctrl)
				mockEvalTargetVersionDao := mysqlmocks.NewMockEvalTargetVersionDAO(ctrl)
				mockEvalTargetRecordDao := mysqlmocks.NewMockEvalTargetRecordDAO(ctrl)
				mockIDGen := idgen.NewMockIIDGenerator(ctrl)
				mockDBProvider := dbmock.NewMockProvider(ctrl)
				mockLWT := platestwrite_mocks.NewMockILatestWriteTracker(ctrl)

				// Mock latest write tracker check for version
				mockLWT.EXPECT().
					CheckWriteFlagByID(gomock.Any(), platestwrite.ResourceTypeTargetVersion, validTargetID).
					Return(false)

				// Mock get version by target
				mockEvalTargetVersionDao.EXPECT().
					GetEvalTargetVersionByTarget(gomock.Any(), validSpaceID, validTargetID, validSourceTargetVersion, gomock.Any()).
					Return(&model.TargetVersion{
						ID:                  validVersionID,
						SpaceID:             validSpaceID,
						TargetID:            validTargetID,
						SourceTargetVersion: validSourceTargetVersion,
						InputSchema:         gptr.Of([]byte("[]")),
						OutputSchema:        gptr.Of([]byte("[]")),
						TargetMeta:          gptr.Of([]byte("{}")),
					}, nil)

				// Mock latest write tracker check for target
				mockLWT.EXPECT().
					CheckWriteFlagByID(gomock.Any(), platestwrite.ResourceTypeTarget, validTargetID).
					Return(false)

				// Mock get target
				mockEvalTargetDao.EXPECT().
					GetEvalTarget(gomock.Any(), validTargetID, gomock.Any()).
					Return(&model.Target{
						ID:             validTargetID,
						SpaceID:        validSpaceID,
						SourceTargetID: validSourceTargetID,
						TargetType:     validEvalTargetType,
					}, nil)

				return &EvalTargetRepoImpl{
					evalTargetDao:        mockEvalTargetDao,
					evalTargetVersionDao: mockEvalTargetVersionDao,
					evalTargetRecordDao:  mockEvalTargetRecordDao,
					idgen:                mockIDGen,
					dbProvider:           mockDBProvider,
					lwt:                  mockLWT,
				}
			},
			want: &entity.EvalTarget{
				ID:             validTargetID,
				SpaceID:        validSpaceID,
				SourceTargetID: validSourceTargetID,
				EvalTargetType: entity.EvalTargetType(validEvalTargetType),
				EvalTargetVersion: &entity.EvalTargetVersion{
					ID:                  validVersionID,
					SpaceID:             validSpaceID,
					TargetID:            validTargetID,
					SourceTargetVersion: validSourceTargetVersion,
				},
			},
			wantErr: false,
		},
		{
			name:                "success - version not found",
			spaceID:             validSpaceID,
			targetID:            validTargetID,
			sourceTargetVersion: validSourceTargetVersion,
			mockSetup: func(ctrl *gomock.Controller) *EvalTargetRepoImpl {
				mockEvalTargetDao := mysqlmocks.NewMockEvalTargetDAO(ctrl)
				mockEvalTargetVersionDao := mysqlmocks.NewMockEvalTargetVersionDAO(ctrl)
				mockEvalTargetRecordDao := mysqlmocks.NewMockEvalTargetRecordDAO(ctrl)
				mockIDGen := idgen.NewMockIIDGenerator(ctrl)
				mockDBProvider := dbmock.NewMockProvider(ctrl)
				mockLWT := platestwrite_mocks.NewMockILatestWriteTracker(ctrl)

				// Mock latest write tracker check for version
				mockLWT.EXPECT().
					CheckWriteFlagByID(gomock.Any(), platestwrite.ResourceTypeTargetVersion, validTargetID).
					Return(false)

				// Mock get version by target returns nil
				mockEvalTargetVersionDao.EXPECT().
					GetEvalTargetVersionByTarget(gomock.Any(), validSpaceID, validTargetID, validSourceTargetVersion, gomock.Any()).
					Return(nil, nil)

				return &EvalTargetRepoImpl{
					evalTargetDao:        mockEvalTargetDao,
					evalTargetVersionDao: mockEvalTargetVersionDao,
					evalTargetRecordDao:  mockEvalTargetRecordDao,
					idgen:                mockIDGen,
					dbProvider:           mockDBProvider,
					lwt:                  mockLWT,
				}
			},
			want:    nil,
			wantErr: false,
		},
		{
			name:                "error - target not found",
			spaceID:             validSpaceID,
			targetID:            validTargetID,
			sourceTargetVersion: validSourceTargetVersion,
			mockSetup: func(ctrl *gomock.Controller) *EvalTargetRepoImpl {
				mockEvalTargetDao := mysqlmocks.NewMockEvalTargetDAO(ctrl)
				mockEvalTargetVersionDao := mysqlmocks.NewMockEvalTargetVersionDAO(ctrl)
				mockEvalTargetRecordDao := mysqlmocks.NewMockEvalTargetRecordDAO(ctrl)
				mockIDGen := idgen.NewMockIIDGenerator(ctrl)
				mockDBProvider := dbmock.NewMockProvider(ctrl)
				mockLWT := platestwrite_mocks.NewMockILatestWriteTracker(ctrl)

				// Mock latest write tracker check for version
				mockLWT.EXPECT().
					CheckWriteFlagByID(gomock.Any(), platestwrite.ResourceTypeTargetVersion, validTargetID).
					Return(false)

				// Mock get version by target
				mockEvalTargetVersionDao.EXPECT().
					GetEvalTargetVersionByTarget(gomock.Any(), validSpaceID, validTargetID, validSourceTargetVersion, gomock.Any()).
					Return(&model.TargetVersion{
						ID:                  validVersionID,
						SpaceID:             validSpaceID,
						TargetID:            validTargetID,
						SourceTargetVersion: validSourceTargetVersion,
					}, nil)

				// Mock latest write tracker check for target
				mockLWT.EXPECT().
					CheckWriteFlagByID(gomock.Any(), platestwrite.ResourceTypeTarget, validTargetID).
					Return(false)

				// Mock get target returns nil
				mockEvalTargetDao.EXPECT().
					GetEvalTarget(gomock.Any(), validTargetID, gomock.Any()).
					Return(nil, nil)

				return &EvalTargetRepoImpl{
					evalTargetDao:        mockEvalTargetDao,
					evalTargetVersionDao: mockEvalTargetVersionDao,
					evalTargetRecordDao:  mockEvalTargetRecordDao,
					idgen:                mockIDGen,
					dbProvider:           mockDBProvider,
					lwt:                  mockLWT,
				}
			},
			want:        nil,
			wantErr:     true,
			wantErrCode: errno.ResourceNotFoundCode,
		},
		{
			name:                "error - version dao error",
			spaceID:             validSpaceID,
			targetID:            validTargetID,
			sourceTargetVersion: validSourceTargetVersion,
			mockSetup: func(ctrl *gomock.Controller) *EvalTargetRepoImpl {
				mockEvalTargetDao := mysqlmocks.NewMockEvalTargetDAO(ctrl)
				mockEvalTargetVersionDao := mysqlmocks.NewMockEvalTargetVersionDAO(ctrl)
				mockEvalTargetRecordDao := mysqlmocks.NewMockEvalTargetRecordDAO(ctrl)
				mockIDGen := idgen.NewMockIIDGenerator(ctrl)
				mockDBProvider := dbmock.NewMockProvider(ctrl)
				mockLWT := platestwrite_mocks.NewMockILatestWriteTracker(ctrl)

				// Mock latest write tracker check for version
				mockLWT.EXPECT().
					CheckWriteFlagByID(gomock.Any(), platestwrite.ResourceTypeTargetVersion, validTargetID).
					Return(false)

				// Mock get version by target returns error
				mockEvalTargetVersionDao.EXPECT().
					GetEvalTargetVersionByTarget(gomock.Any(), validSpaceID, validTargetID, validSourceTargetVersion, gomock.Any()).
					Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))

				return &EvalTargetRepoImpl{
					evalTargetDao:        mockEvalTargetDao,
					evalTargetVersionDao: mockEvalTargetVersionDao,
					evalTargetRecordDao:  mockEvalTargetRecordDao,
					idgen:                mockIDGen,
					dbProvider:           mockDBProvider,
					lwt:                  mockLWT,
				}
			},
			want:        nil,
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := tt.mockSetup(ctrl)
			got, err := repo.GetEvalTargetVersionByTarget(context.Background(), tt.spaceID, tt.targetID, tt.sourceTargetVersion)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					if ok {
						assert.Equal(t, tt.wantErrCode, statusErr.Code())
					}
				}
			} else {
				assert.NoError(t, err)
				if tt.want == nil {
					assert.Nil(t, got)
				} else {
					assert.Equal(t, tt.want.ID, got.ID)
					assert.Equal(t, tt.want.EvalTargetVersion.ID, got.EvalTargetVersion.ID)
				}
			}
		})
	}
}

func TestEvalTargetRepoImpl_GetEvalTargetVersionBySourceTarget(t *testing.T) {
	// Test data
	validSpaceID := int64(123)
	validSourceTargetID := "source-123"
	validSourceTargetVersion := "v1.0"
	validTargetType := entity.EvalTargetTypeLoopPrompt
	validEvalTargetType := int32(validTargetType)
	validTargetID := int64(456)
	validVersionID := int64(789)

	tests := []struct {
		name                string
		spaceID             int64
		sourceTargetID      string
		sourceTargetVersion string
		targetType          entity.EvalTargetType
		mockSetup           func(*gomock.Controller) *EvalTargetRepoImpl
		want                *entity.EvalTarget
		wantErr             bool
	}{
		{
			name:                "success - target not found",
			spaceID:             validSpaceID,
			sourceTargetID:      validSourceTargetID,
			sourceTargetVersion: validSourceTargetVersion,
			targetType:          validTargetType,
			mockSetup: func(ctrl *gomock.Controller) *EvalTargetRepoImpl {
				mockEvalTargetDao := mysqlmocks.NewMockEvalTargetDAO(ctrl)
				mockEvalTargetVersionDao := mysqlmocks.NewMockEvalTargetVersionDAO(ctrl)
				mockEvalTargetRecordDao := mysqlmocks.NewMockEvalTargetRecordDAO(ctrl)
				mockIDGen := idgen.NewMockIIDGenerator(ctrl)
				mockDBProvider := dbmock.NewMockProvider(ctrl)
				mockLWT := platestwrite_mocks.NewMockILatestWriteTracker(ctrl)

				// Mock get target by source ID returns nil
				mockEvalTargetDao.EXPECT().
					GetEvalTargetBySourceID(gomock.Any(), validSpaceID, validSourceTargetID, validEvalTargetType).
					Return(nil, nil)

				return &EvalTargetRepoImpl{
					evalTargetDao:        mockEvalTargetDao,
					evalTargetVersionDao: mockEvalTargetVersionDao,
					evalTargetRecordDao:  mockEvalTargetRecordDao,
					idgen:                mockIDGen,
					dbProvider:           mockDBProvider,
					lwt:                  mockLWT,
				}
			},
			want:    nil,
			wantErr: false,
		},
		{
			name:                "error - target dao error",
			spaceID:             validSpaceID,
			sourceTargetID:      validSourceTargetID,
			sourceTargetVersion: validSourceTargetVersion,
			targetType:          validTargetType,
			mockSetup: func(ctrl *gomock.Controller) *EvalTargetRepoImpl {
				mockEvalTargetDao := mysqlmocks.NewMockEvalTargetDAO(ctrl)
				mockEvalTargetVersionDao := mysqlmocks.NewMockEvalTargetVersionDAO(ctrl)
				mockEvalTargetRecordDao := mysqlmocks.NewMockEvalTargetRecordDAO(ctrl)
				mockIDGen := idgen.NewMockIIDGenerator(ctrl)
				mockDBProvider := dbmock.NewMockProvider(ctrl)
				mockLWT := platestwrite_mocks.NewMockILatestWriteTracker(ctrl)

				// Mock get target by source ID returns error
				mockEvalTargetDao.EXPECT().
					GetEvalTargetBySourceID(gomock.Any(), validSpaceID, validSourceTargetID, validEvalTargetType).
					Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))

				return &EvalTargetRepoImpl{
					evalTargetDao:        mockEvalTargetDao,
					evalTargetVersionDao: mockEvalTargetVersionDao,
					evalTargetRecordDao:  mockEvalTargetRecordDao,
					idgen:                mockIDGen,
					dbProvider:           mockDBProvider,
					lwt:                  mockLWT,
				}
			},
			want:    nil,
			wantErr: true,
		},
		{
			name:                "success - target found but version not found",
			spaceID:             validSpaceID,
			sourceTargetID:      validSourceTargetID,
			sourceTargetVersion: validSourceTargetVersion,
			targetType:          validTargetType,
			mockSetup: func(ctrl *gomock.Controller) *EvalTargetRepoImpl {
				mockEvalTargetDao := mysqlmocks.NewMockEvalTargetDAO(ctrl)
				mockEvalTargetVersionDao := mysqlmocks.NewMockEvalTargetVersionDAO(ctrl)
				mockEvalTargetRecordDao := mysqlmocks.NewMockEvalTargetRecordDAO(ctrl)
				mockIDGen := idgen.NewMockIIDGenerator(ctrl)
				mockDBProvider := dbmock.NewMockProvider(ctrl)
				mockLWT := platestwrite_mocks.NewMockILatestWriteTracker(ctrl)

				// Mock get target by source ID returns target
				mockEvalTargetDao.EXPECT().
					GetEvalTargetBySourceID(gomock.Any(), validSpaceID, validSourceTargetID, validEvalTargetType).
					Return(&model.Target{
						ID:             validTargetID,
						SpaceID:        validSpaceID,
						SourceTargetID: validSourceTargetID,
						TargetType:     validEvalTargetType,
					}, nil)

				// Mock latest write tracker check
				mockLWT.EXPECT().
					CheckWriteFlagByID(gomock.Any(), platestwrite.ResourceTypeTargetVersion, validTargetID).
					Return(false)

				// Mock get version by target returns nil
				mockEvalTargetVersionDao.EXPECT().
					GetEvalTargetVersionByTarget(gomock.Any(), validSpaceID, validTargetID, validSourceTargetVersion).
					Return(nil, nil)

				return &EvalTargetRepoImpl{
					evalTargetDao:        mockEvalTargetDao,
					evalTargetVersionDao: mockEvalTargetVersionDao,
					evalTargetRecordDao:  mockEvalTargetRecordDao,
					idgen:                mockIDGen,
					dbProvider:           mockDBProvider,
					lwt:                  mockLWT,
				}
			},
			want:    nil,
			wantErr: false,
		},
		{
			name:                "success - target and version found",
			spaceID:             validSpaceID,
			sourceTargetID:      validSourceTargetID,
			sourceTargetVersion: validSourceTargetVersion,
			targetType:          validTargetType,
			mockSetup: func(ctrl *gomock.Controller) *EvalTargetRepoImpl {
				mockEvalTargetDao := mysqlmocks.NewMockEvalTargetDAO(ctrl)
				mockEvalTargetVersionDao := mysqlmocks.NewMockEvalTargetVersionDAO(ctrl)
				mockEvalTargetRecordDao := mysqlmocks.NewMockEvalTargetRecordDAO(ctrl)
				mockIDGen := idgen.NewMockIIDGenerator(ctrl)
				mockDBProvider := dbmock.NewMockProvider(ctrl)
				mockLWT := platestwrite_mocks.NewMockILatestWriteTracker(ctrl)

				// Mock get target by source ID returns target
				mockEvalTargetDao.EXPECT().
					GetEvalTargetBySourceID(gomock.Any(), validSpaceID, validSourceTargetID, validEvalTargetType).
					Return(&model.Target{
						ID:             validTargetID,
						SpaceID:        validSpaceID,
						SourceTargetID: validSourceTargetID,
						TargetType:     validEvalTargetType,
					}, nil)

				// Mock latest write tracker check
				mockLWT.EXPECT().
					CheckWriteFlagByID(gomock.Any(), platestwrite.ResourceTypeTargetVersion, validTargetID).
					Return(false)

				// Mock get version by target returns version
				mockEvalTargetVersionDao.EXPECT().
					GetEvalTargetVersionByTarget(gomock.Any(), validSpaceID, validTargetID, validSourceTargetVersion).
					Return(&model.TargetVersion{
						ID:                  validVersionID,
						SpaceID:             validSpaceID,
						TargetID:            validTargetID,
						SourceTargetVersion: validSourceTargetVersion,
						InputSchema:         gptr.Of([]byte("[]")),
						OutputSchema:        gptr.Of([]byte("[]")),
						TargetMeta:          gptr.Of([]byte("{}")),
					}, nil)

				return &EvalTargetRepoImpl{
					evalTargetDao:        mockEvalTargetDao,
					evalTargetVersionDao: mockEvalTargetVersionDao,
					evalTargetRecordDao:  mockEvalTargetRecordDao,
					idgen:                mockIDGen,
					dbProvider:           mockDBProvider,
					lwt:                  mockLWT,
				}
			},
			want: &entity.EvalTarget{
				ID:             validTargetID,
				SpaceID:        validSpaceID,
				SourceTargetID: validSourceTargetID,
				EvalTargetType: validTargetType,
				EvalTargetVersion: &entity.EvalTargetVersion{
					ID:                  validVersionID,
					SpaceID:             validSpaceID,
					TargetID:            validTargetID,
					SourceTargetVersion: validSourceTargetVersion,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := tt.mockSetup(ctrl)

			got, err := repo.GetEvalTargetVersionBySourceTarget(context.Background(), tt.spaceID, tt.sourceTargetID, tt.sourceTargetVersion, tt.targetType)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.want == nil {
					assert.Nil(t, got)
				} else {
					assert.Equal(t, tt.want.ID, got.ID)
					assert.Equal(t, tt.want.EvalTargetVersion.ID, got.EvalTargetVersion.ID)
				}
			}
		})
	}
}

func TestEvalTargetRepoImpl_BatchGetEvalTargetVersion(t *testing.T) {
	// Test data
	validSpaceID := int64(123)
	validVersionIDs := []int64{456, 789}
	validTargetID1 := int64(111)
	validTargetID2 := int64(222)
	validEvalTargetType := int32(1)

	validVersions := []*model.TargetVersion{
		{
			ID:                  validVersionIDs[0],
			SpaceID:             validSpaceID,
			TargetID:            validTargetID1,
			SourceTargetVersion: "v1.0",
			InputSchema:         gptr.Of([]byte("[]")),
			OutputSchema:        gptr.Of([]byte("[]")),
			TargetMeta:          gptr.Of([]byte("{}")),
		},
		{
			ID:                  validVersionIDs[1],
			SpaceID:             validSpaceID,
			TargetID:            validTargetID2,
			SourceTargetVersion: "v1.1",
			InputSchema:         gptr.Of([]byte("[]")),
			OutputSchema:        gptr.Of([]byte("[]")),
			TargetMeta:          gptr.Of([]byte("{}")),
		},
	}

	validTargets := []*model.Target{
		{
			ID:             validTargetID1,
			SpaceID:        validSpaceID,
			SourceTargetID: "source-1",
			TargetType:     validEvalTargetType,
		},
		{
			ID:             validTargetID2,
			SpaceID:        validSpaceID,
			SourceTargetID: "source-2",
			TargetType:     validEvalTargetType,
		},
	}

	tests := []struct {
		name       string
		spaceID    int64
		versionIDs []int64
		mockSetup  func(*gomock.Controller) *EvalTargetRepoImpl
		wantLen    int
		wantErr    bool
	}{
		{
			name:       "success - versions found",
			spaceID:    validSpaceID,
			versionIDs: validVersionIDs,
			mockSetup: func(ctrl *gomock.Controller) *EvalTargetRepoImpl {
				mockEvalTargetDao := mysqlmocks.NewMockEvalTargetDAO(ctrl)
				mockEvalTargetVersionDao := mysqlmocks.NewMockEvalTargetVersionDAO(ctrl)
				mockEvalTargetRecordDao := mysqlmocks.NewMockEvalTargetRecordDAO(ctrl)
				mockIDGen := idgen.NewMockIIDGenerator(ctrl)
				mockDBProvider := dbmock.NewMockProvider(ctrl)
				mockLWT := platestwrite_mocks.NewMockILatestWriteTracker(ctrl)

				// Mock batch get versions
				mockEvalTargetVersionDao.EXPECT().
					BatchGetEvalTargetVersion(gomock.Any(), validSpaceID, validVersionIDs).
					Return(validVersions, nil)

				// Mock batch get targets
				mockEvalTargetDao.EXPECT().
					BatchGetEvalTarget(gomock.Any(), validSpaceID, []int64{validTargetID1, validTargetID2}).
					Return(validTargets, nil)

				return &EvalTargetRepoImpl{
					evalTargetDao:        mockEvalTargetDao,
					evalTargetVersionDao: mockEvalTargetVersionDao,
					evalTargetRecordDao:  mockEvalTargetRecordDao,
					idgen:                mockIDGen,
					dbProvider:           mockDBProvider,
					lwt:                  mockLWT,
				}
			},
			wantLen: 2,
			wantErr: false,
		},
		{
			name:       "success - no versions found",
			spaceID:    validSpaceID,
			versionIDs: validVersionIDs,
			mockSetup: func(ctrl *gomock.Controller) *EvalTargetRepoImpl {
				mockEvalTargetDao := mysqlmocks.NewMockEvalTargetDAO(ctrl)
				mockEvalTargetVersionDao := mysqlmocks.NewMockEvalTargetVersionDAO(ctrl)
				mockEvalTargetRecordDao := mysqlmocks.NewMockEvalTargetRecordDAO(ctrl)
				mockIDGen := idgen.NewMockIIDGenerator(ctrl)
				mockDBProvider := dbmock.NewMockProvider(ctrl)
				mockLWT := platestwrite_mocks.NewMockILatestWriteTracker(ctrl)

				// Mock batch get versions returns empty
				mockEvalTargetVersionDao.EXPECT().
					BatchGetEvalTargetVersion(gomock.Any(), validSpaceID, validVersionIDs).
					Return([]*model.TargetVersion{}, nil)

				return &EvalTargetRepoImpl{
					evalTargetDao:        mockEvalTargetDao,
					evalTargetVersionDao: mockEvalTargetVersionDao,
					evalTargetRecordDao:  mockEvalTargetRecordDao,
					idgen:                mockIDGen,
					dbProvider:           mockDBProvider,
					lwt:                  mockLWT,
				}
			},
			wantLen: 0,
			wantErr: false,
		},
		{
			name:       "success - no targets found",
			spaceID:    validSpaceID,
			versionIDs: validVersionIDs,
			mockSetup: func(ctrl *gomock.Controller) *EvalTargetRepoImpl {
				mockEvalTargetDao := mysqlmocks.NewMockEvalTargetDAO(ctrl)
				mockEvalTargetVersionDao := mysqlmocks.NewMockEvalTargetVersionDAO(ctrl)
				mockEvalTargetRecordDao := mysqlmocks.NewMockEvalTargetRecordDAO(ctrl)
				mockIDGen := idgen.NewMockIIDGenerator(ctrl)
				mockDBProvider := dbmock.NewMockProvider(ctrl)
				mockLWT := platestwrite_mocks.NewMockILatestWriteTracker(ctrl)

				// Mock batch get versions
				mockEvalTargetVersionDao.EXPECT().
					BatchGetEvalTargetVersion(gomock.Any(), validSpaceID, validVersionIDs).
					Return(validVersions, nil)

				// Mock batch get targets returns empty
				mockEvalTargetDao.EXPECT().
					BatchGetEvalTarget(gomock.Any(), validSpaceID, []int64{validTargetID1, validTargetID2}).
					Return([]*model.Target{}, nil)

				return &EvalTargetRepoImpl{
					evalTargetDao:        mockEvalTargetDao,
					evalTargetVersionDao: mockEvalTargetVersionDao,
					evalTargetRecordDao:  mockEvalTargetRecordDao,
					idgen:                mockIDGen,
					dbProvider:           mockDBProvider,
					lwt:                  mockLWT,
				}
			},
			wantLen: 0,
			wantErr: false,
		},
		{
			name:       "success - partial targets found",
			spaceID:    validSpaceID,
			versionIDs: validVersionIDs,
			mockSetup: func(ctrl *gomock.Controller) *EvalTargetRepoImpl {
				mockEvalTargetDao := mysqlmocks.NewMockEvalTargetDAO(ctrl)
				mockEvalTargetVersionDao := mysqlmocks.NewMockEvalTargetVersionDAO(ctrl)
				mockEvalTargetRecordDao := mysqlmocks.NewMockEvalTargetRecordDAO(ctrl)
				mockIDGen := idgen.NewMockIIDGenerator(ctrl)
				mockDBProvider := dbmock.NewMockProvider(ctrl)
				mockLWT := platestwrite_mocks.NewMockILatestWriteTracker(ctrl)

				// Mock batch get versions
				mockEvalTargetVersionDao.EXPECT().
					BatchGetEvalTargetVersion(gomock.Any(), validSpaceID, validVersionIDs).
					Return(validVersions, nil)

				// Mock batch get targets returns only one target
				mockEvalTargetDao.EXPECT().
					BatchGetEvalTarget(gomock.Any(), validSpaceID, []int64{validTargetID1, validTargetID2}).
					Return([]*model.Target{validTargets[0]}, nil)

				return &EvalTargetRepoImpl{
					evalTargetDao:        mockEvalTargetDao,
					evalTargetVersionDao: mockEvalTargetVersionDao,
					evalTargetRecordDao:  mockEvalTargetRecordDao,
					idgen:                mockIDGen,
					dbProvider:           mockDBProvider,
					lwt:                  mockLWT,
				}
			},
			wantLen: 1,
			wantErr: false,
		},
		{
			name:       "error - version dao error",
			spaceID:    validSpaceID,
			versionIDs: validVersionIDs,
			mockSetup: func(ctrl *gomock.Controller) *EvalTargetRepoImpl {
				mockEvalTargetDao := mysqlmocks.NewMockEvalTargetDAO(ctrl)
				mockEvalTargetVersionDao := mysqlmocks.NewMockEvalTargetVersionDAO(ctrl)
				mockEvalTargetRecordDao := mysqlmocks.NewMockEvalTargetRecordDAO(ctrl)
				mockIDGen := idgen.NewMockIIDGenerator(ctrl)
				mockDBProvider := dbmock.NewMockProvider(ctrl)
				mockLWT := platestwrite_mocks.NewMockILatestWriteTracker(ctrl)

				// Mock batch get versions returns error
				mockEvalTargetVersionDao.EXPECT().
					BatchGetEvalTargetVersion(gomock.Any(), validSpaceID, validVersionIDs).
					Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))

				return &EvalTargetRepoImpl{
					evalTargetDao:        mockEvalTargetDao,
					evalTargetVersionDao: mockEvalTargetVersionDao,
					evalTargetRecordDao:  mockEvalTargetRecordDao,
					idgen:                mockIDGen,
					dbProvider:           mockDBProvider,
					lwt:                  mockLWT,
				}
			},
			wantLen: 0,
			wantErr: true,
		},
		{
			name:       "error - target dao error",
			spaceID:    validSpaceID,
			versionIDs: validVersionIDs,
			mockSetup: func(ctrl *gomock.Controller) *EvalTargetRepoImpl {
				mockEvalTargetDao := mysqlmocks.NewMockEvalTargetDAO(ctrl)
				mockEvalTargetVersionDao := mysqlmocks.NewMockEvalTargetVersionDAO(ctrl)
				mockEvalTargetRecordDao := mysqlmocks.NewMockEvalTargetRecordDAO(ctrl)
				mockIDGen := idgen.NewMockIIDGenerator(ctrl)
				mockDBProvider := dbmock.NewMockProvider(ctrl)
				mockLWT := platestwrite_mocks.NewMockILatestWriteTracker(ctrl)

				// Mock batch get versions
				mockEvalTargetVersionDao.EXPECT().
					BatchGetEvalTargetVersion(gomock.Any(), validSpaceID, validVersionIDs).
					Return(validVersions, nil)

				// Mock batch get targets returns error
				mockEvalTargetDao.EXPECT().
					BatchGetEvalTarget(gomock.Any(), validSpaceID, []int64{validTargetID1, validTargetID2}).
					Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))

				return &EvalTargetRepoImpl{
					evalTargetDao:        mockEvalTargetDao,
					evalTargetVersionDao: mockEvalTargetVersionDao,
					evalTargetRecordDao:  mockEvalTargetRecordDao,
					idgen:                mockIDGen,
					dbProvider:           mockDBProvider,
					lwt:                  mockLWT,
				}
			},
			wantLen: 0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := tt.mockSetup(ctrl)
			got, err := repo.BatchGetEvalTargetVersion(context.Background(), tt.spaceID, tt.versionIDs)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.wantLen == 0 {
					assert.Nil(t, got)
				} else {
					assert.Len(t, got, tt.wantLen)
				}
			}
		})
	}
}

func TestEvalTargetRepoImpl_ListEvalTargetRecordByIDsAndSpaceID(t *testing.T) {
	// Test data
	validSpaceID := int64(123)
	validRecordIDs := []int64{456, 789}
	validTargetID := int64(111)
	validVersionID := int64(222)

	validRecords := []*model.TargetRecord{
		{
			ID:              validRecordIDs[0],
			SpaceID:         validSpaceID,
			TargetID:        validTargetID,
			TargetVersionID: validVersionID,
			InputData:       gptr.Of([]byte("{}")),
			OutputData:      gptr.Of([]byte("{}")),
		},
		{
			ID:              validRecordIDs[1],
			SpaceID:         validSpaceID,
			TargetID:        validTargetID,
			TargetVersionID: validVersionID,
			InputData:       gptr.Of([]byte("{}")),
			OutputData:      gptr.Of([]byte("{}")),
		},
	}

	tests := []struct {
		name        string
		spaceID     int64
		recordIDs   []int64
		mockSetup   func(*gomock.Controller) *EvalTargetRepoImpl
		wantLen     int
		wantErr     bool
		wantErrCode int32
	}{
		{
			name:      "success - records found",
			spaceID:   validSpaceID,
			recordIDs: validRecordIDs,
			mockSetup: func(ctrl *gomock.Controller) *EvalTargetRepoImpl {
				mockEvalTargetDao := mysqlmocks.NewMockEvalTargetDAO(ctrl)
				mockEvalTargetVersionDao := mysqlmocks.NewMockEvalTargetVersionDAO(ctrl)
				mockEvalTargetRecordDao := mysqlmocks.NewMockEvalTargetRecordDAO(ctrl)
				mockIDGen := idgen.NewMockIIDGenerator(ctrl)
				mockDBProvider := dbmock.NewMockProvider(ctrl)
				mockLWT := platestwrite_mocks.NewMockILatestWriteTracker(ctrl)

				// Mock list records
				mockEvalTargetRecordDao.EXPECT().
					ListByIDsAndSpaceID(gomock.Any(), validRecordIDs, validSpaceID).
					Return(validRecords, nil)

				return &EvalTargetRepoImpl{
					evalTargetDao:        mockEvalTargetDao,
					evalTargetVersionDao: mockEvalTargetVersionDao,
					evalTargetRecordDao:  mockEvalTargetRecordDao,
					idgen:                mockIDGen,
					dbProvider:           mockDBProvider,
					lwt:                  mockLWT,
				}
			},
			wantLen: 2,
			wantErr: false,
		},
		{
			name:      "success - no records found",
			spaceID:   validSpaceID,
			recordIDs: validRecordIDs,
			mockSetup: func(ctrl *gomock.Controller) *EvalTargetRepoImpl {
				mockEvalTargetDao := mysqlmocks.NewMockEvalTargetDAO(ctrl)
				mockEvalTargetVersionDao := mysqlmocks.NewMockEvalTargetVersionDAO(ctrl)
				mockEvalTargetRecordDao := mysqlmocks.NewMockEvalTargetRecordDAO(ctrl)
				mockIDGen := idgen.NewMockIIDGenerator(ctrl)
				mockDBProvider := dbmock.NewMockProvider(ctrl)
				mockLWT := platestwrite_mocks.NewMockILatestWriteTracker(ctrl)

				// Mock list records returns empty
				mockEvalTargetRecordDao.EXPECT().
					ListByIDsAndSpaceID(gomock.Any(), validRecordIDs, validSpaceID).
					Return([]*model.TargetRecord{}, nil)

				return &EvalTargetRepoImpl{
					evalTargetDao:        mockEvalTargetDao,
					evalTargetVersionDao: mockEvalTargetVersionDao,
					evalTargetRecordDao:  mockEvalTargetRecordDao,
					idgen:                mockIDGen,
					dbProvider:           mockDBProvider,
					lwt:                  mockLWT,
				}
			},
			wantLen: 0,
			wantErr: false,
		},
		{
			name:      "error - dao error",
			spaceID:   validSpaceID,
			recordIDs: validRecordIDs,
			mockSetup: func(ctrl *gomock.Controller) *EvalTargetRepoImpl {
				mockEvalTargetDao := mysqlmocks.NewMockEvalTargetDAO(ctrl)
				mockEvalTargetVersionDao := mysqlmocks.NewMockEvalTargetVersionDAO(ctrl)
				mockEvalTargetRecordDao := mysqlmocks.NewMockEvalTargetRecordDAO(ctrl)
				mockIDGen := idgen.NewMockIIDGenerator(ctrl)
				mockDBProvider := dbmock.NewMockProvider(ctrl)
				mockLWT := platestwrite_mocks.NewMockILatestWriteTracker(ctrl)

				// Mock list records returns error
				mockEvalTargetRecordDao.EXPECT().
					ListByIDsAndSpaceID(gomock.Any(), validRecordIDs, validSpaceID).
					Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))

				return &EvalTargetRepoImpl{
					evalTargetDao:        mockEvalTargetDao,
					evalTargetVersionDao: mockEvalTargetVersionDao,
					evalTargetRecordDao:  mockEvalTargetRecordDao,
					idgen:                mockIDGen,
					dbProvider:           mockDBProvider,
					lwt:                  mockLWT,
				}
			},
			wantLen:     0,
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name:      "error - convert to DO failed",
			spaceID:   validSpaceID,
			recordIDs: validRecordIDs,
			mockSetup: func(ctrl *gomock.Controller) *EvalTargetRepoImpl {
				mockEvalTargetDao := mysqlmocks.NewMockEvalTargetDAO(ctrl)
				mockEvalTargetVersionDao := mysqlmocks.NewMockEvalTargetVersionDAO(ctrl)
				mockEvalTargetRecordDao := mysqlmocks.NewMockEvalTargetRecordDAO(ctrl)
				mockIDGen := idgen.NewMockIIDGenerator(ctrl)
				mockDBProvider := dbmock.NewMockProvider(ctrl)
				mockLWT := platestwrite_mocks.NewMockILatestWriteTracker(ctrl)

				// Mock list records returns invalid data
				mockEvalTargetRecordDao.EXPECT().
					ListByIDsAndSpaceID(gomock.Any(), validRecordIDs, validSpaceID).
					Return([]*model.TargetRecord{
						{
							ID:              validRecordIDs[0],
							SpaceID:         validSpaceID,
							TargetID:        validTargetID,
							TargetVersionID: validVersionID,
							// Invalid JSON data to trigger conversion error - using malformed JSON
							InputData:  gptr.Of([]byte(`{"invalid": json}`)),
							OutputData: gptr.Of([]byte(`{"malformed": json}`)),
						},
					}, nil)

				return &EvalTargetRepoImpl{
					evalTargetDao:        mockEvalTargetDao,
					evalTargetVersionDao: mockEvalTargetVersionDao,
					evalTargetRecordDao:  mockEvalTargetRecordDao,
					idgen:                mockIDGen,
					dbProvider:           mockDBProvider,
					lwt:                  mockLWT,
				}
			},
			wantLen:     0,
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := tt.mockSetup(ctrl)

			got, err := repo.ListEvalTargetRecordByIDsAndSpaceID(context.Background(), tt.spaceID, tt.recordIDs)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					if ok {
						assert.Equal(t, tt.wantErrCode, statusErr.Code())
					}
				}
			} else {
				assert.NoError(t, err)
				assert.Len(t, got, tt.wantLen)
			}
		})
	}
}
