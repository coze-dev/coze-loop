// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	dbMocks "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	benefitMocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	fileserverMocks "github.com/coze-dev/coze-loop/backend/infra/fileserver/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	componentMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestExportCSVHelper_buildColumnEvalTargetContent(t *testing.T) {
	ctx := context.Background()
	helper := &exportCSVHelper{}

	textContent := &entity.Content{
		ContentType: ptr.Of(entity.ContentTypeText),
		Text:        ptr.Of("text-value"),
	}

	tests := []struct {
		name       string
		columnName string
		data       *entity.EvalTargetOutputData
		want       string
	}{
		{
			name:       "data is nil",
			columnName: consts.ReportColumnNameEvalTargetTotalLatency,
			data:       nil,
			want:       "",
		},
		{
			name:       "total latency",
			columnName: consts.ReportColumnNameEvalTargetTotalLatency,
			data:       &entity.EvalTargetOutputData{TimeConsumingMS: ptr.Of(int64(123))},
			want:       "123",
		},
		{
			name:       "input tokens",
			columnName: consts.ReportColumnNameEvalTargetInputTokens,
			data:       &entity.EvalTargetOutputData{EvalTargetUsage: &entity.EvalTargetUsage{InputTokens: 10}},
			want:       "10",
		},
		{
			name:       "output tokens",
			columnName: consts.ReportColumnNameEvalTargetOutputTokens,
			data:       &entity.EvalTargetOutputData{EvalTargetUsage: &entity.EvalTargetUsage{OutputTokens: 20}},
			want:       "20",
		},
		{
			name:       "total tokens",
			columnName: consts.ReportColumnNameEvalTargetTotalTokens,
			data:       &entity.EvalTargetOutputData{EvalTargetUsage: &entity.EvalTargetUsage{TotalTokens: 30}},
			want:       "30",
		},
		{
			name:       "default text field",
			columnName: "custom_col",
			data: &entity.EvalTargetOutputData{
				OutputFields: map[string]*entity.Content{"custom_col": textContent},
			},
			want: "text-value",
		},
		{
			name:       "default missing field",
			columnName: "missing_col",
			data: &entity.EvalTargetOutputData{
				OutputFields: map[string]*entity.Content{},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := helper.buildColumnEvalTargetContent(ctx, tt.columnName, tt.data)
			assert.Equal(t, tt.want, got)
			assert.Nil(t, err)
		})
	}
}

func TestExportCSVHelper_buildRows_EvalTargetColumns(t *testing.T) {
	ctx := context.Background()

	makeBaseHelperAndItem := func() (*exportCSVHelper, *entity.ItemResult, *entity.TurnResult, *entity.ExperimentTurnPayload) {
		turn := &entity.Turn{
			FieldDataList: []*entity.FieldData{},
		}
		payload := &entity.ExperimentTurnPayload{
			EvalSet: &entity.TurnEvalSet{
				Turn:      turn,
				ItemID:    1,
				EvalSetID: 1,
			},
		}
		turnResult := &entity.TurnResult{
			TurnID: 1,
			ExperimentResults: []*entity.ExperimentResult{
				{
					ExperimentID: 100,
					Payload:      payload,
				},
			},
		}
		itemResult := &entity.ItemResult{
			ItemID: 1,
			SystemInfo: &entity.ItemSystemInfo{
				RunState: entity.ItemRunState_Success,
			},
			TurnResults: []*entity.TurnResult{turnResult},
		}

		helper := &exportCSVHelper{
			allItemResults: []*entity.ItemResult{itemResult},
		}
		return helper, itemResult, turnResult, payload
	}

	t.Run("append empty string when target output is nil", func(t *testing.T) {
		helper, _, _, _ := makeBaseHelperAndItem()
		helper.columnsEvalTarget = []*entity.ColumnEvalTarget{
			{Name: consts.ReportColumnNameEvalTargetTotalLatency},
			{Name: consts.ReportColumnNameEvalTargetInputTokens},
		}

		rows, err := helper.buildRows(ctx)
		assert.NoError(t, err)
		if assert.Len(t, rows, 1) {
			row := rows[0]
			// ID, status, then 2 eval-target columns
			if assert.Len(t, row, 2+len(helper.columnsEvalTarget)) {
				assert.Equal(t, "1", row[0])
				assert.Equal(t, "success", row[1])
				assert.Equal(t, "", row[2])
				assert.Equal(t, "", row[3])
			}
		}
	})

	t.Run("append eval target metrics when target output is present", func(t *testing.T) {
		helper, _, _, payload := makeBaseHelperAndItem()
		helper.columnsEvalTarget = []*entity.ColumnEvalTarget{
			{Name: consts.ReportColumnNameEvalTargetTotalLatency},
			{Name: consts.ReportColumnNameEvalTargetInputTokens},
		}

		payload.TargetOutput = &entity.TurnTargetOutput{
			EvalTargetRecord: &entity.EvalTargetRecord{
				EvalTargetOutputData: &entity.EvalTargetOutputData{
					TimeConsumingMS: ptr.Of(int64(123)),
					EvalTargetUsage: &entity.EvalTargetUsage{
						InputTokens: 10,
					},
				},
			},
		}

		rows, err := helper.buildRows(ctx)
		assert.NoError(t, err)
		if assert.Len(t, rows, 1) {
			row := rows[0]
			if assert.Len(t, row, 2+len(helper.columnsEvalTarget)) {
				assert.Equal(t, "1", row[0])
				assert.Equal(t, "success", row[1])
				assert.Equal(t, "123", row[2]) // total latency
				assert.Equal(t, "10", row[3])  // input tokens
			}
		}
	})
}

func newTestExptResultExportService(ctrl *gomock.Controller) *ExptResultExportService {
	return &ExptResultExportService{
		txDB:               dbMocks.NewMockProvider(ctrl),
		repo:               repoMocks.NewMockIExptResultExportRecordRepo(ctrl),
		exptRepo:           repoMocks.NewMockIExperimentRepo(ctrl),
		exptTurnResultRepo: repoMocks.NewMockIExptTurnResultRepo(ctrl),
		exptPublisher:      eventsMocks.NewMockExptEventPublisher(ctrl),
		exptResultService:  svcMocks.NewMockExptResultService(ctrl),
		fileClient:         fileserverMocks.NewMockObjectStorage(ctrl),
		configer:           componentMocks.NewMockIConfiger(ctrl),
		benefitService:     benefitMocks.NewMockIBenefitService(ctrl),
		urlProcessor:       NewDefaultURLProcessor(),
	}
}

func TestExptResultExportService_ExportCSV(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name      string
		spaceID   int64
		exptID    int64
		session   *entity.Session
		setup     func(svc *ExptResultExportService)
		want      int64
		wantErr   bool
		errorCode int
	}{
		{
			name:    "正常导出",
			spaceID: 1,
			exptID:  123,
			session: &entity.Session{UserID: "test"},
			setup: func(svc *ExptResultExportService) {
				// 实验已完成
				svc.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().
					GetByID(gomock.Any(), int64(123), int64(1)).
					Return(&entity.Experiment{ID: 123, Status: entity.ExptStatus_Success}, nil).
					Times(1)

				// 没有运行中的导出任务
				svc.repo.(*repoMocks.MockIExptResultExportRecordRepo).EXPECT().
					List(gomock.Any(), int64(1), int64(123), gomock.Any(), ptr.Of(int32(entity.CSVExportStatus_Running))).
					Return([]*entity.ExptResultExportRecord{}, int64(0), nil).
					Times(1)

				// 创建导出记录
				svc.repo.(*repoMocks.MockIExptResultExportRecordRepo).EXPECT().
					Create(gomock.Any(), gomock.Any()).
					Return(int64(456), nil).
					Times(1)

				// 发布导出事件
				svc.exptPublisher.(*eventsMocks.MockExptEventPublisher).EXPECT().
					PublishExptExportCSVEvent(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).
					Times(1)
				svc.benefitService.(*benefitMocks.MockIBenefitService).EXPECT().BatchCheckEnableTypeBenefit(gomock.Any(), gomock.Any()).
					Return(&benefit.BatchCheckEnableTypeBenefitResult{Results: map[string]bool{"exp_download_report_enabled": true}}, nil)
				svc.configer.(*componentMocks.MockIConfiger).EXPECT().GetExptExportWhiteList(gomock.Any()).
					Return(&entity.ExptExportWhiteList{UserIDs: []int64{}}).AnyTimes()
			},
			want:    456,
			wantErr: false,
		},
		{
			name:    "命中白名单",
			spaceID: 1,
			exptID:  123,
			session: &entity.Session{UserID: "1"},
			setup: func(svc *ExptResultExportService) {
				// 实验已完成
				svc.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().
					GetByID(gomock.Any(), int64(123), int64(1)).
					Return(&entity.Experiment{ID: 123, Status: entity.ExptStatus_Success}, nil).
					Times(1)

				// 没有运行中的导出任务
				svc.repo.(*repoMocks.MockIExptResultExportRecordRepo).EXPECT().
					List(gomock.Any(), int64(1), int64(123), gomock.Any(), ptr.Of(int32(entity.CSVExportStatus_Running))).
					Return([]*entity.ExptResultExportRecord{}, int64(0), nil).
					Times(1)

				// 创建导出记录
				svc.repo.(*repoMocks.MockIExptResultExportRecordRepo).EXPECT().
					Create(gomock.Any(), gomock.Any()).
					Return(int64(456), nil).
					Times(1)

				// 发布导出事件
				svc.exptPublisher.(*eventsMocks.MockExptEventPublisher).EXPECT().
					PublishExptExportCSVEvent(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).
					Times(1)
				svc.configer.(*componentMocks.MockIConfiger).EXPECT().GetExptExportWhiteList(gomock.Any()).
					Return(&entity.ExptExportWhiteList{UserIDs: []int64{1}}).AnyTimes()
			},
			want:    456,
			wantErr: false,
		},
		{
			name:    "实验未完成",
			spaceID: 1,
			exptID:  123,
			session: &entity.Session{UserID: "test"},
			setup: func(svc *ExptResultExportService) {
				svc.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().
					GetByID(gomock.Any(), int64(123), int64(1)).
					Return(&entity.Experiment{ID: 123, Status: entity.ExptStatus_Processing}, nil).
					Times(1)
			},
			want:      0,
			wantErr:   true,
			errorCode: errno.ExperimentUncompleteCode,
		},
		{
			name:    "获取实验失败",
			spaceID: 1,
			exptID:  123,
			session: &entity.Session{UserID: "test"},
			setup: func(svc *ExptResultExportService) {
				svc.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().
					GetByID(gomock.Any(), int64(123), int64(1)).
					Return(nil, errors.New("db error")).
					Times(1)
			},
			want:    0,
			wantErr: true,
		},
		{
			name:    "导出任务数量超限",
			spaceID: 1,
			exptID:  123,
			session: &entity.Session{UserID: "test"},
			setup: func(svc *ExptResultExportService) {
				svc.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().
					GetByID(gomock.Any(), int64(123), int64(1)).
					Return(&entity.Experiment{ID: 123, Status: entity.ExptStatus_Success}, nil).
					Times(1)

				svc.repo.(*repoMocks.MockIExptResultExportRecordRepo).EXPECT().
					List(gomock.Any(), int64(1), int64(123), gomock.Any(), ptr.Of(int32(entity.CSVExportStatus_Running))).
					Return([]*entity.ExptResultExportRecord{{}, {}, {}, {}}, int64(4), nil).
					Times(1)
			},
			want:      0,
			wantErr:   true,
			errorCode: errno.ExportRunningCountLimitCode,
		},
		{
			name:    "创建导出记录失败",
			spaceID: 1,
			exptID:  123,
			session: &entity.Session{UserID: "test"},
			setup: func(svc *ExptResultExportService) {
				svc.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().
					GetByID(gomock.Any(), int64(123), int64(1)).
					Return(&entity.Experiment{ID: 123, Status: entity.ExptStatus_Success}, nil).
					Times(1)

				svc.repo.(*repoMocks.MockIExptResultExportRecordRepo).EXPECT().
					List(gomock.Any(), int64(1), int64(123), gomock.Any(), ptr.Of(int32(entity.CSVExportStatus_Running))).
					Return([]*entity.ExptResultExportRecord{}, int64(0), nil).
					Times(1)

				svc.repo.(*repoMocks.MockIExptResultExportRecordRepo).EXPECT().
					Create(gomock.Any(), gomock.Any()).
					Return(int64(0), errors.New("create error")).
					Times(1)
				svc.benefitService.(*benefitMocks.MockIBenefitService).EXPECT().BatchCheckEnableTypeBenefit(gomock.Any(), gomock.Any()).
					Return(&benefit.BatchCheckEnableTypeBenefitResult{Results: map[string]bool{"exp_download_report_enabled": true}}, nil)
				svc.configer.(*componentMocks.MockIConfiger).EXPECT().GetExptExportWhiteList(gomock.Any()).
					Return(&entity.ExptExportWhiteList{UserIDs: []int64{}}).AnyTimes()
			},
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestExptResultExportService(ctrl)
			tt.setup(svc)

			got, err := svc.ExportCSV(context.Background(), tt.spaceID, tt.exptID, tt.session, (*entity.ExptResultExportColumnSpec)(nil))
			if (err != nil) != tt.wantErr {
				t.Errorf("ExportCSV() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ExportCSV() got = %v, want %v", got, tt.want)
			}
			if tt.wantErr && tt.errorCode != 0 {
				var errx *errno.ErrImpl
				if errors.As(err, &errx) && errx.Code != tt.errorCode {
					t.Errorf("ExportCSV() error code = %v, want %v", errx.Code, tt.errorCode)
				}
			}
		})
	}
}

func TestExptResultExportService_GetExptExportRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name     string
		spaceID  int64
		exportID int64
		setup    func(svc *ExptResultExportService)
		want     *entity.ExptResultExportRecord
		wantErr  bool
	}{
		{
			name:     "正常获取",
			spaceID:  1,
			exportID: 123,
			setup: func(svc *ExptResultExportService) {
				record := &entity.ExptResultExportRecord{
					ID:              123,
					SpaceID:         1,
					ExptID:          456,
					CsvExportStatus: entity.CSVExportStatus_Success,
				}
				svc.repo.(*repoMocks.MockIExptResultExportRecordRepo).EXPECT().
					Get(gomock.Any(), int64(1), int64(123)).
					Return(record, nil).
					Times(1)
			},
			want: &entity.ExptResultExportRecord{
				ID:              123,
				SpaceID:         1,
				ExptID:          456,
				CsvExportStatus: entity.CSVExportStatus_Success,
			},
			wantErr: false,
		},
		{
			name:     "获取失败",
			spaceID:  1,
			exportID: 123,
			setup: func(svc *ExptResultExportService) {
				svc.repo.(*repoMocks.MockIExptResultExportRecordRepo).EXPECT().
					Get(gomock.Any(), int64(1), int64(123)).
					Return(nil, errors.New("db error")).
					Times(1)
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestExptResultExportService(ctrl)
			tt.setup(svc)

			got, err := svc.GetExptExportRecord(context.Background(), tt.spaceID, tt.exportID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetExptExportRecord() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.ID != tt.want.ID {
				t.Errorf("GetExptExportRecord() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExptResultExportService_UpdateExportRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name         string
		exportRecord *entity.ExptResultExportRecord
		setup        func(svc *ExptResultExportService)
		wantErr      bool
	}{
		{
			name: "正常更新",
			exportRecord: &entity.ExptResultExportRecord{
				ID:              123,
				CsvExportStatus: entity.CSVExportStatus_Success,
			},
			setup: func(svc *ExptResultExportService) {
				svc.repo.(*repoMocks.MockIExptResultExportRecordRepo).EXPECT().
					Update(gomock.Any(), gomock.Any()).
					Return(nil).
					Times(1)
			},
			wantErr: false,
		},
		{
			name: "更新失败",
			exportRecord: &entity.ExptResultExportRecord{
				ID:              123,
				CsvExportStatus: entity.CSVExportStatus_Failed,
			},
			setup: func(svc *ExptResultExportService) {
				svc.repo.(*repoMocks.MockIExptResultExportRecordRepo).EXPECT().
					Update(gomock.Any(), gomock.Any()).
					Return(errors.New("update error")).
					Times(1)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestExptResultExportService(ctrl)
			tt.setup(svc)

			err := svc.UpdateExportRecord(context.Background(), tt.exportRecord)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateExportRecord() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptResultExportService_ListExportRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name      string
		spaceID   int64
		exptID    int64
		page      entity.Page
		setup     func(svc *ExptResultExportService)
		want      []*entity.ExptResultExportRecord
		wantCount int64
		wantErr   bool
	}{
		{
			name:    "正常获取列表",
			spaceID: 1,
			exptID:  123,
			page:    entity.NewPage(1, 10),
			setup: func(svc *ExptResultExportService) {
				records := []*entity.ExptResultExportRecord{
					{ID: 1, SpaceID: 1, ExptID: 123, CsvExportStatus: entity.CSVExportStatus_Success},
					{ID: 2, SpaceID: 1, ExptID: 123, CsvExportStatus: entity.CSVExportStatus_Failed},
				}
				svc.repo.(*repoMocks.MockIExptResultExportRecordRepo).EXPECT().
					List(gomock.Any(), int64(1), int64(123), gomock.Any(), nil).
					Return(records, int64(2), nil).
					Times(1)
			},
			want: []*entity.ExptResultExportRecord{
				{ID: 1, SpaceID: 1, ExptID: 123, CsvExportStatus: entity.CSVExportStatus_Success},
				{ID: 2, SpaceID: 1, ExptID: 123, CsvExportStatus: entity.CSVExportStatus_Failed},
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:    "获取列表失败",
			spaceID: 1,
			exptID:  123,
			page:    entity.NewPage(1, 10),
			setup: func(svc *ExptResultExportService) {
				svc.repo.(*repoMocks.MockIExptResultExportRecordRepo).EXPECT().
					List(gomock.Any(), int64(1), int64(123), gomock.Any(), nil).
					Return(nil, int64(0), errors.New("list error")).
					Times(1)
			},
			want:      nil,
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestExptResultExportService(ctrl)
			tt.setup(svc)

			got, count, err := svc.ListExportRecord(context.Background(), tt.spaceID, tt.exptID, tt.page)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListExportRecord() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if count != tt.wantCount {
				t.Errorf("ListExportRecord() count = %v, want %v", count, tt.wantCount)
			}
			if !tt.wantErr && len(got) != len(tt.want) {
				t.Errorf("ListExportRecord() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExptResultExportService_DoExportCSV(t *testing.T) {
	tests := []struct {
		name     string
		spaceID  int64
		exptID   int64
		exportID int64
		setup    func(svc *ExptResultExportService)
		wantErr  bool
	}{
		{
			name:     "正常导出",
			spaceID:  1,
			exptID:   123,
			exportID: 456,
			setup: func(svc *ExptResultExportService) {
				// MGetExperimentResult模拟调用
				colEvaluators := []*entity.ColumnEvaluator{{EvaluatorVersionID: 1, Name: ptr.Of("test_evaluator"), Version: ptr.Of("v1")}}
				colEvalSetFields := []*entity.ColumnEvalSetField{{Name: ptr.Of("test_field")}}
				colAnnotation := []*entity.ColumnAnnotation{{TagKeyID: 1, TagName: "test_tag"}}
				exptColAnnotation := []*entity.ExptColumnAnnotation{{ExptID: 123, ColumnAnnotations: colAnnotation}}
				itemResults := []*entity.ItemResult{
					{ItemID: 1, TurnResults: []*entity.TurnResult{
						{
							TurnID: 1,
							ExperimentResults: []*entity.ExperimentResult{
								{
									ExperimentID: 123,
									Payload: &entity.ExperimentTurnPayload{
										TurnID: 1,
										EvalSet: &entity.TurnEvalSet{
											Turn: &entity.Turn{
												ID: 1,
												FieldDataList: []*entity.FieldData{
													{
														Key:  "key",
														Name: "name",
														Content: &entity.Content{
															ContentType: ptr.Of(entity.ContentTypeText),
															Text:        ptr.Of("text"),
														},
													},
												},
											},
										},
										TargetOutput: &entity.TurnTargetOutput{
											EvalTargetRecord: &entity.EvalTargetRecord{
												ID: 1,
												EvalTargetOutputData: &entity.EvalTargetOutputData{
													OutputFields: map[string]*entity.Content{
														consts.OutputSchemaKey: {
															ContentType: ptr.Of(entity.ContentTypeText),
															Text:        ptr.Of("text"),
														},
													},
												},
											},
										},
										EvaluatorOutput: &entity.TurnEvaluatorOutput{EvaluatorRecords: map[int64]*entity.EvaluatorRecord{
											1: {
												ID:                 1,
												EvaluatorVersionID: 1,
												EvaluatorOutputData: &entity.EvaluatorOutputData{
													EvaluatorResult: &entity.EvaluatorResult{
														Score:      ptr.Of(float64(1)),
														Correction: nil,
														Reasoning:  "理由",
													},
												},
												Status: entity.EvaluatorRunStatusSuccess,
											},
										}},
										SystemInfo: nil,
										AnnotateResult: &entity.TurnAnnotateResult{
											AnnotateRecords: map[int64]*entity.AnnotateRecord{
												1: {
													ID:           1,
													SpaceID:      1,
													TagKeyID:     1,
													ExperimentID: 123,
													AnnotateData: &entity.AnnotateData{
														Score:          ptr.Of(float64(1)),
														TextValue:      nil,
														BoolValue:      nil,
														Option:         nil,
														TagContentType: entity.TagContentTypeContinuousNumber,
													},
													TagValueID: 0,
												},
											},
										},
									},
								},
							},
						},
					}},
				}
				svc.exptResultService.(*svcMocks.MockExptResultService).EXPECT().
					MGetExperimentResult(gomock.Any(), gomock.Any()).
					Return(&entity.MGetExperimentReportResult{
						ColumnEvaluators:      colEvaluators,
						ColumnEvalSetFields:   colEvalSetFields,
						ExptColumnAnnotations: exptColAnnotation,
						ItemResults:           itemResults,
						Total:                 int64(len(itemResults)),
					}, nil).
					Times(1)

				svc.fileClient.(*fileserverMocks.MockObjectStorage).EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name:     "MGetExperimentResult失败",
			spaceID:  1,
			exptID:   123,
			exportID: 456,
			setup: func(svc *ExptResultExportService) {
				// MGetExperimentResult返回错误
				svc.exptResultService.(*svcMocks.MockExptResultService).EXPECT().
					MGetExperimentResult(gomock.Any(), gomock.Any()).
					Return(nil, fmt.Errorf("MGetExperimentResult error"))
			},
			wantErr: true,
		},
		{
			name:     "多页数据导出",
			spaceID:  1,
			exptID:   123,
			exportID: 456,
			setup: func(svc *ExptResultExportService) {
				// 第一页数据
				colEvaluators := []*entity.ColumnEvaluator{{EvaluatorVersionID: 1, Name: ptr.Of("test_evaluator"), Version: ptr.Of("v1")}}
				colEvalSetFields := []*entity.ColumnEvalSetField{{Name: ptr.Of("test_field")}}
				colAnnotation := []*entity.ColumnAnnotation{{TagKeyID: 1, TagName: "test_tag"}}
				exptColAnnotation := []*entity.ExptColumnAnnotation{{ExptID: 123, ColumnAnnotations: colAnnotation}}
				itemResults1 := []*entity.ItemResult{{ItemID: 1}}
				itemResults2 := []*entity.ItemResult{{ItemID: 2}}

				// 第一次调用：游标分页，返回下一批游标以触发第二次拉取
				svc.exptResultService.(*svcMocks.MockExptResultService).EXPECT().
					MGetExperimentResult(gomock.Any(), gomock.Any()).
					Return(&entity.MGetExperimentReportResult{
						ColumnEvaluators:      colEvaluators,
						ColumnEvalSetFields:   colEvalSetFields,
						ExptColumnAnnotations: exptColAnnotation,
						ItemResults:           itemResults1,
						Total:                 int64(25),
						NextTurnListCursor:    &entity.ExptTurnResultListCursor{ItemIdx: 0, TurnIdx: -1, ItemID: 1, TurnID: 0},
					}, nil).
					Times(1)

				// 第二次调用：无更多数据
				svc.exptResultService.(*svcMocks.MockExptResultService).EXPECT().
					MGetExperimentResult(gomock.Any(), gomock.Any()).
					Return(&entity.MGetExperimentReportResult{
						ColumnEvaluators:      colEvaluators,
						ColumnEvalSetFields:   colEvalSetFields,
						ExptColumnAnnotations: exptColAnnotation,
						ItemResults:           itemResults2,
						Total:                 0,
					}, nil).
					Times(1)

				svc.fileClient.(*fileserverMocks.MockObjectStorage).EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name:     "文件上传失败",
			spaceID:  1,
			exptID:   123,
			exportID: 456,
			setup: func(svc *ExptResultExportService) {
				colEvaluators := []*entity.ColumnEvaluator{{EvaluatorVersionID: 1, Name: ptr.Of("test_evaluator"), Version: ptr.Of("v1")}}
				colEvalSetFields := []*entity.ColumnEvalSetField{{Name: ptr.Of("test_field")}}
				colAnnotation := []*entity.ColumnAnnotation{{TagKeyID: 1, TagName: "test_tag"}}
				exptColAnnotation := []*entity.ExptColumnAnnotation{{ExptID: 123, ColumnAnnotations: colAnnotation}}
				itemResults := []*entity.ItemResult{{ItemID: 1}}

				svc.exptResultService.(*svcMocks.MockExptResultService).EXPECT().
					MGetExperimentResult(gomock.Any(), gomock.Any()).
					Return(&entity.MGetExperimentReportResult{
						ColumnEvaluators:      colEvaluators,
						ColumnEvalSetFields:   colEvalSetFields,
						ExptColumnAnnotations: exptColAnnotation,
						ItemResults:           itemResults,
						Total:                 int64(1),
					}, nil).
					Times(1)

				svc.fileClient.(*fileserverMocks.MockObjectStorage).EXPECT().
					Upload(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(fmt.Errorf("upload failed"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			svc := newTestExptResultExportService(ctrl)
			tt.setup(svc)

			out := filepath.Join(t.TempDir(), "file_name")
			err := svc.DoExportCSV(context.Background(), tt.spaceID, tt.exptID, out, true, (*entity.ExptResultExportColumnSpec)(nil))
			if (err != nil) != tt.wantErr {
				t.Errorf("DoExportCSV() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptResultExportService_GetAnnotationData(t *testing.T) {
	tests := []struct {
		name             string
		record           *entity.AnnotateRecord
		columnAnnotation *entity.ColumnAnnotation
		expected         string
	}{
		{
			name:             "空记录",
			record:           nil,
			columnAnnotation: &entity.ColumnAnnotation{TagContentType: entity.TagContentTypeContinuousNumber},
			expected:         "",
		},
		{
			name: "连续数字类型",
			record: &entity.AnnotateRecord{
				AnnotateData: &entity.AnnotateData{
					Score:          ptr.Of(85.5),
					TagContentType: entity.TagContentTypeContinuousNumber,
				},
			},
			columnAnnotation: &entity.ColumnAnnotation{TagContentType: entity.TagContentTypeContinuousNumber},
			expected:         "85.50",
		},
		{
			name: "自由文本类型",
			record: &entity.AnnotateRecord{
				AnnotateData: &entity.AnnotateData{
					TextValue:      ptr.Of("test text"),
					TagContentType: entity.TagContentTypeFreeText,
				},
			},
			columnAnnotation: &entity.ColumnAnnotation{TagContentType: entity.TagContentTypeFreeText},
			expected:         "test text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getAnnotationData(tt.record, tt.columnAnnotation)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExptResultExportService_HandleExportEvent(t *testing.T) {
	tests := []struct {
		name     string
		spaceID  int64
		exptID   int64
		exportID int64
		setup    func(svc *ExptResultExportService)
		wantErr  bool
	}{
		{
			name:     "正常处理导出事件",
			spaceID:  1,
			exptID:   123,
			exportID: 456,
			setup: func(svc *ExptResultExportService) {
				// Mock GetByID获取实验信息
				expt := &entity.Experiment{ID: 123, Name: "test_expt"}
				svc.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().
					GetByID(gomock.Any(), int64(123), int64(1)).
					Return(expt, nil)

				// Mock DoExportCSV成功
				colEvaluators := []*entity.ColumnEvaluator{{EvaluatorVersionID: 1, Name: ptr.Of("test_evaluator"), Version: ptr.Of("v1")}}
				colEvalSetFields := []*entity.ColumnEvalSetField{{Name: ptr.Of("test_field")}}
				colAnnotation := []*entity.ColumnAnnotation{{TagKeyID: 1, TagName: "test_tag"}}
				exptColAnnotation := []*entity.ExptColumnAnnotation{{ExptID: 123, ColumnAnnotations: colAnnotation}}
				itemResults := []*entity.ItemResult{{ItemID: 1}}

				svc.exptResultService.(*svcMocks.MockExptResultService).EXPECT().
					MGetExperimentResult(gomock.Any(), gomock.Any()).
					Return(&entity.MGetExperimentReportResult{
						ColumnEvaluators:      colEvaluators,
						ColumnEvalSetFields:   colEvalSetFields,
						ExptColumnAnnotations: exptColAnnotation,
						ItemResults:           itemResults,
						Total:                 int64(1),
					}, nil).
					Times(1)

				svc.fileClient.(*fileserverMocks.MockObjectStorage).EXPECT().
					Upload(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)

				// Mock UpdateExportRecord成功
				svc.repo.(*repoMocks.MockIExptResultExportRecordRepo).EXPECT().
					Update(gomock.Any(), gomock.Any()).
					Return(nil).
					Times(1)
			},
			wantErr: false,
		},
		{
			name:     "DoExportCSV失败",
			spaceID:  1,
			exptID:   123,
			exportID: 456,
			setup: func(svc *ExptResultExportService) {
				// Mock GetByID获取实验信息
				expt := &entity.Experiment{ID: 123, Name: "test_expt"}
				svc.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().
					GetByID(gomock.Any(), int64(123), int64(1)).
					Return(expt, nil)

				// Mock DoExportCSV失败
				svc.exptResultService.(*svcMocks.MockExptResultService).EXPECT().
					MGetExperimentResult(gomock.Any(), gomock.Any()).
					Return(nil, fmt.Errorf("export failed")).
					Times(1)

				// Mock GetErrCtrl
				svc.configer.(*componentMocks.MockIConfiger).EXPECT().
					GetErrCtrl(gomock.Any()).
					Return(nil)

				// Mock UpdateExportRecord失败状态
				svc.repo.(*repoMocks.MockIExptResultExportRecordRepo).EXPECT().
					Update(gomock.Any(), gomock.Any()).
					Return(nil).
					Times(1)
			},
			wantErr: true,
		},
		{
			name:     "UpdateExportRecord失败",
			spaceID:  1,
			exptID:   123,
			exportID: 456,
			setup: func(svc *ExptResultExportService) {
				// Mock GetByID获取实验信息
				expt := &entity.Experiment{ID: 123, Name: "test_expt"}
				svc.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().
					GetByID(gomock.Any(), int64(123), int64(1)).
					Return(expt, nil)

				// Mock DoExportCSV成功
				colEvaluators := []*entity.ColumnEvaluator{{EvaluatorVersionID: 1, Name: ptr.Of("test_evaluator"), Version: ptr.Of("v1")}}
				colEvalSetFields := []*entity.ColumnEvalSetField{{Name: ptr.Of("test_field")}}
				colAnnotation := []*entity.ColumnAnnotation{{TagKeyID: 1, TagName: "test_tag"}}
				exptColAnnotation := []*entity.ExptColumnAnnotation{{ExptID: 123, ColumnAnnotations: colAnnotation}}
				itemResults := []*entity.ItemResult{{ItemID: 1}}

				svc.exptResultService.(*svcMocks.MockExptResultService).EXPECT().
					MGetExperimentResult(gomock.Any(), gomock.Any()).
					Return(&entity.MGetExperimentReportResult{
						ColumnEvaluators:      colEvaluators,
						ColumnEvalSetFields:   colEvalSetFields,
						ExptColumnAnnotations: exptColAnnotation,
						ItemResults:           itemResults,
						Total:                 int64(1),
					}, nil).
					Times(1)

				svc.fileClient.(*fileserverMocks.MockObjectStorage).EXPECT().
					Upload(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)

				// Mock UpdateExportRecord失败
				svc.repo.(*repoMocks.MockIExptResultExportRecordRepo).EXPECT().
					Update(gomock.Any(), gomock.Any()).
					Return(fmt.Errorf("update failed")).
					Times(1)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			svc := newTestExptResultExportService(ctrl)
			tt.setup(svc)

			err := svc.HandleExportEvent(context.Background(), &entity.ExportCSVEvent{
				SpaceID:      tt.spaceID,
				ExperimentID: tt.exptID,
				ExportID:     tt.exportID,
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("HandleExportEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsExportRecordExpired(t *testing.T) {
	tests := []struct {
		name       string
		targetTime *time.Time
		want       bool
	}{
		{
			name:       "记录未过期",
			targetTime: ptr.Of(time.Now().Add(-23 * time.Hour)),
			want:       false,
		},
		{
			name:       "记录已过期",
			targetTime: ptr.Of(time.Now().Add(-24 * 101 * time.Hour)),
			want:       true,
		},
		{
			name:       "时间为空",
			targetTime: nil,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isExportRecordExpired(tt.targetTime)
			if got != tt.want {
				t.Errorf("isExportRecordExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewExptResultExportService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxDB := dbMocks.NewMockProvider(ctrl)
	mockRepo := repoMocks.NewMockIExptResultExportRecordRepo(ctrl)
	mockExptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	mockExptPublisher := eventsMocks.NewMockExptEventPublisher(ctrl)
	mockExptResultService := svcMocks.NewMockExptResultService(ctrl)
	mockFileClient := fileserverMocks.NewMockObjectStorage(ctrl)
	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	mockBenefit := benefitMocks.NewMockIBenefitService(ctrl)
	urlProcessor := NewDefaultURLProcessor()
	mockEvalSetItemSvc := svcMocks.NewMockEvaluationSetItemService(ctrl)
	svc := NewExptResultExportService(
		mockTxDB,
		mockRepo,
		mockExptRepo,
		mockExptTurnResultRepo,
		mockExptPublisher,
		mockExptResultService,
		mockFileClient,
		mockConfiger,
		mockBenefit,
		urlProcessor,
		mockEvalSetItemSvc,
	)

	impl, ok := svc.(*ExptResultExportService)
	if !ok {
		t.Fatalf("NewExptResultExportService should return *ExptResultExportService")
	}

	// 验证依赖是否正确设置
	if impl.txDB != mockTxDB {
		t.Errorf("txDB not set correctly")
	}
	if impl.repo != mockRepo {
		t.Errorf("repo not set correctly")
	}
	if impl.exptRepo != mockExptRepo {
		t.Errorf("exptRepo not set correctly")
	}
	if impl.exptTurnResultRepo != mockExptTurnResultRepo {
		t.Errorf("exptTurnResultRepo not set correctly")
	}
	if impl.exptPublisher != mockExptPublisher {
		t.Errorf("exptPublisher not set correctly")
	}
	if impl.exptResultService != mockExptResultService {
		t.Errorf("exptResultService not set correctly")
	}
	if impl.fileClient != mockFileClient {
		t.Errorf("fileClient not set correctly")
	}
	if impl.configer != mockConfiger {
		t.Errorf("configer not set correctly")
	}
	if impl.benefitService != mockBenefit {
		t.Errorf("benefit not set correctly")
	}
	if impl.evalSetItemSvc != mockEvalSetItemSvc {
		t.Errorf("evalSetItemSvc not set correctly")
	}
}

func Test_itemRunStateToString(t *testing.T) {
	// 测试用例：所有枚举值映射关系
	tests := []struct {
		name     string
		input    entity.ItemRunState
		expected string
	}{
		{
			name:     "unknown_state",
			input:    entity.ItemRunState_Unknown,
			expected: "unknown",
		},
		{
			name:     "queueing_state",
			input:    entity.ItemRunState_Queueing,
			expected: "queueing",
		},
		{
			name:     "processing_state",
			input:    entity.ItemRunState_Processing,
			expected: "processing",
		},
		{
			name:     "success_state",
			input:    entity.ItemRunState_Success,
			expected: "success",
		},
		{
			name:     "fail_state",
			input:    entity.ItemRunState_Fail,
			expected: "fail",
		},
		{
			name:     "terminal_state",
			input:    entity.ItemRunState_Terminal,
			expected: "terminal",
		},
		{
			name:     "default_case",
			input:    entity.ItemRunState(999), // 未定义枚举值测试默认分支
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := itemRunStateToString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_toContentStr(t *testing.T) {
	ctx := context.Background()
	ins := &exportCSVHelper{}
	// 测试用例：覆盖所有内容类型和边界情况,sss
	tests := []struct {
		name     string
		input    *entity.Content
		expected string
	}{
		{
			name:     "nil_content",
			input:    nil,
			expected: "",
		},
		{
			name: "text_content",
			input: &entity.Content{
				ContentType: ptr.Of(entity.ContentTypeText),
				Text:        ptr.Of("测试文本内容"),
			},
			expected: "测试文本内容",
		},
		{
			name: "image_content",
			input: &entity.Content{
				ContentType: ptr.Of(entity.ContentTypeImage),
				Image: &entity.Image{
					URL: ptr.Of("https://example.com/image.png"),
				},
			},
			expected: "",
		},
		{
			name: "audio_content",
			input: &entity.Content{
				ContentType: ptr.Of(entity.ContentTypeAudio),
			},
			expected: "",
		},
		{
			name: "multipart_text_only",
			input: &entity.Content{
				ContentType: ptr.Of(entity.ContentTypeMultipart),
				MultiPart: []*entity.Content{
					{
						ContentType: ptr.Of(entity.ContentTypeText),
						Text:        ptr.Of("文本段落1"),
					},
					{
						ContentType: ptr.Of(entity.ContentTypeText),
						Text:        ptr.Of("文本段落2"),
					},
				},
			},
			expected: "文本段落1\n文本段落2\n",
		},
		{
			name: "multipart_mixed_content",
			input: &entity.Content{
				ContentType: ptr.Of(entity.ContentTypeMultipart),
				MultiPart: []*entity.Content{
					{
						ContentType: ptr.Of(entity.ContentTypeText),
						Text:        ptr.Of("图文混合"),
					},
					{
						ContentType: ptr.Of(entity.ContentTypeImage),
						Image: &entity.Image{
							URL: ptr.Of("https://example.com/pic.jpg"),
						},
					},
					{
						ContentType: ptr.Of(entity.ContentTypeAudio),
					},
				},
			},
			expected: "图文混合\n<ref_image_url:https://example.com/pic.jpg>\n",
		},
		{
			name: "unknown_content_type",
			input: &entity.Content{
				ContentType: ptr.Of(entity.ContentType("999")), // 未定义的内容类型
				Text:        ptr.Of("不应该被返回"),
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := ins.toContentStr(ctx, tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_formatMultiPartData(t *testing.T) {
	tests := []struct {
		name     string
		input    *entity.Content
		expected string
	}{
		{
			name: "empty_multipart",
			input: &entity.Content{
				ContentType: ptr.Of(entity.ContentTypeMultipart),
				MultiPart:   []*entity.Content{},
			},
			expected: "",
		},
		{
			name: "mixed_content",
			input: &entity.Content{
				ContentType: ptr.Of(entity.ContentTypeMultipart),
				MultiPart: []*entity.Content{
					{
						ContentType: ptr.Of(entity.ContentTypeText),
						Text:        ptr.Of("Hello"),
					},
					{
						ContentType: ptr.Of(entity.ContentTypeImage),
						Image: &entity.Image{
							URL: ptr.Of("http://image.png"),
						},
					},
					{
						ContentType: ptr.Of(entity.ContentTypeAudio),
						Audio: &entity.Audio{
							URL: ptr.Of("http://audio.mp3"),
						},
					},
					{
						ContentType: ptr.Of(entity.ContentTypeVideo),
						Video: &entity.Video{
							URL: ptr.Of("http://video.mp4"),
						},
					},
					{
						ContentType: ptr.Of(entity.ContentTypeMultipart), // Should be skipped
						MultiPart:   []*entity.Content{},
					},
					{
						ContentType: ptr.Of(entity.ContentType("unknown")), // Should be skipped
					},
				},
			},
			expected: "Hello\n<ref_image_url:http://image.png>\n<ref_audio_url:http://audio.mp3>\n<ref_video_url:http://video.mp4>\n",
		},
		{
			name: "content_without_urls",
			input: &entity.Content{
				ContentType: ptr.Of(entity.ContentTypeMultipart),
				MultiPart: []*entity.Content{
					{
						ContentType: ptr.Of(entity.ContentTypeImage),
						Image:       &entity.Image{},
					},
					{
						ContentType: ptr.Of(entity.ContentTypeAudio),
						Audio:       &entity.Audio{},
					},
					{
						ContentType: ptr.Of(entity.ContentTypeVideo),
						Video:       &entity.Video{},
					},
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatMultiPartData(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_getColumnNameEvaluator(t *testing.T) {
	tests := []struct {
		name          string
		evaluatorName string
		version       string
		want          string
	}{
		{
			name:          "basic",
			evaluatorName: "accuracy",
			version:       "v1",
			want:          "accuracy<v1>",
		},
		{
			name:          "empty_name_and_version",
			evaluatorName: "",
			version:       "",
			want:          "<>",
		},
		{
			name:          "special_characters",
			evaluatorName: "eval-name_123",
			version:       "v2.0",
			want:          "eval-name_123<v2.0>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getColumnNameEvaluator(tt.evaluatorName, tt.version)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_getColumnNameEvaluatorReason(t *testing.T) {
	tests := []struct {
		name          string
		evaluatorName string
		version       string
		want          string
	}{
		{
			name:          "basic",
			evaluatorName: "accuracy",
			version:       "v1",
			want:          "accuracy<v1>_reason",
		},
		{
			name:          "empty_name_and_version",
			evaluatorName: "",
			version:       "",
			want:          "<>_reason",
		},
		{
			name:          "special_characters",
			evaluatorName: "eval-name_123",
			version:       "v2.0",
			want:          "eval-name_123<v2.0>_reason",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getColumnNameEvaluatorReason(tt.evaluatorName, tt.version)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_wantWeightedScoreColumn(t *testing.T) {
	tests := []struct {
		name                 string
		reportEvaluatorCount int
		colSelection         *exportColumnSelection
		want                 bool
	}{
		{
			name:                 "no_evaluators",
			reportEvaluatorCount: 0,
			colSelection:         nil,
			want:                 false,
		},
		{
			name:                 "nil_selection_with_evaluators",
			reportEvaluatorCount: 2,
			colSelection:         nil,
			want:                 true,
		},
		{
			name:                 "export_all",
			reportEvaluatorCount: 1,
			colSelection:         &exportColumnSelection{exportAll: true},
			want:                 true,
		},
		{
			name:                 "whitelist_with_weighted_score",
			reportEvaluatorCount: 1,
			colSelection: &exportColumnSelection{
				exportAll: false,
				keys:      map[string]struct{}{exportColKeyWeightedScore: {}},
			},
			want: true,
		},
		{
			name:                 "whitelist_without_weighted_score",
			reportEvaluatorCount: 1,
			colSelection: &exportColumnSelection{
				exportAll: false,
				keys:      map[string]struct{}{"other_key": {}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &exportCSVHelper{
				reportEvaluatorCount: tt.reportEvaluatorCount,
				colSelection:         tt.colSelection,
			}
			got := h.wantWeightedScoreColumn()
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_getEvaluatorScore(t *testing.T) {
	tests := []struct {
		name   string
		record *entity.EvaluatorRecord
		want   string
	}{
		{
			name:   "nil_record",
			record: nil,
			want:   "",
		},
		{
			name: "nil_output_data",
			record: &entity.EvaluatorRecord{
				EvaluatorOutputData: nil,
			},
			want: "",
		},
		{
			name: "nil_result",
			record: &entity.EvaluatorRecord{
				EvaluatorOutputData: &entity.EvaluatorOutputData{
					EvaluatorResult: nil,
				},
			},
			want: "",
		},
		{
			name: "nil_score",
			record: &entity.EvaluatorRecord{
				EvaluatorOutputData: &entity.EvaluatorOutputData{
					EvaluatorResult: &entity.EvaluatorResult{
						Score: nil,
					},
				},
			},
			want: "",
		},
		{
			name: "normal_score",
			record: &entity.EvaluatorRecord{
				EvaluatorOutputData: &entity.EvaluatorOutputData{
					EvaluatorResult: &entity.EvaluatorResult{
						Score: ptr.Of(float64(88.5)),
					},
				},
			},
			want: "88.50",
		},
		{
			name: "correction_score_overrides",
			record: &entity.EvaluatorRecord{
				EvaluatorOutputData: &entity.EvaluatorOutputData{
					EvaluatorResult: &entity.EvaluatorResult{
						Score: ptr.Of(float64(70)),
						Correction: &entity.Correction{
							Score: ptr.Of(float64(95.12)),
						},
					},
				},
			},
			want: "95.12",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getEvaluatorScore(tt.record)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_getEvaluatorReason(t *testing.T) {
	tests := []struct {
		name   string
		record *entity.EvaluatorRecord
		want   string
	}{
		{
			name:   "nil_record",
			record: nil,
			want:   "",
		},
		{
			name: "nil_output_data",
			record: &entity.EvaluatorRecord{
				EvaluatorOutputData: nil,
			},
			want: "",
		},
		{
			name: "nil_result",
			record: &entity.EvaluatorRecord{
				EvaluatorOutputData: &entity.EvaluatorOutputData{
					EvaluatorResult: nil,
				},
			},
			want: "",
		},
		{
			name: "normal_reasoning",
			record: &entity.EvaluatorRecord{
				EvaluatorOutputData: &entity.EvaluatorOutputData{
					EvaluatorResult: &entity.EvaluatorResult{
						Reasoning: "good answer",
					},
				},
			},
			want: "good answer",
		},
		{
			name: "correction_explain_overrides",
			record: &entity.EvaluatorRecord{
				EvaluatorOutputData: &entity.EvaluatorOutputData{
					EvaluatorResult: &entity.EvaluatorResult{
						Reasoning: "original reason",
						Correction: &entity.Correction{
							Explain: "corrected reason",
						},
					},
				},
			},
			want: "corrected reason",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getEvaluatorReason(tt.record)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_getAnnotationData_tagValues(t *testing.T) {
	tests := []struct {
		name             string
		record           *entity.AnnotateRecord
		columnAnnotation *entity.ColumnAnnotation
		want             string
	}{
		{
			name: "boolean_with_matching_tag_value",
			record: &entity.AnnotateRecord{
				AnnotateData: &entity.AnnotateData{
					TagContentType: entity.TagContentTypeBoolean,
				},
				TagValueID: 10,
			},
			columnAnnotation: &entity.ColumnAnnotation{
				TagContentType: entity.TagContentTypeBoolean,
				TagValues: []*entity.TagValue{
					{TagValueId: 10, TagValueName: "true"},
					{TagValueId: 11, TagValueName: "false"},
				},
			},
			want: "true",
		},
		{
			name: "boolean_no_matching_tag_value",
			record: &entity.AnnotateRecord{
				AnnotateData: &entity.AnnotateData{
					TagContentType: entity.TagContentTypeBoolean,
				},
				TagValueID: 99,
			},
			columnAnnotation: &entity.ColumnAnnotation{
				TagContentType: entity.TagContentTypeBoolean,
				TagValues: []*entity.TagValue{
					{TagValueId: 10, TagValueName: "true"},
					{TagValueId: 11, TagValueName: "false"},
				},
			},
			want: "",
		},
		{
			name: "categorical_with_matching_tag_value",
			record: &entity.AnnotateRecord{
				AnnotateData: &entity.AnnotateData{
					TagContentType: entity.TagContentTypeCategorical,
				},
				TagValueID: 20,
			},
			columnAnnotation: &entity.ColumnAnnotation{
				TagContentType: entity.TagContentTypeCategorical,
				TagValues: []*entity.TagValue{
					{TagValueId: 20, TagValueName: "CategoryA"},
					{TagValueId: 21, TagValueName: "CategoryB"},
				},
			},
			want: "CategoryA",
		},
		{
			name: "categorical_no_matching_tag_value",
			record: &entity.AnnotateRecord{
				AnnotateData: &entity.AnnotateData{
					TagContentType: entity.TagContentTypeCategorical,
				},
				TagValueID: 999,
			},
			columnAnnotation: &entity.ColumnAnnotation{
				TagContentType: entity.TagContentTypeCategorical,
				TagValues: []*entity.TagValue{
					{TagValueId: 20, TagValueName: "CategoryA"},
				},
			},
			want: "",
		},
		{
			name: "categorical_with_nil_tag_value_entry",
			record: &entity.AnnotateRecord{
				AnnotateData: &entity.AnnotateData{
					TagContentType: entity.TagContentTypeCategorical,
				},
				TagValueID: 30,
			},
			columnAnnotation: &entity.ColumnAnnotation{
				TagContentType: entity.TagContentTypeCategorical,
				TagValues: []*entity.TagValue{
					nil,
					{TagValueId: 30, TagValueName: "ValidCategory"},
				},
			},
			want: "ValidCategory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getAnnotationData(tt.record, tt.columnAnnotation)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_cloneExptExportColumnSpec(t *testing.T) {
	t.Run("nil_input", func(t *testing.T) {
		got := cloneExptExportColumnSpec(nil)
		assert.Nil(t, got)
	})

	t.Run("normal_input_with_all_fields", func(t *testing.T) {
		weighted := true
		src := &entity.ExptResultExportColumnSpec{
			EvalSetFields:         []string{"field1", "field2"},
			EvalTargetOutputs:     []string{"output1"},
			Metrics:               []string{"metric1", "metric2"},
			EvaluatorVersionIds:   []string{"100", "200"},
			TagKeyIds: []string{"9"},
			WeightedScore:         &weighted,
		}
		got := cloneExptExportColumnSpec(src)
		require.NotNil(t, got)
		assert.Equal(t, src.EvalSetFields, got.EvalSetFields)
		assert.Equal(t, src.EvalTargetOutputs, got.EvalTargetOutputs)
		assert.Equal(t, src.Metrics, got.Metrics)
		assert.Equal(t, src.EvaluatorVersionIds, got.EvaluatorVersionIds)
		assert.Equal(t, src.TagKeyIds, got.TagKeyIds)
		require.NotNil(t, got.WeightedScore)
		assert.True(t, *got.WeightedScore)
	})

	t.Run("deep_copy_modification_does_not_affect_clone", func(t *testing.T) {
		src := &entity.ExptResultExportColumnSpec{
			EvalSetFields:       []string{"a", "b"},
			EvalTargetOutputs:   []string{"x"},
			Metrics:             []string{"m1"},
			EvaluatorVersionIds: []string{"1"},
		}
		got := cloneExptExportColumnSpec(src)
		require.NotNil(t, got)

		src.EvalSetFields[0] = "modified"
		src.EvalTargetOutputs = append(src.EvalTargetOutputs, "y")
		src.Metrics[0] = "m_changed"

		assert.Equal(t, "a", got.EvalSetFields[0])
		assert.Len(t, got.EvalTargetOutputs, 1)
		assert.Equal(t, "m1", got.Metrics[0])
	})
}

func Test_formatMultiPartData_nilObjects(t *testing.T) {
	tests := []struct {
		name     string
		input    *entity.Content
		expected string
	}{
		{
			name: "nil_image_object",
			input: &entity.Content{
				ContentType: ptr.Of(entity.ContentTypeMultipart),
				MultiPart: []*entity.Content{
					{
						ContentType: ptr.Of(entity.ContentTypeImage),
						Image:       nil,
					},
				},
			},
			expected: "",
		},
		{
			name: "nil_audio_object",
			input: &entity.Content{
				ContentType: ptr.Of(entity.ContentTypeMultipart),
				MultiPart: []*entity.Content{
					{
						ContentType: ptr.Of(entity.ContentTypeAudio),
						Audio:       nil,
					},
				},
			},
			expected: "",
		},
		{
			name: "nil_video_object",
			input: &entity.Content{
				ContentType: ptr.Of(entity.ContentTypeMultipart),
				MultiPart: []*entity.Content{
					{
						ContentType: ptr.Of(entity.ContentTypeVideo),
						Video:       nil,
					},
				},
			},
			expected: "",
		},
		{
			name: "mixed_nil_and_valid",
			input: &entity.Content{
				ContentType: ptr.Of(entity.ContentTypeMultipart),
				MultiPart: []*entity.Content{
					{
						ContentType: ptr.Of(entity.ContentTypeImage),
						Image:       nil,
					},
					{
						ContentType: ptr.Of(entity.ContentTypeText),
						Text:        ptr.Of("text data"),
					},
					{
						ContentType: ptr.Of(entity.ContentTypeVideo),
						Video:       nil,
					},
				},
			},
			expected: "text data\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatMultiPartData(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_buildColumns_withWhitelistSelection(t *testing.T) {
	ctx := context.Background()

	t.Run("only_score_column_for_evaluator", func(t *testing.T) {
		sel := &exportColumnSelection{
			exportAll: false,
			keys: map[string]struct{}{
				evaluatorColumnToken(1, "score"): {},
			},
		}
		h := &exportCSVHelper{
			reportEvaluatorCount: 1,
			colSelection:         sel,
			colEvaluators: []*entity.ColumnEvaluator{
				{EvaluatorVersionID: 1, Name: ptr.Of("eval1"), Version: ptr.Of("v1")},
			},
		}
		columns, err := h.buildColumns(ctx)
		require.NoError(t, err)
		assert.Contains(t, columns, "eval1<v1>")
		assert.NotContains(t, columns, "eval1<v1>_reason")
	})

	t.Run("only_reason_column_for_evaluator", func(t *testing.T) {
		sel := &exportColumnSelection{
			exportAll: false,
			keys: map[string]struct{}{
				evaluatorColumnToken(2, "reason"): {},
			},
		}
		h := &exportCSVHelper{
			reportEvaluatorCount: 1,
			colSelection:         sel,
			colEvaluators: []*entity.ColumnEvaluator{
				{EvaluatorVersionID: 2, Name: ptr.Of("eval2"), Version: ptr.Of("v1")},
			},
		}
		columns, err := h.buildColumns(ctx)
		require.NoError(t, err)
		assert.NotContains(t, columns, "eval2<v1>")
		assert.Contains(t, columns, "eval2<v1>_reason")
	})

	t.Run("both_score_and_reason_for_evaluator", func(t *testing.T) {
		sel := &exportColumnSelection{
			exportAll: false,
			keys: map[string]struct{}{
				evaluatorColumnToken(3, "score"):  {},
				evaluatorColumnToken(3, "reason"): {},
			},
		}
		h := &exportCSVHelper{
			reportEvaluatorCount: 1,
			colSelection:         sel,
			colEvaluators: []*entity.ColumnEvaluator{
				{EvaluatorVersionID: 3, Name: ptr.Of("eval3"), Version: ptr.Of("v2")},
			},
		}
		columns, err := h.buildColumns(ctx)
		require.NoError(t, err)
		assert.Contains(t, columns, "eval3<v2>")
		assert.Contains(t, columns, "eval3<v2>_reason")
	})

	t.Run("export_all_includes_both", func(t *testing.T) {
		h := &exportCSVHelper{
			reportEvaluatorCount: 1,
			colSelection:         &exportColumnSelection{exportAll: true},
			colEvaluators: []*entity.ColumnEvaluator{
				{EvaluatorVersionID: 4, Name: ptr.Of("eval4"), Version: ptr.Of("v1")},
			},
		}
		columns, err := h.buildColumns(ctx)
		require.NoError(t, err)
		assert.Contains(t, columns, "eval4<v1>")
		assert.Contains(t, columns, "eval4<v1>_reason")
	})

	t.Run("nil_selection_includes_both", func(t *testing.T) {
		h := &exportCSVHelper{
			reportEvaluatorCount: 1,
			colSelection:         nil,
			colEvaluators: []*entity.ColumnEvaluator{
				{EvaluatorVersionID: 5, Name: ptr.Of("eval5"), Version: ptr.Of("v3")},
			},
		}
		columns, err := h.buildColumns(ctx)
		require.NoError(t, err)
		assert.Contains(t, columns, "eval5<v3>")
		assert.Contains(t, columns, "eval5<v3>_reason")
	})
}
