// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package conf

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	confmocks "github.com/coze-dev/coze-loop/backend/pkg/conf/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestExptExecConfigPerSpaceReload(t *testing.T) {
	loader := confmocks.NewMockIConfigLoader(gomock.NewController(t))
	raw := `{"expt_exec_conf":{"space_expt_concur_limit":3},"space_expt_exec_conf":{"9":{},"10":{"space_expt_concur_limit":5},"11":{"daemon_interval_second":7}}}`
	loader.EXPECT().UnmarshalKey(gomock.Any(), "expt_consumer_conf", gomock.Any()).DoAndReturn(func(_ context.Context, _ string, value any, _ ...conf.DecodeOptionFn) error {
		return json.Unmarshal([]byte(raw), value)
	}).AnyTimes()
	c := &configer{loader: loader}
	require.Zero(t, c.GetExptExecConf(context.Background(), 9).SpaceExptConcurLimit)
	require.Equal(t, 5, c.GetExptExecConf(context.Background(), 10).SpaceExptConcurLimit)
	require.Equal(t, 7, c.GetExptExecConf(context.Background(), 11).DaemonIntervalSecond)
	require.Equal(t, 3, c.GetExptExecConf(context.Background(), 12).SpaceExptConcurLimit)
	raw = `{}`
	require.Nil(t, c.GetExptExecConf(context.Background(), 11))
}
