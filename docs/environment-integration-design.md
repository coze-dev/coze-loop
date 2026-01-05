# 业务环境打通设计方案

## 1. 背景与现状

### 1.1 业务A（Agent平台）
- **环境**: stg / ppe / online
- **隔离情况**: 三个环境资源、数据全部隔离

### 1.2 业务B（评测中台）
- **环境**: boe / ppe / online
- **隔离情况**:
  - boe 和 ppe/online 环境独立（资源、数据、网络不互通）
  - ppe 和 online 服务隔离，但网络、数据互通

### 1.3 需求
- B业务给A业务提供中台能力支持
- 需要将两个业务的测试环境打通
- 需要将两个业务的线上环境打通

## 2. 设计目标

1. **环境隔离性**: 保持各环境内部隔离，避免数据污染
2. **网络连通性**: 实现跨业务环境的网络互通
3. **配置灵活性**: 支持动态配置服务端点
4. **可维护性**: 方案简单清晰，易于维护和扩展
5. **安全性**: 确保跨环境调用的安全认证和授权

## 3. 环境映射方案

### 3.1 环境对应关系

| 业务A环境 | 业务B环境 | 说明 |
|---------|---------|------|
| stg | boe | 测试环境打通 |
| ppe | ppe | 预发环境打通（可选） |
| online | online | 线上环境打通 |

### 3.2 映射策略

**测试环境打通**:
- A业务 stg → B业务 boe
- 理由: boe是B业务的独立测试环境，与ppe/online隔离，适合与A的stg对接

**预发环境打通**（可选）:
- A业务 ppe → B业务 ppe
- 理由: 如果A业务需要预发验证，可以对接B的ppe环境

**线上环境打通**:
- A业务 online → B业务 online
- 理由: 生产环境对接

## 4. 技术架构设计

### 4.1 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                     业务A（Agent平台）                        │
├─────────────────────────────────────────────────────────────┤
│  stg环境          │  ppe环境          │  online环境         │
│  ┌─────────────┐  │  ┌─────────────┐  │  ┌─────────────┐   │
│  │  API Gateway│  │  │  API Gateway│  │  │  API Gateway│   │
│  └──────┬──────┘  │  └──────┬──────┘  │  └──────┬──────┘   │
│         │         │         │         │         │          │
│         └─────────┼─────────┼─────────┘         │          │
│                   │         │                   │          │
└───────────────────┼─────────┼───────────────────┼──────────┘
                    │         │                   │
                    │         │                   │
         ┌──────────┼─────────┼───────────────────┼──────────┐
         │          │         │                   │          │
         │  ┌───────▼─────────▼───────────────────▼──────┐   │
         │  │      跨环境服务路由层（Service Router）      │   │
         │  └───────┬─────────┬───────────────────┬──────┘   │
         │          │         │                   │          │
         └──────────┼─────────┼───────────────────┼──────────┘
                    │         │                   │
┌───────────────────┼─────────┼───────────────────┼──────────┐
│                   │         │                   │          │
│  ┌────────────────▼─────────▼───────────────────▼──────┐  │
│  │             业务B（评测中台）                          │  │
│  ├──────────────────────────────────────────────────────┤  │
│  │  boe环境          │  ppe环境          │  online环境   │  │
│  │  ┌─────────────┐  │  ┌─────────────┐  │  ┌─────────┐ │  │
│  │  │  API Server │  │  │  API Server │  │  │API Server│ │  │
│  │  └─────────────┘  │  └─────────────┘  │  └─────────┘ │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### 4.2 核心组件

#### 4.2.1 环境配置中心（Environment Config Center）

**职责**:
- 管理环境映射关系
- 存储各环境的服务端点配置
- 提供配置查询接口

**配置结构**:
```yaml
# 环境映射配置
environment_mapping:
  business_a:
    stg:
      target_business: "business_b"
      target_env: "boe"
      endpoint: "https://business-b-boe.example.com"
    ppe:
      target_business: "business_b"
      target_env: "ppe"
      endpoint: "https://business-b-ppe.example.com"
    online:
      target_business: "business_b"
      target_env: "online"
      endpoint: "https://business-b-online.example.com"

# 服务端点配置
service_endpoints:
  business_b:
    boe:
      api_base_url: "https://business-b-boe.example.com/api/v1"
      timeout: 30s
      retry_count: 3
    ppe:
      api_base_url: "https://business-b-ppe.example.com/api/v1"
      timeout: 30s
      retry_count: 3
    online:
      api_base_url: "https://business-b-online.example.com/api/v1"
      timeout: 30s
      retry_count: 3
```

#### 4.2.2 服务路由层（Service Router）

**职责**:
- 根据当前环境自动路由到目标业务环境
- 处理请求转发和响应返回
- 实现请求/响应的转换和适配

**实现方式**:
- **方案A**: 在业务A中实现路由逻辑（推荐）
  - 优点: 对业务B无侵入，业务A完全控制
  - 缺点: 需要在业务A中维护路由逻辑

- **方案B**: 独立的API网关层
  - 优点: 解耦，统一管理
  - 缺点: 增加一层网络跳转，需要额外维护

**推荐方案A**，在业务A中实现环境感知的路由逻辑。

#### 4.2.3 认证授权层（Auth Layer）

**职责**:
- 跨环境调用的身份认证
- 请求签名和验证
- 权限校验

**实现方案**:
```go
// 伪代码示例
type CrossEnvAuth struct {
    appID     string
    appSecret string
    env       string
}

func (a *CrossEnvAuth) SignRequest(req *http.Request) error {
    // 1. 添加环境标识头
    req.Header.Set("X-Source-Env", a.env)
    req.Header.Set("X-Source-Business", "business_a")

    // 2. 生成签名
    timestamp := time.Now().Unix()
    nonce := generateNonce()
    signature := generateSignature(a.appSecret, req, timestamp, nonce)

    // 3. 设置认证头
    req.Header.Set("X-Timestamp", strconv.FormatInt(timestamp, 10))
    req.Header.Set("X-Nonce", nonce)
    req.Header.Set("X-Signature", signature)
    req.Header.Set("X-App-ID", a.appID)

    return nil
}
```

## 5. 网络连通方案

### 5.1 网络拓扑

#### 5.1.1 测试环境（stg ↔ boe）

**方案**: 通过内网VPN或专线打通
- 如果两个环境在同一内网，直接配置路由
- 如果不在同一内网，通过VPN或专线连接

**网络配置**:
```
业务A stg环境:
  - 内网IP段: 10.0.1.0/24
  - 出站规则: 允许访问 10.0.2.0/24 (业务B boe)

业务B boe环境:
  - 内网IP段: 10.0.2.0/24
  - 入站规则: 允许来自 10.0.1.0/24 (业务A stg)
```

#### 5.1.2 线上环境（online ↔ online）

**方案**: 通过公网API或内网专线
- 优先使用内网专线（更安全、更稳定）
- 备选公网HTTPS（需要配置域名和证书）

**网络配置**:
```
业务A online环境:
  - 内网IP段: 10.1.1.0/24
  - 出站规则: 允许访问 10.1.2.0/24 (业务B online)
  - 公网域名: api-a-online.example.com

业务B online环境:
  - 内网IP段: 10.1.2.0/24
  - 入站规则: 允许来自 10.1.1.0/24 (业务A online)
  - 公网域名: api-b-online.example.com
```

### 5.2 防火墙规则

**业务A侧**:
- 允许出站访问业务B各环境的API端点
- 限制访问端口（仅HTTPS 443或内网指定端口）

**业务B侧**:
- 允许入站来自业务A各环境的请求
- 配置白名单IP段
- 限制访问端口

## 6. 数据隔离方案

### 6.1 数据隔离原则

1. **数据库隔离**: 各环境使用独立的数据库实例
2. **缓存隔离**: 各环境使用独立的Redis实例
3. **存储隔离**: 各环境使用独立的对象存储bucket

### 6.2 跨环境数据传递

**原则**: 仅传递业务数据，不共享基础设施数据

**数据传递方式**:
- **API调用**: 通过RESTful API传递业务数据
- **消息队列**: 如需异步，使用MQ传递（需要打通MQ网络）
- **文件传输**: 大文件通过对象存储共享（需要配置跨环境访问权限）

### 6.3 数据标识

在跨环境调用时，需要明确标识数据来源环境：

```json
{
  "request_id": "req_123456",
  "source_env": "stg",
  "source_business": "business_a",
  "target_env": "boe",
  "target_business": "business_b",
  "data": {
    // 业务数据
  }
}
```

## 7. 实现方案

### 7.1 业务A侧实现

#### 7.1.1 环境配置管理

创建环境配置模块：

```go
// backend/pkg/env/config.go
package env

type EnvironmentConfig struct {
    CurrentEnv    string
    BusinessName  string
    TargetMapping map[string]TargetConfig
}

type TargetConfig struct {
    Business    string
    Environment string
    Endpoint    string
    AppID       string
    AppSecret   string
}

func GetTargetConfig(currentEnv string) (*TargetConfig, error) {
    // 从配置中心或配置文件读取
    // 返回目标环境的配置
}
```

#### 7.1.2 跨环境HTTP客户端

```go
// backend/infra/http/cross_env_client.go
package http

type CrossEnvClient struct {
    config     *env.TargetConfig
    httpClient *http.Client
    auth       *CrossEnvAuth
}

func (c *CrossEnvClient) DoRequest(ctx context.Context, req *RequestParam) error {
    // 1. 根据当前环境获取目标配置
    targetConfig, err := env.GetTargetConfig(c.currentEnv)
    if err != nil {
        return err
    }

    // 2. 构建目标URL
    targetURL := targetConfig.Endpoint + req.RequestURI

    // 3. 创建请求
    httpReq, err := http.NewRequestWithContext(ctx, req.Method, targetURL, body)
    if err != nil {
        return err
    }

    // 4. 添加认证信息
    auth := NewCrossEnvAuth(targetConfig)
    if err := auth.SignRequest(httpReq); err != nil {
        return err
    }

    // 5. 发送请求
    resp, err := c.httpClient.Do(httpReq)
    // ... 处理响应
}
```

#### 7.1.3 环境感知的服务调用

在需要调用业务B的地方，使用环境感知的客户端：

```go
// backend/modules/evaluation/infra/external/business_b_client.go
package external

type BusinessBClient struct {
    crossEnvClient *http.CrossEnvClient
}

func (c *BusinessBClient) CallEvaluationAPI(ctx context.Context, req *EvaluationRequest) (*EvaluationResponse, error) {
    // 自动根据当前环境路由到对应的业务B环境
    return c.crossEnvClient.DoRequest(ctx, &http.RequestParam{
        Method:     "POST",
        RequestURI: "/api/v1/evaluation",
        Body:       req,
    })
}
```

### 7.2 业务B侧实现

#### 7.2.1 环境识别中间件

```go
// backend/infra/middleware/env_identifier.go
package middleware

func EnvIdentifierMW(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 从请求头中提取环境信息
        sourceEnv := r.Header.Get("X-Source-Env")
        sourceBusiness := r.Header.Get("X-Source-Business")

        // 将环境信息注入到context中
        ctx := context.WithValue(r.Context(), "source_env", sourceEnv)
        ctx = context.WithValue(ctx, "source_business", sourceBusiness)

        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

#### 7.2.2 认证验证中间件

```go
// backend/infra/middleware/cross_env_auth.go
package middleware

func CrossEnvAuthMW(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 1. 提取认证信息
        appID := r.Header.Get("X-App-ID")
        timestamp := r.Header.Get("X-Timestamp")
        nonce := r.Header.Get("X-Nonce")
        signature := r.Header.Get("X-Signature")

        // 2. 验证签名
        if !verifySignature(appID, r, timestamp, nonce, signature) {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        // 3. 验证时间戳（防止重放攻击）
        if !verifyTimestamp(timestamp) {
            http.Error(w, "Request expired", http.StatusUnauthorized)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

#### 7.2.3 白名单配置

```yaml
# backend/conf/cross_env_whitelist.yaml
whitelist:
  business_a:
    stg:
      - ip_range: "10.0.1.0/24"
        app_id: "app_a_stg"
        app_secret: "${APP_A_STG_SECRET}"
    ppe:
      - ip_range: "10.0.3.0/24"
        app_id: "app_a_ppe"
        app_secret: "${APP_A_PPE_SECRET}"
    online:
      - ip_range: "10.1.1.0/24"
        app_id: "app_a_online"
        app_secret: "${APP_A_ONLINE_SECRET}"
```

## 8. 配置管理

### 8.1 配置文件结构

```
backend/
├── conf/
│   ├── environment.yaml          # 环境基础配置
│   ├── cross_env_mapping.yaml    # 跨环境映射配置
│   └── cross_env_whitelist.yaml  # 跨环境白名单配置
```

### 8.2 环境变量

```bash
# 业务A环境变量
CURRENT_ENV=stg                    # 当前环境
BUSINESS_NAME=business_a           # 业务名称
TARGET_BUSINESS_B_ENDPOINT=...    # 目标业务B端点（可选，优先使用配置文件）

# 业务B环境变量
ENABLE_CROSS_ENV_AUTH=true        # 是否启用跨环境认证
CROSS_ENV_WHITELIST_FILE=...      # 白名单配置文件路径
```

## 9. 监控与运维

### 9.1 监控指标

1. **跨环境调用指标**:
   - 调用次数（按环境、按接口）
   - 调用成功率
   - 调用延迟（P50/P95/P99）
   - 错误率（按错误类型）

2. **网络指标**:
   - 网络连通性
   - 带宽使用情况
   - 丢包率

3. **安全指标**:
   - 认证失败次数
   - 非法访问尝试
   - IP白名单命中率

### 9.2 日志规范

**业务A侧日志**:
```
[CrossEnv] source_env=stg target_business=business_b target_env=boe
           endpoint=/api/v1/evaluation status=200 latency=150ms
```

**业务B侧日志**:
```
[CrossEnv] source_business=business_a source_env=stg
           endpoint=/api/v1/evaluation status=200 latency=150ms
```

### 9.3 告警规则

1. **调用失败率 > 5%**: 告警
2. **调用延迟 P95 > 1s**: 告警
3. **认证失败次数 > 10次/分钟**: 告警
4. **网络不通**: 立即告警

## 10. 安全考虑

### 10.1 认证机制

- **AppID + AppSecret**: 基础认证
- **请求签名**: 防止请求被篡改
- **时间戳验证**: 防止重放攻击
- **Nonce机制**: 防止重放攻击

### 10.2 授权机制

- **IP白名单**: 限制来源IP
- **环境白名单**: 限制允许的环境
- **接口权限**: 细粒度的接口访问控制

### 10.3 数据安全

- **HTTPS传输**: 所有跨环境调用使用HTTPS
- **敏感数据加密**: 敏感数据在传输前加密
- **数据脱敏**: 日志中敏感数据脱敏

## 11. 实施步骤

### 阶段一: 基础设施准备（1-2周）

1. **网络打通**:
   - [ ] 配置stg ↔ boe网络连通
   - [ ] 配置online ↔ online网络连通
   - [ ] 配置防火墙规则

2. **配置准备**:
   - [ ] 创建环境映射配置文件
   - [ ] 配置服务端点
   - [ ] 生成AppID和AppSecret

### 阶段二: 业务A侧开发（2-3周）

1. **环境配置模块**:
   - [ ] 实现环境配置读取
   - [ ] 实现环境映射逻辑

2. **跨环境客户端**:
   - [ ] 实现CrossEnvClient
   - [ ] 实现认证签名逻辑
   - [ ] 实现请求路由逻辑

3. **集成测试**:
   - [ ] 单元测试
   - [ ] 集成测试
   - [ ] 端到端测试

### 阶段三: 业务B侧开发（2-3周）

1. **认证中间件**:
   - [ ] 实现环境识别中间件
   - [ ] 实现认证验证中间件
   - [ ] 实现白名单验证

2. **接口适配**:
   - [ ] 适配现有接口支持跨环境调用
   - [ ] 添加环境标识处理

3. **集成测试**:
   - [ ] 单元测试
   - [ ] 集成测试
   - [ ] 安全测试

### 阶段四: 联调与上线（1-2周）

1. **环境联调**:
   - [ ] stg ↔ boe联调
   - [ ] online ↔ online联调
   - [ ] 性能测试
   - [ ] 压力测试

2. **监控配置**:
   - [ ] 配置监控指标
   - [ ] 配置告警规则
   - [ ] 配置日志收集

3. **灰度发布**:
   - [ ] 小流量灰度
   - [ ] 逐步放量
   - [ ] 全量上线

## 12. 风险与应对

### 12.1 技术风险

| 风险 | 影响 | 应对措施 |
|-----|------|---------|
| 网络不稳定 | 高 | 实现重试机制、熔断机制 |
| 配置错误 | 高 | 配置校验、配置热更新 |
| 性能问题 | 中 | 连接池、异步调用、限流 |

### 12.2 安全风险

| 风险 | 影响 | 应对措施 |
|-----|------|---------|
| 未授权访问 | 高 | 严格认证、IP白名单 |
| 数据泄露 | 高 | HTTPS、数据加密 |
| 重放攻击 | 中 | 时间戳验证、Nonce机制 |

### 12.3 运维风险

| 风险 | 影响 | 应对措施 |
|-----|------|---------|
| 环境配置不一致 | 中 | 配置中心统一管理 |
| 故障排查困难 | 中 | 完善日志、链路追踪 |
| 回滚困难 | 中 | 版本管理、快速回滚机制 |

## 13. 总结

本方案通过以下核心设计实现两个业务环境的打通：

1. **环境映射**: 明确的环境对应关系（stg↔boe, online↔online）
2. **服务路由**: 环境感知的自动路由机制
3. **安全认证**: 完善的跨环境认证授权机制
4. **网络连通**: 内网优先的网络打通方案
5. **数据隔离**: 保持各环境数据隔离，仅传递业务数据

该方案在保证环境隔离性的同时，实现了跨环境的服务调用，满足业务需求。

