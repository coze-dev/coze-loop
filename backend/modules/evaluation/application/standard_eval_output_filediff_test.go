// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

// fakeFileDiffFetcher 记录调用次数，让「没截断时一个字节都不拉」成为可断言的事实
// 而不是只靠读代码相信。
type fakeFileDiffFetcher struct {
	calls []string
	text  string
	err   error
}

func (f *fakeFileDiffFetcher) FetchFullDiff(_ context.Context, url string) (string, error) {
	f.calls = append(f.calls, url)
	return f.text, f.err
}

// textContent 复用 standard_eval_output_strip_test.go 里的同名 helper。

// roundsWithFileDiff 造一份 FORNAX_output payload，形状对齐 runtime buildOutput 的实际产出：
// file_diff 在 **output** 内部的 rounds map 里，不在顶层 FORNAX_rounds（那是 context/token 数组）。
func roundsWithFileDiff(t *testing.T, fd map[string]any) map[string]*entity.Content {
	t.Helper()
	payload := map[string]any{
		"detail": map[string]any{
			"output": map[string]any{"actual_output": map[string]any{"text": "done"}},
		},
		"rounds": map[string]any{
			"run_1_round_1": map[string]any{
				"file_diff": fd,
				"output":    map[string]any{"actual_output": map[string]any{"text": "done"}},
			},
		},
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	return map[string]*entity.Content{
		standardEvalOutputFornaxPrefix + "output": textContent(string(raw)),
	}
}

// fileDiffOfFields 从展开后的 fields 里取回 file_diff，供断言。
func fileDiffOfFields(t *testing.T, fields map[string]*entity.Content) map[string]any {
	t.Helper()
	c, ok := lookupFornaxField(fields, "output")
	require.True(t, ok)
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(c.GetText()), &payload))
	rounds, ok := payload["rounds"].(map[string]any)
	require.True(t, ok)
	round, ok := rounds["run_1_round_1"].(map[string]any)
	require.True(t, ok)
	fd, ok := round["file_diff"].(map[string]any)
	require.True(t, ok)
	return fd
}

const twoFileDiff = `diff --git a/middleware/slash.go b/middleware/slash.go
index 0492b33..72ea2b9 100644
--- a/middleware/slash.go
+++ b/middleware/slash.go
@@ -60,7 +60,7 @@ func AddTrailingSlashWithConfig(config TrailingSlashConfig) echo.MiddlewareFunc
 		// Redirect
 		if config.RedirectCode != 0 {
-			return c.Redirect(config.RedirectCode, uri)
+			return c.Redirect(config.RedirectCode, path)
 		}
diff --git a/middleware/slash_test.go b/middleware/slash_test.go
index 2a8e9ee..f159745 100644
--- a/middleware/slash_test.go
+++ b/middleware/slash_test.go
@@ -47,6 +47,25 @@ func TestAddTrailingSlash(t *testing.T) {
 	}
+func TestNew(t *testing.T) {
+	is := assert.New(t)
+}
`

// 没有截断信号时绝不发请求。这是整个特性的性能前提：MGet 一次最多 100 个 item，
// 无条件拉会变成 100 次串行 HTTP。
func TestExpandTruncatedFileDiffs_NoTruncation_DoesNotFetch(t *testing.T) {
	t.Parallel()

	fields := roundsWithFileDiff(t, map[string]any{
		"diff_files":  2,
		"diff_lines":  51,
		"archive_url": "https://tos.example/round_1_git_diff.diff.gz.txt",
		"diff_details": []any{
			map[string]any{"file_name": "a.go", "changed_lines": 13, "diff_content": "..."},
		},
	})
	f := &fakeFileDiffFetcher{text: twoFileDiff}

	got := expandTruncatedFileDiffs(context.Background(), fields, f)

	assert.Empty(t, f.calls, "没有截断信号却发起了取全文请求")
	assert.Equal(t, fields, got, "未截断时应原样返回同一份 fields")
}

// files_omitted > 0：整个文件条目被 64KB 总预算丢掉。
func TestExpandTruncatedFileDiffs_FilesOmitted_Expands(t *testing.T) {
	t.Parallel()

	fields := roundsWithFileDiff(t, map[string]any{
		"diff_files":    2,
		"diff_lines":    51,
		"files_omitted": 1,
		"archive_url":   "https://tos.example/round_1_git_diff.diff.gz.txt",
		"diff_details": []any{
			map[string]any{"file_name": "middleware/slash.go", "changed_lines": 13, "diff_content": "clipped"},
		},
	})
	f := &fakeFileDiffFetcher{text: twoFileDiff}

	got := expandTruncatedFileDiffs(context.Background(), fields, f)

	require.Len(t, f.calls, 1)
	fd := fileDiffOfFields(t, got)
	details, ok := fd["diff_details"].([]any)
	require.True(t, ok)
	assert.Len(t, details, 2, "应补成全量两个文件")
	// 补全后必须清掉 files_omitted：留着会让读者以为仍有文件没给。
	assert.NotContains(t, fd, "files_omitted")
	assert.EqualValues(t, 2, fd["diff_files"])
}

// diff_details[].truncated：某个文件正文被单文件 1000 行上限剪掉。
func TestExpandTruncatedFileDiffs_EntryTruncated_Expands(t *testing.T) {
	t.Parallel()

	fields := roundsWithFileDiff(t, map[string]any{
		"archive_url": "https://tos.example/round_1_git_diff.diff.gz.txt",
		"diff_details": []any{
			map[string]any{
				"file_name": "middleware/slash.go", "changed_lines": 13,
				"diff_content": "clipped", "truncated": true, "total_lines": 4200,
			},
		},
	})
	f := &fakeFileDiffFetcher{text: twoFileDiff}

	got := expandTruncatedFileDiffs(context.Background(), fields, f)

	require.Len(t, f.calls, 1)
	fd := fileDiffOfFields(t, got)
	details, ok := fd["diff_details"].([]any)
	require.True(t, ok)
	require.Len(t, details, 2)
	// 全量条目不应再带截断痕迹。
	for _, d := range details {
		entry := d.(map[string]any)
		assert.NotContains(t, entry, "truncated")
		assert.NotContains(t, entry, "total_lines")
	}
}

// archive_url 为空（TOS 关闭的本地/debug 跑）：保留截断版，不尝试、不报错。
func TestExpandTruncatedFileDiffs_NoArchiveURL_KeepsTruncated(t *testing.T) {
	t.Parallel()

	fields := roundsWithFileDiff(t, map[string]any{
		"files_omitted": 3,
		"diff_details":  []any{map[string]any{"file_name": "a.go", "diff_content": "clipped"}},
	})
	f := &fakeFileDiffFetcher{text: twoFileDiff}

	got := expandTruncatedFileDiffs(context.Background(), fields, f)

	assert.Empty(t, f.calls)
	assert.Equal(t, fields, got)
}

// 取全文失败：降级保留截断版。这是"增强不是依赖"——TOS 抖动不能让读接口失败。
func TestExpandTruncatedFileDiffs_FetchFails_KeepsTruncated(t *testing.T) {
	t.Parallel()

	fields := roundsWithFileDiff(t, map[string]any{
		"files_omitted": 3,
		"archive_url":   "https://tos.example/x.diff.gz.txt",
		"diff_details":  []any{map[string]any{"file_name": "a.go", "diff_content": "clipped"}},
	})
	f := &fakeFileDiffFetcher{err: errors.New("tos 503")}

	got := expandTruncatedFileDiffs(context.Background(), fields, f)

	require.Len(t, f.calls, 1)
	fd := fileDiffOfFields(t, got)
	assert.EqualValues(t, 3, fd["files_omitted"], "失败时截断信号必须保留")
	details := fd["diff_details"].([]any)
	assert.Len(t, details, 1)
}

// 全文取回来却切不出任何文件（HTML 错误页 / 空内容）：保留截断版，
// 拿它覆盖掉本来有效的预览是净损失。
func TestExpandTruncatedFileDiffs_UnparsableFullText_KeepsTruncated(t *testing.T) {
	t.Parallel()

	fields := roundsWithFileDiff(t, map[string]any{
		"files_omitted": 3,
		"archive_url":   "https://tos.example/x.diff.gz.txt",
		"diff_details":  []any{map[string]any{"file_name": "a.go", "diff_content": "clipped"}},
	})
	f := &fakeFileDiffFetcher{text: "<html>403 Forbidden</html>"}

	got := expandTruncatedFileDiffs(context.Background(), fields, f)

	fd := fileDiffOfFields(t, got)
	assert.EqualValues(t, 3, fd["files_omitted"])
}

// fetcher 为 nil（未注入）时整个特性静默关闭，不影响既有输出。
func TestExpandTruncatedFileDiffs_NilFetcher_NoOp(t *testing.T) {
	t.Parallel()

	fields := roundsWithFileDiff(t, map[string]any{
		"files_omitted": 3,
		"archive_url":   "https://tos.example/x.diff.gz.txt",
	})

	assert.Equal(t, fields, expandTruncatedFileDiffs(context.Background(), fields, nil))
}

// 回归：file_diff 挂在 FORNAX_output.rounds（map）里，而 FORNAX_rounds 是一个
// context/token 数组、压根没有 file_diff。曾把宿主字段写成 rounds，导致永远命中不到、
// 静默不补全 —— 线上实测才发现。这里同时喂两个字段，锁住"只认 output、不被 rounds 干扰"。
func TestExpandTruncatedFileDiffs_HostFieldIsOutputNotRounds(t *testing.T) {
	t.Parallel()

	fields := roundsWithFileDiff(t, map[string]any{
		"files_omitted": 2,
		"archive_url":   "https://tos.example/round_1_git_diff.diff.gz.txt",
		"diff_details": []any{
			map[string]any{"file_name": "src/App.jsx", "changed_lines": 8, "diff_content": "clipped"},
		},
	})
	// 真实的 FORNAX_rounds：数组，每项是一轮的 context/token，没有 file_diff。
	roundsArr, err := json.Marshal([]any{
		map[string]any{
			"round_id": "run_1_round_1", "round_no": 1,
			"context": map[string]any{"trace_id": "6a78c653", "log_id": "0217862999"},
			"tokens":  map[string]any{"total_tokens": 252878},
		},
	})
	require.NoError(t, err)
	fields[standardEvalOutputFornaxPrefix+"rounds"] = textContent(string(roundsArr))

	f := &fakeFileDiffFetcher{text: twoFileDiff}
	got := expandTruncatedFileDiffs(context.Background(), fields, f)

	require.Len(t, f.calls, 1, "应从 FORNAX_output.rounds 里找到 file_diff 并取全文")
	fd := fileDiffOfFields(t, got)
	assert.Len(t, fd["diff_details"].([]any), 2)
	assert.NotContains(t, fd, "files_omitted")
	// FORNAX_rounds 原样不动。
	assert.Equal(t, string(roundsArr), got[standardEvalOutputFornaxPrefix+"rounds"].GetText())
}

// 回归：output 里其它子字段（detail / manifest 等）必须原样保留 ——
// 补全时回写的是整包 output，漏拷会静默丢掉 detail。
func TestExpandTruncatedFileDiffs_PreservesSiblingsInOutput(t *testing.T) {
	t.Parallel()

	fields := roundsWithFileDiff(t, map[string]any{
		"files_omitted": 2,
		"archive_url":   "https://tos.example/x.diff.gz.txt",
		"diff_details":  []any{map[string]any{"file_name": "src/App.jsx", "diff_content": "clipped"}},
	})
	f := &fakeFileDiffFetcher{text: twoFileDiff}

	got := expandTruncatedFileDiffs(context.Background(), fields, f)

	c, ok := lookupFornaxField(got, "output")
	require.True(t, ok)
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(c.GetText()), &payload))
	detail, ok := payload["detail"].(map[string]any)
	require.True(t, ok, "detail 必须还在")
	assert.Contains(t, detail, "output")
}

// ---- HTTP fetcher ----

// archive_url 正常指向 orchestrator 存的明文副本：直接当文本读。
func TestHTTPFileDiffFetcher_PlainText(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(twoFileDiff))
	}))
	defer srv.Close()

	got, err := newHTTPFileDiffFetcher().FetchFullDiff(context.Background(), srv.URL)
	require.NoError(t, err)
	assert.Equal(t, twoFileDiff, got)
}

// 明文 PUT 失败时 orchestrator 会 fallback 到 .diff.gz 本身，所以必须能吃压缩流。
// 判据是 magic bytes 而非 URL 后缀 —— 这条路径本身就是"约定没兑现"才走到的。
func TestHTTPFileDiffFetcher_GzipFallback(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write([]byte(twoFileDiff))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(buf.Bytes())
	}))
	defer srv.Close()

	// URL 故意不带 .gz 后缀，证明判据用的是 magic bytes。
	got, err := newHTTPFileDiffFetcher().FetchFullDiff(context.Background(), srv.URL+"/looks-like-text.txt")
	require.NoError(t, err)
	assert.Equal(t, twoFileDiff, got)
}

func TestHTTPFileDiffFetcher_NonOKStatus(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := newHTTPFileDiffFetcher().FetchFullDiff(context.Background(), srv.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403", "错误里要带状态码：403 指向权限/URL 配错，5xx 指向 TOS 抖动")
}

// ---- parser：以下四条对应 runtime 侧实测打出来的四个 bug，重写必复现 ----

// git 不转义路径里的空格，用 strings.Fields 会把 `my file.txt` 切成 `file.txt`,
// 报出一个 agent 从没碰过的文件名。
func TestParseUnifiedDiffFull_PathWithSpace(t *testing.T) {
	t.Parallel()

	raw := "diff --git a/my file.txt b/my file.txt\n" +
		"--- a/my file.txt\n+++ b/my file.txt\n@@ -1 +1 @@\n-old\n+new\n"
	got := parseUnifiedDiffFull(raw)

	require.Len(t, got, 1)
	assert.Equal(t, "my file.txt", got[0].fileName)
}

// core.quotePath 默认开，非 ASCII 路径一律 C-quoted，不 Unquote 就是一串转义。
func TestParseUnifiedDiffFull_CQuotedPath(t *testing.T) {
	t.Parallel()

	raw := "diff --git \"a/\\344\\270\\255.txt\" \"b/\\344\\270\\255.txt\"\n" +
		"@@ -1 +1 @@\n-a\n+b\n"
	got := parseUnifiedDiffFull(raw)

	require.Len(t, got, 1)
	assert.Equal(t, "中.txt", got[0].fileName)
}

// `--- ` 只在首个 @@ 之前是元信息；hunk 内部它是正文以 `-- ` 开头的合法删除行
// （Markdown 分隔线、SQL 注释、CLI flag 文档）。无条件跳会少数。
func TestCountFileDiffChangedLines_DashDashInsideHunkCounts(t *testing.T) {
	t.Parallel()

	raw := "diff --git a/q.sql b/q.sql\n" +
		"--- a/q.sql\n+++ b/q.sql\n@@ -1,4 +1,2 @@\n" +
		"-- comment one\n-- comment two\n+SELECT 1;\n"
	got := parseUnifiedDiffFull(raw)

	require.Len(t, got, 1)
	// 两条 `-- comment` 删除行 + 一条 + 行 = 3；头部的 ---/+++ 不计。
	assert.Equal(t, 3, got[0].changedLines)
}

// git diff 输出以换行结尾，直接 Split 会多出幻影空行 —— 曾让恰好 1000 行的文件
// 被数成 1001 行而误判为截断。
func TestSplitFileDiffLines_DropsOnlyOnePhantomLine(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"a", "b"}, splitFileDiffLines("a\nb\n"))
	// 正文真以空行结尾时有两个空元素，真实的那个要留。
	assert.Equal(t, []string{"a", "b", ""}, splitFileDiffLines("a\nb\n\n"))
}

// 以 `diff --git` 为界而非 `--- `/`+++ `：被 diff 的对象本身是 patch 文件时后者会出现在正文里，
// 而 eval 用例里 .patch fixture 是常态。
func TestParseUnifiedDiffFull_PatchFixtureNotSplitByTripleDash(t *testing.T) {
	t.Parallel()

	raw := "diff --git a/fix.patch b/fix.patch\n" +
		"--- a/fix.patch\n+++ b/fix.patch\n@@ -1,3 +1,3 @@\n" +
		"+--- a/inner.go\n++++ b/inner.go\n"
	got := parseUnifiedDiffFull(raw)

	require.Len(t, got, 1, "patch fixture 内部的 ---/+++ 不能被当成文件边界")
	assert.Equal(t, "fix.patch", got[0].fileName)
}

func TestParseUnifiedDiffFull_MultiFileAndEmpty(t *testing.T) {
	t.Parallel()

	got := parseUnifiedDiffFull(twoFileDiff)
	require.Len(t, got, 2)
	assert.Equal(t, "middleware/slash.go", got[0].fileName)
	assert.Equal(t, "middleware/slash_test.go", got[1].fileName)
	assert.True(t, strings.HasPrefix(got[0].diffContent, "diff --git "))

	assert.Nil(t, parseUnifiedDiffFull(""))
	assert.Nil(t, parseUnifiedDiffFull("   \n  "))
}

// rename 取 b 侧（post-image）才是正确答案。
func TestParseUnifiedDiffFull_RenameTakesBSide(t *testing.T) {
	t.Parallel()

	raw := "diff --git a/old.go b/new.go\nsimilarity index 90%\n@@ -1 +1 @@\n-x\n+y\n"
	got := parseUnifiedDiffFull(raw)

	require.Len(t, got, 1)
	assert.Equal(t, "new.go", got[0].fileName)
}

func TestIsGzipped(t *testing.T) {
	t.Parallel()

	assert.True(t, isGzipped([]byte{0x1f, 0x8b, 0x08}))
	assert.False(t, isGzipped([]byte("diff --git")))
	assert.False(t, isGzipped([]byte{0x1f}))
	assert.False(t, isGzipped(nil))
}
