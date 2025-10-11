#!/usr/bin/env deno run --allow-net --allow-env --allow-read --allow-write --allow-run

/// <reference types="https://deno.land/x/deno@v1.45.5/cli/dts/lib.deno.d.ts" />
// @deno-types="https://deno.land/x/deno@v1.45.5/cli/dts/lib.deno.d.ts"

/**
 * Pyodide Python FaaS 服务器 (池化优化版)
 *
 * 使用 Pyodide WebAssembly Python 执行环境
 * 基于进程池和预加载技术优化执行速度
 * 基于 deno run -A jsr:@eyurtsev/pyodide-sandbox
 */

import { PyodidePoolManager, type PoolConfig, type ExecutionResult } from "./pyodide_pool_manager.ts";

// ==================== 类型定义 ====================

interface HealthStatus {
  status: string;
  timestamp: string;
  runtime: string;
  version: string;
  execution_count: number;
  python_version?: string;
  security: {
    sandbox: string;
    isolation: string;
    permissions: string;
  };
}

// ==================== 池化执行器 ====================

class PooledPyodideExecutor {
  private poolManager: PyodidePoolManager;
  private executionCount = 0;

  constructor(poolConfig?: Partial<PoolConfig>) {
    this.poolManager = new PyodidePoolManager(poolConfig);
  }

  /**
   * 启动执行器
   */
  async start(): Promise<void> {
    await this.poolManager.start();
  }

  /**
   * 执行 Python 代码（使用池化Pyodide）
   */
  async executePython(code: string, timeout = 30000): Promise<ExecutionResult> {
    this.executionCount++;
    console.log(`🚀 执行 Python 代码 (池化Pyodide)，超时: ${timeout}ms`);

    try {
      const result = await this.poolManager.executePython(code, timeout);
      console.log(`✅ 执行完成，耗时: ${result.metadata.duration}ms，进程: ${result.metadata.processId}`);
      return result;
    } catch (error) {
      console.error("❌ 池化执行失败:", error);
      throw error;
    }
  }

  /**
   * 获取执行统计
   */
  getExecutionCount(): number {
    return this.executionCount;
  }

  /**
   * 获取池状态
   */
  getPoolStatus() {
    return this.poolManager.getPoolStatus();
  }

  /**
   * 关闭执行器
   */
  async shutdown(): Promise<void> {
    await this.poolManager.shutdown();
  }
}

// ==================== Pyodide FaaS 服务器 ====================

class PyodideFaaSServer {
  private readonly executor: PooledPyodideExecutor;
  private readonly startTime = Date.now();

  constructor() {
    // 配置进程池参数
    const poolConfig: Partial<PoolConfig> = {
      minSize: parseInt(Deno.env.get("FAAS_POOL_MIN_SIZE") || "2"),
      maxSize: parseInt(Deno.env.get("FAAS_POOL_MAX_SIZE") || "8"),
      idleTimeout: parseInt(Deno.env.get("FAAS_POOL_IDLE_TIMEOUT") || "300000"), // 5分钟
      maxExecutionTime: parseInt(Deno.env.get("FAAS_MAX_EXECUTION_TIME") || "30000"), // 30秒
      preloadTimeout: parseInt(Deno.env.get("FAAS_PRELOAD_TIMEOUT") || "60000"), // 1分钟
    };

    this.executor = new PooledPyodideExecutor(poolConfig);
  }

  /**
   * 启动服务器
   */
  async start(): Promise<void> {
    console.log("🚀 启动池化Pyodide执行器...");
    await this.executor.start();
    console.log("✅ 池化Pyodide执行器启动完成");
  }

  /**
   * 处理代码执行请求
   */
  async handleRunCode(request: Request): Promise<Response> {
    try {
      let body;
      try {
        body = await request.json();
      } catch (jsonError) {
        console.error("JSON解析错误:", jsonError);
        return new Response(
          JSON.stringify({
            error: "Invalid JSON format",
            details: jsonError instanceof Error ? jsonError.message : String(jsonError)
          }),
          { status: 400, headers: { "Content-Type": "application/json" } }
        );
      }

      const { language, code, timeout = 30000 } = body;

      if (!code) {
        return new Response(
          JSON.stringify({ error: "Missing required parameter: code" }),
          { status: 400, headers: { "Content-Type": "application/json" } }
        );
      }

      if (typeof code !== 'string') {
        return new Response(
          JSON.stringify({ error: "Parameter 'code' must be a string" }),
          { status: 400, headers: { "Content-Type": "application/json" } }
        );
      }

      // 语言检查
      if (language && !["python", "py"].includes(language.toLowerCase())) {
        return new Response(
          JSON.stringify({ error: "This service only supports Python code execution" }),
          { status: 400, headers: { "Content-Type": "application/json" } }
        );
      }

      console.log(`📝 执行Python代码，长度: ${code.length}字符，超时: ${timeout}ms`);

      const startTime = Date.now();
      const result = await this.executor.executePython(code, timeout);
      const duration = Date.now() - startTime;

      const response = {
        output: {
          stdout: result.stdout,
          stderr: result.stderr,
          ret_val: result.returnValue
        },
        workload_info: {
          id: "e6008730-9475-4b7d-9fc6-19511e1b2785",
          status: "Used"
        },
        metadata: {
          language: "python",
          runtime: "pyodide-webassembly",
          duration,
          status: result.metadata.exitCode === 0 ? "success" : "error",
          exit_code: result.metadata.exitCode,
          timed_out: result.metadata.timedOut
        }
      };

      console.log(`✅ 执行完成，耗时: ${duration}ms，退出码: ${result.metadata.exitCode}`);

      return new Response(JSON.stringify(response), {
        status: 200,
        headers: { "Content-Type": "application/json" }
      });

    } catch (error) {
      console.error("❌ 处理run_code请求时发生错误:", error);
      const errorMessage = error instanceof Error ? error.message : String(error);

      let statusCode = 500;
      let errorType = "Execution failed";

      if (error instanceof SyntaxError) {
        statusCode = 400;
        errorType = "JSON parsing error";
      } else if (errorMessage.includes('timeout')) {
        statusCode = 408;
        errorType = "Execution timeout";
      }

      return new Response(
        JSON.stringify({
          error: errorType,
          details: errorMessage
        }),
        { status: statusCode, headers: { "Content-Type": "application/json" } }
      );
    }
  }

  /**
   * 处理健康检查
   */
  handleHealth(): Response {
    const poolStatus = this.executor.getPoolStatus();
    const healthData: HealthStatus = {
      status: "healthy",
      timestamp: new Date().toISOString(),
      runtime: "pyodide-webassembly-pooled",
      version: "pyodide-faas-v2.0.0-pooled",
      execution_count: this.executor.getExecutionCount(),
      python_version: "Pyodide WebAssembly Python (Pooled)",
      security: {
        sandbox: "pyodide-webassembly",
        isolation: "deno-permissions",
        permissions: "restricted"
      }
    };

    // 添加池状态信息
    const responseData = {
      ...healthData,
      pool_status: poolStatus
    };

    return new Response(JSON.stringify(responseData), {
      status: 200,
      headers: { "Content-Type": "application/json" }
    });
  }

  /**
   * 处理指标请求
   */
  handleMetrics(): Response {
    const uptime = Date.now() - this.startTime;
    const poolStatus = this.executor.getPoolStatus();
    const metrics = {
      execution_count: this.executor.getExecutionCount(),
      uptime_seconds: Math.floor(uptime / 1000),
      runtime: "pyodide-webassembly-pooled",
      python_version: "Pyodide WebAssembly Python (Pooled)",
      status: "healthy",
      pool_metrics: poolStatus
    };

    return new Response(JSON.stringify(metrics), {
      status: 200,
      headers: { "Content-Type": "application/json" }
    });
  }

  /**
   * 关闭服务器
   */
  async shutdown(): Promise<void> {
    console.log("🛑 关闭Pyodide FaaS服务器...");
    await this.executor.shutdown();
    console.log("✅ Pyodide FaaS服务器关闭完成");
  }
}

// ==================== 主函数 ====================

async function main() {
  const port = parseInt(Deno.env.get("FAAS_PORT") || "8000");

  console.log(`🚀 启动 Pyodide Python FaaS 服务器 (池化优化版)，端口: ${port}...`);
  console.log("🔒 安全特性: Deno 权限控制 + Pyodide WebAssembly 沙箱");
  console.log("⚡ 运行模式: 池化 Pyodide WebAssembly Python 执行器");
  console.log("🏊 性能优化: 进程池 + Pyodide 预加载");

  const faasServer = new PyodideFaaSServer();

  // 启动服务器（包括进程池初始化）
  await faasServer.start();

  const handler = async (request: Request): Promise<Response> => {
    const url = new URL(request.url);
    const method = request.method;

    console.log(`${method} ${url.pathname}`);

    // 路由处理
    switch (url.pathname) {
      case "/health":
        return faasServer.handleHealth();

      case "/metrics":
        return faasServer.handleMetrics();

      case "/run_code":
        if (method === "POST") {
          return await faasServer.handleRunCode(request);
        }
        break;
    }

    return new Response("Not Found", { status: 404 });
  };

  // 启动服务器
  Deno.serve({
    port,
    hostname: "0.0.0.0"
  }, handler);

  console.log(`✅ Pyodide Python FaaS 服务器启动成功: http://0.0.0.0:${port}`);
  console.log("📡 可用端点:");
  console.log("  GET  /health    - 健康检查 (包含池状态)");
  console.log("  GET  /metrics   - 指标信息 (包含池指标)");
  console.log("  POST /run_code  - 执行 Python 代码 (池化Pyodide)");
  console.log("");
  console.log("🔐 安全保障:");
  console.log("  ✅ Deno 权限控制");
  console.log("  ✅ Pyodide WebAssembly 沙箱");
  console.log("  ✅ 代码执行隔离");
  console.log("");
  console.log("⚡ 性能优化特性:");
  console.log("  ✅ 进程池管理 (2-8个进程)");
  console.log("  ✅ Pyodide 预加载");
  console.log("  ✅ 智能负载均衡");
  console.log("  ✅ 空闲进程自动清理");
  console.log("  ✅ 连接复用");
  console.log("");
  console.log("🐍 Python 执行特性:");
  console.log("  ✅ WebAssembly Python 执行");
  console.log("  ✅ 完整的 Python 标准库");
  console.log("  ✅ stdout/stderr 捕获");
  console.log("  ✅ return_val 函数支持");
  console.log("  ✅ 执行超时控制");
  console.log("  ✅ API 兼容性");
}

if (import.meta.main) {
  await main();
}
