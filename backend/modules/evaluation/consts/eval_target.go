// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consts

import (
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
)

const (
	EvalTargetInputFieldKeyPromptUserQuery = expt.PromptUserQueryFieldKey

	EvalTargetOutputFieldKeyActualOutput       = common.ArgSchemaKeyActualOutput
	EvalTargetOutputFieldKeyTrajectory         = common.ArgSchemaKeyTrajectory
	EvalTargetOutputFieldKeyScreenRecordingURI = "screen_recording_uri"
	EvalTargetOutputFieldKeyScreenRecordingURL = "screen_recording_url"

	// OutputDataExtKeySandboxExecuteIDs 沙箱执行 ID 列表 (JSON 字符串数组) 的 output ext key。
	// 由具体实现 (SandboxAgent operator) 在 AsyncExecute 返回的 ext 里回传自己实际创建的 execution id:
	// 双沙箱一次调用会创建多个 (agent / orchestrator 各一个), id 命名规则属实现细节, 平台侧不做推断。
	// 销毁沙箱时按此列表逐个 Destroy; 缺省 (老数据 / 单沙箱实现未回传) 退回用 record.ID。
	OutputDataExtKeySandboxExecuteIDs = "sandbox_execute_ids"
)
