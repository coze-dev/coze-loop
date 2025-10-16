// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	idgenmock "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/data/domain/component/conf/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/component/vfs"
	mock_vfs "github.com/coze-dev/coze-loop/backend/modules/data/domain/component/vfs/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/entity"
	mock_repo "github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/repo/mocks"
)

func TestNewImportHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	service := &DatasetServiceImpl{
		repo: mockRepo,
	}

	tests := []struct {
		name     string
		job      *entity.IOJob
		ds       *DatasetWithSchema
		mockRepo func()
		wantErr  bool
	}{
		{
			name: "正常场景",
			job:  &entity.IOJob{},
			ds:   &DatasetWithSchema{},
			mockRepo: func() {
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockRepo()
			handler := service.newImportHandler(tt.job, tt.ds)
			if (handler == nil) != tt.wantErr {
				t.Errorf("newImportHandler() error = %v, wantErr %v", handler == nil, tt.wantErr)
			}
		})
	}
}

func TestNewImportUnit(t *testing.T) {
	tests := []struct {
		name    string
		job     *entity.IOJob
		wantErr bool
	}{
		{
			name: "正常场景",
			job: &entity.IOJob{
				Progress: &entity.DatasetIOJobProgress{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unit := newImportUnit(tt.job)
			if (unit == nil) != tt.wantErr {
				t.Errorf("newImportUnit() error = %v, wantErr %v", unit == nil, tt.wantErr)
			}
		})
	}
}

func TestNewImportWorkspace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFS := mock_vfs.NewMockIUnionFS(ctrl)
	ctx := context.Background()
	job := &entity.IOJob{
		Source: &entity.DatasetIOEndpoint{
			File: &entity.DatasetIOFile{},
		},
	}

	tests := []struct {
		name       string
		mockRepo   func()
		wantErr    bool
		wantResult *importWorkspace
	}{
		{
			name: "正常场景",
			mockRepo: func() {
				mockROFS := mock_vfs.NewMockROFileSystem(ctrl)
				mockFS.EXPECT().GetROFileSystem(gomock.Any()).Return(mockROFS, nil)
			},
			wantErr: false,
		},
		// 可以根据需要添加更多测试用例，如边界场景等
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockRepo()
			_, err := newImportWorkspace(ctx, job, mockFS)
			if (err != nil) != tt.wantErr {
				t.Errorf("newImportWorkspace() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestImportHandler_startJob(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	mockProvider := mocks.NewMockProvider(ctrl)
	service := &DatasetServiceImpl{
		repo: mockRepo,
		txDB: mockProvider,
	}

	tests := []struct {
		name        string
		handler     *importHandler
		mockRepo    func()
		wantSuccess bool
		wantErr     bool
	}{
		{
			name: "正常场景",
			handler: &importHandler{
				svc: service,
				ds: &DatasetWithSchema{
					Dataset: &entity.Dataset{
						ID: 1,
					},
				},
				job: &entity.IOJob{
					Option: &entity.DatasetIOJobOption{
						OverwriteDataset: gptr.Of(true),
					},
				},
				repo:        mockRepo,
				currentUnit: &importUnit{},
			},
			mockRepo: func() {
				mockRepo.EXPECT().MGetDatasetOperations(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
				mockRepo.EXPECT().AddDatasetOperation(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockRepo.EXPECT().DelDatasetOperation(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockRepo.EXPECT().SetItemCount(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockProvider.EXPECT().Transaction(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			wantSuccess: true,
			wantErr:     false,
		},
		{
			name: "异常场景",
			handler: &importHandler{
				svc: service,
				ds: &DatasetWithSchema{
					Dataset: &entity.Dataset{
						ID: 1,
					},
				},
				job: &entity.IOJob{
					Option: &entity.DatasetIOJobOption{
						OverwriteDataset: gptr.Of(true),
					},
				},
				repo:        mockRepo,
				currentUnit: &importUnit{},
			},
			mockRepo: func() {
				mockRepo.EXPECT().MGetDatasetOperations(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
				mockRepo.EXPECT().AddDatasetOperation(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockRepo.EXPECT().DelDatasetOperation(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockProvider.EXPECT().Transaction(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("tx err"))
				mockRepo.EXPECT().UpdateIOJob(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			wantSuccess: false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockRepo()
			success, err := tt.handler.startJob(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("startJob() error = %v, wantErr %v", err, tt.wantErr)
			}
			if success != tt.wantSuccess {
				t.Errorf("startJob() success = %v, wantSuccess %v", success, tt.wantSuccess)
			}
		})
	}
}

func TestKV2Item(t *testing.T) {
	tests := []struct {
		name     string
		inputKV  map[string]any
		wantItem *entity.Item
		wantErr  bool
	}{
		{
			name: "正常场景",
			inputKV: map[string]any{
				"Field1": "value1",
				"Field2": 123,
			},
			wantItem: &entity.Item{
				ItemKey: "value1",
			},
			wantErr: false,
		},
	}

	// 假设 importHandler 有一个实例，根据实际情况创建
	h := &importHandler{
		fieldMapping: map[string][]string{
			"Field1": {"value1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := h.kv2Item(tt.inputKV)
			if (item == nil) != tt.wantErr {
				t.Errorf("kv2Item() error = %v, wantErr %v", item == nil, tt.wantErr)
				return
			}
		})
	}
}

func TestImportWorkspace_noMoreFile(t *testing.T) {
	w := &importWorkspace{}
	// 可根据实际情况设置 w 的属性值

	result := w.noMoreFile()
	// 可根据预期结果修改断言
	if result != true {
		t.Errorf("noMoreFile() = %v, 期望为 %v", result, false)
	}
}

// 测试 u.onInternalErr 方法
func TestImportUnit_onInternalErr(t *testing.T) {
	u := &importUnit{}
	ctx := context.Background()
	err := fmt.Errorf("test error")
	msg := "test message"

	// 调用要测试的方法
	u.onInternalErr(ctx, err, msg)
	// 可根据实际情况添加断言，检查 u 的状态是否符合预期
	assert.NotNil(t, u.errors)
}

// 测试 u.onDatasetFull 方法
func TestImportUnit_onDatasetFull(t *testing.T) {
	u := &importUnit{}

	// 调用要测试的方法
	u.onDatasetFull()
	// 可根据实际情况添加断言，检查 u 的状态是否符合预期
	assert.NotNil(t, u.status)
}

// 测试 u.onIllegalContentErr 方法
func TestImportUnit_onIllegalContentErr(t *testing.T) {
	u := &importUnit{}
	ctx := context.Background()
	err := fmt.Errorf("test error")
	msg := "test message"

	// 调用要测试的方法
	u.onIllegalContentErr(ctx, err, msg)
	// 可根据实际情况添加断言，检查 u 的状态是否符合预期
	assert.NotNil(t, u.errors)
}

// 测试 u.onBadItems 方法
func TestImportUnit_onBadItems(t *testing.T) {
	u := &importUnit{}
	ctx := context.Background()
	errors := []*entity.ItemErrorGroup{
		{},
	}

	// 调用要测试的方法
	u.onBadItems(ctx, errors...)
	// 可根据实际情况添加断言，检查 u 的状态是否符合预期
}

// 测试 u.appendError 方法
func TestImportUnit_appendError(t *testing.T) {
	u := &importUnit{}
	eg := &entity.ItemErrorGroup{}

	// 调用要测试的方法
	u.appendError(eg)
	// 可根据实际情况添加断言，检查 u 的状态是否符合预期
	assert.NotNil(t, u.errors)
}

// 测试 u.onFlush 方法
func TestImportUnit_onFlush(t *testing.T) {
	u := &importUnit{
		progresses: map[string]*entity.DatasetIOJobProgress{},
		added:      1,
		items: []*IndexedItem{
			{},
		},
	}

	// 调用要测试的方法
	u.onFlush()
	// 可根据实际情况添加断言，检查 u 的状态是否符合预期
	assert.Equal(t, int64(0), u.added)
}

// 测试 u.isEmpty 方法
func TestImportUnit_isEmpty(t *testing.T) {
	tests := []struct {
		name string
		unit *importUnit
		want bool
	}{
		{
			name: "完全空的单元",
			unit: &importUnit{
				status:       entity.JobStatus_Running,
				preProcessed: 0,
				errors:       nil,
				progresses:   nil,
				filename:     "",
				startedAt:    nil,
				total:        nil,
				processed:    0,
				added:        0,
				items:        nil,
			},
			want: true,
		},
		{
			name: "有处理记录",
			unit: &importUnit{
				status:       entity.JobStatus_Running,
				preProcessed: 0,
				processed:    100,
				errors:       nil,
				progresses:   nil,
			},
			want: false,
		},
		{
			name: "有添加记录",
			unit: &importUnit{
				status:       entity.JobStatus_Running,
				preProcessed: 0,
				added:        50,
				errors:       nil,
				progresses:   nil,
			},
			want: false,
		},
		{
			name: "有待处理项",
			unit: &importUnit{
				status:       entity.JobStatus_Running,
				preProcessed: 0,
				items: []*IndexedItem{
					{
						Item:  &entity.Item{},
						Index: 1,
					},
				},
				errors:     nil,
				progresses: nil,
			},
			want: false,
		},
		{
			name: "有错误记录",
			unit: &importUnit{
				status:       entity.JobStatus_Running,
				preProcessed: 0,
				errors: map[entity.ItemErrorType]*entity.ItemErrorGroup{
					entity.ItemErrorType_InternalError: {
						Type: gptr.Of(entity.ItemErrorType_InternalError),
					},
				},
				progresses: nil,
			},
			want: false,
		},
		{
			name: "非运行状态",
			unit: &importUnit{
				status:       entity.JobStatus_Failed,
				preProcessed: 0,
				errors:       nil,
				progresses:   nil,
			},
			want: false,
		},
		{
			name: "有开始时间",
			unit: &importUnit{
				status:       entity.JobStatus_Running,
				preProcessed: 0,
				startedAt:    gptr.Of(time.Now()),
				errors:       nil,
				progresses:   nil,
			},
			want: false,
		},
		{
			name: "有总数记录",
			unit: &importUnit{
				status:       entity.JobStatus_Running,
				preProcessed: 0,
				total:        gptr.Of(int64(100)),
				errors:       nil,
				progresses:   nil,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.unit.isEmpty()
			assert.Equal(t, tt.want, got, "isEmpty() = %v, want %v", got, tt.want)
		})
	}
}

// 测试 u.mergeProgress 方法
func TestImportUnit_mergeProgress(t *testing.T) {
	u := &importUnit{}
	// 可根据实际情况设置 u 的属性值

	result := u.mergeProgress()
	// 可根据预期结果修改断言
	if result == nil {
		t.Errorf("mergeProgress() 返回 nil，期望非 nil")
	}
}

// 测试 u.toDeltaDatasetIOJob 方法
func TestImportUnit_toDeltaDatasetIOJob(t *testing.T) {
	u := &importUnit{}
	// 可根据实际情况设置 u 的属性值

	result := u.toDeltaDatasetIOJob()
	// 可根据预期结果修改断言
	if result == nil {
		t.Errorf("toDeltaDatasetIOJob() 返回 nil，期望非 nil")
	}
}

func TestImportHandler_Handle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	mockFS := mock_vfs.NewMockIUnionFS(ctrl)
	mockProvider := mocks.NewMockProvider(ctrl)
	mockIConfig := confmocks.NewMockIConfig(ctrl)

	service := &DatasetServiceImpl{
		repo:          mockRepo,
		txDB:          mockProvider,
		storageConfig: mockIConfig.GetDatasetItemStorage,
		fsUnion:       mockFS,
	}

	tests := []struct {
		name      string
		handler   *importHandler
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "获取文件系统失败",
			handler: &importHandler{
				svc: service,
				ds: &DatasetWithSchema{
					Dataset: &entity.Dataset{ID: 1},
				},
				job: &entity.IOJob{
					ID: 1,
					Source: &entity.DatasetIOEndpoint{
						File: &entity.DatasetIOFile{
							Provider: "s3",
							Path:     "test.csv",
						},
					},
				},
				currentUnit: &importUnit{},
				repo:        mockRepo,
				fsUnion:     mockFS,
			},
			mockSetup: func() {
				mockFS.EXPECT().GetROFileSystem(gomock.Any()).Return(nil, fmt.Errorf("failed to get filesystem"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := tt.handler.Handle(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Handle() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestImportWorkspace_nextFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockROFS := mock_vfs.NewMockROFileSystem(ctrl)

	tests := []struct {
		name      string
		workspace *importWorkspace
		mockSetup func()
		wantOK    bool
		wantErr   bool
	}{
		{
			name: "非法文件",
			workspace: &importWorkspace{
				fs:     mockROFS,
				files:  []string{"test.csv"},
				cursor: 0,
				source: &entity.DatasetIOFile{
					Format: gptr.Of(entity.FileFormat(0)),
				},
				progress: map[string]*entity.DatasetIOJobProgress{
					"test.csv": {
						Name:      gptr.Of("test.csv"),
						Total:     gptr.Of(int64(100)),
						Processed: gptr.Of(int64(50)),
					},
				},
			},
			mockSetup: func() {
				mockROFS.EXPECT().ReadFile(gomock.Any(), "test.csv").Return(nil, nil)
				mockROFS.EXPECT().Stat(gomock.Any(), "test.csv").Return(nil, nil)
			},
			wantOK:  false,
			wantErr: true,
		},
		{
			name: "文件已处理完成",
			workspace: &importWorkspace{
				fs:     mockROFS,
				files:  []string{"test.csv"},
				cursor: 0,
				progress: map[string]*entity.DatasetIOJobProgress{
					"test.csv": {
						Name:      gptr.Of("test.csv"),
						Total:     gptr.Of(int64(100)),
						Processed: gptr.Of(int64(100)),
					},
				},
			},
			mockSetup: func() {},
			wantOK:    false,
			wantErr:   false,
		},
		{
			name: "读取文件失败",
			workspace: &importWorkspace{
				fs:     mockROFS,
				files:  []string{"test.csv"},
				cursor: 0,
				progress: map[string]*entity.DatasetIOJobProgress{
					"test.csv": {
						Name:      gptr.Of("test.csv"),
						Total:     gptr.Of(int64(100)),
						Processed: gptr.Of(int64(50)),
					},
				},
			},
			mockSetup: func() {
				mockROFS.EXPECT().ReadFile(gomock.Any(), "test.csv").Return(nil, fmt.Errorf("read file error"))
			},
			wantOK:  false,
			wantErr: true,
		},
		{
			name: "没有更多文件",
			workspace: &importWorkspace{
				fs:       mockROFS,
				files:    []string{"test.csv"},
				cursor:   1, // 超出文件列表范围
				progress: map[string]*entity.DatasetIOJobProgress{},
			},
			mockSetup: func() {},
			wantOK:    false,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			fr, ok, err := tt.workspace.nextFile(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("nextFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if ok != tt.wantOK {
				t.Errorf("nextFile() ok = %v, wantOK %v", ok, tt.wantOK)
				return
			}
			if tt.wantOK {
				assert.NotNil(t, fr, "FileReader should not be nil when ok is true")
			} else {
				assert.Nil(t, fr, "FileReader should be nil when ok is false")
			}
		})
	}
}

func TestImportHandler_saveCurrentUnit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	mockProvider := mocks.NewMockProvider(ctrl)
	mockIConfig := confmocks.NewMockIConfig(ctrl)
	mockIIDGenerator := idgenmock.NewMockIIDGenerator(ctrl)

	service := &DatasetServiceImpl{
		repo:          mockRepo,
		txDB:          mockProvider,
		storageConfig: mockIConfig.GetDatasetItemStorage,
		idgen:         mockIIDGenerator,
	}

	tests := []struct {
		name      string
		handler   *importHandler
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "空单元不需要保存",
			handler: &importHandler{
				svc: service,
				ds: &DatasetWithSchema{
					Dataset: &entity.Dataset{ID: 1},
					Schema:  &entity.DatasetSchema{},
				},
				job: &entity.IOJob{ID: 1},
				currentUnit: &importUnit{
					status:       entity.JobStatus_Running,
					preProcessed: 0,
					errors:       nil,
					progresses:   nil,
					filename:     "",
					startedAt:    nil,
					total:        nil,
					processed:    0,
					added:        0,
					items:        nil,
				},
				repo: mockRepo,
			},
			mockSetup: func() {},
			wantErr:   false,
		},
		{
			name: "成功保存有内容的单元",
			handler: &importHandler{
				svc: service,
				ds: &DatasetWithSchema{
					Dataset: &entity.Dataset{ID: 1, Features: &entity.DatasetFeatures{RepeatedData: false}, Spec: &entity.DatasetSpec{}},
					Schema:  &entity.DatasetSchema{},
				},
				job: &entity.IOJob{ID: 1},
				currentUnit: &importUnit{
					status:       entity.JobStatus_Running,
					preProcessed: 0,
					processed:    100,
					added:        50,
					items: []*IndexedItem{
						{
							Item:  &entity.Item{Data: []*entity.FieldData{{Key: "test", Content: "value"}}},
							Index: 1,
						},
					},
					filename:   "test.csv",
					progresses: map[string]*entity.DatasetIOJobProgress{},
				},
				repo: mockRepo,
			},
			mockSetup: func() {
				mockIIDGenerator.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).Return([]int64{1}, nil)
				mockRepo.EXPECT().IncrItemCount(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1), nil).MaxTimes(2)
				mockRepo.EXPECT().UpdateIOJob(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "批量创建项目失败",
			handler: &importHandler{
				svc: service,
				ds: &DatasetWithSchema{
					Dataset: &entity.Dataset{ID: 1, Features: &entity.DatasetFeatures{RepeatedData: false}, Spec: &entity.DatasetSpec{}},
					Schema:  &entity.DatasetSchema{},
				},
				job: &entity.IOJob{ID: 1},
				currentUnit: &importUnit{
					status:       entity.JobStatus_Running,
					preProcessed: 0,
					processed:    100,
					added:        50,
					items: []*IndexedItem{
						{
							Item:  &entity.Item{Data: []*entity.FieldData{{Key: "test", Content: "value"}}},
							Index: 1,
						},
					},
					filename:   "test.csv",
					progresses: map[string]*entity.DatasetIOJobProgress{},
				},
				repo: mockRepo,
			},
			mockSetup: func() {
				mockIIDGenerator.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).Return([]int64{}, errors.New("failed to generate ids"))
				mockRepo.EXPECT().UpdateIOJob(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "更新任务状态失败",
			handler: &importHandler{
				svc: service,
				ds: &DatasetWithSchema{
					Dataset: &entity.Dataset{ID: 1, Features: &entity.DatasetFeatures{RepeatedData: false}, Spec: &entity.DatasetSpec{}},
					Schema:  &entity.DatasetSchema{},
				},
				job: &entity.IOJob{ID: 1},
				currentUnit: &importUnit{
					status:       entity.JobStatus_Running,
					preProcessed: 0,
					processed:    100,
					added:        50,
					items: []*IndexedItem{
						{
							Item:  &entity.Item{Data: []*entity.FieldData{{Key: "test", Content: "value"}}},
							Index: 1,
						},
					},
					filename:   "test.csv",
					progresses: map[string]*entity.DatasetIOJobProgress{},
				},
				repo: mockRepo,
			},
			mockSetup: func() {
				mockIIDGenerator.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).Return([]int64{1}, nil)
				mockRepo.EXPECT().IncrItemCount(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1), nil).MaxTimes(2)
				mockRepo.EXPECT().UpdateIOJob(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("update job error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := tt.handler.saveCurrentUnit(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("saveCurrentUnit() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// 模拟 Reader 接口
type MockReader struct {
	content []byte
	pos     int
}

func (m *MockReader) ReadAt(p []byte, off int64) (n int, err error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockReader) Read(p []byte) (n int, err error) {
	if m.pos >= len(m.content) {
		return 0, io.EOF
	}
	n = copy(p, m.content[m.pos:])
	m.pos += n
	return
}

func (m *MockReader) Close() error {
	return nil
}

// 模拟 fs.FileInfo 接口
type MockFileInfo struct{}

func (m *MockFileInfo) Name() string       { return "testfile" }
func (m *MockFileInfo) Size() int64        { return 1024 }
func (m *MockFileInfo) Mode() os.FileMode  { return 0o644 }
func (m *MockFileInfo) ModTime() time.Time { return time.Now() }
func (m *MockFileInfo) IsDir() bool        { return false }
func (m *MockFileInfo) Sys() interface{}   { return nil }

func TestImportHandler_importFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	mockProvider := mocks.NewMockProvider(ctrl)
	mockIConfig := confmocks.NewMockIConfig(ctrl)
	mockIIDGenerator := idgenmock.NewMockIIDGenerator(ctrl)
	mockFS := mock_vfs.NewMockIUnionFS(ctrl)
	mockROFS := mock_vfs.NewMockROFileSystem(ctrl)

	service := &DatasetServiceImpl{
		repo:          mockRepo,
		txDB:          mockProvider,
		storageConfig: mockIConfig.GetDatasetItemStorage,
		idgen:         mockIIDGenerator,
		fsUnion:       mockFS,
	}
	// 以 CSV 格式为例
	content := "col1,col2\nval1,val2\nval3,val4"
	mockReader := &MockReader{content: []byte(content)}
	mockInfo := &MockFileInfo{}
	fr, err := vfs.NewFileReader("testfile", mockReader, mockInfo, entity.FileFormat_CSV)
	if err != nil {
		t.Fatalf("NewFileReader() error = %v", err)
	}
	tests := []struct {
		name      string
		handler   *importHandler
		workspace *importWorkspace
		fr        *vfs.FileReader
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "成功导入文件 - 非最后一个文件",
			handler: &importHandler{
				svc: service,
				ds: &DatasetWithSchema{
					Dataset: &entity.Dataset{
						ID:       1,
						Features: &entity.DatasetFeatures{RepeatedData: false},
						Spec:     &entity.DatasetSpec{},
					},
					Schema: &entity.DatasetSchema{},
				},
				job: &entity.IOJob{ID: 1},
				currentUnit: &importUnit{
					status:       entity.JobStatus_Running,
					preProcessed: 0,
					progresses:   map[string]*entity.DatasetIOJobProgress{},
				},
				repo: mockRepo,
			},
			workspace: &importWorkspace{
				fs:     mockROFS,
				files:  []string{"test1.csv", "test2.csv"},
				cursor: 0,
			},
			fr: fr,
			mockSetup: func() {
				// 模拟批量创建items
				mockIIDGenerator.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).Return([]int64{1}, nil)
				mockRepo.EXPECT().IncrItemCount(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1), nil).MaxTimes(2)
				mockRepo.EXPECT().UpdateIOJob(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "成功导入文件 - 最后一个文件",
			handler: &importHandler{
				svc: service,
				ds: &DatasetWithSchema{
					Dataset: &entity.Dataset{
						ID:       1,
						Features: &entity.DatasetFeatures{RepeatedData: false},
						Spec:     &entity.DatasetSpec{},
					},
					Schema: &entity.DatasetSchema{},
				},
				job: &entity.IOJob{ID: 1},
				currentUnit: &importUnit{
					status:       entity.JobStatus_Running,
					preProcessed: 100,
					progresses:   map[string]*entity.DatasetIOJobProgress{},
				},
				repo: mockRepo,
			},
			workspace: &importWorkspace{
				fs:     mockROFS,
				files:  []string{"test.csv"},
				cursor: 0,
			},
			fr: fr,
			mockSetup: func() {
				// 模拟批量创建items
				// mockIIDGenerator.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).Return([]int64{1}, nil)
				mockRepo.EXPECT().IncrItemCount(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1), nil).MaxTimes(2)
				// mockRepo.EXPECT().UpdateIOJob(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "文件读取错误",
			handler: &importHandler{
				svc: service,
				ds: &DatasetWithSchema{
					Dataset: &entity.Dataset{
						ID:       1,
						Features: &entity.DatasetFeatures{RepeatedData: false},
						Spec:     &entity.DatasetSpec{},
					},
					Schema: &entity.DatasetSchema{},
				},
				job: &entity.IOJob{ID: 1},
				currentUnit: &importUnit{
					status:       entity.JobStatus_Running,
					preProcessed: 0,
					progresses:   map[string]*entity.DatasetIOJobProgress{},
				},
				repo: mockRepo,
			},
			fr: fr,
			workspace: &importWorkspace{
				fs:     mockROFS,
				files:  []string{"test.csv"},
				cursor: 0,
			},
			mockSetup: func() {
				// 模拟更新任务状态
				// mockRepo.EXPECT().UpdateIOJob(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := tt.handler.importFile(context.Background(), tt.workspace, tt.fr)
			if (err != nil) != tt.wantErr {
				t.Errorf("importFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestImportHandler_scanFileWithoutSave(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	mockProvider := mocks.NewMockProvider(ctrl)
	mockIConfig := confmocks.NewMockIConfig(ctrl)
	mockFS := mock_vfs.NewMockIUnionFS(ctrl)
	mockROFS := mock_vfs.NewMockROFileSystem(ctrl)

	service := &DatasetServiceImpl{
		repo:          mockRepo,
		txDB:          mockProvider,
		storageConfig: mockIConfig.GetDatasetItemStorage,
		fsUnion:       mockFS,
	}

	// 创建一个CSV格式的测试数据
	content := "col1,col2\nval1,val2\nval3,val4"
	mockReader := &MockReader{content: []byte(content)}
	mockInfo := &MockFileInfo{}
	fr, err := vfs.NewFileReader("test.csv", mockReader, mockInfo, entity.FileFormat_CSV)
	if err != nil {
		t.Fatalf("NewFileReader() error = %v", err)
	}

	tests := []struct {
		name      string
		handler   *importHandler
		workspace *importWorkspace
		fr        *vfs.FileReader
		mockSetup func()
	}{
		{
			name: "成功扫描单个文件",
			handler: &importHandler{
				svc: service,
				ds: &DatasetWithSchema{
					Dataset: &entity.Dataset{
						ID:       1,
						Features: &entity.DatasetFeatures{RepeatedData: false},
						Spec:     &entity.DatasetSpec{},
					},
					Schema: &entity.DatasetSchema{},
				},
				job: &entity.IOJob{ID: 1},
				currentUnit: &importUnit{
					status:       entity.JobStatus_Completed,
					preProcessed: 100,
					processed:    50,
					progresses: map[string]*entity.DatasetIOJobProgress{
						"test.csv": {
							Name:      gptr.Of("test.csv"),
							Total:     gptr.Of(int64(0)),
							Processed: gptr.Of(int64(0)),
						},
					},
				},
				repo: mockRepo,
			},
			workspace: &importWorkspace{
				fs:     mockROFS,
				files:  []string{"test.csv"},
				cursor: 10,
			},
			fr: fr,
			mockSetup: func() {
				mockRepo.EXPECT().UpdateIOJob(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			tt.handler.scanFileWithoutSave(context.Background(), tt.workspace, tt.fr)
		})
	}
}
