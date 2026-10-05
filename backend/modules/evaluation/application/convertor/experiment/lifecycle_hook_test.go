// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"

	domain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	openapidomain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
)

func TestLifecycleHookOpenAPIReadView(t *testing.T) {
	in := &domain.Experiment{ID: gptr.Of(int64(42)), Name: gptr.Of("original"), LifecycleHookConf: &domain.LifecycleHookConf{Before: &domain.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(`{"large":9007199254740993,"nested":{"x":null}}`), Retry: &domain.HookRetryConf{Enabled: gptr.Of(false), MaxRetries: gptr.Of(int32(7))}}}, LifecycleHookSummary: &domain.LifecycleHookRunSummary{RunID: gptr.Of("123"), Before: &domain.HookRunSummary{Status: gptr.Of(domain.HookOperationStatus("running")), Response: &spi.InvokeExperimentHookResponse{Status: gptr.Of(spi.HookResultStatus("SUCCESS")), Result_: map[string]string{"stale": "yes"}}, Error: &spi.HookError{Code: gptr.Of("STALE")}}, After: &domain.HookRunSummary{Status: gptr.Of(domain.HookOperationStatus("succeeded")), Response: &spi.InvokeExperimentHookResponse{Status: gptr.Of(spi.HookResultStatus("SUCCESS")), Result_: map[string]string{"result": "可展示"}}}}}
	out := DomainExperimentDTO2OpenAPI(in)
	require.Equal(t, "original", out.GetName())
	require.NotNil(t, out.LifecycleHookConf, "OpenAPI must not discard management hook config")
	require.False(t, out.LifecycleHookConf.Before.GetEnabled())
	require.Equal(t, `{"large":9007199254740993,"nested":{"x":null}}`, out.LifecycleHookConf.Before.GetParametersJSON())
	require.Equal(t, int32(7), out.LifecycleHookConf.Before.Retry.GetMaxRetries())
	require.NotNil(t, out.LifecycleHookSummary)
	require.Equal(t, "123", out.LifecycleHookSummary.GetRunID())
	require.Nil(t, out.LifecycleHookSummary.Before.Response)
	require.Nil(t, out.LifecycleHookSummary.Before.Error)
	require.Equal(t, "可展示", out.LifecycleHookSummary.After.Response.Result_["result"])
	*out.LifecycleHookConf.Before.Enabled = true
	out.LifecycleHookSummary.After.Response.Result_["result"] = "changed"
	require.False(t, in.LifecycleHookConf.Before.GetEnabled())
	require.Equal(t, "可展示", in.LifecycleHookSummary.After.Response.Result_["result"])
}

func TestLifecycleHookUpdateConvertersAcceptValidatedConfig(t *testing.T) {
	c := &domain.LifecycleHookConf{Before: &domain.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(`{"keep":1}`)}}
	o := &openapidomain.LifecycleHookConf{Before: &openapidomain.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(`{"keep":1}`)}}
	tests := map[string]func() error{
		"template_update": func() error {
			_, err := ConvertUpdateExptTemplateReq(&expt.UpdateExperimentTemplateRequest{WorkspaceID: 7, TemplateID: 42, LifecycleHookConf: c})
			return err
		},
		"openapi_template_update": func() error {
			_, err := OpenAPIUpdateExptTemplateReq2Domain(&openapi.UpdateExptTemplateOApiRequest{WorkspaceID: gptr.Of(int64(7)), TemplateID: gptr.Of(int64(42)), LifecycleHookConf: o})
			return err
		},
	}
	for name, run := range tests {
		t.Run(name, func(t *testing.T) { require.NoError(t, run()) })
	}
}

func TestLifecycleHookCreationConvertersAcceptValidatedConfig(t *testing.T) {
	for _, parameters := range []string{`{"keep":9007199254740993}`, `[]`} {
		t.Run(parameters, func(t *testing.T) {
			c := &domain.LifecycleHookConf{Before: &domain.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(parameters)}}
			o := &openapidomain.LifecycleHookConf{Before: &openapidomain.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(parameters)}}
			_, exptErr := ConvertCreateReq(&expt.CreateExperimentRequest{WorkspaceID: 7, Name: gptr.Of("original"), LifecycleHookConf: c}, nil)
			_, templateErr := ConvertCreateExptTemplateReq(&expt.CreateExperimentTemplateRequest{WorkspaceID: 7, LifecycleHookConf: c})
			_, openapiErr := OpenAPICreateExptTemplateReq2Domain(&openapi.CreateExptTemplateOApiRequest{WorkspaceID: gptr.Of(int64(7)), LifecycleHookConf: o})
			for _, err := range []error{exptErr, templateErr, openapiErr} {
				if parameters == `[]` {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			}
		})
	}
}
