# FaaS输出结构改造说明

## 改造概述

本次改造旨在简化FaaS服务的输出结构，让业务层不再处理复杂的嵌套JSON解析。改造后的输出结构更清晰，职责分离更明确。

## 目标输出结构

```json
{
    "output": {
        "stdout": "Evaluation Results:\nScore: 0.8\nReason: 测试评估成功",
        "stderr": "",
        "ret_val": "{\"score\": 0.8, \"reason\": \"测试评估成功\", \"err_msg\": \"\"}"
    }
}
```

## 改造内容

### 1. Python FaaS服务改造

**文件**: `release/deployment/docker-compose/bootstrap/python-faas/python_faas_server.py`

**主要改动**:
- 在`PythonExecutor`中添加`_capture_return_val`方法来捕获`return_val`函数的输出
- 在代码执行命名空间中注入`return_val`函数
- 将`return_val`函数的输出单独存储在`return_val_output`中，作为`ret_val`字段返回

**核心实现**:
```python
def _capture_return_val(self, value):
    """捕获return_val函数的输出"""
    self.return_val_output = value

def execute_python(self, code, timeout=30000):
    # 创建命名空间并注入return_val函数
    namespace = {
        'return_val': self._capture_return_val
    }
    
    with redirect_stdout(stdout_capture), redirect_stderr(stderr_capture):
        exec(code, namespace)
    
    return {
        "stdout": stdout_capture.getvalue(),
        "stderr": stderr_capture.getvalue(),
        "returnValue": self.return_val_output if self.return_val_output is not None else ""
    }
```

### 2. JavaScript FaaS服务改造

**文件**: `release/deployment/docker-compose/bootstrap/js-faas/js_faas_server.ts`

**主要改动**:
- 在包装代码中实现`return_val`函数来捕获返回值
- 分离用户的`console.log`输出和`return_val`的输出
- 确保`stdout`包含用户代码的输出，`ret_val`包含评估结果

**核心实现**:
```javascript
let returnValue = '';

// 实现return_val函数来捕获返回值
function return_val(value) {
  returnValue = value;
}

try {
  // 执行用户代码
  ${code}
  
  // 输出最终结果给FaaS服务解析
  console.log(JSON.stringify({
    stdout: stdout,
    stderr: stderr,
    ret_val: returnValue
  }));
}
```

### 3. 业务层简化

**文件**: `backend/modules/evaluation/domain/service/evaluator_source_code_impl.go`

**主要改动**:
- 简化`cleanStdoutForUser`函数，直接返回FaaS的`stdout`
- 简化`parseEvaluationExecutionResult`函数，直接从`ret_val`字段解析评估结果
- 移除复杂的嵌套JSON解析逻辑

**核心实现**:
```go
// 直接使用FaaS返回的stdout，不再做复杂解析
func (c *EvaluatorSourceCodeServiceImpl) cleanStdoutForUser(stdout string) string {
    return stdout
}

// 简化评估结果解析
func (c *EvaluatorSourceCodeServiceImpl) parseEvaluationExecutionResult(result *entity.ExecutionResult) (*entity.EvaluatorResult, error) {
    // 直接从RetVal字段解析score和reason
    if result.Output != nil && result.Output.RetVal != "" {
        if score, reason, _, parseErr := c.parseEvaluationRetVal(result.Output.RetVal); parseErr == nil {
            // 构造评估结果
        }
    }
}
```

## 模板中的return_val函数

模板中的`{{RETURN_VAL_FUNCTION}}`占位符已经在CodeBuilder中实现：

### Python模板
```python
def return_val(value):
    """
    标准return_val函数实现 - 输出返回值供FaaS服务捕获
    """
    print(value, flush=True)
```

### JavaScript模板
```javascript
function return_val(value) {
    /**
     * 标准return_val函数实现 - 输出返回值供FaaS服务捕获
     */
    console.log(value);
}
```

## 数据流程

### 改造前
1. 用户代码执行 → 复杂的stdout输出（包含系统信息和用户输出）
2. 业务层复杂解析 → 从stdout中提取JSON → 嵌套解析

### 改造后
1. 用户代码执行 → `print/console.log`输出到`stdout`，`return_val`输出到`ret_val`
2. FaaS服务分离处理 → 清晰的输出结构
3. 业务层简单解析 → 直接使用`stdout`和`ret_val`

## 优势

1. **职责分离**: FaaS服务负责执行和输出分离，业务层负责结果解析
2. **简化解析**: 业务层不再需要复杂的嵌套JSON解析
3. **清晰结构**: `stdout`用于用户输出，`ret_val`用于评估结果
4. **向后兼容**: 保持现有HTTP FaaS接口不变
5. **易于维护**: 代码逻辑更清晰，错误处理更简单

## 测试验证

使用`test_faas_output_structure.py`脚本可以验证改造效果：

```bash
python test_faas_output_structure.py
```

该脚本会测试：
- Python FaaS服务的输出结构
- JavaScript FaaS服务的输出结构  
- `stdout`和`ret_val`字段的正确性
- 评估结果的解析能力

## 注意事项

1. **FaaS服务启动**: 测试前需要确保Python FaaS（端口8000）和JavaScript FaaS（端口8001）服务正在运行
2. **端口配置**: 如果FaaS服务使用不同端口，需要相应调整测试脚本
3. **错误处理**: 改造后的错误处理更直接，错误信息会直接出现在`stderr`字段中
4. **性能影响**: 改造后的处理逻辑更简单，性能应有所提升