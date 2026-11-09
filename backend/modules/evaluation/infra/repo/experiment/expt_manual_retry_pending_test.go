// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"syscall"
	"testing"

	redisc "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	redismocks "github.com/coze-dev/coze-loop/backend/infra/redis/mocks"
)

func TestPendingRetryMissingKeyAndGetErrors(t *testing.T) {
	networkErr := &net.OpError{Op: "read", Net: "tcp", Err: syscall.ECONNRESET}
	for _, tc := range []struct {
		name string
		raw  string
		err  error
	}{
		{name: "bare_nil", err: redisc.Nil},
		{name: "joined_nil", err: errors.Join(redisc.Nil, redisc.Nil)},
		{name: "network_error", err: networkErr},
		{name: "bad_json", raw: "{"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cache := redismocks.NewMockPersistentCmdable(gomock.NewController(t))
			cache.EXPECT().Get(gomock.Any(), pendingRetryKey(1, 2, 3)).Return(redisc.NewStringResult(tc.raw, tc.err))
			r := &ExptManualRetryRepoImpl{cache: cache}
			pending, err := r.GetPendingRetry(context.Background(), 1, 2, 3)
			require.Nil(t, pending)
			switch tc.name {
			case "network_error":
				require.ErrorIs(t, err, networkErr)
			case "bad_json":
				var syntax *json.SyntaxError
				require.ErrorAs(t, err, &syntax)
			default:
				require.NoError(t, err)
			}
		})
	}
}
