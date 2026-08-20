// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"github.com/google/wire"
)

var MQProducerSet = wire.NewSet(
	NewExptEventPublisher,
	NewEvaluatorEventPublisher,
)
