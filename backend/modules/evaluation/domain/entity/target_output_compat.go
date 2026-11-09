// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"

	jsoncodec "github.com/coze-dev/coze-loop/backend/pkg/json"
)

func (output *EvalTargetOutputData) UnmarshalJSON(data []byte) error {
	type plain EvalTargetOutputData
	decoded := plain(*output)
	fields := struct {
		*plain
		Current json.RawMessage `json:"EvalTargetUsage"`
		Legacy  json.RawMessage `json:"EvaluatorUsage"`
	}{plain: &decoded}
	// Only usage fragments are retained; large output fields are decoded once.
	if err := jsoncodec.Unmarshal(data, &fields); err != nil {
		return err
	}
	usage := fields.Current
	if len(usage) == 0 {
		usage = fields.Legacy
	}
	if len(usage) > 0 {
		if err := jsoncodec.Unmarshal(usage, &decoded.EvalTargetUsage); err != nil {
			return err
		}
	}
	*output = EvalTargetOutputData(decoded)
	return nil
}
