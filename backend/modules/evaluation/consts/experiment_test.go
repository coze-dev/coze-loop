// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consts

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// 执行期 Ext 是一张**跨仓库的字符串键值表**: 平台侧在 callTarget 里按这些 key 写入,
// 评测算子(另一个仓库/另一个部署单元)按同样的字面量读出。两端没有共享的类型, 只有这些字符串。
//
// 因此改字面量的后果是: 本仓编译通过、单测通过、请求成功、实验 success ——
// 只是算子那一侧 `ext[key]` 取到空串, 于是按"未配置"走缺省。这类回归没有任何报错面,
// 只能靠钉住字面量本身来防。下面每个常量都直接断言裸字符串, **不允许**改成
// `assert.Equal(t, TargetExecuteExtRunConfKey, TargetExecuteExtRunConfKey)` 那种自证式写法。

// TargetExecuteExtRunConfKey 承载题目级多轮/SUA 配置 (ItemRunConf JSON)。
// 值漂了 ⇒ 算子读不到题目级配置 ⇒ 每题都按实验级默认值跑: fixed_query_list 丢失
// (多轮题变单轮)、题目级 max_turns / sua_* 覆盖全部失效, 而实验照样 success。
func TestTargetExecuteExtRunConfKey(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "builtin_run_conf", TargetExecuteExtRunConfKey,
		"该 key 由评测算子按字面量读取; 改了它题目级多轮配置会静默失效")
}

// TargetExecuteExtRunModeConfigKey 承载实验级跑法配置 (RunModeConfig JSON)。
// 值漂了 ⇒ 算子填 case-file experiment_info 时读不到 run_mode/sua_mode ⇒ 整个实验回落
// 默认跑法。这是"实验按另一个跑法跑完并 success"的其中一条路径。
func TestTargetExecuteExtRunModeConfigKey(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "builtin_run_mode_config", TargetExecuteExtRunModeConfigKey,
		"该 key 由评测算子按字面量读取; 改了它整个实验会回落默认跑法")
}

// 三个 builtin_* Ext key 必须互不相同 —— 撞名会让后写的那个覆盖前一个,
// 表现为"其中一份配置凭空消失"(map 写入不会报错)。
//
// 尤其 run_conf 与 run_mode_config 是**同一次 callTarget 里前后写入**的两个 key,
// 名字又只差一个词, 是最可能被复制粘贴改错的一对。
func TestTargetExecuteExtKeysAreDistinct(t *testing.T) {
	t.Parallel()

	keys := []string{
		TargetExecuteExtRuntimeParamKey,
		TargetExecuteExtRunConfKey,
		TargetExecuteExtRunModeConfigKey,
		FieldAdapterBuiltinFieldNameSkillTOSKeys,
	}
	seen := make(map[string]bool, len(keys))
	for _, k := range keys {
		assert.NotEmpty(t, k, "Ext key 不能为空串 —— 空 key 会写进 map 且永远读不到")
		assert.False(t, seen[k], "Ext key %q 重复; 同一个 ext map 里后写的会静默覆盖前一个", k)
		seen[k] = true
	}
}

// FieldAdapterBuiltinFieldNameRuntimeParam 与 TargetExecuteExtRuntimeParamKey 是**刻意同值**的
// 一对: 前者是 CustomConf 里的字段名(平台内部约定), 后者是 Ext 里的 key(跨仓约定),
// callTarget 就是靠"字段名 == ext key"这条恒等式把 CustomConf 的值搬进 Ext 的
// (`if fc.FieldName == FieldAdapterBuiltinFieldNameRuntimeParam { ext[TargetExecuteExtRuntimeParamKey] = fc.Value }`)。
//
// 只改其中一个 ⇒ 那个 if 永远不成立 ⇒ runtime_param 再也不会被透传, 且没有任何报错。
// 这条恒等式在代码里是隐式的(两个不同名的常量), 所以必须由测试显式钉住。
func TestRuntimeParamFieldNameEqualsExtKey(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "builtin_runtime_param", FieldAdapterBuiltinFieldNameRuntimeParam)
	assert.Equal(t, "builtin_runtime_param", TargetExecuteExtRuntimeParamKey)
	assert.Equal(t, FieldAdapterBuiltinFieldNameRuntimeParam, TargetExecuteExtRuntimeParamKey,
		"两者刻意同值: callTarget 靠这条恒等式把 CustomConf 的值搬进 Ext; 只改一个会让透传静默失效")
}
