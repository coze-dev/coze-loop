// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluatorResultsSerializeCompat(t *testing.T) {
	t.Run("new format Serialize → JSON contains 'registered' key", func(t *testing.T) {
		er := &EvaluatorResults{
			Registered: []*RegisteredEvalResult{
				{VersionID: 10, Alias: "alias1", RecordID: 100},
			},
			Inline: []*InlineEvalResult{
				{InlineKey: "ik1", RecordID: 200},
			},
		}
		data, err := er.Serialize()
		require.NoError(t, err)

		// Should contain the "registered" key (new format JSON tag)
		var raw map[string]interface{}
		err = json.Unmarshal(data, &raw)
		require.NoError(t, err)
		_, hasRegistered := raw["registered"]
		assert.True(t, hasRegistered, "serialized JSON should contain 'registered' key for new format")

		// Should NOT contain the old format key (omitempty ensures it's absent)
		_, hasOld := raw["EvalVerIDToResID"]
		assert.False(t, hasOld, "serialized JSON should not contain 'EvalVerIDToResID' key when using new format")
	})

	t.Run("old format JSON deserializes → EvalVerIDToResID non-empty, IsNewFormat()=false", func(t *testing.T) {
		// Simulate old-format JSON (only EvalVerIDToResID map)
		oldFormatJSON := `{"EvalVerIDToResID":{"100":999}}`
		var er EvaluatorResults
		err := json.Unmarshal([]byte(oldFormatJSON), &er)
		require.NoError(t, err)

		assert.False(t, er.IsNewFormat(), "old format JSON should produce IsNewFormat()=false")
		require.NotEmpty(t, er.EvalVerIDToResID, "EvalVerIDToResID should be non-empty for old format")
		assert.Equal(t, int64(999), er.EvalVerIDToResID[100])
		assert.Nil(t, er.Registered)
		assert.Nil(t, er.Inline)
	})

	t.Run("new format JSON deserializes → IsNewFormat()=true", func(t *testing.T) {
		// Simulate new-format JSON (registered array present)
		newFormatJSON := `{"registered":[{"version_id":10,"alias":"a1","record_id":100}],"inline":[{"inline_key":"ik1","record_id":200}]}`
		var er EvaluatorResults
		err := json.Unmarshal([]byte(newFormatJSON), &er)
		require.NoError(t, err)

		assert.True(t, er.IsNewFormat(), "new format JSON should produce IsNewFormat()=true")
		require.Len(t, er.Registered, 1)
		assert.Equal(t, int64(10), er.Registered[0].VersionID)
		assert.Equal(t, "a1", er.Registered[0].Alias)
		assert.Equal(t, int64(100), er.Registered[0].RecordID)
		require.Len(t, er.Inline, 1)
		assert.Equal(t, "ik1", er.Inline[0].InlineKey)
		assert.Equal(t, int64(200), er.Inline[0].RecordID)
	})
}
