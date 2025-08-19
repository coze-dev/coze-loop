# Sandbox API 接口文档

本文档详细描述了Cozeloop Sandbox代码执行环境的API接口。

## 基础信息

- **Base URL**: `http://localhost:8080`
- **Content-Type**: `application/json`
- **支持的语言**: `python`, `javascript`, `typescript`

## API 接口

### 1. 代码执行接口

执行用户提交的代码并返回执行结果。

#### 请求

**POST** `/api/v1/sandbox/run`

##### 请求头
```
Content-Type: application/json
```

##### 请求体参数

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| code | string | 是 | 要执行的代码内容 |
| language | string | 是 | 代码语言：python/javascript/typescript |
| eval_input | object | 否 | 评估输入数据 |

##### eval_input 结构
```json
{
  "run": {
    "input": {
      "content_type": "text",
      "text": "输入内容"
    },
    "output": {
      "content_type": "text", 
      "text": "用户输出"
    },
    "reference_output": {
      "content_type": "text",
      "text": "参考答案"
    }
  }
}
```

##### 请求示例

**Python代码执行**:
```json
{
  "code": "score = 1.0\nreason = '评估通过'",
  "language": "python",
  "eval_input": {
    "run": {
      "input": {"content_type": "text", "text": "2+2"},
      "output": {"content_type": "text", "text": "4"},
      "reference_output": {"content_type": "text", "text": "4"}
    }
  }
}
```

**JavaScript代码执行**:
```json
{
  "code": "const score = 1.0; const reason = '评估通过';",
  "language": "javascript"
}
```

#### 响应

##### 成功响应 (200 OK)
```json
{
  "success": true,
  "result": {
    "output": {
      "score": 1.0,
      "reason": "评估说明"
    },
    "success": true,
    "duration": 850000000
  }
}
```

##### 响应字段说明

| 字段 | 类型 | 描述 |
|------|------|------|
| success | boolean | 请求是否成功 |
| result.output.score | number | 评估分数 (0.0-1.0) |
| result.output.reason | string | 评估说明 |
| result.success | boolean | 代码执行是否成功 |
| result.duration | number | 执行时间 (纳秒) |

##### 错误响应 (400/500)
```json
{
  "success": false,
  "error": "错误描述"
}
```
### 2. 代码验证接口

验证代码是否可编译通过且执行，不关注执行过程中的具体错误，只返回验证是否通过的结果。

#### 请求

**POST** `/api/v1/sandbox/validate`

##### 请求头
```
Content-Type: application/json
```

##### 请求体参数

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| code | string | 是 | 要验证的代码内容 |
| language | string | 是 | 代码语言：python/javascript/typescript |

##### 请求示例

```json
{
  "code": "const score = 1.0; const reason = '测试';",
  "language": "javascript"
}
```

#### 响应

##### 成功响应 (200 OK)
```json
{
  "success": true,
  "valid": true
}
```

##### 响应字段说明

| 字段 | 类型 | 描述 |
|------|------|------|
| success | boolean | 请求是否成功 |
| valid | boolean | 代码是否验证通过 |

##### 错误响应 (400)
```json
{
  "success": false,
  "error": "错误描述"
}
```

### 3. 健康检查接口
### 2. 健康检查接口

检查服务运行状态。

#### 请求

**GET** `/api/v1/sandbox/health`

#### 响应

##### 成功响应 (200 OK)
```json
{
  "status": "healthy",
  "timestamp": "2024-01-01T12:00:00Z",
  "version": "1.0.0"
}
```

## 错误码说明

### HTTP状态码

| 状态码 | 描述 |
|--------|------|
| 200 | 请求成功 |
| 400 | 请求参数错误 |
| 500 | 服务器内部错误 |

### 业务错误码

| 错误类型 | 错误信息格式 | 描述 |
|----------|-------------|------|
| INVALID_LANGUAGE | `不支持的语言: {language}` | 不支持的编程语言 |
| EMPTY_CODE | `代码内容不能为空` | 代码内容为空 |
| RUNTIME_ERROR | `{language}执行失败: {error}` | 代码运行时错误 |
| TIMEOUT_ERROR | `代码执行超时` | 执行时间超过限制 |
| MEMORY_ERROR | `内存使用超出限制` | 内存使用超出限制 |

## 使用示例

### cURL示例

```bash
# 执行Python代码
curl -X POST http://localhost:8080/api/v1/sandbox/run \
  -H "Content-Type: application/json" \
  -d '{"code": "score = 1.0; reason = \"测试\"", "language": "python"}'

# 验证代码
curl -X POST http://localhost:8080/api/v1/sandbox/validate \
  -H "Content-Type: application/json" \
  -d '{"code": "const score = 1.0;", "language": "javascript"}'

# 健康检查
curl http://localhost:8080/api/v1/sandbox/health
```

### 客户端示例

**JavaScript**:
```javascript
async function runCode(code, language) {
  const response = await fetch('http://localhost:8080/api/v1/sandbox/run', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ code, language })
  });
  return await response.json();
}

async function validateCode(code, language) {
  const response = await fetch('http://localhost:8080/api/v1/sandbox/validate', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ code, language })
  });
  return await response.json();
}
```

**Go**:
```go
type RunCodeRequest struct {
    Code     string `json:"code"`
    Language string `json:"language"`
}

type ValidateCodeRequest struct {
    Code     string `json:"code"`
    Language string `json:"language"`
}

func runCode(code, language string) (*RunCodeResponse, error) {
    // 实现HTTP POST请求到 /api/v1/sandbox/run
}

func validateCode(code, language string) (*ValidateCodeResponse, error) {
    // 实现HTTP POST请求到 /api/v1/sandbox/validate
}
```

## 性能指标

### 执行时间

| 语言 | 首次执行 | 后续执行 |
|------|----------|----------|
| Python | 800-1000ms | 100-200ms |
| JavaScript | 50-100ms | 20-50ms |
| TypeScript | 100-150ms | 50-100ms |

### 资源限制

| 资源 | 限制 |
|------|------|
| 内存 | 128MB |
| 执行时间 | 30秒 |
| 输出大小 | 1MB |

### 并发性能

- **最大并发请求**: 100个
- **平均响应时间**: <200ms (JavaScript), <500ms (Python)
- **吞吐量**: 500 requests/second

## 注意事项

1. **首次Python执行较慢**: Pyodide初始化需要时间，建议预热
2. **代码安全性**: 所有代码在沙箱环境中执行，但仍需注意恶意代码
3. **网络访问**: 默认禁用网络访问，确保安全性
4. **包支持**: Python仅支持Pyodide预编译的包
5. **内存管理**: 大量数据处理可能触发内存限制