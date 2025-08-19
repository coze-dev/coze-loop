// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package templates

// JavaScriptTemplate JavaScript代码执行模板
const JavaScriptTemplate = `
/**
 * JavaScript 用户代码模板
 * 参考 user_code_template.py 实现，保持相同的评估逻辑和输出格式
 */

// 参数和输出类 (与 Python 版本保持一致)
class Args {
    constructor(params) {
        this.params = params;
    }
}

class Output extends Object {}

/**
 * 评估输出数据结构
 */
class EvalOutput {
    constructor(score, reason, err_msg = '') {
        this.score = score;
        this.reason = reason;
        this.err_msg = err_msg;
    }
}

// 全局参数
let args = {};

// 测试数据 (动态替换)
const turn = {{TURN_DATA}};

{{EXEC_EVALUATION_FUNCTION}}

/**
 * 主函数 - 异步执行评估
 * @param {Args} args - 参数对象
 * @returns {Object} 评估结果
 */
async function main(args) {
    /**
     * JavaScript 版本的用户代码模板
     * 适配 js_sandbox.js 执行格式
     */

    // 执行评估
    const result = exec_evaluation(turn);

    // 打印结果 (与 Python 版本相同的输出格式)
    console.log("Evaluation Results:");
    console.log("Score: " + result.score);
    console.log("Reason: " + result.reason);
    if (result.err_msg) {
        console.log("Error: " + result.err_msg);
    }

    // 返回结果供沙箱使用 - 转换为普通对象以便 JSON 序列化
    return {
        "score": result.score,
        "reason": result.reason,
        "err_msg": result.err_msg
    };
}

// 执行主函数并处理结果 (与 Python 版本的 suffix 逻辑相同)
let result = null;
try {
    result = await main(new Args(args));
} catch (error) {
    console.error(error.constructor.name + ": " + error.message);
    Deno.exit(1);
}

// 输出最终结果
result;
`

// JavaScriptSyntaxCheckTemplate JavaScript语法检查模板
const JavaScriptSyntaxCheckTemplate = `
// JavaScript语法检查
const userCode = ` + "`" + `{{USER_CODE}}` + "`" + `;

try {
    // 使用Function构造函数进行语法检查
    new Function(userCode);
    
    // 语法正确，输出JSON结果
    const result = {"valid": true, "error": null};
    console.log(JSON.stringify(result));
} catch (error) {
    // 捕获语法错误，输出JSON结果
    const result = {"valid": false, "error": "语法错误: " + error.message};
    console.log(JSON.stringify(result));
}
`