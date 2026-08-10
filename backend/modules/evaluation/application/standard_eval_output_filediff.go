// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// 本文件负责把评测对象自报的 **被截断的** file_diff 补全。
//
// 背景：沙箱侧 `git diff HEAD | gzip` 上传 TOS，orchestrator 在 report 阶段下载、解压、
// 切分成 diff_details 后随 FORNAX_output 回写。为了不撞 ext_output 的 ~200KB 天花板，
// orchestrator 会做两层截断（单文件 1000 行 / 总量 64KB），并把被截掉的事实记在
// files_omitted、diff_details[].truncated 上，完整原文留在 archive_url。
//
// 平台在被调 standard eval output 时，只要发现这两个截断信号，就去 archive_url 取回全文
// 重新切分，把 diff_details 换成全量。**没有截断信号则一个字节都不拉** —— 绝大多数 diff
// 本来就完整，无条件拉会把一次 MGet（最多 100 个 item）变成 100 次串行 HTTP。
//
// 这是"增强"而非"依赖"：取全文的任何一步失败，一律保留对象自报的截断版本，绝不让读接口
// 因为 TOS 抖动而失败。

const (
	// fileDiffFetchTimeout 单次取全文的超时。
	//
	// MGet 一次最多 maxStandardEvalOutputMGetItemIDs 个 item，且整条读路径没有 deadline，
	// 所以这里必须自带上限：否则一个卡住的 TOS 连接会拖死整个请求。
	fileDiffFetchTimeout = 10 * time.Second
	// fileDiffMaxDownloadBytes 下载上限（对应 runtime 侧同名约束）。
	// 超过这个量级的 diff 已经不是代码评审材料（生成物 / vendor 目录进了提交）。
	fileDiffMaxDownloadBytes = 32 << 20
	// fileDiffMaxDecompressedBytes 解压后上限，**必须与下载上限各自独立**：
	// gzip 对重复文本常超 100:1，只卡下载等于放过一个能把进程打爆的解压炸弹。
	fileDiffMaxDecompressedBytes = 64 << 20
)

// file_diff 子字段名。与 runtime 侧 fileDiffPayload / fileDiff 的 json tag 逐字对齐 ——
// 两侧靠这些字面量对接，改名必须同步改 runtime。
const (
	fileDiffKey                = "file_diff"
	fileDiffKeyDiffFiles       = "diff_files"
	fileDiffKeyDiffLines       = "diff_lines"
	fileDiffKeyFilesOmitted    = "files_omitted"
	fileDiffKeyArchiveURL      = "archive_url"
	fileDiffKeyDiffDetails     = "diff_details"
	fileDiffKeyFileName        = "file_name"
	fileDiffKeyChangedLines    = "changed_lines"
	fileDiffKeyDiffContent     = "diff_content"
	fileDiffKeyTruncated       = "truncated"
	fileDiffKeyTotalLines      = "total_lines"
	fileDiffKeyRoundsFileDiffs = "rounds"
)

// fileDiffFullTextFetcher 取 archive_url 全文的能力，抽成接口只为单测能注入假实现
// （真实现要发 HTTP，测里不能真连网）。
type fileDiffFullTextFetcher interface {
	FetchFullDiff(ctx context.Context, url string) (string, error)
}

// httpFileDiffFetcher 用 HTTP GET 取全文，形状对齐 doFetchInstallScript：
// 带 ctx 超时、LimitReader 多读一字节判截断、状态码分类。
type httpFileDiffFetcher struct {
	client *http.Client
}

func newHTTPFileDiffFetcher() *httpFileDiffFetcher {
	return &httpFileDiffFetcher{client: &http.Client{Timeout: fileDiffFetchTimeout}}
}

// FetchFullDiff 取回 url 指向的完整 unified diff。
//
// archive_url 正常指向 orchestrator 额外存的明文副本（key 以 .diff.gz.txt 结尾），此时直接
// 就是文本。但那次明文 PUT 失败时 orchestrator 会 fallback 到 .diff.gz 本身，所以这里必须
// 能吃压缩包 —— 判据用 **gzip magic bytes**，不看 URL 后缀：后缀是约定，magic 是事实，
// 而这条路径本身就是"约定没兑现"时才走到的。
func (f *httpFileDiffFetcher) FetchFullDiff(ctx context.Context, url string) (string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, fileDiffFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		// 带状态码：403/404 指向 URL 或权限配错，5xx 指向 TOS 抖动，两者处置不同。
		return "", fmt.Errorf("fetch full diff %s: status %d", url, resp.StatusCode)
	}
	// 多读 1 字节，让"撞上限"可被检测：截断的 gzip 流解压会报形似数据损坏的
	// unexpected EOF，把配置问题伪装成数据问题。
	raw, err := io.ReadAll(io.LimitReader(resp.Body, fileDiffMaxDownloadBytes+1))
	if err != nil {
		return "", err
	}
	if len(raw) > fileDiffMaxDownloadBytes {
		return "", fmt.Errorf("full diff %s exceeds %d bytes", url, fileDiffMaxDownloadBytes)
	}
	if isGzipped(raw) {
		return gunzipLimited(raw, fileDiffMaxDecompressedBytes)
	}
	return string(raw), nil
}

// isGzipped 按 magic bytes(1f 8b) 判断是否 gzip 流。
func isGzipped(b []byte) bool {
	return len(b) >= 2 && b[0] == 0x1f && b[1] == 0x8b
}

// gunzipLimited 解压 gz，上限施加在 **输出** 流上 —— 只有那里才知道真实膨胀比。
func gunzipLimited(gz []byte, limit int64) (string, error) {
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return "", err
	}
	defer func() { _ = zr.Close() }()
	out, err := io.ReadAll(io.LimitReader(zr, limit+1))
	if err != nil {
		return "", err
	}
	if int64(len(out)) > limit {
		return "", fmt.Errorf("full diff decompresses past %d bytes", limit)
	}
	return string(out), nil
}

// expandTruncatedFileDiffs 就地把 fields 里被截断的 file_diff 补成全量。
//
// 返回值是一份**新的** fields（浅拷贝 + 只替换被改写的那个 key），不改调用方传入的 map：
// OutputFields 来自 item 结果对象，可能被其它字段的组装逻辑共享。
//
// 任何一步失败都返回原 fields（并留一条 warn 日志）：补全是增强，不是正确性前提。
func expandTruncatedFileDiffs(ctx context.Context, fields map[string]*entity.Content, fetcher fileDiffFullTextFetcher) map[string]*entity.Content {
	if len(fields) == 0 || fetcher == nil {
		return fields
	}
	c, ok := lookupFornaxField(fields, fileDiffKeyRoundsFileDiffs)
	if !ok || !standardFieldContentMergeable(c) {
		// 未报 rounds，或内容本身已被省略/非 JSON —— 后者连解析都做不到，谈不上补全。
		return fields
	}
	var rounds map[string]any
	if err := json.Unmarshal([]byte(c.GetText()), &rounds); err != nil {
		return fields
	}
	changed := false
	for _, rv := range rounds {
		round, okRound := rv.(map[string]any)
		if !okRound {
			continue
		}
		fd, okFD := round[fileDiffKey].(map[string]any)
		if !okFD || !fileDiffTruncated(fd) {
			continue
		}
		url, _ := fd[fileDiffKeyArchiveURL].(string)
		if strings.TrimSpace(url) == "" {
			// TOS 关闭的本地/debug 跑没有 URL，保留截断版。
			continue
		}
		text, err := fetcher.FetchFullDiff(ctx, url)
		if err != nil {
			logs.CtxWarn(ctx, "[standardEvalOutput] fetch full file_diff failed, keep truncated payload, url=%s, err=%v", url, err)
			continue
		}
		if !rewriteFileDiffWithFullText(fd, text) {
			continue
		}
		changed = true
	}
	if !changed {
		return fields
	}
	merged, err := json.Marshal(rounds)
	if err != nil {
		return fields
	}
	return replaceFornaxField(fields, fileDiffKeyRoundsFileDiffs, string(merged))
}

// fileDiffTruncated 判断一个 file_diff 是否被截断过。
//
// 两个信号语义不同、都要看：files_omitted 是"整个文件条目被总字节预算丢掉"，
// diff_details[].truncated 是"某个文件的正文被单文件行数上限剪掉"。只看一个会漏。
func fileDiffTruncated(fd map[string]any) bool {
	if n, ok := toFloat(fd[fileDiffKeyFilesOmitted]); ok && n > 0 {
		return true
	}
	details, ok := fd[fileDiffKeyDiffDetails].([]any)
	if !ok {
		return false
	}
	for _, d := range details {
		entry, okEntry := d.(map[string]any)
		if !okEntry {
			continue
		}
		if t, okT := entry[fileDiffKeyTruncated].(bool); okT && t {
			return true
		}
	}
	return false
}

// rewriteFileDiffWithFullText 用全文重切的结果替换 diff_details，并清掉截断痕迹。
//
// 清 files_omitted 是必须的：补全后再留着 files_omitted=8 会让读者以为仍有 8 个文件没给。
// 同理每个条目的 truncated / total_lines 也要随正文一起消失。
//
// 解析不出任何文件时返回 false，保留原截断版 —— 全文取回来却切不出东西，更可能是我们
// 拿错了内容（HTML 错误页、空文件），拿它覆盖掉本来有效的截断预览是净损失。
func rewriteFileDiffWithFullText(fd map[string]any, text string) bool {
	details := parseUnifiedDiffFull(text)
	if len(details) == 0 {
		return false
	}
	out := make([]any, 0, len(details))
	totalLines := 0
	for _, d := range details {
		totalLines += d.changedLines
		out = append(out, map[string]any{
			fileDiffKeyFileName:     d.fileName,
			fileDiffKeyChangedLines: d.changedLines,
			fileDiffKeyDiffContent:  d.diffContent,
		})
	}
	fd[fileDiffKeyDiffDetails] = out
	fd[fileDiffKeyDiffFiles] = len(details)
	fd[fileDiffKeyDiffLines] = totalLines
	delete(fd, fileDiffKeyFilesOmitted)
	return true
}

// replaceFornaxField 返回 fields 的浅拷贝，其中 field 对应的 Content 换成 text。
// 优先改带 FORNAX_ 前缀的 key（新协议），没有才改裸 key —— 与 lookupFornaxField 的
// 查找顺序保持一致，否则会出现"改了裸 key、读的却是前缀 key"的自相矛盾。
func replaceFornaxField(fields map[string]*entity.Content, field, text string) map[string]*entity.Content {
	key := field
	if _, ok := fields[standardEvalOutputFornaxPrefix+field]; ok {
		key = standardEvalOutputFornaxPrefix + field
	}
	out := make(map[string]*entity.Content, len(fields))
	for k, v := range fields {
		out[k] = v
	}
	out[key] = &entity.Content{
		ContentType: gptr.Of(entity.ContentTypeText),
		Text:        &text,
	}
	return out
}

// toFloat 把 JSON 反序列化出的数字归一成 float64。
// encoding/json 反序列化进 any 时数字恒为 float64，其余分支只为兼容手工构造的测试数据。
func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

// ---- unified diff 解析 ----
//
// 以下逻辑与 fornax_agent_eval_runtime 的 internal/infrastructure/report/filediff.go 同源，
// 因为两侧要切出**可比对**的结果（同一份 diff，orchestrator 切的截断版与平台切的全量版
// 必须 file_name / changed_lines 一致）。
//
// ⚠️ 这里的四处反直觉写法都是线上实测打出来的，别"简化"：
//  1. 不能用 strings.Fields 拆 `diff --git a/x b/x`：git 不转义空格，`my file.txt`
//     会被切成 `file.txt` —— 报出一个 agent 从没碰过的文件名；
//  2. 路径可能是 C-quoted（core.quotePath 默认开），中文名一律形如 "a/\344\270\255.txt"，
//     不 Unquote 就是一串不可读的转义；
//  3. `--- ` 只在 **首个 @@ 之前** 是元信息；hunk 内部它是正文以 `-- ` 开头的合法删除行
//     （Markdown 分隔线、SQL 注释、CLI flag 文档、.patch fixture）。无条件跳会少数;
//  4. `git diff` 输出以换行结尾，直接 Split 会多出一个幻影空行 —— 曾让恰好 1000 行的
//     文件被数成 1001 行而误判为截断。

// parsedFileDiff 是一个文件的解析结果。
type parsedFileDiff struct {
	fileName     string
	changedLines int
	diffContent  string
}

// parseUnifiedDiffFull 把 unified diff 按文件切开。
//
// 以 `diff --git ` 头为界，而非 `--- `/`+++ `：后者会出现在 diff 正文里（当被 diff 的
// 对象本身就是 patch 文件时），而 eval 用例里 .patch fixture 是常态。
func parseUnifiedDiffFull(raw string) []parsedFileDiff {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out []parsedFileDiff
	var cur *strings.Builder
	var curName string
	flush := func() {
		if cur == nil {
			return
		}
		out = append(out, newParsedFileDiff(curName, cur.String()))
		cur, curName = nil, ""
	}
	for _, line := range splitFileDiffLines(raw) {
		if name, ok := fileDiffHeaderName(line); ok {
			flush()
			cur, curName = &strings.Builder{}, name
		}
		if cur == nil {
			continue // 首个文件头之前的前言（例如 commit message）
		}
		cur.WriteString(line)
		cur.WriteString("\n")
	}
	flush()
	return out
}

// fileDiffHeaderName 从 `diff --git a/x b/x` 取文件名，取 **b 侧**（rename 时才是正确答案）。
func fileDiffHeaderName(line string) (string, bool) {
	const prefix = "diff --git "
	if !strings.HasPrefix(line, prefix) {
		return "", false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	a, b, ok := splitFileDiffPaths(rest)
	if !ok {
		// 头畸形。仍然是文件边界 —— 报一个无名文件比把它的 hunk 并进上一个文件更诚实。
		return "", true
	}
	if name := trimFileDiffSidePrefix(b, "b/"); name != "" && name != "/dev/null" {
		return name, true
	}
	return trimFileDiffSidePrefix(a, "a/"), true
}

// splitFileDiffPaths 把 `a/<path> b/<path>` 拆成两半。
// 先处理引号形态：被引起来的路径其转义里可以合法地含有 ` b/`，只有两侧都没引号时
// 才能用"最右一个 ` b/`"这个启发式。
func splitFileDiffPaths(rest string) (a, b string, ok bool) {
	if strings.HasPrefix(rest, `"`) {
		if end := strings.Index(rest[1:], `"`); end >= 0 {
			a = rest[:end+2]
			b = strings.TrimSpace(rest[end+2:])
			return a, b, b != ""
		}
		return "", "", false
	}
	if i := strings.LastIndex(rest, " b/"); i > 0 {
		return rest[:i], rest[i+1:], true
	}
	if f := strings.Fields(rest); len(f) == 2 {
		return f[0], f[1], true
	}
	return "", "", false
}

// trimFileDiffSidePrefix 去掉一侧的 a//b/ 标记，必要时解 C-quoted 转义。
// 解不开的就只去掉引号返回 —— 一个变形的名字仍然比没有名字更能定位文件。
func trimFileDiffSidePrefix(side, marker string) string {
	side = strings.TrimSpace(side)
	if strings.HasPrefix(side, `"`) && strings.HasSuffix(side, `"`) && len(side) > 1 {
		if unquoted, err := strconv.Unquote(side); err == nil {
			side = unquoted
		} else {
			side = strings.Trim(side, `"`)
		}
	}
	return strings.TrimPrefix(side, marker)
}

// newParsedFileDiff 构造一个条目。
// 平台侧**不做**任何截断：补全的全部意义就是把完整正文交出去。
func newParsedFileDiff(name, content string) parsedFileDiff {
	lines := splitFileDiffLines(content)
	return parsedFileDiff{
		fileName:     name,
		changedLines: countFileDiffChangedLines(lines),
		diffContent:  strings.Join(lines, "\n"),
	}
}

// splitFileDiffLines 按行切，丢掉行尾换行留下的那个幻影空元素。
// 只丢 **一个**：正文真的以空行结尾时会有两个，真实的那个要留。
func splitFileDiffLines(content string) []string {
	lines := strings.Split(content, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	return lines
}

// countFileDiffChangedLines 数增删行，排除 diff 自身的头。
// 只在 **首个 @@ 之前** 把 `--- `/`+++ ` 当元信息跳过；hunk 内它们是合法正文。
func countFileDiffChangedLines(lines []string) int {
	n := 0
	inHunk := false
	for _, l := range lines {
		if !inHunk {
			if strings.HasPrefix(l, "@@") {
				inHunk = true
				continue
			}
			if strings.HasPrefix(l, "+++ ") || strings.HasPrefix(l, "--- ") {
				continue
			}
		}
		if strings.HasPrefix(l, "+") || strings.HasPrefix(l, "-") {
			n++
		}
	}
	return n
}
