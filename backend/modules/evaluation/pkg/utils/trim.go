package utils

import (
	"strings"
)

// countJsonArrayElements 计算 json array 中包含的元素数量
// notice: 出于性能考虑，直接对字符串计算元素数量. 当 array string 非法时会直接返回 0
func countJsonArrayElements(jsonBytes []byte) int {
	jsonStr := strings.TrimSpace(string(jsonBytes))
	if len(strings.TrimSpace(jsonStr)) <= 2 || !strings.HasPrefix(jsonStr, "[") || !strings.HasSuffix(jsonStr, "]") {
		return 0
	}
	var (
		count, bracketCount = 1, 0
		inQuotes, escaped   = false, false
	)
	for i := 1; i < len(jsonStr)-1; /*跳过首尾的括号*/ i++ {
		if escaped {
			escaped = false
			continue
		}
		switch jsonStr[i] {
		case '\\':
			escaped = true
		case '"':
			if !escaped {
				inQuotes = !inQuotes
			}
		case '{', '[':
			if !inQuotes {
				bracketCount++
			}
		case '}', ']':
			if !inQuotes {
				bracketCount--
			}
		case ',':
			if !inQuotes && bracketCount == 0 {
				count++
			}
		default:
		}
		// 处理 Unicode 转义序列
		if jsonStr[i] == '\\' && i+1 < len(jsonStr) && jsonStr[i+1] == 'u' {
			i += 5 // 跳过 \uXXXX
		}
	}
	return count
}

// GenerateJsonObjectPreview 生成 json object 的预览内容
// notice: 出于性能考虑，直接根据字符串生成.
// * 对于字符串、数值等简单类型，仅保留前五位作为预览内容
// * 对于object、array 等复杂类型，不递归解析，直接展示为 "{...}" 或 "[...]"
func GenerateJsonObjectPreview(jsonBytes []byte) string {
	jsonStr := strings.TrimSpace(string(jsonBytes))
	if len(jsonStr) < 2 || jsonStr[0] != '{' || jsonStr[len(jsonStr)-1] != '}' {
		return ""
	}
	var (
		preview           strings.Builder
		inQuotes, escaped = false, false
		depth, valueStart = 0, -1
		keyStart, keyEnd  = 1, -1
	)
	preview.WriteString("{")
	for i := 1; i < len(jsonStr)-1; /*跳过首尾的括号*/ i++ {
		char := jsonStr[i]
		if escaped {
			escaped = false
			continue
		}
		switch char {
		case '\\':
			escaped = true
		case '"':
			inQuotes = !inQuotes
			if !inQuotes && keyEnd == -1 {
				keyEnd = i
			}
		case ':':
			if !inQuotes && keyEnd != -1 && valueStart == -1 {
				valueStart = i + 1
			}
		case '{', '[':
			if !inQuotes {
				depth++
			}
		case '}', ']':
			if !inQuotes {
				depth--
			}
		case ',':
			if inQuotes || depth != 0 {
				continue
			}
			if keyEnd != -1 && valueStart != -1 {
				key := strings.TrimSpace(jsonStr[keyStart : keyEnd+1])
				value := strings.TrimSpace(jsonStr[valueStart:i])
				preview.WriteString(key + ": " + summarizeValue(value) + ", ")
			}
			keyStart = i + 1
			keyEnd = -1
			valueStart = -1
		default:
		}
	}
	if keyEnd != -1 && valueStart != -1 {
		key := strings.TrimSpace(jsonStr[keyStart : keyEnd+1])
		value := strings.TrimSpace(jsonStr[valueStart : len(jsonStr)-1])
		preview.WriteString(key + ": " + summarizeValue(value))
	}
	preview.WriteString("}")
	return preview.String()
}
func summarizeValue(value string) string {
	if len(value) == 0 {
		return "..."
	}
	switch value[0] {
	case '"':
		if len(value) > 5 {
			return `"..."`
		}
		return value
	case '{':
		return `"{...}"` // 缩略内容以 string 展示，不影响 json 格式渲染
	case '[':
		return `"[...]"`
	default:
		if len(value) > 5 {
			return value[:5] + "..."
		}
		return value
	}
}

// generateTextPreview 生成文本类型的预览内容
func generateTextPreview(content []byte) string {
	const previewContentLength = 100
	runes := []rune(string(content))
	if len(runes) <= previewContentLength {
		return string(content)
	}
	return string(runes[:previewContentLength]) + "..."
}
