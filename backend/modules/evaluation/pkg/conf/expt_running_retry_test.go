// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package conf

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	confmocks "github.com/coze-dev/coze-loop/backend/pkg/conf/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRunningRetryConfigPerSpaceReload(t *testing.T) {
	loader := confmocks.NewMockIConfigLoader(gomock.NewController(t))
	raw := `{"expt_exec_conf":{"running_retry_enabled":true},"space_expt_exec_conf":{"9":{},"10":{"running_retry_enabled":false},"11":{"running_retry_enabled":true}}}`
	loader.EXPECT().UnmarshalKey(gomock.Any(), "expt_consumer_conf", gomock.Any()).DoAndReturn(func(_ context.Context, _ string, value any, _ ...conf.DecodeOptionFn) error {
		return json.Unmarshal([]byte(raw), value)
	}).AnyTimes()
	c := &configer{loader: loader}
	require.False(t, c.GetExptExecConf(context.Background(), 9).IsRunningRetryEnabled())
	require.False(t, c.GetExptExecConf(context.Background(), 10).IsRunningRetryEnabled())
	require.True(t, c.GetExptExecConf(context.Background(), 11).IsRunningRetryEnabled())
	raw = `{}`
	require.False(t, c.GetExptExecConf(context.Background(), 11).IsRunningRetryEnabled())
	var unset *entity.ExptExecConf
	require.False(t, unset.IsRunningRetryEnabled())
}
