// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/mq/rocket/consumer"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func startHookRuntime(ctx context.Context, app application.IExperimentApplication) (func(), error) {
	runtime, err := application.HookRuntimeFromApplication(app)
	if err != nil {
		return nil, err
	}
	handler, err := consumer.NewHookWakeConsumer(runtime.ExecutionScope, runtime.Worker)
	if err != nil {
		return nil, err
	}
	stop, err := runtime.Start(ctx, handler)
	if err != nil {
		return nil, err
	}
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if stop(ctx) != nil {
			logs.Error("lifecycle hook worker shutdown incomplete")
		}
	}, nil
}
