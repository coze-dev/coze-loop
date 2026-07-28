// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package sandbox_agent

// ClassifyErrorType 将一次执行的错误归入看板类别。
//
// 分类规则（对齐 modules/evaluation/Trae评测迁移fornax稳定性技术方案.docx §Step2 的伪 SQL）：
//   - err == nil && code == 0            → "-"                成功不带错误分类
//   - code 命中工程错误码集合             → "engineering"      Fornax 平台侧稳定性问题
//   - code != 0 且非工程错误码           → "non_engineering"  用户侧/业务侧问题（如模型限流、Schema 校验失败等）
//   - err != nil 且 code == 0             → "unknown"          error 存在但未落错误码
//
// 工程错误码集合与 errno.evaluation.go 中 NoAffectStability=false 的常量对齐。
// 后续如新增工程错误码，只需修改本文件的 engineeringErrorCodes 表。
func ClassifyErrorType(err error, code int32) string {
	if err == nil && code == 0 {
		return "-"
	}
	if code == 0 {
		// error 存在但沙箱侧未提供错误码，暂归 unknown 便于看板筛选补齐
		return "unknown"
	}
	if _, ok := engineeringErrorCodes[code]; ok {
		return "engineering"
	}
	return "non_engineering"
}

// engineeringErrorCodes 属于"工程问题"分类的错误码集合。
// 与 backend/pkg/errno/errno.evaluation.go 中 NoAffectStability=false 的常量保持一致。
var engineeringErrorCodes = map[int32]struct{}{
	601200701: {}, // CommonNetworkTimeOut
	601200702: {}, // CommonInternalError
	601200703: {}, // CommonRPCError
	601200801: {}, // CommonMySqlError
	601200803: {}, // CommonRedisError
	601205015: {}, // InvalidOutputFromModel
	601205036: {}, // FileURLRetrieveFailed
	601205037: {}, // GoroutinePoolCreateFailed
	601205038: {}, // BatchTaskExecutionFailed
}
