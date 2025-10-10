// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package templates

// JavaScriptTemplate JavaScript代码执行模板
const JavaScriptTemplate = `
/**
 * JavaScript 用户代码模板
 */

{{RETURN_VAL_FUNCTION}}

/**
 * 评估输出数据结构
 */
class EvalOutput {
    constructor(score, reason) {
        this.score = score;
        this.reason = reason;
    }
}

// 测试数据 (动态替换)
const turn = {{TURN_DATA}};

{{EXEC_EVALUATION_FUNCTION}}

/**
 * 主函数 - 执行评估并返回EvalOutput
 * @returns {EvalOutput} 评估结果
 */
function main() {
    // 执行评估，返回EvalOutput类型
    const result = exec_evaluation(turn);
    
    return result;
}

// 执行主函数并处理结果
(function() {
    let result = null;
    try {
        result = main();
    } catch (error) {
        console.error(error.constructor.name + ": " + error.message);
        process.exit(1);
    }
    
    // 输出最终结果
    return_val(JSON.stringify(result));
})();
`

// JavaScriptSyntaxCheckTemplate JavaScript语法检查模板
const JavaScriptSyntaxCheckTemplate = `
{{RETURN_VAL_FUNCTION}}

// JavaScript语法检查
const userCode = ` + "`" + `{{USER_CODE}}` + "`" + `;

function parseJSError(error, code) {
    const errorInfo = {
        message: error.message,
        line: null,
        column: null,
        full_message: "语法错误: " + error.message
    };
    
    // 尝试从错误信息中解析行号（不同JS引擎格式可能不同）
    const lineMatch = error.message.match(/line (\d+)/i) || 
                     error.message.match(/行 (\d+)/) ||
                     error.message.match(/(\d+):/);
    
    if (lineMatch) {
        errorInfo.line = parseInt(lineMatch[1]);
        errorInfo.full_message += " (行号: " + errorInfo.line + ")";
    }
    
    // 尝试从错误信息中解析列号
    const colMatch = error.message.match(/column (\d+)/i) || 
                    error.message.match(/列 (\d+)/) ||
                    error.message.match(/:(\d+)$/);
    
    if (colMatch) {
        errorInfo.column = parseInt(colMatch[1]);
        if (errorInfo.line) {
            errorInfo.full_message = errorInfo.full_message.replace(")", ", 列号: " + errorInfo.column + ")");
        } else {
            errorInfo.full_message += " (列号: " + errorInfo.column + ")";
        }
    }
    
    return errorInfo;
}

try {
    // 使用Function构造函数进行语法检查
    new Function(userCode);
    
    // 语法正确，输出JSON结果
    const result = {"valid": true, "error": null};
    return_val(JSON.stringify(result));
} catch (error) {
    // 捕获语法错误，解析并输出详细的错误信息
    const errorDetail = parseJSError(error, userCode);
    const result = {"valid": false, "error": errorDetail};
    return_val(JSON.stringify(result));
}
`
