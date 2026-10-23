// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment_test

import (
	"encoding/json"
	"testing"

	domainexpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	openapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	exptapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	store "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/stretchr/testify/require"
)

func TestAgentBudgetAPISaveReadback(t *testing.T) {
	for _, value := range []string{"", "null", "0", "20"} {
		t.Run(value, func(t *testing.T) {
			var input openapi.RunModeConfig
			raw := `{}`
			if value != "" {
				raw = `{"max_run_minutes":` + value + `}`
			}
			require.NoError(t, json.Unmarshal([]byte(raw), &input))
			dto, err := experiment.OpenAPIRunModeConfigDTO2Domain(&input)
			require.NoError(t, err)
			conf, err := experiment.NewEvalConfConvert().ConvertToEntity(&exptapi.CreateExperimentRequest{RunModeConfig: dto}, nil)
			require.NoError(t, err)
			po, err := (store.ExptConverter{}).DO2PO(&entity.Experiment{ID: 1, EvalConf: conf})
			require.NoError(t, err)
			saved, err := (store.ExptConverter{}).PO2DO(po, nil)
			require.NoError(t, err)
			output := experiment.RunModeConfigDomain2OpenAPI(experiment.ToExptDTO(saved).RunModeConfig)
			wire, err := json.Marshal(output)
			require.NoError(t, err)
			var got map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(wire, &got))
			if value == "" || value == "null" || value == "0" {
				require.True(t, len(got["max_run_minutes"]) == 0 || string(got["max_run_minutes"]) == "null")
			} else {
				require.JSONEq(t, value, string(got["max_run_minutes"]))
			}
		})
	}
}

func TestAgentBudgetAPIRejectsInvalidInput(t *testing.T) {
	for _, value := range []string{"-1", "1.5", `"bad"`} {
		t.Run(value, func(t *testing.T) {
			var input openapi.RunModeConfig
			err := json.Unmarshal([]byte(`{"max_run_minutes":`+value+`}`), &input)
			if err == nil {
				_, err = experiment.OpenAPIRunModeConfigDTO2Domain(&input)
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), "max_run_minutes")
		})
	}
}

func TestAgentBudgetAPICaseInsensitiveDecodedValue(t *testing.T) {
	for _, tc := range []struct {
		raw     string
		want    int32
		invalid bool
	}{
		{`{"MAX_RUN_MINUTES":20}`, 20, false},
		{`{"max_run_minutes":0,"MAX_RUN_MINUTES":20}`, 20, false},
		{`{"MAX_RUN_MINUTES":20,"max_run_minutes":0}`, 0, false},
		{`{"MAX_RUN_MINUTES":-1}`, 0, true},
		{`{"MAX_RUN_MINUTES":1.5}`, 0, true},
		{`{"MAX_RUN_MINUTES":"bad"}`, 0, true},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			var input openapi.RunModeConfig
			err := json.Unmarshal([]byte(tc.raw), &input)
			if err == nil {
				_, err = experiment.OpenAPIRunModeConfigDTO2Domain(&input)
			}
			if tc.invalid {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.want, *input.MaxRunMinutes)
			}
			var internal domainexpt.RunModeConfig
			err = json.Unmarshal([]byte(tc.raw), &internal)
			if err == nil {
				_, err = experiment.NewEvalConfConvert().ConvertToEntity(&exptapi.CreateExperimentRequest{RunModeConfig: &internal}, nil)
			}
			if tc.invalid {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.want, *internal.MaxRunMinutes)
			}
		})
	}
}

func TestAgentBudgetGeneratedValidationAndWire(t *testing.T) {
	for _, raw := range []string{`{}`, `{"max_run_minutes":0}`, `{"max_run_minutes":20}`, `{"max_run_minutes":-1}`} {
		t.Run(raw, func(t *testing.T) {
			var input domainexpt.RunModeConfig
			require.NoError(t, json.Unmarshal([]byte(raw), &input))
			var public openapi.RunModeConfig
			require.NoError(t, json.Unmarshal([]byte(raw), &public))
			negative := input.MaxRunMinutes != nil && *input.MaxRunMinutes < 0
			if negative {
				require.Error(t, input.IsValid())
				require.Error(t, public.IsValid())
				return
			}
			require.NoError(t, input.IsValid())
			require.NoError(t, public.IsValid())
			buf := make([]byte, input.BLength())
			require.Equal(t, len(buf), input.FastWriteNocopy(buf, nil))
			var decoded domainexpt.RunModeConfig
			n, err := decoded.FastRead(buf)
			require.NoError(t, err)
			require.Equal(t, len(buf), n)
			require.Equal(t, input.MaxRunMinutes, decoded.MaxRunMinutes)
			if input.MaxRunMinutes != nil {
				require.Equal(t, []byte{8, 0, 2}, buf[:3], "existing i32 field 2 must remain unchanged")
			}
		})
	}
}
