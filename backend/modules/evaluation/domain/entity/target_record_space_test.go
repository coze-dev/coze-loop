// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTargetRecordExecutionSpace(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		source     int64
		experiment int64
		owner      int64
	}{
		{name: "shared target", source: 10, experiment: 99, owner: 99},
		{name: "same workspace", source: 10, experiment: 10, owner: 10},
		{name: "debug", source: 10, owner: 10},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			input := map[string]string{"custom": "value", targetRecordSourceSpaceKey: "999", targetRecordExecutionSpaceKey: "999"}
			record := &EvalTargetRecord{Ext: input}
			record.BindExecutionSpace(testCase.source, testCase.experiment)
			require.Equal(t, testCase.owner, record.SpaceID)
			require.Equal(t, testCase.source, record.TargetSourceSpaceID())
			require.Equal(t, testCase.experiment > 0, record.ExperimentOwned())
			require.Equal(t, "value", record.Ext["custom"])
			require.Equal(t, "999", input[targetRecordSourceSpaceKey])
			record.Ext["custom"] = "changed"
			require.Equal(t, "value", input["custom"])
		})
	}
}

func TestTargetRecordSourceSpaceRejectsInvalidMetadata(t *testing.T) {
	for _, source := range []string{"", "bad", "0", "-1"} {
		record := &EvalTargetRecord{SpaceID: 99, Ext: map[string]string{targetRecordSourceSpaceKey: source}}
		require.Zero(t, record.TargetSourceSpaceID())
	}
	require.Zero(t, (*EvalTargetRecord)(nil).TargetSourceSpaceID())
	require.False(t, (*EvalTargetRecord)(nil).ExperimentOwned())
}
