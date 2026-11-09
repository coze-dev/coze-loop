// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildExptFilterProjectionDraftVersionAliases(t *testing.T) {
	for _, tc := range []struct {
		name          string
		configIndex   int
		configVersion int64
		refSet        int64
		refVersion    int64
		refSpace      int64
		wantError     bool
	}{
		{name: "main draft set ID with zero reference", configIndex: 0, configVersion: 20, refSet: 20, refSpace: 8},
		{name: "secondary draft set ID with zero reference", configIndex: 1, configVersion: 30, refSet: 30, refSpace: 8},
		{name: "zero draft config with set ID reference", configIndex: 1, refSet: 30, refVersion: 30, refSpace: 8},
		{name: "different published versions", configIndex: 1, configVersion: 31, refSet: 30, refVersion: 999, refSpace: 8, wantError: true},
		{name: "draft config with published reference", configIndex: 1, configVersion: 30, refSet: 30, refVersion: 31, refSpace: 8, wantError: true},
		{name: "draft reference belongs to another set", configIndex: 1, configVersion: 30, refSet: 99, refSpace: 8, wantError: true},
		{name: "draft reference belongs to another space", configIndex: 1, configVersion: 30, refSet: 30, refSpace: 99, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newProjectionFixture(t)
			f.expt.EvalConf.EvalSetConfigs[tc.configIndex].EvalSetVersionID = tc.configVersion
			if tc.configIndex == 0 {
				f.expt.EvalSetVersionID = tc.configVersion
			}
			f.refs[0].EvalSetID = tc.refSet
			f.refs[0].EvalSetVersionID = tc.refVersion
			f.refs[0].EvalSetSourceSpaceID = tc.refSpace
			f.refs[0].ItemVersionID = 0
			svc := f.service()
			rows, err := svc.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
			if tc.wantError {
				require.Error(t, err)
				require.Empty(t, rows)
				return
			}
			require.NoError(t, err)
			require.Len(t, rows, 1)
			require.Equal(t, tc.refSet, rows[0].EvalSetID)
			require.Equal(t, tc.refVersion, rows[0].EvalSetVersionID)
			require.Equal(t, "preview", rows[0].EvalTargetData["custom"])
		})
	}
}
