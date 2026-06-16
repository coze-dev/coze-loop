// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/http"
	httpmocks "github.com/coze-dev/coze-loop/backend/infra/http/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func float64Ptr(v float64) *float64 {
	return &v
}

// newPromptEvaluator builds an Evaluator backed by a prompt version so that
// GetEvaluatorVersionID / GetEvaluatorID return the provided ids.
func newPromptEvaluator(name string, evaluatorID, versionID int64) *entity.Evaluator {
	return &entity.Evaluator{
		Name:          name,
		EvaluatorType: entity.EvaluatorTypePrompt,
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
			ID:          versionID,
			EvaluatorID: evaluatorID,
		},
	}
}

// newRecordWithScore builds an EvaluatorRecord whose result carries the given raw score.
func newRecordWithScore(score *float64) *entity.EvaluatorRecord {
	return &entity.EvaluatorRecord{
		EvaluatorOutputData: &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{
				Score: score,
			},
		},
	}
}

// newRecordWithCorrection builds an EvaluatorRecord with both raw and correction scores.
func newRecordWithCorrection(raw, correction *float64) *entity.EvaluatorRecord {
	return &entity.EvaluatorRecord{
		EvaluatorOutputData: &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{
				Score: raw,
				Correction: &entity.Correction{
					Score: correction,
				},
			},
		},
	}
}

func TestNewEvaluatorScoreCalculator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configer := componentmocks.NewMockIConfiger(ctrl)
	client := httpmocks.NewMockIClient(ctrl)

	calc := NewEvaluatorScoreCalculator(configer, client)
	assert.NotNil(t, calc)
}

func TestEffectiveEvaluatorScore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		record *entity.EvaluatorRecord
		want   *float64
	}{
		{
			name:   "nil record",
			record: nil,
			want:   nil,
		},
		{
			name:   "nil output data",
			record: &entity.EvaluatorRecord{},
			want:   nil,
		},
		{
			name: "nil evaluator result",
			record: &entity.EvaluatorRecord{
				EvaluatorOutputData: &entity.EvaluatorOutputData{},
			},
			want: nil,
		},
		{
			name:   "raw score only",
			record: newRecordWithScore(float64Ptr(0.8)),
			want:   float64Ptr(0.8),
		},
		{
			name:   "correction score preferred over raw",
			record: newRecordWithCorrection(float64Ptr(0.8), float64Ptr(0.5)),
			want:   float64Ptr(0.5),
		},
		{
			name:   "correction present but score nil falls back to raw",
			record: newRecordWithCorrection(float64Ptr(0.8), nil),
			want:   float64Ptr(0.8),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := effectiveEvaluatorScore(tt.record)
			if tt.want == nil {
				assert.Nil(t, got)
				return
			}
			assert.NotNil(t, got)
			assert.InDelta(t, *tt.want, *got, 1e-9)
		})
	}
}

func TestBuildEvaluatorVersionRefs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		expt           *entity.Experiment
		version2Record map[string]*entity.EvaluatorRecord
		wantLen        int
		wantEvaluator  map[int64]int64 // versionID -> evaluatorID
	}{
		{
			name:           "empty records returns nil",
			expt:           &entity.Experiment{},
			version2Record: map[string]*entity.EvaluatorRecord{},
			wantLen:        0,
			wantEvaluator:  map[int64]int64{},
		},
		{
			name: "nil expt only fills version id",
			expt: nil,
			version2Record: map[string]*entity.EvaluatorRecord{
				"100": newRecordWithScore(float64Ptr(1)),
			},
			wantLen:       1,
			wantEvaluator: map[int64]int64{100: 0},
		},
		{
			name: "expt with evaluators maps version to evaluator id",
			expt: &entity.Experiment{
				Evaluators: []*entity.Evaluator{
					newPromptEvaluator("e1", 11, 100),
					nil,
				},
			},
			version2Record: map[string]*entity.EvaluatorRecord{
				"100": newRecordWithScore(float64Ptr(1)),
				"200": newRecordWithScore(float64Ptr(1)),
			},
			wantLen:       2,
			wantEvaluator: map[int64]int64{100: 11, 200: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			refs := buildEvaluatorVersionRefs(tt.expt, tt.version2Record)
			assert.Len(t, refs, tt.wantLen)
			got := make(map[int64]int64, len(refs))
			for _, ref := range refs {
				got[ref.EvaluatorVersionID] = ref.EvaluatorID
			}
			assert.Equal(t, tt.wantEvaluator, got)
		})
	}
}

func TestBuildCaseScoreRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		expt           *entity.Experiment
		version2Record map[string]*entity.EvaluatorRecord
		wantNil        bool
		wantExptID     int64
		wantItemLen    int
		wantNames      map[int64]string // versionID -> evaluator name
	}{
		{
			name:           "empty records returns nil",
			expt:           &entity.Experiment{ID: 5},
			version2Record: map[string]*entity.EvaluatorRecord{},
			wantNil:        true,
		},
		{
			name: "nil expt still builds items without names",
			expt: nil,
			version2Record: map[string]*entity.EvaluatorRecord{
				"100": newRecordWithScore(float64Ptr(0.6)),
			},
			wantExptID:  0,
			wantItemLen: 1,
			wantNames:   map[int64]string{100: ""},
		},
		{
			name: "skips records without effective score",
			expt: &entity.Experiment{
				ID: 7,
				Evaluators: []*entity.Evaluator{
					newPromptEvaluator("scored", 11, 100),
					newPromptEvaluator("unscored", 12, 200),
				},
			},
			version2Record: map[string]*entity.EvaluatorRecord{
				"100": newRecordWithScore(float64Ptr(0.6)),
				"200": newRecordWithScore(nil),
			},
			wantExptID:  7,
			wantItemLen: 1,
			wantNames:   map[int64]string{100: "scored"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := buildCaseScoreRequest(tt.expt, tt.version2Record)
			if tt.wantNil {
				assert.Nil(t, req)
				return
			}
			assert.NotNil(t, req)
			assert.Equal(t, tt.wantExptID, req.ExptID)
			assert.Len(t, req.EvaluatorScore, tt.wantItemLen)
			names := make(map[int64]string, len(req.EvaluatorScore))
			for _, item := range req.EvaluatorScore {
				names[item.EvaluatorVersionID] = item.EvaluatorName
			}
			assert.Equal(t, tt.wantNames, names)
		})
	}
}

func TestCalculateWeightedScore_Local(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		evaluatorRecords map[string]*entity.EvaluatorRecord
		weights          map[string]float64
		wantNil          bool
		want             float64
	}{
		{
			name:             "empty records returns nil",
			evaluatorRecords: map[string]*entity.EvaluatorRecord{},
			weights:          nil,
			wantNil:          true,
		},
		{
			name: "no weights computes simple average",
			evaluatorRecords: map[string]*entity.EvaluatorRecord{
				"100": newRecordWithScore(float64Ptr(0.6)),
				"200": newRecordWithScore(float64Ptr(0.8)),
			},
			weights: map[string]float64{},
			want:    0.7,
		},
		{
			name: "no weights prefers correction score",
			evaluatorRecords: map[string]*entity.EvaluatorRecord{
				"100": newRecordWithCorrection(float64Ptr(0.6), float64Ptr(1.0)),
			},
			weights: map[string]float64{},
			want:    1.0,
		},
		{
			name: "no weights skips nil records and nil scores",
			evaluatorRecords: map[string]*entity.EvaluatorRecord{
				"100": nil,
				"200": newRecordWithScore(nil),
				"300": newRecordWithScore(float64Ptr(0.4)),
			},
			weights: map[string]float64{},
			want:    0.4,
		},
		{
			name: "no weights all invalid returns nil",
			evaluatorRecords: map[string]*entity.EvaluatorRecord{
				"100": nil,
				"200": newRecordWithScore(nil),
			},
			weights: map[string]float64{},
			wantNil: true,
		},
		{
			name: "weighted average",
			evaluatorRecords: map[string]*entity.EvaluatorRecord{
				"100": newRecordWithScore(float64Ptr(1.0)),
				"200": newRecordWithScore(float64Ptr(0.0)),
			},
			weights: map[string]float64{
				"100": 3,
				"200": 1,
			},
			want: 0.75,
		},
		{
			name: "weighted skips records with non positive or missing weight",
			evaluatorRecords: map[string]*entity.EvaluatorRecord{
				"100": newRecordWithScore(float64Ptr(1.0)),
				"200": newRecordWithScore(float64Ptr(0.2)), // weight 0, skipped
				"300": newRecordWithScore(float64Ptr(0.5)), // no weight, skipped
			},
			weights: map[string]float64{
				"100": 2,
				"200": 0,
			},
			want: 1.0,
		},
		{
			name: "weighted skips nil records and nil scores",
			evaluatorRecords: map[string]*entity.EvaluatorRecord{
				"100": nil,
				"200": newRecordWithScore(nil),
				"300": newRecordWithScore(float64Ptr(0.9)),
			},
			weights: map[string]float64{
				"300": 2,
			},
			want: 0.9,
		},
		{
			name: "weighted total weight zero returns nil",
			evaluatorRecords: map[string]*entity.EvaluatorRecord{
				"100": newRecordWithScore(float64Ptr(1.0)),
			},
			weights: map[string]float64{
				"100": 0,
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := calculateWeightedScore(tt.evaluatorRecords, tt.weights)
			if tt.wantNil {
				assert.Nil(t, got)
				return
			}
			assert.NotNil(t, got)
			assert.InDelta(t, tt.want, *got, 1e-9)
		})
	}
}

func TestEvaluatorScoreCalculator_CalculateWeightedScore(t *testing.T) {
	expt := &entity.Experiment{
		ID:      7,
		SpaceID: 3,
		Evaluators: []*entity.Evaluator{
			newPromptEvaluator("e1", 11, 100),
		},
	}
	records := map[string]*entity.EvaluatorRecord{
		"100": newRecordWithScore(float64Ptr(0.6)),
	}

	tests := []struct {
		name           string
		expt           *entity.Experiment
		version2Record map[string]*entity.EvaluatorRecord
		scoreWeights   map[string]float64
		setup          func(configer *componentmocks.MockIConfiger, client *httpmocks.MockIClient)
		wantNil        bool
		want           float64
	}{
		{
			name:           "hook miss falls back to local calculation",
			expt:           expt,
			version2Record: records,
			scoreWeights:   map[string]float64{},
			setup: func(configer *componentmocks.MockIConfiger, client *httpmocks.MockIClient) {
				configer.EXPECT().
					GetExptTurnScoreHookConf(gomock.Any(), int64(3), int64(7), gomock.Any()).
					Return(nil, false)
			},
			want: 0.6,
		},
		{
			name:           "nil expt with hook miss falls back to local",
			expt:           nil,
			version2Record: records,
			scoreWeights:   map[string]float64{},
			setup: func(configer *componentmocks.MockIConfiger, client *httpmocks.MockIClient) {
				configer.EXPECT().
					GetExptTurnScoreHookConf(gomock.Any(), int64(0), int64(0), gomock.Any()).
					Return(nil, false)
			},
			want: 0.6,
		},
		{
			name: "hook hit but request has no scores returns nil",
			expt: expt,
			version2Record: map[string]*entity.EvaluatorRecord{
				"100": newRecordWithScore(nil),
			},
			scoreWeights: map[string]float64{},
			setup: func(configer *componentmocks.MockIConfiger, client *httpmocks.MockIClient) {
				configer.EXPECT().
					GetExptTurnScoreHookConf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&entity.ExptTurnScoreHookConf{URL: "http://hook"}, true)
			},
			wantNil: true,
		},
		{
			name:           "hook hit and http error returns nil",
			expt:           expt,
			version2Record: records,
			scoreWeights:   map[string]float64{},
			setup: func(configer *componentmocks.MockIConfiger, client *httpmocks.MockIClient) {
				configer.EXPECT().
					GetExptTurnScoreHookConf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&entity.ExptTurnScoreHookConf{URL: "http://hook"}, true)
				client.EXPECT().
					DoHTTPRequest(gomock.Any(), gomock.Any()).
					Return(errors.New("network error"))
			},
			wantNil: true,
		},
		{
			name:           "hook hit and response carries error returns nil",
			expt:           expt,
			version2Record: records,
			scoreWeights:   map[string]float64{},
			setup: func(configer *componentmocks.MockIConfiger, client *httpmocks.MockIClient) {
				configer.EXPECT().
					GetExptTurnScoreHookConf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&entity.ExptTurnScoreHookConf{URL: "http://hook"}, true)
				client.EXPECT().
					DoHTTPRequest(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, param *http.RequestParam) error {
						resp := param.Response.(*entity.CaseScoreResponse)
						resp.Error = "logic error"
						return nil
					})
			},
			wantNil: true,
		},
		{
			name:           "hook hit success returns rounded score",
			expt:           expt,
			version2Record: records,
			scoreWeights:   map[string]float64{},
			setup: func(configer *componentmocks.MockIConfiger, client *httpmocks.MockIClient) {
				configer.EXPECT().
					GetExptTurnScoreHookConf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&entity.ExptTurnScoreHookConf{URL: "http://hook", Method: "POST", TimeoutMS: 1000}, true)
				client.EXPECT().
					DoHTTPRequest(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, param *http.RequestParam) error {
						resp := param.Response.(*entity.CaseScoreResponse)
						resp.Score = 0.876
						return nil
					})
			},
			want: 0.88,
		},
		{
			name:           "hook hit with empty method defaults to post",
			expt:           expt,
			version2Record: records,
			scoreWeights:   map[string]float64{},
			setup: func(configer *componentmocks.MockIConfiger, client *httpmocks.MockIClient) {
				configer.EXPECT().
					GetExptTurnScoreHookConf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&entity.ExptTurnScoreHookConf{URL: "http://hook"}, true)
				client.EXPECT().
					DoHTTPRequest(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, param *http.RequestParam) error {
						assert.Equal(t, "POST", param.Method)
						resp := param.Response.(*entity.CaseScoreResponse)
						resp.Score = 0.5
						return nil
					})
			},
			want: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			configer := componentmocks.NewMockIConfiger(ctrl)
			client := httpmocks.NewMockIClient(ctrl)
			if tt.setup != nil {
				tt.setup(configer, client)
			}

			calc := NewEvaluatorScoreCalculator(configer, client)
			got := calc.CalculateWeightedScore(context.Background(), tt.expt, tt.version2Record, tt.scoreWeights)

			if tt.wantNil {
				assert.Nil(t, got)
				return
			}
			assert.NotNil(t, got)
			assert.InDelta(t, tt.want, *got, 1e-9)
		})
	}
}

func TestEvaluatorScoreCalculator_CalculateWeightedScore_NilConfiger(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := httpmocks.NewMockIClient(ctrl)
	calc := &evaluatorScoreCalculator{configer: nil, httpClient: client}

	records := map[string]*entity.EvaluatorRecord{
		"100": newRecordWithScore(float64Ptr(0.6)),
	}
	got := calc.CalculateWeightedScore(context.Background(), &entity.Experiment{}, records, map[string]float64{})
	assert.NotNil(t, got)
	assert.InDelta(t, 0.6, *got, 1e-9)
}

func TestEvaluatorScoreCalculator_callCaseScoreHook(t *testing.T) {
	t.Parallel()

	validReq := &entity.CaseScoreRequest{
		ExptID:         1,
		EvaluatorScore: []*entity.CaseScoreItem{{EvaluatorVersionID: 100, Score: 0.6}},
	}

	tests := []struct {
		name       string
		httpClient bool
		conf       *entity.ExptTurnScoreHookConf
		req        *entity.CaseScoreRequest
		setup      func(client *httpmocks.MockIClient)
		wantErr    bool
		wantNil    bool
		want       float64
	}{
		{
			name:       "nil http client returns error",
			httpClient: false,
			conf:       &entity.ExptTurnScoreHookConf{URL: "http://hook"},
			req:        validReq,
			wantErr:    true,
		},
		{
			name:       "nil conf returns error",
			httpClient: true,
			conf:       nil,
			req:        validReq,
			wantErr:    true,
		},
		{
			name:       "empty url returns error",
			httpClient: true,
			conf:       &entity.ExptTurnScoreHookConf{URL: ""},
			req:        validReq,
			wantErr:    true,
		},
		{
			name:       "http request error propagated",
			httpClient: true,
			conf:       &entity.ExptTurnScoreHookConf{URL: "http://hook"},
			req:        validReq,
			setup: func(client *httpmocks.MockIClient) {
				client.EXPECT().DoHTTPRequest(gomock.Any(), gomock.Any()).Return(errors.New("boom"))
			},
			wantErr: true,
		},
		{
			name:       "response error returns error",
			httpClient: true,
			conf:       &entity.ExptTurnScoreHookConf{URL: "http://hook"},
			req:        validReq,
			setup: func(client *httpmocks.MockIClient) {
				client.EXPECT().DoHTTPRequest(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, param *http.RequestParam) error {
						param.Response.(*entity.CaseScoreResponse).Error = "logic failure"
						return nil
					})
			},
			wantErr: true,
		},
		{
			name:       "custom method preserved and score returned",
			httpClient: true,
			conf:       &entity.ExptTurnScoreHookConf{URL: "http://hook", Method: "PUT"},
			req:        validReq,
			setup: func(client *httpmocks.MockIClient) {
				client.EXPECT().DoHTTPRequest(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, param *http.RequestParam) error {
						assert.Equal(t, "PUT", param.Method)
						param.Response.(*entity.CaseScoreResponse).Score = 0.42
						return nil
					})
			},
			want: 0.42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			calc := &evaluatorScoreCalculator{}
			if tt.httpClient {
				client := httpmocks.NewMockIClient(ctrl)
				if tt.setup != nil {
					tt.setup(client)
				}
				calc.httpClient = client
			}

			got, err := calc.callCaseScoreHook(context.Background(), tt.conf, tt.req)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
				return
			}
			assert.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, got)
				return
			}
			assert.NotNil(t, got)
			assert.InDelta(t, tt.want, *got, 1e-9)
		})
	}
}
