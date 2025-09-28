// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package templates

// PythonTemplate Python代码执行模板
const PythonTemplate = `
import json
import sys
import asyncio
from dataclasses import dataclass

{{RETURN_VAL_FUNCTION}}

class Args:
    def __init__(self, params):
        self.params = params

class Output(dict):
    pass

@dataclass
class EvalOutput:
    score: float
    reason: str


args = {}
turn = {{TURN_DATA}}
# EvalOutput dataclass is now used directly

{{EXEC_EVALUATION_FUNCTION}}

def main(args):
    """
    Fixed version of the original user_code.py
    Adapted for py_sandbox.py execution format
    """

    # Test data (using English to avoid UTF-8 issues)


    # Execute evaluation
    result = exec_evaluation(turn)

    # Return result for sandbox - convert to dict for JSON serialization
    return {
        "score": result.score,
        "reason": result.reason
    }

result = None
try:
    result = main(Args(args))
except Exception as e:
    print(f"{type(e).__name__}: {str(e)}", file=sys.stderr)
    sys.exit(1)
return_val(json.dumps(result))
`

// PythonSyntaxCheckTemplate Python语法检查模板
const PythonSyntaxCheckTemplate = `
import ast
import json

{{RETURN_VAL_FUNCTION}}

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
        # 捕获语法错误并返回详细的错误信息，包含行列号
        error_msg = f"语法错误: {e.msg}"
        if e.lineno is not None:
            error_msg += f" (行号: {e.lineno - 14}"
            if e.offset is not None:
                error_msg += f", 列号: {e.offset}"
            error_msg += ")"
        
        # 构建详细的错误结果
        error_detail = {
            "message": e.msg,
            "line": e.lineno - 14,
            "column": e.offset,
            "full_message": error_msg
        }
        return (True, error_detail)

# 用户代码
user_code = """{{USER_CODE}}"""

# 检查语法
has_error, error_info = check_syntax(user_code)
if has_error:
    result = {"valid": False, "error": error_info}
else:
    result = {"valid": True, "error": None}

# 输出结果
return_val(json.dumps(result))
`
