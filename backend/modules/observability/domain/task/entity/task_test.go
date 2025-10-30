// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package entity

import (
	"context"
	"reflect"
	"testing"
)

func TestObservabilityTask_SetTaskStatus(t *testing.T) {
	tests := []struct {
		name         string             // 测试用例名称
		initialTask  ObservabilityTask  // 任务的初始状态
		targetStatus TaskStatus         // 目标设置的状态
		wantEvent    *StatusChangeEvent // 期望返回的事件
		wantErr      bool               // 是否期望发生错误
		finalStatus  TaskStatus         // 期望的最终任务状态
	}{
		{
			name:         "状态相同时不进行变更",
			initialTask:  ObservabilityTask{TaskStatus: TaskStatusRunning},
			targetStatus: TaskStatusRunning,
			wantEvent:    nil,
			wantErr:      false,
			finalStatus:  TaskStatusRunning,
		},
		{
			name:         "有效状态流转：从未开始到运行中",
			initialTask:  ObservabilityTask{TaskStatus: TaskStatusUnstarted},
			targetStatus: TaskStatusRunning,
			wantEvent: &StatusChangeEvent{
				Before: TaskStatusUnstarted,
				After:  TaskStatusRunning,
			},
			wantErr:     false,
			finalStatus: TaskStatusRunning,
		},
		{
			name:         "有效状态流转：从挂起到运行中",
			initialTask:  ObservabilityTask{TaskStatus: TaskStatusPending},
			targetStatus: TaskStatusRunning,
			wantEvent: &StatusChangeEvent{
				Before: TaskStatusPending,
				After:  TaskStatusRunning,
			},
			wantErr:     false,
			finalStatus: TaskStatusRunning,
		},
		{
			name:         "无效状态流转：从禁用状态到其他状态",
			initialTask:  ObservabilityTask{TaskStatus: TaskStatusDisabled},
			targetStatus: TaskStatusRunning,
			wantEvent:    nil,
			wantErr:      true,
			finalStatus:  TaskStatusDisabled,
		},
	}

	// 遍历并执行所有测试用例
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: 创建一个任务副本以防止并发测试时修改原始测试用例数据
			task := tt.initialTask

			// Act: 调用被测方法
			gotEvent, err := task.SetTaskStatus(context.Background(), tt.targetStatus)

			// Assert: 校验错误是否符合预期
			if (err != nil) != tt.wantErr {
				t.Errorf("SetTaskStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Assert: 校验返回的事件是否符合预期
			if !reflect.DeepEqual(gotEvent, tt.wantEvent) {
				t.Errorf("SetTaskStatus() gotEvent = %v, want %v", gotEvent, tt.wantEvent)
			}

			// Assert: 校验任务的最终状态是否符合预期
			if task.TaskStatus != tt.finalStatus {
				t.Errorf("Final task status = %v, want %v", task.TaskStatus, tt.finalStatus)
			}
		})
	}
}
