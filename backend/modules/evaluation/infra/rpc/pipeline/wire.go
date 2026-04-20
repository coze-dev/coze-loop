// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	evalrpc "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/google/wire"
)

var PipelineRPCSet = wire.NewSet(
	NewPipelineListAdapter,
	wire.Bind(new(evalrpc.IPipelineListAdapter), new(*PipelineListAdapter)),
)
