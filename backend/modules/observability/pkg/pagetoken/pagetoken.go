// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package pagetoken

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// PageToken 是 span keyset 分页游标 (start_time 微秒, span_id)，base64(JSON) 编码。
// 提到 pkg 供 service 自行拼接锚点游标、infra 消费与产出，二者共用同一编码契约。
type PageToken struct {
	StartTime int64  `json:"StartTime"`
	SpanID    string `json:"SpanID"`
}

func Encode(startTimeMicroSec int64, spanID string) string {
	pt, _ := json.Marshal(&PageToken{StartTime: startTimeMicroSec, SpanID: spanID})
	return base64.StdEncoding.EncodeToString(pt)
}

func Decode(token string) (*PageToken, error) {
	if token == "" {
		return nil, nil
	}
	raw, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("fail to decode pageToken %s, %v", token, err)
	}
	pt := new(PageToken)
	if err := json.Unmarshal(raw, pt); err != nil {
		return nil, fmt.Errorf("fail to unmarshal pageToken %s, %v", string(raw), err)
	}
	return pt, nil
}
