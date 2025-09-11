// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"crypto/md5"
	"fmt"
	"strconv"
)

// KeyGenerator 缓存Key生成器接口
type KeyGenerator interface {
	TaskKey(id int64) string
	TaskListKey(workspaceID int64, filterHash string) string
	TaskRunKey(id int64) string
	TaskRunListKey(taskID int64, filterHash string) string
	GenerateFilterHash(param interface{}) string
}

// keyGeneratorImpl Key生成器实现
type keyGeneratorImpl struct{}

// NewKeyGenerator 创建Key生成器
func NewKeyGenerator() KeyGenerator {
	return &keyGeneratorImpl{}
}

// TaskKey 生成任务缓存Key
func (k *keyGeneratorImpl) TaskKey(id int64) string {
	return fmt.Sprintf("observability:task:%d", id)
}

// TaskListKey 生成任务列表缓存Key
func (k *keyGeneratorImpl) TaskListKey(workspaceID int64, filterHash string) string {
	return fmt.Sprintf("observability:task:list:%d:%s", workspaceID, filterHash)
}

// TaskRunKey 生成任务运行记录缓存Key
func (k *keyGeneratorImpl) TaskRunKey(id int64) string {
	return fmt.Sprintf("observability:taskrun:%d", id)
}

// TaskRunListKey 生成任务运行记录列表缓存Key
func (k *keyGeneratorImpl) TaskRunListKey(taskID int64, filterHash string) string {
	return fmt.Sprintf("observability:taskrun:list:%d:%s", taskID, filterHash)
}

// GenerateFilterHash 基于查询参数生成哈希值
func (k *keyGeneratorImpl) GenerateFilterHash(param interface{}) string {
	// 将参数转换为字符串并生成MD5哈希
	paramStr := fmt.Sprintf("%+v", param)
	hash := md5.Sum([]byte(paramStr))
	return fmt.Sprintf("%x", hash)
}