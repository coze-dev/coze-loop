#!/usr/bin/env python3
"""
测试FaaS输出结构改造
验证Python和JavaScript FaaS服务能够正确处理return_val函数输出
"""

import json
import requests
import time

def test_python_faas():
    """测试Python FaaS服务"""
    print("=== 测试Python FaaS服务 ===")
    
    # 测试代码：包含print输出和return_val调用
    test_code = '''
import json

def exec_evaluation(turn):
    # 用户代码的print输出
    print("Evaluation Results:")
    print("Score: 0.8")
    print("Reason: 测试评估成功")
    
    # 返回评估结果
    result = {
        "score": 0.8,
        "reason": "测试评估成功",
        "err_msg": ""
    }
    
    return result

# 模拟turn数据
turn = {"eval_set": {"input": "测试输入"}}

# 执行评估
result = exec_evaluation(turn)

# 调用return_val函数
return_val(json.dumps(result))
'''
    
    # 发送请求到Python FaaS服务
    response = requests.post(
        "http://localhost:8000/run_code",
        json={
            "language": "python",
            "code": test_code,
            "timeout": 10000
        }
    )
    
    print(f"状态码: {response.status_code}")
    if response.status_code == 200:
        result = response.json()
        print("Python FaaS响应:")
        print(json.dumps(result, indent=2, ensure_ascii=False))
        
        # 验证输出结构
        output = result.get("output", {})
        stdout = output.get("stdout", "")
        stderr = output.get("stderr", "")
        ret_val = output.get("ret_val", "")
        
        print(f"\nstdout: {stdout}")
        print(f"stderr: {stderr}")
        print(f"ret_val: {ret_val}")
        
        # 验证ret_val是否包含评估结果
        if ret_val:
            try:
                eval_result = json.loads(ret_val)
                print(f"解析的评估结果: {eval_result}")
                assert "score" in eval_result
                assert "reason" in eval_result
                print("✅ Python FaaS测试通过")
            except json.JSONDecodeError as e:
                print(f"❌ ret_val解析失败: {e}")
        else:
            print("❌ ret_val为空")
    else:
        print(f"❌ 请求失败: {response.text}")

def test_javascript_faas():
    """测试JavaScript FaaS服务"""
    print("\n=== 测试JavaScript FaaS服务 ===")
    
    # 测试代码：包含console.log输出和return_val调用
    test_code = '''
function exec_evaluation(turn) {
    // 用户代码的console.log输出
    console.log("Evaluation Results:");
    console.log("Score: 0.9");
    console.log("Reason: JavaScript测试评估成功");
    
    // 返回评估结果
    const result = {
        score: 0.9,
        reason: "JavaScript测试评估成功",
        err_msg: ""
    };
    
    return result;
}

// 模拟turn数据
const turn = {eval_set: {input: "测试输入"}};

// 执行评估
const result = exec_evaluation(turn);

// 调用return_val函数
return_val(JSON.stringify(result));
'''
    
    # 发送请求到JavaScript FaaS服务
    response = requests.post(
        "http://localhost:8001/run_code",
        json={
            "language": "javascript",
            "code": test_code,
            "timeout": 10000
        }
    )
    
    print(f"状态码: {response.status_code}")
    if response.status_code == 200:
        result = response.json()
        print("JavaScript FaaS响应:")
        print(json.dumps(result, indent=2, ensure_ascii=False))
        
        # 验证输出结构
        output = result.get("output", {})
        stdout = output.get("stdout", "")
        stderr = output.get("stderr", "")
        ret_val = output.get("ret_val", "")
        
        print(f"\nstdout: {stdout}")
        print(f"stderr: {stderr}")
        print(f"ret_val: {ret_val}")
        
        # 验证ret_val是否包含评估结果
        if ret_val:
            try:
                eval_result = json.loads(ret_val)
                print(f"解析的评估结果: {eval_result}")
                assert "score" in eval_result
                assert "reason" in eval_result
                print("✅ JavaScript FaaS测试通过")
            except json.JSONDecodeError as e:
                print(f"❌ ret_val解析失败: {e}")
        else:
            print("❌ ret_val为空")
    else:
        print(f"❌ 请求失败: {response.text}")

def main():
    """主测试函数"""
    print("FaaS输出结构改造测试")
    print("=" * 50)
    
    try:
        test_python_faas()
        test_javascript_faas()
    except requests.exceptions.ConnectionError:
        print("❌ 无法连接到FaaS服务，请确保服务正在运行")
    except Exception as e:
        print(f"❌ 测试失败: {e}")

if __name__ == "__main__":
    main()