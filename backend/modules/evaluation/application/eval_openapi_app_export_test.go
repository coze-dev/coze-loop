// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	domainexpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	openapiExperiment "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	exptpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestEvalOpenAPIApplication_ExportExperimentResultOApi(t *testing.T) {
	t.Parallel()

	workspaceID := int64(91001)
	experimentID := int64(91002)
	exportID := int64(91003)

	buildBaseReq := func() *openapi.ExportExperimentResultOApiRequest {
		return &openapi.ExportExperimentResultOApiRequest{
			WorkspaceID:  gptr.Of(workspaceID),
			ExperimentID: gptr.Of(experimentID),
		}
	}

	tests := []struct {
		name        string
		buildReq    func() *openapi.ExportExperimentResultOApiRequest
		setup       func(fakeApp *fakeExperimentApp)
		wantErr     int32
		wantNoInner bool // 期望未调用底层
		check       func(t *testing.T, fakeApp *fakeExperimentApp)
	}{
		{
			name:        "nil request",
			buildReq:    func() *openapi.ExportExperimentResultOApiRequest { return nil },
			wantErr:     errno.CommonInvalidParamCode,
			wantNoInner: true,
		},
		{
			name: "invalid workspace_id",
			buildReq: func() *openapi.ExportExperimentResultOApiRequest {
				req := buildBaseReq()
				req.WorkspaceID = gptr.Of(int64(0))
				return req
			},
			wantErr:     errno.CommonInvalidParamCode,
			wantNoInner: true,
		},
		{
			name: "invalid experiment_id",
			buildReq: func() *openapi.ExportExperimentResultOApiRequest {
				req := buildBaseReq()
				req.ExperimentID = gptr.Of(int64(0))
				return req
			},
			wantErr:     errno.CommonInvalidParamCode,
			wantNoInner: true,
		},
		{
			name:     "inner export error",
			buildReq: buildBaseReq,
			setup: func(fakeApp *fakeExperimentApp) {
				fakeApp.exportErr = errors.New("oops")
			},
			wantErr: -1,
		},
		{
			name:     "success without optional fields",
			buildReq: buildBaseReq,
			setup: func(fakeApp *fakeExperimentApp) {
				fakeApp.exportResp = &exptpb.ExportExptResultResponse{ExportID: exportID}
			},
			check: func(t *testing.T, fakeApp *fakeExperimentApp) {
				if assert.NotNil(t, fakeApp.lastExportReq) {
					assert.Equal(t, workspaceID, fakeApp.lastExportReq.GetWorkspaceID())
					assert.Equal(t, experimentID, fakeApp.lastExportReq.GetExptID())
					assert.Nil(t, fakeApp.lastExportReq.ExportColumns)
					assert.Nil(t, fakeApp.lastExportReq.ExportType)
				}
			},
		},
		{
			name: "success with export columns and type",
			buildReq: func() *openapi.ExportExperimentResultOApiRequest {
				req := buildBaseReq()
				req.ExportColumns = &openapiExperiment.ExptResultExportColumnSpec{
					EvalSetFields:       []string{"input"},
					EvaluatorVersionIds: []int64{101, 102},
					TagKeyIds:           []int64{7},
					WeightedScore:       gptr.Of(true),
				}
				exportType := openapiExperiment.ExptResultExportTypeCSV
				req.ExportType = &exportType
				return req
			},
			setup: func(fakeApp *fakeExperimentApp) {
				fakeApp.exportResp = &exptpb.ExportExptResultResponse{ExportID: exportID}
			},
			check: func(t *testing.T, fakeApp *fakeExperimentApp) {
				if assert.NotNil(t, fakeApp.lastExportReq) && assert.NotNil(t, fakeApp.lastExportReq.ExportColumns) {
					assert.Equal(t, []string{"input"}, fakeApp.lastExportReq.ExportColumns.EvalSetFields)
					assert.Equal(t, []string{"101", "102"}, fakeApp.lastExportReq.ExportColumns.EvaluatorVersionIds)
					assert.Equal(t, []string{"7"}, fakeApp.lastExportReq.ExportColumns.TagKeyIds)
					assert.True(t, fakeApp.lastExportReq.ExportColumns.GetWeightedScore())
				}
				if assert.NotNil(t, fakeApp.lastExportReq.ExportType) {
					assert.Equal(t, domainexpt.ExptResultExportTypeCSV, *fakeApp.lastExportReq.ExportType)
				}
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			metric := &fakeOpenAPIMetric{}
			fakeApp := &fakeExperimentApp{}

			app := &EvalOpenAPIApplication{
				experimentApp: fakeApp,
				metric:        metric,
			}

			req := tc.buildReq()
			if tc.setup != nil {
				tc.setup(fakeApp)
			}

			resp, err := app.ExportExperimentResultOApi(context.Background(), req)

			if tc.wantErr != 0 {
				assert.Error(t, err)
				if tc.wantErr > 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tc.wantErr, statusErr.Code())
				}
				assert.Nil(t, resp)
				if tc.wantNoInner {
					assert.Nil(t, fakeApp.lastExportReq)
				}
			} else {
				assert.NoError(t, err)
				if assert.NotNil(t, resp) && assert.NotNil(t, resp.Data) {
					assert.Equal(t, gptr.Of(exportID), resp.Data.ExportID)
				}
			}

			if tc.check != nil {
				tc.check(t, fakeApp)
			}

			assert.True(t, metric.called)
		})
	}
}

func TestEvalOpenAPIApplication_GetExperimentResultExportRecordOApi(t *testing.T) {
	t.Parallel()

	workspaceID := int64(92001)
	experimentID := int64(92002)
	exportID := int64(92003)

	buildBaseReq := func() *openapi.GetExperimentResultExportRecordOApiRequest {
		return &openapi.GetExperimentResultExportRecordOApiRequest{
			WorkspaceID:  gptr.Of(workspaceID),
			ExperimentID: gptr.Of(experimentID),
			ExportID:     gptr.Of(exportID),
		}
	}

	tests := []struct {
		name        string
		buildReq    func() *openapi.GetExperimentResultExportRecordOApiRequest
		setup       func(fakeApp *fakeExperimentApp)
		wantErr     int32
		wantNoInner bool
		check       func(t *testing.T, resp *openapi.GetExperimentResultExportRecordOApiResponse, fakeApp *fakeExperimentApp)
	}{
		{
			name:        "nil request",
			buildReq:    func() *openapi.GetExperimentResultExportRecordOApiRequest { return nil },
			wantErr:     errno.CommonInvalidParamCode,
			wantNoInner: true,
		},
		{
			name: "invalid workspace_id",
			buildReq: func() *openapi.GetExperimentResultExportRecordOApiRequest {
				req := buildBaseReq()
				req.WorkspaceID = gptr.Of(int64(0))
				return req
			},
			wantErr:     errno.CommonInvalidParamCode,
			wantNoInner: true,
		},
		{
			name: "invalid experiment_id",
			buildReq: func() *openapi.GetExperimentResultExportRecordOApiRequest {
				req := buildBaseReq()
				req.ExperimentID = gptr.Of(int64(0))
				return req
			},
			wantErr:     errno.CommonInvalidParamCode,
			wantNoInner: true,
		},
		{
			name: "invalid export_id",
			buildReq: func() *openapi.GetExperimentResultExportRecordOApiRequest {
				req := buildBaseReq()
				req.ExportID = gptr.Of(int64(0))
				return req
			},
			wantErr:     errno.CommonInvalidParamCode,
			wantNoInner: true,
		},
		{
			name:     "inner get error",
			buildReq: buildBaseReq,
			setup: func(fakeApp *fakeExperimentApp) {
				fakeApp.getExportRecordErr = errors.New("downstream")
			},
			wantErr: -1,
		},
		{
			name:     "success with full record",
			buildReq: buildBaseReq,
			setup: func(fakeApp *fakeExperimentApp) {
				url := "https://example/file.csv"
				fakeApp.getExportRecordResp = &exptpb.GetExptResultExportRecordResponse{
					ExptResultExportRecords: &domainexpt.ExptResultExportRecord{
						ExportID:        exportID,
						WorkspaceID:     workspaceID,
						ExptID:          experimentID,
						CsvExportStatus: domainexpt.CSVExportStatusSuccess,
						StartTime:       gptr.Of(int64(100)),
						EndTime:         gptr.Of(int64(200)),
						URL:             gptr.Of(url),
						Expired:         gptr.Of(false),
					},
				}
			},
			check: func(t *testing.T, resp *openapi.GetExperimentResultExportRecordOApiResponse, fakeApp *fakeExperimentApp) {
				if assert.NotNil(t, resp) && assert.NotNil(t, resp.Data) && assert.NotNil(t, resp.Data.ExptResultExportRecord) {
					rec := resp.Data.ExptResultExportRecord
					assert.Equal(t, exportID, rec.GetExportID())
					assert.Equal(t, workspaceID, rec.GetWorkspaceID())
					assert.Equal(t, experimentID, rec.GetExptID())
					if assert.NotNil(t, rec.CsvExportStatus) {
						assert.Equal(t, openapiExperiment.CSVExportStatusSuccess, *rec.CsvExportStatus)
					}
					assert.Equal(t, int64(100), rec.GetStartTime())
					assert.Equal(t, int64(200), rec.GetEndTime())
					assert.Equal(t, "https://example/file.csv", rec.GetURL())
					assert.False(t, rec.GetExpired())
				}
				if assert.NotNil(t, fakeApp.lastGetExportRecordReq) {
					assert.Equal(t, workspaceID, fakeApp.lastGetExportRecordReq.GetWorkspaceID())
					assert.Equal(t, experimentID, fakeApp.lastGetExportRecordReq.GetExptID())
					assert.Equal(t, exportID, fakeApp.lastGetExportRecordReq.GetExportID())
				}
			},
		},
		{
			name:     "success with nil downstream record",
			buildReq: buildBaseReq,
			setup: func(fakeApp *fakeExperimentApp) {
				fakeApp.getExportRecordResp = &exptpb.GetExptResultExportRecordResponse{}
			},
			check: func(t *testing.T, resp *openapi.GetExperimentResultExportRecordOApiResponse, _ *fakeExperimentApp) {
				if assert.NotNil(t, resp) && assert.NotNil(t, resp.Data) {
					assert.Nil(t, resp.Data.ExptResultExportRecord)
				}
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			metric := &fakeOpenAPIMetric{}
			fakeApp := &fakeExperimentApp{}

			app := &EvalOpenAPIApplication{
				experimentApp: fakeApp,
				metric:        metric,
			}

			req := tc.buildReq()
			if tc.setup != nil {
				tc.setup(fakeApp)
			}

			resp, err := app.GetExperimentResultExportRecordOApi(context.Background(), req)

			if tc.wantErr != 0 {
				assert.Error(t, err)
				if tc.wantErr > 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tc.wantErr, statusErr.Code())
				}
				assert.Nil(t, resp)
				if tc.wantNoInner {
					assert.Nil(t, fakeApp.lastGetExportRecordReq)
				}
			} else {
				assert.NoError(t, err)
			}

			if tc.check != nil {
				tc.check(t, resp, fakeApp)
			}

			assert.True(t, metric.called)
		})
	}
}
