// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package sandbox_agent

import (
	"errors"
	"testing"
)

func TestClassifyErrorType(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code int32
		want string
	}{
		{name: "success", err: nil, code: 0, want: "-"},
		{name: "unknown_err_no_code", err: errors.New("boom"), code: 0, want: "unknown"},
		{name: "engineering_network_timeout", err: nil, code: 601200701, want: "engineering"},
		{name: "engineering_internal", err: nil, code: 601200702, want: "engineering"},
		{name: "engineering_rpc", err: nil, code: 601200703, want: "engineering"},
		{name: "engineering_mysql", err: nil, code: 601200801, want: "engineering"},
		{name: "engineering_redis", err: nil, code: 601200803, want: "engineering"},
		{name: "engineering_invalid_output_from_model", err: nil, code: 601205015, want: "engineering"},
		{name: "engineering_file_url_retrieve_failed", err: nil, code: 601205036, want: "engineering"},
		{name: "engineering_goroutine_pool", err: nil, code: 601205037, want: "engineering"},
		{name: "engineering_batch_task", err: nil, code: 601205038, want: "engineering"},
		{name: "non_engineering_random_biz_code", err: nil, code: 601299999, want: "non_engineering"},
		{name: "non_engineering_with_err", err: errors.New("model rate limit"), code: 601298000, want: "non_engineering"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyErrorType(tc.err, tc.code)
			if got != tc.want {
				t.Fatalf("ClassifyErrorType(%v, %d) = %q, want %q", tc.err, tc.code, got, tc.want)
			}
		})
	}
}
