// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package component

import (
	"testing"

	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/stretchr/testify/require"
)

// This is the original consumer payload, with decimal IDs above JSON's exact integer range.
const originalItemCompletePayload = `{"eval_target_workspace_id":"7384603622000000001","eval_target_id":"7384603622000000002","source_target_id":"application","expt_workspace_id":"7384603622000000003","expt_id":"7384603622000000004","expt_run_id":"7384603622000000005","dataset_workspace_id":"7384603622000000006","dataset_id":"7384603622000000007","dataset_version_id":"7384603622000000008","dataset_key":"dataset","dataset_version_name":"0.0.1","experiment_group_key":"group","item_id":"7384603622000000009","item_key":"case","enable_analysis":true,"created_by":"user"}`

func TestItemCompleteOriginalPayloadRoundTrip(t *testing.T) {
	var event ItemCompleteEvent
	require.NoError(t, json.Unmarshal([]byte(originalItemCompletePayload), &event))
	body, err := json.Marshal(&event)
	require.NoError(t, err)
	require.Equal(t, originalItemCompletePayload, string(body))
	// Repeated delivery preserves the existing consumer identity and exact payload.
	var duplicate ItemCompleteEvent
	require.NoError(t, json.Unmarshal(body, &duplicate))
	repeated, err := json.Marshal(&duplicate)
	require.NoError(t, err)
	require.Equal(t, body, repeated)
}

func TestItemCompleteExtraProtocolDoesNotPollutePayload(t *testing.T) {
	for _, request := range []string{`""`, `"manual-retry"`} {
		extra := originalItemCompletePayload[:len(originalItemCompletePayload)-1] + `,"request_id":` + request + `,"source_turn_records":[{"turn_id":"0","target_record_id":"7384603622000000010","registered":[{"version_id":"11","alias":"a","record_id":"12"}],"inline":[{"inline_key":"key","record_id":"13"}]}]}`
		var event ItemCompleteEvent
		require.NoError(t, json.Unmarshal([]byte(extra), &event))
		body, err := json.Marshal(&event)
		require.NoError(t, err)
		require.Equal(t, originalItemCompletePayload, string(body), "extra source protocol must not enter the original notification")
	}
}
