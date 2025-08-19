// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/coreos/go-semver/semver"

	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/errno"
)

const (
	MinSem = 0
	MaxSem = 999
)

func ValidateVersion(preVersion, newVersion string) error {
	newV, err := semver.NewVersion(newVersion)
	if err != nil {
		return errno.InvalidParamErr(err, "version '%s' not a valid semantic version", newVersion)
	}
	seg := newV.Slice()
	for _, v := range seg {
		if v < MinSem || v > MaxSem {
			return errno.InvalidParamErrorf("each segment of sem version must be between 0 and 999")
		}
	}

	if preVersion == "" { // 无历史版本，直接返回
		return nil
	}

	preV, err := semver.NewVersion(preVersion)
	if err != nil {
		return errno.InternalErr(err, "previous version '%s' not a valid semantic version", preVersion)
	}
	if !preV.LessThan(*newV) {
		return errno.InvalidParamErrorf("new version '%s' should be greater than '%s'", newVersion, preVersion)
	}
	return nil
}

// SimpleIncrementVersion 简单的满则进位版本号递增,一般不需要这样方法，这里支持特殊的场景
// 每段范围0-999，满了就进位
func SimpleIncrementVersion(version string) (string, error) {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return "", errno.InvalidParamErrorf("version %s is invalid", version)
	}

	// 将各部分转换为数字
	var nums [3]int
	for i, part := range parts {
		num, err := strconv.Atoi(part)
		if err != nil {
			return "", errno.InvalidParamErrorf("version %s is invalid", version)
		}
		if num < 0 || num > 999 {
			return "", errno.InvalidParamErrorf("version %s is invalid", version)
		}
		nums[i] = num
	}

	// 从最后一位开始递增并处理进位
	nums[2]++ // 增加修订号

	// 处理进位
	if nums[2] > 999 {
		nums[2] = 0
		nums[1]++
	}
	if nums[1] > 999 {
		nums[1] = 0
		nums[0]++
	}
	if nums[0] > 999 {
		return "", errno.InvalidParamErrorf("version %s is more than max supported version", version)
	}

	// 重新组合为字符串
	return fmt.Sprintf("%d.%d.%d", nums[0], nums[1], nums[2]), nil
}
