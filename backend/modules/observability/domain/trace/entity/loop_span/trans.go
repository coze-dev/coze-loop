// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package loop_span

import (
	"context"
	"encoding/json"
	"time"

	"github.com/coze-dev/coze-loop/backend/pkg/lang/slices"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

/*
span trans handler config

	{
		"platform_cfg": {
			"cozeloop": [
					{
						"span_filter": {
							"query_and_or": 1,
							"filter_fields": []
						},
						"is_filter_tag": true,
						"tag_key_black_list": ["a"]
					}
			],
			"prompt": {}
		}
	}
*/
type SpanTransCfgList []*SpanTransConfig

type SpanTransConfig struct {
	SpanFilter   *FilterFields `mapstructure:"span_filter" json:"span_filter"`
	TagFilter    *TagFilter    `mapstructure:"tag_filter" json:"tag_filter"`
	InputFilter  *InputFilter  `mapstructure:"input_filter" json:"input_filter"`
	OutputFilter *OutputFilter `mapstructure:"output_filter" json:"output_filter"`
}

type TagFilter struct {
	KeyBlackList    []string `mapstructure:"key_black_list" json:"key_black_list"`
	keyBlackListMap map[string]bool
}

type InputFilter struct {
	KeyWhiteList    []string `mapstructure:"key_white_list" json:"key_white_list"`
	keyWhiteListMap map[string]bool
}

type OutputFilter struct {
	KeyWhiteList    []string `mapstructure:"key_white_list" json:"key_white_list"`
	keyWhiteListMap map[string]bool
}

func (c *TagFilter) transform(ctx context.Context, span *Span) {
	for key := range span.TagsString {
		if c.keyBlackListMap[key] {
			delete(span.TagsString, key)
		}
	}
	for key := range span.TagsDouble {
		if c.keyBlackListMap[key] {
			delete(span.TagsDouble, key)
		}
	}
	for key := range span.TagsBool {
		if c.keyBlackListMap[key] {
			delete(span.TagsBool, key)
		}
	}
	for key := range span.TagsLong {
		if c.keyBlackListMap[key] {
			delete(span.TagsLong, key)
		}
	}
	for key := range span.TagsByte {
		if c.keyBlackListMap[key] {
			delete(span.TagsByte, key)
		}
	}
}

func (c *InputFilter) transform(ctx context.Context, span *Span) {
	if span.Input == "" {
		return
	}
	out := make(map[string]any)
	if err := json.Unmarshal([]byte(span.Input), &out); err != nil {
		logs.CtxWarn(ctx, "fail to trans input %s into map", span.Input)
		return
	}
	for key := range out {
		if !c.keyWhiteListMap[key] {
			delete(out, key)
		}
	}
	newInput, err := json.Marshal(out)
	if err != nil {
		logs.CtxWarn(ctx, "fail to marshal new input %v: %v", out, err)
		return
	}
	span.Input = string(newInput)
}

func (c *OutputFilter) transform(ctx context.Context, span *Span) {
	if span.Output == "" {
		return
	}
	out := make(map[string]any)
	if err := json.Unmarshal([]byte(span.Output), &out); err != nil {
		logs.CtxWarn(ctx, "fail to trans output %s into map", span.Output)
		return
	}
	for key := range out {
		if !c.keyWhiteListMap[key] {
			delete(out, key)
		}
	}
	newOutput, err := json.Marshal(out)
	if err != nil {
		logs.CtxWarn(ctx, "fail to marshal new output %v: %v", out, err)
		return
	}
	span.Output = string(newOutput)
}

func (c *SpanTransConfig) init() {
	if c.TagFilter != nil {
		c.TagFilter.keyBlackListMap = slices.ToMap(c.TagFilter.KeyBlackList, func(e string) (string, bool) { return e, true })
	}
	if c.InputFilter != nil {
		c.InputFilter.keyWhiteListMap = slices.ToMap(c.InputFilter.KeyWhiteList, func(e string) (string, bool) { return e, true })
	}
	if c.OutputFilter != nil {
		c.OutputFilter.keyWhiteListMap = slices.ToMap(c.OutputFilter.KeyWhiteList, func(e string) (string, bool) { return e, true })
	}
}

func (c *SpanTransConfig) satisfyFilter(span *Span) bool {
	if c.SpanFilter == nil { // 没有filter条件
		return true
	}
	return c.SpanFilter.Satisfied(span)
}

func (c *SpanTransConfig) process(ctx context.Context, span *Span) {
	if c.TagFilter != nil {
		c.TagFilter.transform(ctx, span)
	}
	if c.InputFilter != nil {
		c.InputFilter.transform(ctx, span)
	}
	if c.OutputFilter != nil {
		c.OutputFilter.transform(ctx, span)
	}
}

func (p SpanTransCfgList) init() {
	for _, cfg := range p {
		cfg.init()
	}
}

// 是否满足Filter条件, 满足则保留
func (p SpanTransCfgList) satisfyFilter(span *Span) bool {
	if len(p) == 0 {
		return true
	}
	// or relation
	for _, cfg := range p {
		if cfg.satisfyFilter(span) {
			return true
		}
	}
	return false
}

func (p SpanTransCfgList) doFilter(ctx context.Context, spans SpanList) (SpanList, error) {
	st := time.Now().UnixMicro()
	out := make(SpanList, 0, len(spans))
	redirectMap := make(map[string]string)
	for _, span := range spans {
		if !p.satisfyFilter(span) { // 不满足条件, 去除该Span
			redirectMap[span.SpanID] = span.ParentID
			continue
		} else if span.LogicDeleteTime > 0 && span.LogicDeleteTime < st {
			logs.CtxInfo(ctx, "span_id %s delete time %d reached, just drop the span", span.SpanID, span.LogicDeleteTime)
			redirectMap[span.SpanID] = span.ParentID
			continue
		}
		out = append(out, span)
	}
	p.redirectSpansParentID(ctx, out, redirectMap)
	return out, nil
}

func (p SpanTransCfgList) redirectSpansParentID(ctx context.Context, spans SpanList, redirectMap map[string]string) {
	if len(redirectMap) == 0 {
		return
	}
	for _, sp := range spans {
		p.redirectSpanParentID(ctx, sp, redirectMap, 1000)
	}
}

func (p SpanTransCfgList) redirectSpanParentID(ctx context.Context, span *Span, redirectMap map[string]string, leftTimes int) {
	if leftTimes == 0 {
		return
	}
	newParentID, needRedirect := redirectMap[span.ParentID]
	if !needRedirect {
		return
	}
	if newParentID == span.ParentID {
		return
	}
	span.ParentID = newParentID
	p.redirectSpanParentID(ctx, span, redirectMap, leftTimes-1)
}

func (p SpanTransCfgList) doProcess(ctx context.Context, spans SpanList) (SpanList, error) {
	for _, span := range spans {
		for _, cfg := range p {
			if cfg.satisfyFilter(span) {
				cfg.process(ctx, span)
			}
		}
	}
	return spans, nil
}

func (p SpanTransCfgList) Transform(ctx context.Context, spans SpanList) (SpanList, error) {
	p.init()
	spans, err := p.doFilter(ctx, spans)
	if err != nil {
		return nil, err
	}
	spans, err = p.doProcess(ctx, spans)
	if err != nil {
		return nil, err
	}
	return spans, nil
}
