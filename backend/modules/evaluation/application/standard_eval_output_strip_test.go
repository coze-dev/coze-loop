// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"slices"
	"strings"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func textContent(s string) *entity.Content {
	return &entity.Content{
		ContentType: gptr.Of(entity.ContentTypeText),
		Text:        gptr.Of(s),
	}
}

// TestStripStandardEvalOutputSelfFields 校验 detail.output 的去嵌套过滤。
//
// 背景：外层响应的 output/eval/agent/rounds/detail/extra 已由 mergeStandardEvalOutputField
// 逐个装配，而 output_fields 顶层恰好含着这些字段本身。若整包塞进 detail.output，
// 同一份数据会在响应里出现两次（外层一份 + detail.output 里套一份）。
func TestStripStandardEvalOutputSelfFields(t *testing.T) {
	t.Run("剔 FORNAX_ 前缀 + 裸标准 key，保留业务字段", func(t *testing.T) {
		in := map[string]*entity.Content{
			// ① FORNAX_ 前缀：新协议上报，全部剔掉
			"FORNAX_output": textContent(`{"detail":{"output":{}}}`),
			"FORNAX_eval":   textContent(`{"task_config":{}}`),
			"FORNAX_agent":  textContent(`{"agent_id":"codex"}`),
			"FORNAX_rounds": textContent(`[]`),
			"FORNAX_detail": textContent(`{}`),
			"FORNAX_extra":  textContent(`{}`),
			"FORNAX_source": textContent(`{}`),
			// ② 裸标准 key：存量上报的向前兼容形态，也被 lookupFornaxField 消费，同样剔掉
			"output": textContent(`{}`),
			"eval":   textContent(`{}`),
			"agent":  textContent(`{}`),
			"rounds": textContent(`[]`),
			"detail": textContent(`{}`),
			"extra":  textContent(`{}`),
			// ③ 业务字段与平台元信息：外层不消费、不构成嵌套，必须保留
			"actual_output":          textContent("1.0000"),
			"source":                 textContent(`{"type":"evaluation"}`),
			"fornax_sandbox_log_url": textContent("https://example.invalid/a.log"),
			"my_custom_field":        textContent("x"),
		}

		got := stripStandardEvalOutputSelfFields(in)

		wantKeys := []string{"actual_output", "source", "fornax_sandbox_log_url", "my_custom_field"}
		assert.Len(t, got, len(wantKeys))
		for _, k := range wantKeys {
			assert.Contains(t, got, k, "业务字段 %q 不该被剔", k)
		}
		for k := range in {
			if _, keep := got[k]; keep {
				continue
			}
			assert.True(t,
				strings.HasPrefix(k, standardEvalOutputFornaxPrefix) ||
					slices.Contains(standardEvalOutputMergeableFields, k),
				"被剔的 key %q 必须属于 FORNAX_ 前缀或裸标准字段", k)
		}
		// 剔掉的不该改动保留字段的内容
		assert.Equal(t, "1.0000", got["actual_output"].GetText())
	})

	// ★ source 刻意保留：它不在 standardEvalOutputMergeableFields 内（恒由平台生成），
	//   外层不消费裸 source，故不构成嵌套。
	t.Run("裸 source 保留", func(t *testing.T) {
		got := stripStandardEvalOutputSelfFields(map[string]*entity.Content{
			"source": textContent(`{"type":"evaluation"}`),
		})
		assert.Contains(t, got, "source")
	})

	// ★ 小写 fornax_ 不等于 FORNAX_：平台元信息，保留。
	t.Run("小写 fornax_ 前缀保留", func(t *testing.T) {
		got := stripStandardEvalOutputSelfFields(map[string]*entity.Content{
			"fornax_sandbox_log_url": textContent("u"),
			"fornax_output":          textContent("v"),
		})
		assert.Len(t, got, 2)
	})

	// 返回值恒非 nil：避免 detail.output 从 {} 变成 null，让消费方多一种形态要处理。
	t.Run("空入参返回空 map 而非 nil", func(t *testing.T) {
		for name, in := range map[string]map[string]*entity.Content{
			"nil":   nil,
			"empty": {},
		} {
			got := stripStandardEvalOutputSelfFields(in)
			assert.NotNil(t, got, "%s 入参应返回非 nil map", name)
			assert.Empty(t, got)
		}
	})

	t.Run("全是标准字段时返回空 map 而非 nil", func(t *testing.T) {
		got := stripStandardEvalOutputSelfFields(map[string]*entity.Content{
			"FORNAX_output": textContent(`{}`),
			"eval":          textContent(`{}`),
		})
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	// 过滤口径必须与 lookupFornaxField 的查找口径对齐：它认前缀也认裸 key，
	// 故凡是它能查到的 key，都不该留在 detail.output 里（否则就是嵌套）。
	t.Run("与 lookupFornaxField 口径对齐", func(t *testing.T) {
		for _, f := range standardEvalOutputMergeableFields {
			for _, k := range []string{standardEvalOutputFornaxPrefix + f, f} {
				in := map[string]*entity.Content{k: textContent(`{}`)}
				_, found := lookupFornaxField(in, f)
				assert.True(t, found, "lookupFornaxField 应能查到 %q", k)
				assert.Empty(t, stripStandardEvalOutputSelfFields(in),
					"lookupFornaxField 能查到的 %q 必须被剔", k)
			}
		}
	})
}
