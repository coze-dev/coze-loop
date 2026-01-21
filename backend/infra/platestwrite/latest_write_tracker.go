// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package platestwrite

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	goredis "github.com/redis/go-redis/v9"

	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type LatestWriteTracker struct {
	redisCli redis.Cmdable
}

func NewLatestWriteTracker(redisCli redis.Cmdable) ILatestWriteTracker {
	return &LatestWriteTracker{redisCli: redisCli}
}

//go:generate mockgen -destination ./mocks/latest_write_tracker.go  --package mocks . ILatestWriteTracker
type ILatestWriteTracker interface {
	SetWriteFlag(ctx context.Context, resourceType ResourceType, resourceID int64, opts ...SetWriteFlagOpt)
	CheckWriteFlagByID(ctx context.Context, resourceType ResourceType, id int64) bool
	CheckWriteFlagBySearchParam(ctx context.Context, resourceType ResourceType, searchParam string) bool
}

// setWriteFlagOpt 记录最近一次写操作，用于解决主从延迟问题
type setWriteFlagOpt struct {
	ttl time.Duration

	searchParam string // 范围查询时，在此查询条件下最近是否有过写操作，使用时需要自行保证set和get时查询的值一致
}

type SetWriteFlagOpt func(opt *setWriteFlagOpt)

func SetWithSearchParam(searchParam string) SetWriteFlagOpt {
	return func(opt *setWriteFlagOpt) {
		opt.searchParam = searchParam
	}
}

type resourceInfo struct {
	resourceType ResourceType
	id           int64  // 根据 ID 获取单个资源时，当前资源最近是否有过写操作
	searchParam  string // 在当前查询条件下，资源最近是否有写操作，通常为 spaceID
}

// SetWriteFlag 记录实体的写操作
func (t *LatestWriteTracker) SetWriteFlag(ctx context.Context, resourceType ResourceType, resourceID int64, opts ...SetWriteFlagOpt) {
	opt := &setWriteFlagOpt{ttl: 5 * time.Second}
	for _, o := range opts {
		o(opt)
	}
	resource := &resourceInfo{
		resourceType: resourceType,
		id:           resourceID,
	}
	t.setWriteFlag(ctx, resource, false, opt)
	if opt.searchParam != "" {
		resource.searchParam = opt.searchParam
		t.setWriteFlag(ctx, resource, true, opt)
	}
}

func (t *LatestWriteTracker) setWriteFlag(ctx context.Context, resource *resourceInfo, isSearch bool, opt *setWriteFlagOpt) {
	if resource == nil {
		return
	}
	key := getWriteFlagKey(resource, isSearch)
	cli := t.redisCli
	if err := cli.Set(ctx, key, 1, opt.ttl).Err(); err != nil {
		// 写入失败不影响主流程
		logs.CtxWarn(ctx, "set latest write flag failed, key=%s", key)
	}
}

// CheckWriteFlagByID 如果 writeFlag 存在，则说明当前资源最近有写入记录，需要读主
func (t *LatestWriteTracker) CheckWriteFlagByID(ctx context.Context, resourceType ResourceType, id int64) bool {
	key := getWriteFlagKey(&resourceInfo{resourceType: resourceType, id: id}, false)
	cli := t.redisCli
	if err := cli.Get(ctx, key).Err(); err == nil {
		return true
	} else if !errors.Is(err, goredis.Nil) {
		// 读取失败，不影响查询
		logs.CtxWarn(ctx, "get write flag failed, key=%s", key)
	}
	return false
}

// CheckWriteFlagBySearchParam 如果 writeFlag 存在，则说明在当前查询范围内，当前资源有写入记录，需要读主
// notice: 这边的 searchParam 需要自行在 SetWriteFlag 时通过 SetWithSearchParam 设置，并自行保证查询的值和 set 的值一致
func (t *LatestWriteTracker) CheckWriteFlagBySearchParam(ctx context.Context, resourceType ResourceType, searchParam string) bool {
	key := getWriteFlagKey(&resourceInfo{resourceType: resourceType, searchParam: searchParam}, true)
	cli := t.redisCli
	if err := cli.Get(ctx, key).Err(); err == nil {
		return true
	} else if !errors.Is(err, goredis.Nil) {
		// 读取失败，不影响查询
		logs.CtxWarn(ctx, "get write flag failed, key=%s", key)
	}
	return false
}

func getWriteFlagKey(info *resourceInfo, isSearch bool) string {
	if isSearch {
		return fmt.Sprintf("db_last_write_flag:search:%s:%s", info.resourceType, info.searchParam)
	} else {
		return fmt.Sprintf("db_last_write_flag:resource:%s:%d", info.resourceType, info.id)
	}
}

type ResourceType string

const (
	ResourceTypeDataset ResourceType = "dataset"
	ResourceTypeSchema  ResourceType = "schema"
	ResourceTypeIOJob   ResourceType = "io_job"
	ResourceTypeVersion ResourceType = "version"
	ResourceTypeItem    ResourceType = "item"

	ResourceTypePromptBasic              ResourceType = "prompt_basic"
	ResourceTypePromptDraft              ResourceType = "prompt_draft"
	ResourceTypePromptCommit             ResourceType = "prompt_commit"
	ResourceTypePromptLabel              ResourceType = "prompt_label"
	ResourceTypePromptCommitLabelMapping ResourceType = "prompt_commit_label_mapping"
	ResourceTypeCozeloopOptimizeTask     ResourceType = "cozeloop_optimize_task" // 外场智能优化
	ResourceTypePromptRelation           ResourceType = "prompt_relation"
	ResourceTypePromptRelease            ResourceType = "prompt_release"
	ResourceTypeReleaseTask              ResourceType = "release_task"
	ResourceTypeReleaseSubtask           ResourceType = "release_subtask"
	ResourceTypeReleaseTaskResource      ResourceType = "release_task_resource"

	ResourceTypeExperiment    ResourceType = "experiment"
	ResourceTypeEvalSet       ResourceType = "eval_set"
	ResourceTypeTarget        ResourceType = "eval_target"
	ResourceTypeTargetVersion ResourceType = "eval_target_version"
	ResourceTypeEvaluator     ResourceType = "evaluator"

	ResourceTypeExptInsightAnalysisRecord   ResourceType = "expt_insight_analysis_record"
	ResourceTypeExptInsightAnalysisFeedback ResourceType = "expt_insight_analysis_feedback"
)
