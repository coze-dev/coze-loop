// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package conf

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	confmocks "github.com/coze-dev/coze-loop/backend/pkg/conf/mocks"
)

// 旧的 running_retry_enabled 开关与空间白名单已下线：存量 TCC JSON 里残留该 key 时
// 必须被安全忽略，不影响同一条配置里其他执行参数的解析。
func TestLegacyRunningRetryKeyIsIgnored(t *testing.T) {
	loader := confmocks.NewMockIConfigLoader(gomock.NewController(t))
	raw := `{"expt_exec_conf":{"running_retry_enabled":true,"daemon_interval_second":7,"expt_zombie_second":11,"space_expt_concur_limit":5},` +
		`"space_expt_exec_conf":{"9":{"running_retry_enabled":false,"daemon_interval_second":3},"10":{},"11":{"running_retry_enabled":true}}}`
	loader.EXPECT().UnmarshalKey(gomock.Any(), "expt_consumer_conf", gomock.Any()).DoAndReturn(func(_ context.Context, _ string, value any, _ ...conf.DecodeOptionFn) error {
		return json.Unmarshal([]byte(raw), value)
	}).AnyTimes()
	c := &configer{loader: loader}

	// 无空间覆盖：回落到全局配置，其余字段正常解析。
	global := c.GetExptExecConf(context.Background(), 12)
	require.Equal(t, 7, global.DaemonIntervalSecond)
	require.Equal(t, 11, global.ZombieIntervalSecond)
	require.Equal(t, 5, global.SpaceExptConcurLimit)

	// 空间覆盖生效，旧开关 key 不参与解析也不影响其他字段。
	space9 := c.GetExptExecConf(context.Background(), 9)
	require.Equal(t, 3, space9.DaemonIntervalSecond)
	require.Zero(t, space9.ZombieIntervalSecond)

	// 仅含旧 key 的空间配置解析为默认值，不得报错、不得影响准入。
	space11 := c.GetExptExecConf(context.Background(), 11)
	require.NotNil(t, space11)
	require.Zero(t, space11.DaemonIntervalSecond)
	require.Zero(t, space11.ZombieIntervalSecond)

	// 旧 key 的 true / false / 缺失三种形态解析结果一致。
	legacyTrue, legacyFalse, absent := &entity.ExptExecConf{}, &entity.ExptExecConf{}, &entity.ExptExecConf{}
	require.NoError(t, json.Unmarshal([]byte(`{"running_retry_enabled":true}`), legacyTrue))
	require.NoError(t, json.Unmarshal([]byte(`{"running_retry_enabled":false}`), legacyFalse))
	require.NoError(t, json.Unmarshal([]byte(`{}`), absent))
	require.Equal(t, absent, legacyTrue)
	require.Equal(t, absent, legacyFalse)
	encoded, err := json.Marshal(legacyTrue)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "running_retry_enabled")
}

// retry_complete_enabled 是运行中 RetryComplete 的发布门：JSON 结构里
// 缺省 / false / true 与空间级覆盖必须按原语义解析，只有显式 true 才开启。
func TestRetryCompleteEnabledParsing(t *testing.T) {
	loader := confmocks.NewMockIConfigLoader(gomock.NewController(t))
	raw := `{"expt_exec_conf":{"retry_complete_enabled":true,"daemon_interval_second":7},` +
		`"space_expt_exec_conf":{"9":{"retry_complete_enabled":false},"10":{"retry_complete_enabled":true},"11":{}}}`
	loader.EXPECT().UnmarshalKey(gomock.Any(), "expt_consumer_conf", gomock.Any()).DoAndReturn(func(_ context.Context, _ string, value any, _ ...conf.DecodeOptionFn) error {
		return json.Unmarshal([]byte(raw), value)
	}).AnyTimes()
	c := &configer{loader: loader}

	// 未命中空间覆盖时回落全局配置。
	require.True(t, c.GetExptExecConf(context.Background(), 12).GetRetryCompleteEnabled())

	// 空间级显式 false / true 覆盖全局。
	require.False(t, c.GetExptExecConf(context.Background(), 9).GetRetryCompleteEnabled())
	require.True(t, c.GetExptExecConf(context.Background(), 10).GetRetryCompleteEnabled())

	// 空间配置存在但缺该字段 → 关闭。
	spaceAbsent := c.GetExptExecConf(context.Background(), 11)
	require.NotNil(t, spaceAbsent)
	require.False(t, spaceAbsent.GetRetryCompleteEnabled())

	// 直接按 JSON 结构解析时缺省与显式 false 均为关闭。
	absent, explicitFalse := &entity.ExptExecConf{}, &entity.ExptExecConf{}
	require.NoError(t, json.Unmarshal([]byte(`{}`), absent))
	require.NoError(t, json.Unmarshal([]byte(`{"retry_complete_enabled":false}`), explicitFalse))
	require.False(t, absent.GetRetryCompleteEnabled())
	require.False(t, explicitFalse.GetRetryCompleteEnabled())
}
