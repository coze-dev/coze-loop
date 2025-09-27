// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"reflect"
	"runtime"
	"strconv"

	"github.com/bytedance/gg/gcond"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/pkg/errors"

	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func GetCode(err error) (code, isError int64) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 0, 4096)
			buf = buf[:runtime.Stack(buf, false)]
			err = errors.Errorf("GetCode panic: %v, stack: \n%s", r, buf)
			logs.Error("GetCode err: %v", err)
			return
		}
	}()
	if err == nil {
		return 0, 0
	}
	code = int64(601200702)
	isError = int64(1)
	value := reflect.ValueOf(err)
	if value.Kind() == reflect.Ptr { // 如果传入的是指针类型
		value = reflect.Indirect(value) // 解引用指针
	}
	kitexBizErrWrapper := value.FieldByName("kitexBizErrWrapper")
	if kitexBizErrWrapper.IsValid() { // 检查字段是否存在且有效
		if kitexBizErrWrapper.Kind() == reflect.Ptr { // 如果传入的是指针类型
			kitexBizErrWrapper = reflect.Indirect(kitexBizErrWrapper) // 解引用指针
		}
		status := kitexBizErrWrapper.FieldByName("status")
		if status.IsValid() {
			if status.Kind() == reflect.Ptr {
				status = reflect.Indirect(status)
			}
			statusCode := status.FieldByName("statusCode")
			if statusCode.IsValid() {
				code = statusCode.Int()
			}
			ext := status.FieldByName("ext")
			if ext.IsValid() {
				isAffectStability := ext.FieldByName("IsAffectStability")
				isError = gcond.If(isAffectStability.Bool(), int64(1), int64(0))
			}
		}
	}
	// 解析非errorx类型的错误
	var iface kerrors.BizStatusErrorIface
	ok := errors.As(err, &iface)
	if ok {
		code = int64(iface.BizStatusCode())
		extra := iface.BizExtra()
		if len(extra) != 0 {
			parseInt, _ := strconv.ParseInt(extra["biz_err_affect_stability"], 10, 64)
			isError = parseInt
		}
	}
	return code, isError
}
