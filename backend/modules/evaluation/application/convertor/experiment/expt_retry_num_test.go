// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment_test

import (
	"encoding/json"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"

	domainexpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	exptpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	mysqlconvert "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
)

// Dropping explicit zero, accepting a negative value, or clamping a positive
// value during creation must change these independently specified expectations.
func TestConvertCreateReq_ItemRetryNum(t *testing.T) {
	cases := []struct {
		name  string
		input *int32
		want  *int
	}{
		{name: "unset"},
		{name: "explicit_zero", input: gptr.Of(int32(0)), want: gptr.Of(0)},
		{name: "positive", input: gptr.Of(int32(3)), want: gptr.Of(3)},
		{name: "negative", input: gptr.Of(int32(-1))},
		{name: "int32_max", input: gptr.Of(int32(2147483647)), want: gptr.Of(2147483647)},
	}
	for _, multi := range []bool{false, true} {
		branch := "single_set"
		if multi {
			branch = "multi_set_config"
		}
		t.Run(branch, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					req := &exptpb.CreateExperimentRequest{
						WorkspaceID:  1001,
						ItemRetryNum: tc.input,
					}
					if multi {
						req.EvalSetSourceType = gptr.Of(domainexpt.ExptEvalSetSourceType_MultiSetConfig)
						req.EvalSetConfigs = []*domainexpt.EvalSetConfig{{EvalSetID: 111, EvalSetVersionID: 222}}
					}
					require.NoError(t, req.IsValid())
					param, err := experiment.ConvertCreateReq(req, nil)
					require.NoError(t, err)
					require.NotNil(t, param)
					require.NotNil(t, param.ExptConf)
					require.Equal(t, tc.want, param.ExptConf.ItemRetryNum)

					// CreateExpt freezes req.ExptConf as Experiment.EvalConf unchanged.
					// Exercise its actual persistence serializer, without a DB or platform call.
					frozen := &entity.Experiment{EvalConf: param.ExptConf}
					po, err := mysqlconvert.NewExptConverter().DO2PO(frozen)
					require.NoError(t, err)
					require.NotNil(t, po.EvalConf)
					t.Logf("FREEZE_JSON branch=%s case=%s json=%s", branch, tc.name, string(*po.EvalConf))
					var fields map[string]json.RawMessage
					require.NoError(t, json.Unmarshal(*po.EvalConf, &fields))
					raw, present := fields["ItemRetryNum"]
					require.True(t, present)
					if tc.want == nil {
						require.JSONEq(t, "null", string(raw))
					} else {
						var retryNum int
						require.NoError(t, json.Unmarshal(raw, &retryNum))
						require.Equal(t, *tc.want, retryNum)
					}
				})
			}
		})
	}
}
