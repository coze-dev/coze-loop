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

try {
    // 使用Function构造函数进行语法检查
    new Function(userCode);
    
    // 语法正确，输出JSON结果
    const result = {"valid": true, "error": null};
    return_val(JSON.stringify(result));
} catch (error) {
    // 捕获语法错误，输出JSON结果
    const result = {"valid": false, "error": "语法错误: " + error.message};
    return_val(JSON.stringify(result));
}
`