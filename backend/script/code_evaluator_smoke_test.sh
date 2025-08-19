#!/bin/bash

# Code类型评估器冒烟测试脚本
# 基于测试用例文档生成的可执行脚本

set -e

# 默认配置
DEFAULT_BASE_URL="https://loop.bots-boe.bytedance.net"
DEFAULT_WORKSPACE_ID="7327585901742161964"
DEFAULT_EVALUATOR_ID="7543200588889063425"
DEFAULT_EVAL_TARGET_ID="7486784446176165890"
DEFAULT_EVAL_TARGET_VERSION_ID="7530209624226545666"
DEFAULT_COOKIE="email=wangziqi.9425@bytedance.com; user_token=JTdCJTIybmFtZSUyMiUzQSUyMiVFNyU4RSU4QiVFNSVBRCU5MCVFNSVBNSU4NyUyMiUyQyUyMmZ1bGxfbmFtZSUyMiUzQSUyMiVFNyU4RSU4QiVFNSVBRCU5MCVFNSVBNSU4NyUyMDc5NjkxMzYlMjIlMkMlMjJlbWFpbCUyMiUzQSUyMndhbmd6aXFpLjk0MjUlNDBieXRlZGFuY2UuY29tJTIyJTJDJTIycGljdHVyZSUyMiUzQSUyMmh0dHBzJTNBJTJGJTJGczMtaW1maWxlLmZlaXNodWNkbi5jb20lMkZzdGF0aWMtcmVzb3VyY2UlMkZ2MSUyRnYzXzAwa21fNGQ1NWU0Y2EtOTkwYS00ZmZkLThlM2MtNGFhYTA4NjA0NDhnfiUzRmltYWdlX3NpemUlM0QyNDB4MjQwJTI2Y3V0X3R5cGUlM0QlMjZxdWFsaXR5JTNEJTI2Zm9ybWF0JTNEcG5nJTI2c3RpY2tlcl9mb3JtYXQlM0Qud2VicCUyMiUyQyUyMmVtcGxveWVlX2lkJTIyJTNBJTIyNzk2OTEzNiUyMiUyQyUyMmVtcGxveWVlX251bWJlciUyMiUzQSUyMjc5NjkxMzYlMjIlMkMlMjJ0ZW5hbnRfYWxpYXMlMjIlM0ElMjJieXRlZGFuY2UlMjIlMkMlMjJ1c2VyX2lkJTIyJTNBJTIyemplODBmM2hrcXo0dDY1NTNrbDYlMjIlN0Q=; passport_csrf_token=405d78e50fc2b1a80f9d6610e39e718b; people-lang=zh; csrfToken=9a916ed40520b2f2835511d5f800cdfc; csrfToken=9a916ed40520b2f2835511d5f800cdfc; userInfo=eyJhbGciOiJSUzI1NiIsImtpZCI6ImZkZjBmNzFjNzI5MjExZjA5YmVmMDAxNjNlMDY0MGQ4In0.eyJhY2NfaSI6MjEwMDI2NTM3MywiYXVkIjpbInZvbGNlbmdpbmUiXSwiZXhwIjoxNzU3MDYxMjYwLCJpIjoiYTMxMDM2M2UtY2M3Yy00ZjQ5LWJkMDgtN2EyNzM2YzJjOTMxIiwiaWRfbiI6ImNvemVfdHBmeiIsIm1zZyI6bnVsbCwic3NfbiI6ImNvemVfdHBmeiIsInQiOiJBY2NvdW50IiwidG9waWMiOiJzaWduaW5fdXNlcl9pbmZvIiwidmVyc2lvbiI6InYxIiwiemlwIjoiIn0.mr0bAH7PgM9hAyQbffzt8Ey8FQhILJAakRLsQD80THuQp1IjXJcGLy05D5pcR0slvgqMiJ-ewaCmuCm3X67t-VSGJ0Lid5IH9mFq8y6dpBrWCRBDba7JhxC87ImooeIz9tI35ebVbpNUNdZwH7Myj-5rmEy_XCuFngZMhnsdscadr966rCHHdwtYd7iuNHxx44gFolEMXltFcAHkunKLSYnpDKXdgGYI2MnjAWvbsTCUObeO1Mm0VERWI6I7QCzRDoJ6rCQr1geWLZY-XROPCYooPpro22Eb9k3kFCG5GX2S6E8-qzgTZfMZcmkQHuIeW00Ns71_jIneSiFDnH2yFQ; digest=a310363e-cc7c-4f49-bd08-7a2736c2c931; digest=a310363e-cc7c-4f49-bd08-7a2736c2c931; AccountID=2100265373; AccountID=2100265373; passport_auth_status_ss_alice_boe=c24cd84900a6aec3e16566104c1c36fb%2C; uid_tt_ss_alice_boe=b46ad72379b0eee2fea089f2d3986531; sessionid_ss_alice_boe=cca43f910461c9924cb824a5696048b0; sid_ucp_v1_alice_boe=1.0.0-KGY1MjlkMDJiMDZjODc2MjU3ZmYwYzRmZTE5ZGZhZWVhNzQwMTg0MTMKHQjLguGvsM32BBCTp8zEBhjHkB8gDDCvpcOtBjgIEH8aA2JvZSIgY2NhNDNmOTEwNDYxYzk5MjRjYjgyNGE1Njk2MDQ4YjA; ssid_ucp_v1_alice_boe=1.0.0-KGY1MjlkMDJiMDZjODc2MjU3ZmYwYzRmZTE5ZGZhZWVhNzQwMTg0MTMKHQjLguGvsM32BBCTp8zEBhjHkB8gDDCvpcOtBjgIEH8aA2JvZSIgY2NhNDNmOTEwNDYxYzk5MjRjYjgyNGE1Njk2MDQ4YjA; ttwid=1%7C_nN33i74uekbkIZC5piKOlAOtK-qcK6GnkN9yPAKhwU%7C1755086596%7C5189935edebddc25e48626561eec116a3270b6494944b6a42ef6775e554e647c; titan_passport_id=cn/bytedance/197157e5-a92f-4352-9fc5-13dfcb836b0d|boe/bytedance/d245f57e-c205-41ea-8092-1a0865d92532"
DEFAULT_ENV_VALUE="boe_wzq_code_evaluator"

# 全局变量
BASE_URL="${BASE_URL:-$DEFAULT_BASE_URL}"
WORKSPACE_ID="${WORKSPACE_ID:-$DEFAULT_WORKSPACE_ID}"
EVALUATOR_ID="${EVALUATOR_ID:-$DEFAULT_EVALUATOR_ID}"
EVAL_TARGET_ID="${EVAL_TARGET_ID:-$DEFAULT_EVAL_TARGET_ID}"
EVAL_TARGET_VERSION_ID="${EVAL_TARGET_VERSION_ID:-$DEFAULT_EVAL_TARGET_VERSION_ID}"
COOKIE="${COOKIE:-$DEFAULT_COOKIE}"
ENV_VALUE="${ENV_VALUE:-$DEFAULT_ENV_VALUE}"
VERBOSE=false
OUTPUT_FILE=""
TEST_RESULTS=()

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印帮助信息
show_help() {
    cat << EOF
Code类型评估器冒烟测试脚本

用法: $0 [选项] [测试用例]

选项:
    -h, --help              显示此帮助信息
    -v, --verbose           详细输出模式
    -u, --url URL          设置API基础URL (默认: $DEFAULT_BASE_URL)
    -w, --workspace ID     设置workspace_id (默认: $DEFAULT_WORKSPACE_ID)
    -e, --evaluator ID     设置evaluator_id (默认: $DEFAULT_EVALUATOR_ID)
    -t, --target ID        设置eval_target_id (默认: $DEFAULT_EVAL_TARGET_ID)
    -tv, --target-version ID 设置eval_target_version_id (默认: $DEFAULT_EVAL_TARGET_VERSION_ID)
    -c, --cookie COOKIE    设置认证Cookie (默认: $DEFAULT_COOKIE)
    -x, --env ENV          设置x-tt-env头部值 (默认: $DEFAULT_ENV_VALUE)
    -o, --output FILE      将结果输出到文件

测试用例:
    all                     执行所有测试用例 (默认)
    create-python          创建Python代码评估器
    create-js              创建JavaScript代码评估器
    update-python          更新Python评估器草稿
    update-js              更新JavaScript评估器草稿
    debug-python           调试Python评估器
    debug-js               调试JavaScript评估器
    error-syntax           语法错误测试
    error-security         安全性测试
    mock-basic             基础mock输出测试
    mock-parameterized     参数化mock输出测试
    mock-invalid-id        无效ID错误测试
    mock-invalid-version   无效版本错误测试

环境变量:
    BASE_URL               API基础URL
    WORKSPACE_ID           工作空间ID
    EVALUATOR_ID           评估器ID
    EVAL_TARGET_ID         评测目标ID
    EVAL_TARGET_VERSION_ID 评测目标版本ID
    COOKIE                 认证Cookie
    ENV_VALUE              x-tt-env头部值

示例:
    $0 create-python                              # 执行Python创建测试
    $0 -v all                                    # 详细模式执行所有测试
    $0 -u https://loop.bots-boe.bytedance.net -w 123 all  # 自定义环境执行所有测试
    $0 -x prod -o results.log debug-python       # 设置环境头部并将结果输出到文件
    $0 -t 123456 -tv 789012 mock-parameterized   # 使用自定义target_id和version_id执行参数化mock测试
    $0 mock-basic                                # 执行基础mock输出测试

EOF
}

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_verbose() {
    if [ "$VERBOSE" = true ]; then
        echo -e "${BLUE}[VERBOSE]${NC} $1"
    fi
}

# 执行curl请求并处理响应
execute_curl() {
    local test_name="$1"
    local curl_cmd="$2"
    local expected_status="${3:-200}"
    
    log_info "执行测试: $test_name"
    log_verbose "执行命令: $curl_cmd"
    
    # 创建临时文件存储响应
    local response_file=$(mktemp)
    local status_file=$(mktemp)
    
    # 执行curl命令
    eval "$curl_cmd -w '%{http_code}' -s -o '$response_file'" > "$status_file" 2>&1
    local curl_exit_code=$?
    
    # 读取HTTP状态码和响应内容
    local http_status=$(cat "$status_file" 2>/dev/null || echo "000")
    local response_content=$(cat "$response_file" 2>/dev/null || echo "")
    
    # 清理临时文件
    rm -f "$response_file" "$status_file"
    
    # 检查curl执行结果
    if [ $curl_exit_code -ne 0 ]; then
        log_error "$test_name - curl执行失败 (退出码: $curl_exit_code)"
        TEST_RESULTS+=("FAIL: $test_name - curl执行失败")
        return 1
    fi
    
    # 检查HTTP状态码
    if [[ "$http_status" =~ ^[45][0-9][0-9]$ ]]; then
        log_error "$test_name - HTTP错误 (状态码: $http_status)"
        log_verbose "响应内容: $response_content"
        TEST_RESULTS+=("FAIL: $test_name - HTTP错误 ($http_status)")
        return 1
    elif [[ "$http_status" =~ ^[23][0-9][0-9]$ ]]; then
        log_success "$test_name - 测试通过 (状态码: $http_status)"
        log_verbose "响应内容: $response_content"
        TEST_RESULTS+=("PASS: $test_name")
        
        # 输出响应到文件（如果指定）
        if [ -n "$OUTPUT_FILE" ]; then
            echo "=== $test_name ===" >> "$OUTPUT_FILE"
            echo "HTTP状态码: $http_status" >> "$OUTPUT_FILE"
            echo "响应内容:" >> "$OUTPUT_FILE"
            echo "$response_content" >> "$OUTPUT_FILE"
            echo "" >> "$OUTPUT_FILE"
        fi
        return 0
    else
        log_warning "$test_name - 未知状态码: $http_status"
        TEST_RESULTS+=("WARN: $test_name - 未知状态码 ($http_status)")
        return 0
    fi
}

# 创建Python代码评估器
test_create_python() {
    local curl_cmd="curl -X POST \"${BASE_URL}/api/evaluation/v1/evaluators\" \
  -H \"Content-Type: application/json\" \
  -H \"Cookie: ${COOKIE}\" \
  -H \"x-tt-env: ${ENV_VALUE}\" \
  -H \"agw-js-conv: str\" \
  -H \"agw-js-conv: str\" \
  -d '{
    \"evaluator\": {
      \"workspace_id\": ${WORKSPACE_ID},
      \"evaluator_type\": 2,
      \"name\": \"Python代码评估器测试2\",
      \"description\": \"用于测试Python代码评估器的创建功能\",
      \"current_version\": {
        \"version\": \"0.0.1\",
        \"evaluator_content\": {
          \"receive_chat_history\": false,
          \"code_evaluator\": {
            \"language_type\": \"Python\",
            \"code_content\": \"def exec_evaluation(turn_data):\\n    try:\\n        # 获取实际输出和参考输出\\n        actual_text = turn_data[\\\"turn\\\"][\\\"eval_target\\\"][\\\"actual_output\\\"][\\\"text\\\"]\\n        reference_text = turn_data[\\\"turn\\\"][\\\"eval_set\\\"][\\\"reference_output\\\"][\\\"text\\\"]\\n        \\n        # 比较文本相似性或相等性\\n        is_equal = actual_text.strip() == reference_text.strip()\\n        score = 1.0 if is_equal else 0.0\\n        \\n        if is_equal:\\n            status = \\\"匹配\\\"\\n        else:\\n            status = \\\"不匹配\\\"\\n        reason = f\\\"实际输出与参考输出{status}。实际输出: '{actual_text}', 参考输出: '{reference_text}'\\\"\\n        \\n        return EvalOutput(score=score, reason=reason, err_msg=\\\"\\\")\\n        \\n    except KeyError as e:\\n        return EvalOutput(score=0.0, reason=f\\\"字段路径未找到: {e}\\\", err_msg=str(e))\\n    except Exception as e:\\n        return EvalOutput(score=0.0, reason=f\\\"评估失败: {e}\\\", err_msg=str(e))\",
            \"code_template_key\": \"equals_checker\",
            \"code_template_name\": \"相等性检查器\"
          }
        }
      }
    },
    \"cid\": \"test_create_python_evaluator_001\"
  }'"
    
    execute_curl "创建Python代码评估器" "$curl_cmd"
}

# 创建JavaScript代码评估器
test_create_js() {
    local curl_cmd="curl -X POST \"${BASE_URL}/api/evaluation/v1/evaluators\" \
  -H \"Content-Type: application/json\" \
  -H \"Cookie: ${COOKIE}\" \
  -H \"x-tt-env: ${ENV_VALUE}\" \
  -H \"agw-js-conv: str\" \
  -d '{
    \"evaluator\": {
      \"workspace_id\": ${WORKSPACE_ID},
      \"evaluator_type\": 2,
      \"name\": \"JavaScript代码评估器测试\",
      \"description\": \"用于测试JavaScript代码评估器的创建功能\", 
      \"draft_submitted\": false,
      \"current_version\": {
        \"version\": \"v1.0.0\",
        \"description\": \"初始版本\",
        \"evaluator_content\": {
          \"receive_chat_history\": false,
          \"input_schemas\": [
            {
              \"name\": \"input\",
              \"type\": \"string\",
              \"description\": \"评估输入内容\"
            },
            {
              \"name\": \"reference_output\",
              \"type\": \"string\", 
              \"description\": \"参考输出内容\"
            },
            {
              \"name\": \"actual_output\",
              \"type\": \"string\",
              \"description\": \"实际输出内容\"
            }
          ],
          \"code_evaluator\": {
            \"language_type\": \"JS\",
            \"code_content\": \"def exec_evaluation(turn_data):\\n    try:\\n        # 获取实际输出和参考输出\\n        actual_text = turn_data[\\\"turn\\\"][\\\"eval_target\\\"][\\\"actual_output\\\"][\\\"text\\\"]\\n        reference_text = turn_data[\\\"turn\\\"][\\\"eval_set\\\"][\\\"reference_output\\\"][\\\"text\\\"]\\n        \\n        # 比较文本相似性或相等性\\n        is_equal = actual_text.strip() == reference_text.strip()\\n        score = 1.0 if is_equal else 0.0\\n        \\n        if is_equal:\\n            status = \\\"匹配\\\"\\n        else:\\n            status = \\\"不匹配\\\"\\n        reason = f\\\"实际输出与参考输出{status}。实际输出: '{actual_text}', 参考输出: '{reference_text}'\\\"\\n        \\n        return EvalOutput(score=score, reason=reason, err_msg=\\\"\\\")\\n        \\n    except KeyError as e:\\n        return EvalOutput(score=0.0, reason=f\\\"字段路径未找到: {e}\\\", err_msg=str(e))\\n    except Exception as e:\\n        return EvalOutput(score=0.0, reason=f\\\"评估失败: {e}\\\", err_msg=str(e))\",
            \"code_template_key\": \"contains_checker\",
            \"code_template_name\": \"包含性检查器\"
          }
        }
      }
    },
    \"cid\": \"test_create_js_evaluator_001\"
  }'"
    
    execute_curl "创建JavaScript代码评估器" "$curl_cmd"
}

# 更新Python评估器草稿
test_update_python() {
    local curl_cmd="curl -X PATCH \"${BASE_URL}/api/evaluation/v1/evaluators/${EVALUATOR_ID}/update_draft\" \
  -H \"Content-Type: application/json\" \
  -H \"Cookie: ${COOKIE}\" \
  -H \"x-tt-env: ${ENV_VALUE}\" \
  -H \"agw-js-conv: str\" \
  -H \"agw-js-conv: str\" \
  -d '{
    \"workspace_id\": ${WORKSPACE_ID},
    \"evaluator_type\": 2,
    \"evaluator_content\": {
      \"receive_chat_history\": true,
      \"input_schemas\": [
        {
          \"name\": \"input\",
          \"type\": \"string\",
          \"description\": \"评估输入内容\"
        },
        {
          \"name\": \"reference_output\",
          \"type\": \"string\",
          \"description\": \"参考输出内容\"
        },
        {
          \"name\": \"actual_output\", 
          \"type\": \"string\",
          \"description\": \"实际输出内容\"
        }
      ],
      \"code_evaluator\": {
        \"language_type\": \"Python\",
        \"code_content\": \"def exec_evaluation(turn_data):\\n    try:\\n        # 获取实际输出和参考输出\\n        actual_text = turn_data[\\\"turn\\\"][\\\"eval_target\\\"][\\\"actual_output\\\"][\\\"text\\\"]\\n        reference_text = turn_data[\\\"turn\\\"][\\\"eval_set\\\"][\\\"reference_output\\\"][\\\"text\\\"]\\n        \\n        # 比较文本相似性或相等性\\n        is_equal = actual_text.strip() == reference_text.strip()\\n        score = 1.0 if is_equal else 0.0\\n        \\n        if is_equal:\\n            status = \\\"匹配\\\"\\n        else:\\n            status = \\\"不匹配\\\"\\n        reason = f\\\"实际输出与参考输出{status}。实际输出: '{actual_text}', 参考输出: '{reference_text}'\\\"\\n        \\n        return EvalOutput(score=score, reason=reason, err_msg=\\\"\\\")\\n        \\n    except KeyError as e:\\n        return EvalOutput(score=0.0, reason=f\\\"字段路径未找到: {e}\\\", err_msg=str(e))\\n    except Exception as e:\\n        return EvalOutput(score=0.0, reason=f\\\"评估失败: {e}\\\", err_msg=str(e))\",
        \"code_template_key\": \"similarity_checker\",
        \"code_template_name\": \"相似度检查器\"
      }
    }
  }'"
    
    execute_curl "更新Python评估器草稿" "$curl_cmd"
}

# 更新JavaScript评估器草稿
test_update_js() {
    local curl_cmd="curl -X PATCH \"${BASE_URL}/api/evaluation/v1/evaluators/${EVALUATOR_ID}/update_draft\" \
  -H \"Content-Type: application/json\" \
  -H \"Cookie: ${COOKIE}\" \
  -H \"x-tt-env: ${ENV_VALUE}\" \
  -H \"agw-js-conv: str\" \
  -H \"agw-js-conv: str\" \
  -d '{
    \"workspace_id\": ${WORKSPACE_ID},
    \"evaluator_type\": 2,
    \"evaluator_content\": {
      \"receive_chat_history\": false,
      \"input_schemas\": [
        {
          \"name\": \"input\",
          \"type\": \"string\",
          \"description\": \"评估输入内容\"
        },
        {
          \"name\": \"reference_output\",
          \"type\": \"string\",
          \"description\": \"参考输出内容\"
        },
        {
          \"name\": \"actual_output\",
          \"type\": \"string\", 
          \"description\": \"实际输出内容\"
        }
      ],
      \"code_evaluator\": {
        \"language_type\": \"JS\",
        \"code_content\": \"def exec_evaluation(turn_data):\\n    try:\\n        # 获取实际输出和参考输出\\n        actual_text = turn_data[\\\"turn\\\"][\\\"eval_target\\\"][\\\"actual_output\\\"][\\\"text\\\"]\\n        reference_text = turn_data[\\\"turn\\\"][\\\"eval_set\\\"][\\\"reference_output\\\"][\\\"text\\\"]\\n        \\n        # 比较文本相似性或相等性\\n        is_equal = actual_text.strip() == reference_text.strip()\\n        score = 1.0 if is_equal else 0.0\\n        \\n        if is_equal:\\n            status = \\\"匹配\\\"\\n        else:\\n            status = \\\"不匹配\\\"\\n        reason = f\\\"实际输出与参考输出{status}。实际输出: '{actual_text}', 参考输出: '{reference_text}'\\\"\\n        \\n        return EvalOutput(score=score, reason=reason, err_msg=\\\"\\\")\\n        \\n    except KeyError as e:\\n        return EvalOutput(score=0.0, reason=f\\\"字段路径未找到: {e}\\\", err_msg=str(e))\\n    except Exception as e:\\n        return EvalOutput(score=0.0, reason=f\\\"评估失败: {e}\\\", err_msg=str(e))\",
        \"code_template_key\": \"length_format_checker\",
        \"code_template_name\": \"长度格式检查器\"
      }
    }
  }'"
    
    execute_curl "更新JavaScript评估器草稿" "$curl_cmd"
}

# 调试Python评估器
test_debug_python() {
    local curl_cmd="curl -X POST \"${BASE_URL}/api/evaluation/v1/evaluators/debug\" \
  -H \"Content-Type: application/json\" \
  -H \"Cookie: ${COOKIE}\" \
  -H \"x-tt-env: ${ENV_VALUE}\" \
  -H \"agw-js-conv: str\" \
  -H \"agw-js-conv: str\" \
  -H \"agw-js-conv: str\" \
  -H \"agw-js-conv: str\" \
  -d '{
    \"workspace_id\": ${WORKSPACE_ID},
    \"evaluator_type\": 2,
    \"evaluator_content\": {
      \"receive_chat_history\": false,
      \"input_schemas\": [
        {
          \"name\": \"input\",
          \"type\": \"string\",
          \"description\": \"评估输入内容\"
        },
        {
          \"name\": \"reference_output\",
          \"type\": \"string\",
          \"description\": \"参考输出内容\"
        },
        {
          \"name\": \"actual_output\",
          \"type\": \"string\",
          \"description\": \"实际输出内容\"
        }
      ],
      \"code_evaluator\": {
        \"language_type\": \"Python\",
         \"code_content\": \"def exec_evaluation(turn_data):\\n    try:\\n        # 获取实际输出和参考输出\\n        actual_text = turn_data[\\\"turn\\\"][\\\"eval_target\\\"][\\\"actual_output\\\"][\\\"text\\\"]\\n        reference_text = turn_data[\\\"turn\\\"][\\\"eval_set\\\"][\\\"reference_output\\\"][\\\"text\\\"]\\n        \\n        # 比较文本相似性或相等性\\n        is_equal = actual_text.strip() == reference_text.strip()\\n        score = 1.0 if is_equal else 0.0\\n        \\n        if is_equal:\\n            status = \\\"匹配\\\"\\n        else:\\n            status = \\\"不匹配\\\"\\n        reason = f\\\"实际输出与参考输出{status}。实际输出: '{actual_text}', 参考输出: '{reference_text}'\\\"\\n        \\n        return EvalOutput(score=score, reason=reason, err_msg=\\\"\\\")\\n        \\n    except KeyError as e:\\n        return EvalOutput(score=0.0, reason=f\\\"字段路径未找到: {e}\\\", err_msg=str(e))\\n    except Exception as e:\\n        return EvalOutput(score=0.0, reason=f\\\"评估失败: {e}\\\", err_msg=str(e))\",
        \"code_template_key\": \"debug_length_checker\",
        \"code_template_name\": \"调试长度检查器\"
      }
    },
    \"input_data\": {
      \"from_eval_set_fields\": {
        \"input\": {
          \"content_type\": \"Text\",
          \"text\": \"什么是人工智能？\"
        },
        \"reference_output\": {
          \"content_type\": \"Text\", 
          \"text\": \"人工智能是一种让机器能够模拟人类智能行为的技术。\"
        }
      },
      \"from_eval_target_fields\": {
        \"actual_output\": {
          \"content_type\": \"Text\",
          \"text\": \"人工智能是计算机科学的一个分支，致力于创建能够执行通常需要人类智能的任务的系统。\"
        }
      },
      \"ext\": {
        \"tenant\": \"coze_loop\",
        \"user_id\": \"test_user_001\"
      }
    }
  }'"
    
    execute_curl "调试Python评估器" "$curl_cmd"
}

# 调试JavaScript评估器
test_debug_js() {
    local curl_cmd="curl -X POST \"${BASE_URL}/api/evaluation/v1/evaluators/debug\" \
  -H \"Content-Type: application/json\" \
  -H \"Cookie: ${COOKIE}\" \
  -H \"x-tt-env: ${ENV_VALUE}\" \
  -H \"agw-js-conv: str\" \
  -H \"agw-js-conv: str\" \
  -H \"agw-js-conv: str\" \
  -H \"agw-js-conv: str\" \
  -d '{
    \"workspace_id\": ${WORKSPACE_ID},
    \"evaluator_type\": 2,
    \"evaluator_content\": {
      \"receive_chat_history\": false,
      \"input_schemas\": [
        {
          \"name\": \"input\",
          \"type\": \"string\",
          \"description\": \"评估输入内容\"
        },
        {
          \"name\": \"reference_output\",
          \"type\": \"string\",
          \"description\": \"参考输出内容\"
        },
        {
          \"name\": \"actual_output\",
          \"type\": \"string\",
          \"description\": \"实际输出内容\"
        }
      ],
      \"code_evaluator\": {
        \"language_type\": \"JS\",
        \"code_content\": \"def exec_evaluation(turn_data):\\n    try:\\n        # 获取实际输出和参考输出\\n        actual_text = turn_data[\\\"turn\\\"][\\\"eval_target\\\"][\\\"actual_output\\\"][\\\"text\\\"]\\n        reference_text = turn_data[\\\"turn\\\"][\\\"eval_set\\\"][\\\"reference_output\\\"][\\\"text\\\"]\\n        \\n        # 比较文本相似性或相等性\\n        is_equal = actual_text.strip() == reference_text.strip()\\n        score = 1.0 if is_equal else 0.0\\n        \\n        if is_equal:\\n            status = \\\"匹配\\\"\\n        else:\\n            status = \\\"不匹配\\\"\\n        reason = f\\\"实际输出与参考输出{status}。实际输出: '{actual_text}', 参考输出: '{reference_text}'\\\"\\n        \\n        return EvalOutput(score=score, reason=reason, err_msg=\\\"\\\")\\n        \\n    except KeyError as e:\\n        return EvalOutput(score=0.0, reason=f\\\"字段路径未找到: {e}\\\", err_msg=str(e))\\n    except Exception as e:\\n        return EvalOutput(score=0.0, reason=f\\\"评估失败: {e}\\\", err_msg=str(e))\",
        \"code_template_key\": \"debug_sentiment_checker\",
        \"code_template_name\": \"调试情感检查器\"
      }
    },
    \"input_data\": {
      \"from_eval_set_fields\": {
        \"input\": {
          \"content_type\": \"Text\",
          \"text\": \"请评价这个产品的质量\"
        },
        \"reference_output\": {
          \"content_type\": \"Text\",
          \"text\": \"这个产品质量很好，值得推荐。\"
        }
      },
      \"from_eval_target_fields\": {
        \"actual_output\": {
          \"content_type\": \"Text\",
          \"text\": \"这个产品质量优秀，做工精细，用户体验很棒，强烈推荐购买。\"
        }
      },
      \"ext\": {
        \"tenant\": \"coze_loop\", 
        \"user_id\": \"test_user_002\"
      }
    }
  }'"
    
    execute_curl "调试JavaScript评估器" "$curl_cmd"
}

# 语法错误测试
test_error_syntax() {
    local curl_cmd="curl -X POST \"${BASE_URL}/api/evaluation/v1/evaluators/debug\" \
  -H \"Content-Type: application/json\" \
  -H \"Cookie: ${COOKIE}\" \
  -H \"x-tt-env: ${ENV_VALUE}\" \
  -H \"agw-js-conv: str\" \
  -H \"agw-js-conv: str\" \
  -H \"agw-js-conv: str\" \
  -H \"agw-js-conv: str\" \
  -d '{
    \"workspace_id\": ${WORKSPACE_ID},
    \"evaluator_type\": 2,
    \"evaluator_content\": {
      \"receive_chat_history\": false,
      \"input_schemas\": [],
      \"code_evaluator\": {
        \"language_type\": \"Python\",
        \"code_content\": \"def exec_evaluation(turn_data):\\n    try:\\n        # 获取实际输出和参考输出\\n        actual_text = turn_data[\\\"turn\\\"][\\\"eval_target\\\"][\\\"actual_output\\\"][\\\"text\\\"]\\n        reference_text = turn_data[\\\"turn\\\"][\\\"eval_set\\\"][\\\"reference_output\\\"][\\\"text\\\"]\\n        \\n        # 比较文本相似性或相等性\\n        is_equal = actual_text.strip() == reference_text.strip()\\n        score = 1.0 if is_equal else 0.0\\n        \\n        if is_equal:\\n            status = \\\"匹配\\\"\\n        else:\\n            status = \\\"不匹配\\\"\\n        reason = f\\\"实际输出与参考输出{status}。实际输出: '{actual_text}', 参考输出: '{reference_text}'\\\"\\n        \\n        return EvalOutput(score=score, reason=reason, err_msg=\\\"\\\")\\n        \\n    except KeyError as e:\\n        return EvalOutput(score=0.0, reason=f\\\"字段路径未找到: {e}\\\", err_msg=str(e))\\n    except Exception as e:\\n        return EvalOutput(score=0.0, reason=f\\\"评估失败: {e}\\\", err_msg=str(e))\",
        \"code_template_key\": \"syntax_error_test\",
        \"code_template_name\": \"语法错误测试\"
      }
    },
    \"input_data\": {
      \"from_eval_set_fields\": {
        \"input\": {
          \"content_type\": \"Text\",
          \"text\": \"测试输入\"
        }
      },
      \"from_eval_target_fields\": {
        \"actual_output\": {
          \"content_type\": \"Text\",
          \"text\": \"测试输出\"
        }
      }
    }
  }'"
    
    # 对于语法错误测试，我们期望返回400错误
    execute_curl "语法错误测试" "$curl_cmd" "400"
}

# 安全性测试
test_error_security() {
    local curl_cmd="curl -X POST \"${BASE_URL}/api/evaluation/v1/evaluators/debug\" \
  -H \"Content-Type: application/json\" \
  -H \"Cookie: ${COOKIE}\" \
  -H \"x-tt-env: ${ENV_VALUE}\" \
  -H \"agw-js-conv: str\" \
  -H \"agw-js-conv: str\" \
  -H \"agw-js-conv: str\" \
  -H \"agw-js-conv: str\" \
  -d '{
    \"workspace_id\": ${WORKSPACE_ID},
    \"evaluator_type\": 2,
    \"evaluator_content\": {
      \"receive_chat_history\": false,
      \"input_schemas\": [],
      \"code_evaluator\": {
        \"language_type\": \"Python\",
        \"code_content\": \"def exec_evaluation(turn_data):\\n    try:\\n        # 获取实际输出和参考输出\\n        actual_text = turn_data[\\\"turn\\\"][\\\"eval_target\\\"][\\\"actual_output\\\"][\\\"text\\\"]\\n        reference_text = turn_data[\\\"turn\\\"][\\\"eval_set\\\"][\\\"reference_output\\\"][\\\"text\\\"]\\n        \\n        # 比较文本相似性或相等性\\n        is_equal = actual_text.strip() == reference_text.strip()\\n        score = 1.0 if is_equal else 0.0\\n        \\n        if is_equal:\\n            status = \\\"匹配\\\"\\n        else:\\n            status = \\\"不匹配\\\"\\n        reason = f\\\"实际输出与参考输出{status}。实际输出: '{actual_text}', 参考输出: '{reference_text}'\\\"\\n        \\n        return EvalOutput(score=score, reason=reason, err_msg=\\\"\\\")\\n        \\n    except KeyError as e:\\n        return EvalOutput(score=0.0, reason=f\\\"字段路径未找到: {e}\\\", err_msg=str(e))\\n    except Exception as e:\\n        return EvalOutput(score=0.0, reason=f\\\"评估失败: {e}\\\", err_msg=str(e))\",
        \"code_template_key\": \"security_test\",
        \"code_template_name\": \"安全性测试\"
      }
    },
    \"input_data\": {
      \"from_eval_set_fields\": {
        \"input\": {
          \"content_type\": \"Text\",
          \"text\": \"安全测试输入\"
        }
      },
      \"from_eval_target_fields\": {
        \"actual_output\": {
          \"content_type\": \"Text\",
          \"text\": \"安全测试输出\"
        }
      }
    }
  }'"
    
    # 对于安全性测试，我们期望返回400或403错误
    execute_curl "安全性测试" "$curl_cmd" "400"
}

# 基础Mock输出测试
test_mock_eval_target_output_basic() {
    local curl_cmd="curl -X POST \"${BASE_URL}/api/evaluation/v1/eval_targets/mock_output\" \
  -H \"Content-Type: application/json\" \
  -H \"Cookie: ${COOKIE}\" \
  -H \"x-tt-env: ${ENV_VALUE}\" \
  -H \"agw-js-conv: str\" \
  -d '{
    \"workspace_id\": ${WORKSPACE_ID},
    \"eval_target_id\": ${EVAL_TARGET_ID},
    \"eval_target_version_id\": ${EVAL_TARGET_VERSION_ID}
  }'"
    
    execute_curl "基础Mock输出测试" "$curl_cmd"
}

# 参数化Mock输出测试
test_mock_eval_target_output_parameterized() {
    local target_id="${1:-$EVAL_TARGET_ID}"
    local version_id="${2:-$EVAL_TARGET_VERSION_ID}"
    
    local curl_cmd="curl -X POST \"${BASE_URL}/api/evaluation/v1/eval_targets/mock_output\" \
  -H \"Content-Type: application/json\" \
  -H \"Cookie: ${COOKIE}\" \
  -H \"x-tt-env: ${ENV_VALUE}\" \
  -H \"agw-js-conv: str\" \
  -d '{
    \"workspace_id\": ${WORKSPACE_ID},
    \"eval_target_id\": ${target_id},
    \"eval_target_version_id\": ${version_id}
  }'"
    
    execute_curl "参数化Mock输出测试 target_id=${target_id} version_id=${version_id}" "$curl_cmd"
}

# 无效ID错误测试
test_mock_eval_target_output_invalid_id() {
    local invalid_target_id="999999999"
    
    local curl_cmd="curl -X POST \"${BASE_URL}/api/evaluation/v1/eval_targets/mock_output\" \
  -H \"Content-Type: application/json\" \
  -H \"Cookie: ${COOKIE}\" \
  -H \"x-tt-env: ${ENV_VALUE}\" \
  -H \"agw-js-conv: str\" \
  -d '{
    \"workspace_id\": ${WORKSPACE_ID},
    \"eval_target_id\": ${invalid_target_id},
    \"eval_target_version_id\": ${EVAL_TARGET_VERSION_ID}
  }'"
    
    # 对于无效ID测试，我们期望返回400或404错误
    execute_curl "无效ID错误测试" "$curl_cmd" "400"
}

# 无效版本错误测试
test_mock_eval_target_output_invalid_version() {
    local invalid_version_id="999999999"
    
    local curl_cmd="curl -X POST \"${BASE_URL}/api/evaluation/v1/eval_targets/mock_output\" \
  -H \"Content-Type: application/json\" \
  -H \"Cookie: ${COOKIE}\" \
  -H \"x-tt-env: ${ENV_VALUE}\" \
  -H \"agw-js-conv: str\" \
  -d '{
    \"workspace_id\": ${WORKSPACE_ID},
    \"eval_target_id\": ${EVAL_TARGET_ID},
    \"eval_target_version_id\": ${invalid_version_id}
  }'"
    
    # 对于无效版本测试，我们期望返回400或404错误
    execute_curl "无效版本错误测试" "$curl_cmd" "400"
}

# 显示测试结果摘要
show_summary() {
    echo ""
    log_info "============= 测试结果摘要 ============="
    
    local total_tests=${#TEST_RESULTS[@]}
    local passed_tests=0
    local failed_tests=0
    local warned_tests=0
    
    for result in "${TEST_RESULTS[@]}"; do
        if [[ $result == PASS:* ]]; then
            ((passed_tests++))
            log_success "$result"
        elif [[ $result == FAIL:* ]]; then
            ((failed_tests++))
            log_error "$result"
        elif [[ $result == WARN:* ]]; then
            ((warned_tests++))
            log_warning "$result"
        fi
    done
    
    echo ""
    log_info "总计: $total_tests 个测试"
    log_success "通过: $passed_tests 个"
    log_error "失败: $failed_tests 个"
    log_warning "警告: $warned_tests 个"
    
    if [ $failed_tests -eq 0 ]; then
        log_success "所有测试执行完成！"
        return 0
    else
        log_error "存在失败的测试用例！"
        return 1
    fi
}

# 验证环境配置
validate_config() {
    log_info "验证环境配置..."
    
    if [ "$COOKIE" = "$DEFAULT_COOKIE" ]; then
        log_warning "使用默认COOKIE，请确保已设置正确的认证Cookie"
    fi
    
    if [ "$WORKSPACE_ID" = "$DEFAULT_WORKSPACE_ID" ]; then
        log_warning "使用默认WORKSPACE_ID，请确保已设置正确的工作空间ID"
    fi
    
    if [ "$EVALUATOR_ID" = "$DEFAULT_EVALUATOR_ID" ]; then
        log_warning "使用默认EVALUATOR_ID，请确保已设置正确的评估器ID"
    fi
    
    if [ "$EVAL_TARGET_ID" = "$DEFAULT_EVAL_TARGET_ID" ]; then
        log_warning "使用默认EVAL_TARGET_ID，请确保已设置正确的评测目标ID"
    fi
    
    if [ "$EVAL_TARGET_VERSION_ID" = "$DEFAULT_EVAL_TARGET_VERSION_ID" ]; then
        log_warning "使用默认EVAL_TARGET_VERSION_ID，请确保已设置正确的评测目标版本ID"
    fi
    
    log_verbose "配置信息:"
    log_verbose "  BASE_URL: $BASE_URL"
    log_verbose "  WORKSPACE_ID: $WORKSPACE_ID"
    log_verbose "  EVALUATOR_ID: $EVALUATOR_ID"
    log_verbose "  EVAL_TARGET_ID: $EVAL_TARGET_ID"
    log_verbose "  EVAL_TARGET_VERSION_ID: $EVAL_TARGET_VERSION_ID"
    log_verbose "  COOKIE: ${COOKIE:0:10}..."
    log_verbose "  ENV_VALUE: $ENV_VALUE"
    
    # 检查curl是否可用
    if ! command -v curl &> /dev/null; then
        log_error "curl命令不可用，请安装curl"
        exit 1
    fi
}

# 主函数
main() {
    local test_case="${1:-all}"
    
    # 验证环境配置
    validate_config
    
    # 初始化输出文件
    if [ -n "$OUTPUT_FILE" ]; then
        echo "Code类型评估器冒烟测试结果" > "$OUTPUT_FILE"
        echo "测试时间: $(date)" >> "$OUTPUT_FILE"
        echo "配置信息: BASE_URL=$BASE_URL, WORKSPACE_ID=$WORKSPACE_ID, ENV_VALUE=$ENV_VALUE" >> "$OUTPUT_FILE"
        echo "" >> "$OUTPUT_FILE"
    fi
    
    log_info "开始执行Code类型评估器冒烟测试"
    log_info "测试用例: $test_case"
    
    case "$test_case" in
        "all")
            test_create_python
            sleep 1
            test_create_js
            sleep 1
            test_update_python
            sleep 1
            test_update_js
            sleep 1
            test_debug_python
            sleep 1
            test_debug_js
            sleep 1
            test_error_syntax
            sleep 1
            test_error_security
            sleep 1
            test_mock_eval_target_output_basic
            sleep 1
            test_mock_eval_target_output_parameterized
            sleep 1
            test_mock_eval_target_output_invalid_id
            sleep 1
            test_mock_eval_target_output_invalid_version
            ;;
        "create-python")
            test_create_python
            ;;
        "create-js")
            test_create_js
            ;;
        "update-python")
            test_update_python
            ;;
        "update-js")
            test_update_js
            ;;
        "debug-python")
            test_debug_python
            ;;
        "debug-js")
            test_debug_js
            ;;
        "error-syntax")
            test_error_syntax
            ;;
        "error-security")
            test_error_security
            ;;
        "mock-basic")
            test_mock_eval_target_output_basic
            ;;
        "mock-parameterized")
            test_mock_eval_target_output_parameterized
            ;;
        "mock-invalid-id")
            test_mock_eval_target_output_invalid_id
            ;;
        "mock-invalid-version")
            test_mock_eval_target_output_invalid_version
            ;;
        *)
            log_error "未知的测试用例: $test_case"
            show_help
            exit 1
            ;;
    esac
    
    # 显示测试结果摘要
    show_summary
}

# 解析命令行参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -u|--url)
            BASE_URL="$2"
            shift 2
            ;;
        -w|--workspace)
            WORKSPACE_ID="$2"
            shift 2
            ;;
        -e|--evaluator)
            EVALUATOR_ID="$2"
            shift 2
            ;;
        -t|--target)
            EVAL_TARGET_ID="$2"
            shift 2
            ;;
        -tv|--target-version)
            EVAL_TARGET_VERSION_ID="$2"
            shift 2
            ;;
        -c|--cookie)
            COOKIE="$2"
            shift 2
            ;;
        -x|--env)
            ENV_VALUE="$2"
            shift 2
            ;;
        -o|--output)
            OUTPUT_FILE="$2"
            shift 2
            ;;
        -*)
            log_error "未知选项: $1"
            show_help
            exit 1
            ;;
        *)
            # 这是测试用例参数
            main "$1"
            exit $?
            ;;
    esac
done

# 如果没有指定测试用例，执行所有测试
main "all"