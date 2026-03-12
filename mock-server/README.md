# Code Evaluator Mock Server

基于 Express 的 Code Evaluator API Mock Server，用于开发和测试。

## 项目结构

```
mock-server/
├── src/
│   ├── types/          # 类型定义模块
│   ├── mock-data/      # Mock 数据模块
│   ├── api/            # API 路由模块
│   └── index.ts        # 服务器入口
├── package.json
├── tsconfig.json
└── README.md
```

## 环境变量

- `PORT`: 服务器端口（默认: 9999）

## 安装依赖

```bash
npm install
```

## 开发

启动开发服务器

```bash
npm run dev
```

## API 端点

### 基础信息

- **服务器地址**: `http://localhost:9999`
- **API 基础路径**: `/api/xxx/xxx`
- **健康检查**: `GET /health`

### 可用接口

#### 1. xxxx
- **接口**: `POST /api/xxx/xxx/api`
- **描述**: xxxxxxx

#### 2. xxxx
- **接口**: `POST /api/xxx/xxx/api2`
- **描述**: xxxxxxx
