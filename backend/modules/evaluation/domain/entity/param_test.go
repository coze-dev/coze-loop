// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
)

func TestWithCluster(t *testing.T) {
	t.Parallel()

	t.Run("set cluster", func(t *testing.T) {
		opt := &Opt{}
		WithCluster(gptr.Of("default"))(opt)
		assert.Equal(t, gptr.Of("default"), opt.Cluster)
	})

	t.Run("set nil cluster", func(t *testing.T) {
		opt := &Opt{}
		WithCluster(nil)(opt)
		assert.Nil(t, opt.Cluster)
	})
}

func TestWithAgentConnection(t *testing.T) {
	t.Parallel()

	t.Run("set agent connection", func(t *testing.T) {
		conn := &AgentConnection{
			IP:     "127.0.0.1",
			Region: "cn",
			IDC:    "lf",
			PSM:    "test.psm",
			FrontierInfo: &FrontierInfo{
				AppID:     100,
				ProductID: 200,
				UserID:    300,
				DeviceID:  400,
			},
			AgentImpl: &AgentImpl{
				Language:  "go",
				Framework: "Eino",
				Kind:      "custom",
			},
		}
		opt := &Opt{}
		WithAgentConnection(conn)(opt)
		assert.Equal(t, conn, opt.AgentConnection)
		assert.Equal(t, "127.0.0.1", opt.AgentConnection.IP)
		assert.Equal(t, int64(100), opt.AgentConnection.FrontierInfo.AppID)
		assert.Equal(t, "go", opt.AgentConnection.AgentImpl.Language)
	})

	t.Run("set nil agent connection", func(t *testing.T) {
		opt := &Opt{}
		WithAgentConnection(nil)(opt)
		assert.Nil(t, opt.AgentConnection)
	})
}
