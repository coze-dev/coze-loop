# 用户具体用例解决方案

## 问题背景

用户提到的具体用例包含：
- 函数定义：`def exec_evaluation(turn):`
- 复杂的数据访问：`turn["from_eval_target_fields"]["actual_output"]["text"]`
- 字符串处理和比较
- 自定义对象返回：`EvalOutput(score=score, reason=reason)`

之前返回的是模拟的变量赋值日志，现在可以真正执行这些复杂的 Python 代码。

## 解决方案演示

### 1. 用户的评估代码示例

```python
class EvalOutput:
    def __init__(self, score, reason):
        self.score = score
        self.reason = reason
    
    def __str__(self):
        return f"EvalOutput(score={self.score}, reason='{self.reason}')"
    
    def to_dict(self):
        return {"score": self.score, "reason": self.reason}

def exec_evaluation(turn):
    """
    用户的评估函数 - 现在可以真正执行
    """
    try:
        # 复杂的数据访问
        actual_output = turn["from_eval_target_fields"]["actual_output"]["text"]
        expected_output = turn.get("expected_output", "")
        
        # 字符串处理和比较
        actual_clean = actual_output.strip().lower()
        expected_clean = expected_output.strip().lower()
        
        # 评估逻辑
        if actual_clean == expected_clean:
            score = 1.0
            reason = "输出完全匹配预期结果"
        elif actual_clean in expected_clean or expected_clean in actual_clean:
            score = 0.8
            reason = "输出部分匹配预期结果"
        elif len(actual_clean) > 0:
            score = 0.3
            reason = f"输出不匹配，实际: '{actual_output}', 预期: '{expected_output}'"
        else:
            score = 0.0
            reason = "输出为空"
        
        # 返回自定义对象
        return EvalOutput(score=score, reason=reason)
        
    except Exception as e:
        return EvalOutput(score=0.0, reason=f"评估过程出错: {str(e)}")

# 测试数据
test_turn = {
    "from_eval_target_fields": {
        "actual_output": {
            "text": "这是一个测试结果"
        }
    },
    "expected_output": "这是一个测试结果"
}

# 执行评估
result = exec_evaluation(test_turn)
print(f"评估完成: {result}")
print(f"分数: {result.score}")
print(f"原因: {result.reason}")

# 返回结构化结果
return_val(result.to_dict())
```

### 2. API 调用示例

```bash
curl -X POST http://localhost:8890/run_code \
  -H "Content-Type: application/json" \
  -d '{
    "language": "python",
    "code": "class EvalOutput:\n    def __init__(self, score, reason):\n        self.score = score\n        self.reason = reason\n    \n    def __str__(self):\n        return f\"EvalOutput(score={self.score}, reason='\''{self.reason}'\'')\"\n    \n    def to_dict(self):\n        return {\"score\": self.score, \"reason\": self.reason}\n\ndef exec_evaluation(turn):\n    try:\n        actual_output = turn[\"from_eval_target_fields\"][\"actual_output\"][\"text\"]\n        expected_output = turn.get(\"expected_output\", \"\")\n        \n        actual_clean = actual_output.strip().lower()\n        expected_clean = expected_output.strip().lower()\n        \n        if actual_clean == expected_clean:\n            score = 1.0\n            reason = \"输出完全匹配预期结果\"\n        elif actual_clean in expected_clean or expected_clean in actual_clean:\n            score = 0.8\n            reason = \"输出部分匹配预期结果\"\n        elif len(actual_clean) > 0:\n            score = 0.3\n            reason = f\"输出不匹配，实际: '\''{actual_output}'\'', 预期: '\''{expected_output}'\''\"\n        else:\n            score = 0.0\n            reason = \"输出为空\"\n        \n        return EvalOutput(score=score, reason=reason)\n        \n    except Exception as e:\n        return EvalOutput(score=0.0, reason=f\"评估过程出错: {str(e)}\")\n\ntest_turn = {\n    \"from_eval_target_fields\": {\n        \"actual_output\": {\n            \"text\": \"这是一个测试结果\"\n        }\n    },\n    \"expected_output\": \"这是一个测试结果\"\n}\n\nresult = exec_evaluation(test_turn)\nprint(f\"评估完成: {result}\")\nprint(f\"分数: {result.score}\")\nprint(f\"原因: {result.reason}\")\n\nreturn_val(result.to_dict())"
  }'
```

### 3. 预期响应

```json
{
  "output": {
    "stdout": "评估完成: EvalOutput(score=1.0, reason='输出完全匹配预期结果')\n分数: 1.0\n原因: 输出完全匹配预期结果\n",
    "stderr": "",
    "ret_val": "{'score': 1.0, 'reason': '输出完全匹配预期结果'}"
  },
  "metadata": {
    "language": "python",
    "runtime": "enhanced-real-executor",
    "duration": 67,
    "status": "success",
    "exit_code": 0,
    "timed_out": false
  }
}
```

## 对比：之前 vs 现在

### 之前（模拟器）
```json
{
  "output": {
    "stdout": "变量赋值: turn = {...}\n变量赋值: result = exec_evaluation(...)",
    "stderr": "",
    "ret_val": ""
  },
  "metadata": {
    "runtime": "stable-simulator"
  }
}
```

### 现在（真实执行）
```json
{
  "output": {
    "stdout": "评估完成: EvalOutput(score=1.0, reason='输出完全匹配预期结果')\n分数: 1.0\n原因: 输出完全匹配预期结果\n",
    "stderr": "",
    "ret_val": "{'score': 1.0, 'reason': '输出完全匹配预期结果'}"
  },
  "metadata": {
    "runtime": "enhanced-real-executor",
    "status": "success"
  }
}
```

## 更多复杂用例

### 1. 数据分析用例

```python
import json
from datetime import datetime

class DataAnalyzer:
    def __init__(self):
        self.results = []
    
    def analyze_conversation(self, conversation_data):
        """分析对话数据"""
        total_turns = len(conversation_data.get("turns", []))
        
        user_turns = [t for t in conversation_data.get("turns", []) if t.get("role") == "user"]
        assistant_turns = [t for t in conversation_data.get("turns", []) if t.get("role") == "assistant"]
        
        avg_user_length = sum(len(t.get("content", "")) for t in user_turns) / max(len(user_turns), 1)
        avg_assistant_length = sum(len(t.get("content", "")) for t in assistant_turns) / max(len(assistant_turns), 1)
        
        analysis = {
            "timestamp": datetime.now().isoformat(),
            "total_turns": total_turns,
            "user_turns": len(user_turns),
            "assistant_turns": len(assistant_turns),
            "avg_user_message_length": round(avg_user_length, 2),
            "avg_assistant_message_length": round(avg_assistant_length, 2),
            "conversation_balance": round(len(user_turns) / max(len(assistant_turns), 1), 2)
        }
        
        self.results.append(analysis)
        return analysis

# 测试数据
test_conversation = {
    "turns": [
        {"role": "user", "content": "你好，请帮我分析一下数据"},
        {"role": "assistant", "content": "好的，我来帮您分析数据。请提供具体的数据内容。"},
        {"role": "user", "content": "这是我的销售数据"},
        {"role": "assistant", "content": "收到您的销售数据，让我为您进行详细分析..."}
    ]
}

analyzer = DataAnalyzer()
result = analyzer.analyze_conversation(test_conversation)

print("对话分析结果:")
print(json.dumps(result, indent=2, ensure_ascii=False))

return_val(result)
```

### 2. 机器学习评估用例

```python
class MLEvaluator:
    def __init__(self):
        self.metrics = {}
    
    def calculate_accuracy(self, predictions, ground_truth):
        """计算准确率"""
        if len(predictions) != len(ground_truth):
            raise ValueError("预测结果和真实标签长度不匹配")
        
        correct = sum(1 for p, g in zip(predictions, ground_truth) if p == g)
        accuracy = correct / len(predictions)
        return accuracy
    
    def calculate_precision_recall(self, predictions, ground_truth, positive_label=1):
        """计算精确率和召回率"""
        tp = sum(1 for p, g in zip(predictions, ground_truth) if p == positive_label and g == positive_label)
        fp = sum(1 for p, g in zip(predictions, ground_truth) if p == positive_label and g != positive_label)
        fn = sum(1 for p, g in zip(predictions, ground_truth) if p != positive_label and g == positive_label)
        
        precision = tp / (tp + fp) if (tp + fp) > 0 else 0
        recall = tp / (tp + fn) if (tp + fn) > 0 else 0
        f1 = 2 * (precision * recall) / (precision + recall) if (precision + recall) > 0 else 0
        
        return {
            "precision": round(precision, 4),
            "recall": round(recall, 4),
            "f1_score": round(f1, 4)
        }
    
    def evaluate_model(self, predictions, ground_truth):
        """完整的模型评估"""
        accuracy = self.calculate_accuracy(predictions, ground_truth)
        pr_metrics = self.calculate_precision_recall(predictions, ground_truth)
        
        evaluation_result = {
            "accuracy": round(accuracy, 4),
            **pr_metrics,
            "total_samples": len(predictions),
            "evaluation_summary": f"模型在 {len(predictions)} 个样本上的准确率为 {accuracy:.2%}"
        }
        
        return evaluation_result

# 测试数据
test_predictions = [1, 0, 1, 1, 0, 1, 0, 0, 1, 1]
test_ground_truth = [1, 0, 1, 0, 0, 1, 0, 1, 1, 1]

evaluator = MLEvaluator()
result = evaluator.evaluate_model(test_predictions, test_ground_truth)

print("机器学习模型评估结果:")
for key, value in result.items():
    print(f"{key}: {value}")

return_val(result)
```

## 总结

现在用户可以：

1. **✅ 执行真正的函数定义和调用**
2. **✅ 处理复杂的数据结构和嵌套访问**
3. **✅ 创建和使用自定义类和对象**
4. **✅ 进行字符串处理、数值计算和逻辑判断**
5. **✅ 返回结构化的结果数据**
6. **✅ 处理异常和错误情况**
7. **✅ 使用标准库进行日期时间、JSON 等操作**

**错误码 601205032 (CodeExecutionFailedCode) 问题已完全解决**，用户的 Python 代码现在可以真正执行并返回预期的结果。