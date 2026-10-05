// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestHookSummaryRejectsResultEnvelopeEscape(t *testing.T) {
	for _, raw := range []string{`{},"unexpected":"private"`, " \n "} {
		t.Run(raw, func(t *testing.T) {
			row := hookSummaryRow{OperationID: "op", Status: "succeeded", Attempt: 1, UpdatedAt: gptr.Of(time.Now()), ResultRedacted: []byte(raw)}
			out, err := hookOperationSummary(row)
			require.ErrorIs(t, err, entity.ErrHookSummaryUnavailable)
			require.Nil(t, out.Response)
		})
	}
}

func TestHookSummaryProtocolBounds(t *testing.T) {
	base := hookSummaryRow{OperationID: "op", Status: "succeeded", Attempt: 1, UpdatedAt: gptr.Of(time.Now())}
	for _, raw := range [][]byte{nil, []byte(`null`), []byte(`{}`), []byte(`{"a":"` + strings.Repeat("x", 8192) + `"}`)} {
		row := base
		row.ResultRedacted = raw
		out, err := hookOperationSummary(row)
		require.NoError(t, err)
		require.NotNil(t, out.Response)
	}
	for _, mutate := range []func(*hookSummaryRow){
		func(r *hookSummaryRow) { r.UpdatedAt = nil },
		func(r *hookSummaryRow) { r.UpdatedAt = gptr.Of(time.Time{}) },
		func(r *hookSummaryRow) { r.OperationID = "private\nvalue" },
		func(r *hookSummaryRow) { r.OperationID = strings.Repeat("x", 129) },
		func(r *hookSummaryRow) { r.Status = "running"; r.Attempt = 0 },
		func(r *hookSummaryRow) { r.Status = "retry_wait"; r.Attempt = 0 },
		func(r *hookSummaryRow) { r.Status = "pending"; r.Attempt = 1 },
		func(r *hookSummaryRow) { r.Attempt = 0 },
		func(r *hookSummaryRow) { r.ResultRedacted = []byte(`{"` + strings.Repeat("x", 129) + `":"value"}`) },
		func(r *hookSummaryRow) { r.Status = "failed"; r.ErrorCode = gptr.Of(strings.Repeat("x", 129)) },
		func(r *hookSummaryRow) { r.Status = "failed"; r.ErrorMessage = gptr.Of("\xff") },
		func(r *hookSummaryRow) { r.Status = "failed"; r.ErrorCode = gptr.Of("\xff") },
	} {
		row := base
		mutate(&row)
		out, err := hookOperationSummary(row)
		require.ErrorIs(t, err, entity.ErrHookSummaryUnavailable)
		require.Nil(t, out.Response)
		require.Nil(t, out.Error)
	}
	fields := make([]string, 129)
	for i := range fields {
		fields[i] = `"` + strconv.Itoa(i) + `":"v"`
	}
	base.ResultRedacted = []byte("{" + strings.Join(fields, ",") + "}")
	_, err := hookOperationSummary(base)
	require.ErrorIs(t, err, entity.ErrHookSummaryUnavailable)
}
