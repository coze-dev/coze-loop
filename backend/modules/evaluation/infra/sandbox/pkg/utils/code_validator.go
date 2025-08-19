package utils

import (
	"fmt"
	"regexp"
	"strings"

	"code.byted.org/flowdevops/cozeloop/backend/modules/evaluation/sandbox/pkg/errors"
)

// CodeValidator 代码验证器
type CodeValidator struct {
	blacklist []string
	patterns  []*regexp.Regexp
}

// NewCodeValidator 创建代码验证器
func NewCodeValidator() *CodeValidator {
	return &CodeValidator{
		blacklist: []string{},
		patterns:  []*regexp.Regexp{},
	}
}

// Validate 验证代码安全性
func (v *CodeValidator) Validate(code, language string) error {
	if strings.TrimSpace(code) == "" {
		return errors.NewInvalidCodeError("代码不能为空")
	}

	// 检查危险函数调用
	if err := v.checkDangerousFunctions(code, language); err != nil {
		return err
	}

	// 检查危险模块导入
	if err := v.checkDangerousImports(code, language); err != nil {
		return err
	}

	// 检查恶意模式
	if err := v.checkMaliciousPatterns(code, language); err != nil {
		return err
	}

	return nil
}

// checkDangerousFunctions 检查危险函数调用
func (v *CodeValidator) checkDangerousFunctions(code, language string) error {
	dangerousFunctions := map[string][]string{
		"javascript": {"eval", "Function", "setTimeout", "setInterval", "XMLHttpRequest", "fetch"},
		"typescript": {"eval", "Function", "setTimeout", "setInterval", "XMLHttpRequest", "fetch"},
		"python":     {"exec", "eval", "__import__", "open", "input", "compile", "globals", "locals"},
	}

	normalizedLang := strings.ToLower(strings.TrimSpace(language))
	functions, exists := dangerousFunctions[normalizedLang]
	if !exists {
		return nil
	}

	for _, fn := range functions {
		// 创建正则表达式匹配函数调用
		pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(fn) + `\s*\(`)
		if pattern.MatchString(code) {
			return errors.NewSecurityViolationError(
				"检测到危险函数调用: " + fn,
			)
		}
	}

	return nil
}

// checkDangerousImports 检查危险模块导入
func (v *CodeValidator) checkDangerousImports(code, language string) error {
	dangerousImports := map[string][]string{
		"javascript": {"fs", "child_process", "os", "path", "net", "http", "https"},
		"typescript": {"fs", "child_process", "os", "path", "net", "http", "https"},
		"python":     {"os", "sys", "subprocess", "socket", "urllib", "requests", "__builtin__", "builtins"},
	}

	normalizedLang := strings.ToLower(strings.TrimSpace(language))
	imports, exists := dangerousImports[normalizedLang]
	if !exists {
		return nil
	}

	for _, imp := range imports {
		var patterns []string

		switch normalizedLang {
		case "python":
			patterns = []string{
				`import\s+` + regexp.QuoteMeta(imp),
				`from\s+` + regexp.QuoteMeta(imp) + `\s+import`,
				`__import__\s*\(\s*['"` + regexp.QuoteMeta(imp) + `'"]`,
			}
		case "javascript", "typescript":
			patterns = []string{
				`import\s+.*from\s+['"]` + regexp.QuoteMeta(imp) + `['"]`,
				`require\s*\(\s*['"]` + regexp.QuoteMeta(imp) + `['"]`,
			}
		}

		for _, pattern := range patterns {
			regex := regexp.MustCompile(pattern)
			if regex.MatchString(code) {
				return errors.NewSecurityViolationError(
					"检测到危险模块导入: " + imp,
				)
			}
		}
	}

	return nil
}

// checkMaliciousPatterns 检查恶意模式
func (v *CodeValidator) checkMaliciousPatterns(code, language string) error {
	// 通用恶意模式
	maliciousPatterns := []string{
		`while\s+True\s*:`,       // Python 无限循环
		`while\s*\(\s*true\s*\)`, // JS 无限循环
		`for\s*\(\s*;\s*;\s*\)`,  // JS 无限循环
		`setInterval\s*\(`,       // JS 定时器
		`setTimeout\s*\(`,        // JS 定时器
		`process\.exit`,          // 进程退出
		`System\.exit`,           // 系统退出
		`exit\s*\(`,              // 退出函数
		`quit\s*\(`,              // 退出函数
	}

	for _, pattern := range maliciousPatterns {
		regex := regexp.MustCompile(pattern)
		if regex.MatchString(code) {
			return errors.NewSecurityViolationError(
				"检测到潜在恶意代码模式",
			)
		}
	}

	return nil
}

// ValidateCodeLength 验证代码长度
func (v *CodeValidator) ValidateCodeLength(code string, maxLength int) error {
	if len(code) > maxLength {
		return errors.NewInvalidCodeError(
			"代码长度超过限制",
		)
	}
	return nil
}

// SanitizeCode 清理代码
func (v *CodeValidator) SanitizeCode(code string) string {
	// 移除潜在的危险注释
	code = regexp.MustCompile(`#.*`).ReplaceAllString(code, "")
	code = regexp.MustCompile(`//.*`).ReplaceAllString(code, "")
	code = regexp.MustCompile(`/\*.*?\*/`).ReplaceAllString(code, "")

	// 移除多余的空白字符
	code = strings.TrimSpace(code)

	return code
}

// ValidateCompilation 验证代码编译（仅语法检查，不执行）
func (v *CodeValidator) ValidateCompilation(code, language string) error {
	// 基础验证
	if err := v.Validate(code, language); err != nil {
		return err
	}
	
	// 基础语法检查
	if err := v.checkBasicSyntax(code, language); err != nil {
		return err
	}
	
	return nil
}

// checkBasicSyntax 检查基础语法
func (v *CodeValidator) checkBasicSyntax(code, language string) error {
	normalizedLang := strings.ToLower(strings.TrimSpace(language))
	
	switch normalizedLang {
	case "python", "py":
		return v.checkPythonBasicSyntax(code)
	case "javascript", "js", "typescript", "ts":
		return v.checkJavaScriptBasicSyntax(code)
	default:
		return nil // 不支持的语言跳过基础语法检查
	}
}

// checkPythonBasicSyntax 检查Python基础语法
func (v *CodeValidator) checkPythonBasicSyntax(code string) error {
	// 检查基础的括号匹配
	if !v.checkBracketMatching(code, []rune{'(', ')'}) {
		return errors.NewInvalidCodeError("Python代码括号不匹配")
	}
	if !v.checkBracketMatching(code, []rune{'[', ']'}) {
		return errors.NewInvalidCodeError("Python代码方括号不匹配")
	}
	if !v.checkBracketMatching(code, []rune{'{', '}'}) {
		return errors.NewInvalidCodeError("Python代码花括号不匹配")
	}
	
	// 检查缩进一致性（简单检查）
	if err := v.checkPythonIndentation(code); err != nil {
		return err
	}
	
	return nil
}

// checkJavaScriptBasicSyntax 检查JavaScript基础语法
func (v *CodeValidator) checkJavaScriptBasicSyntax(code string) error {
	// 检查基础的括号匹配
	if !v.checkBracketMatching(code, []rune{'(', ')'}) {
		return errors.NewInvalidCodeError("JavaScript代码括号不匹配")
	}
	if !v.checkBracketMatching(code, []rune{'[', ']'}) {
		return errors.NewInvalidCodeError("JavaScript代码方括号不匹配")
	}
	if !v.checkBracketMatching(code, []rune{'{', '}'}) {
		return errors.NewInvalidCodeError("JavaScript代码花括号不匹配")
	}
	
	return nil
}

// checkBracketMatching 检查括号匹配
func (v *CodeValidator) checkBracketMatching(code string, brackets []rune) bool {
	if len(brackets) != 2 {
		return true
	}
	
	open, close := brackets[0], brackets[1]
	stack := 0
	inString := false
	var stringChar rune
	
	for i, char := range code {
		// 处理字符串
		if char == '"' || char == '\'' || char == '`' {
			if !inString {
				inString = true
				stringChar = char
			} else if char == stringChar {
				// 检查是否是转义字符
				if i > 0 && rune(code[i-1]) != '\\' {
					inString = false
				}
			}
			continue
		}
		
		if inString {
			continue
		}
		
		if char == open {
			stack++
		} else if char == close {
			stack--
			if stack < 0 {
				return false
			}
		}
	}
	
	return stack == 0
}

// checkPythonIndentation 检查Python缩进
func (v *CodeValidator) checkPythonIndentation(code string) error {
	lines := strings.Split(code, "\n")
	indentStack := []int{0} // 缩进栈
	expectIndent := false   // 是否期待下一行有缩进
	
	for lineNum, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue // 跳过空行和注释行
		}
		
		// 计算缩进级别
		indent := 0
		for _, char := range line {
			if char == ' ' {
				indent++
			} else if char == '\t' {
				indent += 4 // 制表符按4个空格计算
			} else {
				break
			}
		}
		
		// 检查缩进逻辑
		if expectIndent {
			// 上一行以冒号结尾，期待缩进增加
			if indent <= indentStack[len(indentStack)-1] {
				return errors.NewInvalidCodeError(
					fmt.Sprintf("第%d行缩进错误：期待缩进增加", lineNum+1),
				)
			}
			indentStack = append(indentStack, indent)
			expectIndent = false
		} else {
			// 检查当前缩进是否合法
			validIndent := false
			for i := len(indentStack) - 1; i >= 0; i-- {
				if indent == indentStack[i] {
					validIndent = true
					// 弹出更深的缩进级别
					indentStack = indentStack[:i+1]
					break
				}
			}
			
			if !validIndent {
				return errors.NewInvalidCodeError(
					fmt.Sprintf("第%d行缩进错误：缩进级别不匹配", lineNum+1),
				)
			}
		}
		
		// 检查这一行是否以冒号结尾
		if strings.HasSuffix(trimmed, ":") {
			expectIndent = true
		}
	}
	
	return nil
}