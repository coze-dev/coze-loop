// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package templates

// PythonTemplate Python代码执行模板
const PythonTemplate = `
import json
import sys
import asyncio
from dataclasses import dataclass


class Args:
    def __init__(self, params):
        self.params = params

class Output(dict):
    pass

@dataclass
class EvalOutput:
    score: float
    reason: str
    err_msg: str

args = {}
turn = {{TURN_DATA}}
# EvalOutput dataclass is now used directly

{{EXEC_EVALUATION_FUNCTION}}

async def main(args):
    """
    Fixed version of the original user_code.py
    Adapted for py_sandbox.py execution format
    """

    # Test data (using English to avoid UTF-8 issues)


    # Execute evaluation
    result = exec_evaluation(turn)

    # Print results
    print("Evaluation Results:")
    print(f"Score: {result.score}")
    print(f"Reason: {result.reason}")
    if result.err_msg:
        print(f"Error: {result.err_msg}")

    # Return result for sandbox - convert to dict for JSON serialization
    return {
        "score": result.score,
        "reason": result.reason,
        "err_msg": result.err_msg
    }

result = None
try:
    result = asyncio.run(main(Args(args)))
except Exception as e:
    print(f"{type(e).__name__}: {str(e)}", file=sys.stderr)
    sys.exit(1)
result
`

// PythonSyntaxCheckTemplate Python语法检查模板
const PythonSyntaxCheckTemplate = `
import ast
import json

def check_syntax(code):
    """
    检查Python代码是否有语法错误
    返回 (是否有错误, 错误信息或None)
    """
    try:
        # 尝试解析代码
        ast.parse(code)
        return (False, None)  # 没有语法错误
    except SyntaxError as e:
        # 捕获语法错误并返回错误信息
        error_msg = f"语法错误: {e.msg} (行号: {e.lineno}, 列号: {e.offset})"
        return (True, error_msg)

# 用户代码
user_code = """{{USER_CODE}}"""

# 检查语法
has_error, msg = check_syntax(user_code)
if has_error:
    result = {"valid": False, "error": msg}
else:
    result = {"valid": True, "error": None}

# 输出结果
print(json.dumps(result))
`