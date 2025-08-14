// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consts

import (
	"math"
)

const (
	TypeString  = "string"
	TypeInteger = "integer"
	TypeNumber  = "number"
	TypeBoolean = "boolean"
	TypeObject  = "object"
	TypeArray   = "array"
	TypeNull    = "null"
)

const MaxVersionNum int64 = math.MaxUint16

const (
	DataConfigFileName = "data.yaml"

	FallbackTagValueDefaultName = "其他「系统自动生成」"
)
