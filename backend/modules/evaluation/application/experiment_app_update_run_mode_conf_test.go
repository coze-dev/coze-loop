// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	exptpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

func TestUpdateExptRunConf_NoLimitFields(t *testing.T) {
	for _, typ := range []reflect.Type{
		reflect.TypeOf(exptpb.UpdateExptRunConfRequest{}),
		reflect.TypeOf(entity.UpdateRunConfParam{}),
	} {
		t.Run(typ.Name(), func(t *testing.T) {
			for _, field := range []string{"MaxRunMinutes", "MaxTurns"} {
				_, exists := typ.FieldByName(field)
				assert.False(t, exists, "%s must not expose %s", typ.Name(), field)
				for _, prefix := range []string{"Get", "Set", "IsSet"} {
					_, exists := reflect.PointerTo(typ).MethodByName(prefix + field)
					assert.False(t, exists, "%s must not expose %s%s", typ.Name(), prefix, field)
				}
			}
		})
	}
}

func TestUpdateExptRunConf_LegacyLimitsIgnored(t *testing.T) {
	const (
		workspaceID = int64(123)
		exptID      = int64(456)
	)
	want := &exptpb.UpdateExptRunConfRequest{
		WorkspaceID: workspaceID, ExptID: exptID, ItemRetryNum: gptr.Of(int32(0)),
	}

	// Keep the removed field IDs on the wire to exercise legacy callers.
	transport := thrift.NewTMemoryBuffer()
	protocol := thrift.NewTBinaryProtocolTransport(transport)
	require.NoError(t, protocol.WriteStructBegin("UpdateExptRunConfRequest"))
	for _, field := range []struct {
		id    int16
		value int64
	}{{1, workspaceID}, {2, exptID}} {
		require.NoError(t, protocol.WriteFieldBegin("", thrift.I64, field.id))
		require.NoError(t, protocol.WriteI64(field.value))
		require.NoError(t, protocol.WriteFieldEnd())
	}
	for _, field := range []struct {
		id    int16
		value int32
	}{{4, 0}, {5, 45}, {6, 12}} {
		require.NoError(t, protocol.WriteFieldBegin("", thrift.I32, field.id))
		require.NoError(t, protocol.WriteI32(field.value))
		require.NoError(t, protocol.WriteFieldEnd())
	}
	require.NoError(t, protocol.WriteFieldStop())
	require.NoError(t, protocol.WriteStructEnd())
	legacyThrift := append([]byte(nil), transport.Bytes()...)

	for name, decode := range map[string]func(*exptpb.UpdateExptRunConfRequest) error{
		"json": func(req *exptpb.UpdateExptRunConfRequest) error {
			return json.Unmarshal([]byte(`{"workspace_id":123,"expt_id":456,"item_retry_num":0,"max_run_minutes":45,"max_turns":12}`), req)
		},
		"thrift": func(req *exptpb.UpdateExptRunConfRequest) error {
			return req.Read(protocol)
		},
		"thrift_fast": func(req *exptpb.UpdateExptRunConfRequest) error {
			n, err := req.FastRead(legacyThrift)
			assert.Equal(t, len(legacyThrift), n)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			req := new(exptpb.UpdateExptRunConfRequest)
			require.NoError(t, decode(req))
			require.Equal(t, want, req)

			raw, err := json.Marshal(req)
			require.NoError(t, err)
			assert.NotContains(t, string(raw), "max_run_minutes")
			assert.NotContains(t, string(raw), "max_turns")
			buf := make([]byte, req.BLength())
			require.Equal(t, len(buf), req.FastWriteNocopy(buf, nil))
			wantBuf := make([]byte, want.BLength())
			want.FastWriteNocopy(wantBuf, nil)
			assert.Equal(t, wantBuf, buf)

			ctrl := gomock.NewController(t)
			manager := servicemocks.NewMockIExptManager(ctrl)
			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			manager.EXPECT().Get(gomock.Any(), exptID, workspaceID, &entity.Session{}).Return(
				&entity.Experiment{ID: exptID, SpaceID: workspaceID, Status: entity.ExptStatus_Processing}, nil)
			auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
			manager.EXPECT().UpdateRunConf(gomock.Any(), &entity.UpdateRunConfParam{
				ExptID: exptID, SpaceID: workspaceID, ItemRetryNum: gptr.Of(0), Session: &entity.Session{},
			}).Return(nil)
			app := &experimentApplication{manager: manager, auth: auth}
			_, err = app.UpdateExptRunConf(context.Background(), req)
			require.NoError(t, err)
		})
	}
}
