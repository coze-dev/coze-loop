// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

			got, err := svc.ExportCSV(context.Background(), tt.spaceID, tt.exptID, tt.session)
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
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

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
				// 获取实验信息
				expt := &entity.Experiment{
					ID:   123,
					Name: "test_expt",
				}
				svc.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().
					GetByID(gomock.Any(), int64(123), int64(1)).
					Return(expt, nil).
					Times(1)

				// 新增MGetExperimentResult模拟调用
				colEvaluators := []*entity.ColumnEvaluator{{EvaluatorVersionID: 1, Name: ptr.Of("test_evaluator"), Version: ptr.Of("v1")}}
				colEvalSetFields := []*entity.ColumnEvalSetField{{Name: ptr.Of("test_field")}}
				colAnnotation := []*entity.ColumnAnnotation{{TagKeyID: 1, TagName: "test_tag"}}
				exptColAnnotation := []*entity.ExptColumnAnnotation{{ExptID: 1, ColumnAnnotations: colAnnotation}}
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
					Return(colEvaluators, nil, colEvalSetFields, exptColAnnotation, itemResults, int64(len(itemResults)), nil).
					Times(1)

				svc.fileClient.(*fileserverMocks.MockObjectStorage).EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				// // 获取实验统计信息
				// svc.exptResultService.(*svcMocks.MockExptResultService).EXPECT().
				//	GetStats(gomock.Any(), int64(123), int64(1), gomock.Any()).
				//	Return(&entity.ExptStats{}, nil).
				//	Times(1)
				//
				// // 获取实验轮次结果
				// svc.exptTurnResultRepo.(*repoMocks.MockIExptTurnResultRepo).EXPECT().
				//	ListTurnResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				//	Return([]*entity.ExptTurnResult{}, int64(0), nil).
				//	Times(1)

				// 更新导出记录为成功
				svc.repo.(*repoMocks.MockIExptResultExportRecordRepo).EXPECT().
					Update(gomock.Any(), gomock.Any()).
					Return(nil).
					Times(1)
			},
			wantErr: false,
		},
		// {
		//	name:     "获取导出记录失败",
		//	spaceID:  1,
		//	exptID:   123,
		//	exportID: 456,
		//	setup: func(svc *ExptResultExportService) {
		//		svc.repo.(*repoMocks.MockIExptResultExportRecordRepo).EXPECT().
		//			Get(gomock.Any(), int64(1), int64(456)).
		//			Return(nil, errors.New("get record error")).
		//			Times(1)
		//	},
		//	wantErr: true,
		// },
		{
			name:     "获取实验信息失败",
			spaceID:  1,
			exptID:   123,
			exportID: 456,
			setup: func(svc *ExptResultExportService) {
				// record := &entity.ExptResultExportRecord{
				//	ID:              456,
				//	SpaceID:         1,
				//	ExptID:          123,
				//	CsvExportStatus: entity.CSVExportStatus_Running,
				// }
				// svc.repo.(*repoMocks.MockIExptResultExportRecordRepo).EXPECT().
				//	Get(gomock.Any(), int64(1), int64(456)).
				//	Return(record, nil)

				// svc.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().
				//	GetByID(gomock.Any(), int64(123), int64(1)).
				//	Return(nil, errors.New("get expt error"))
				colEvaluators := []*entity.ColumnEvaluator{{Name: ptr.Of("test_evaluator"), Version: ptr.Of("v1")}}
				colEvalSetFields := []*entity.ColumnEvalSetField{{Name: ptr.Of("test_field")}}
				colAnnotation := []*entity.ColumnAnnotation{{TagName: "test_tag"}}
				exptColAnnotation := []*entity.ExptColumnAnnotation{{ExptID: 1, ColumnAnnotations: colAnnotation}}
				itemResults := []*entity.ItemResult{{ItemID: 1}}
				svc.exptResultService.(*svcMocks.MockExptResultService).EXPECT().
					MGetExperimentResult(gomock.Any(), gomock.Any()).
					Return(colEvaluators, nil, colEvalSetFields, exptColAnnotation, itemResults, int64(len(itemResults)), fmt.Errorf("err"))
				// 更新导出记录为失败
				svc.repo.(*repoMocks.MockIExptResultExportRecordRepo).EXPECT().
					Update(gomock.Any(), gomock.Any()).
					Return(nil)
				svc.configer.(*componentMocks.MockIConfiger).EXPECT().GetErrCtrl(gomock.Any()).Return(&entity.ExptErrCtrl{
					ResultErrConverts: []*entity.ResultErrConvert{{MatchedText: "err", ToErrMsg: "err"}},
					SpaceErrRetryCtrl: map[int64]*entity.ErrRetryCtrl{1: {RetryConf: &entity.RetryConf{RetryTimes: 2}}},
					ErrRetryCtrl:      &entity.ErrRetryCtrl{RetryConf: &entity.RetryConf{RetryTimes: 1}},
				})
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestExptResultExportService(ctrl)
			tt.setup(svc)

			err := svc.DoExportCSV(context.Background(), tt.spaceID, tt.exptID, tt.exportID)
			if (err != nil) != tt.wantErr {
				t.Errorf("DoExportCSV() error = %v, wantErr %v", err, tt.wantErr)
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

func Test_geDatasetCellOrActualOutputData(t *testing.T) {
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
			result := geDatasetCellOrActualOutputData(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
