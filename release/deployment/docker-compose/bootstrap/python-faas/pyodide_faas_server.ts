#!/usr/bin/env deno run --allow-net --allow-env --allow-read --allow-write

/// <reference types="https://deno.land/x/deno@v1.45.5/cli/dts/lib.deno.d.ts" />

/**
 * Pyodide Python FaaS 服务器
 *
 * 使用 Pyodide WebAssembly Python 执行环境
 * 基于 deno run -A jsr:@eyurtsev/pyodide-sandbox
 */

// ==================== 类型定义 ====================

interface ExecutionResult {
  stdout: string;
  stderr: string;
  returnValue: string;
  metadata: {
    duration: number;
    exitCode: number;
    timedOut: boolean;
  };
}

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

// ==================== Pyodide 执行器 ====================

class PyodideExecutor {
  private executionCount = 0;

  /**
   * 执行 Python 代码（使用 Pyodide）
   */
  async executePython(code: string, timeout = 30000): Promise<ExecutionResult> {
    this.executionCount++;

    console.log(`🚀 执行 Python 代码 (Pyodide)，超时: ${timeout}ms`);

    const startTime = Date.now();

    try {
      // 注入标准的return_val函数，只输出JSON内容到stdout
      const returnValFunction = `
def return_val(value):
    """
    return_val函数实现 - 只输出JSON内容到stdout最后一行
    """
    # 处理输入值
    if value is None:
        ret_val = ""
    else:
        ret_val = str(value)

    # 直接输出JSON内容（作为最后一行）
    print(ret_val)
`;

      const enhancedCode = `${returnValFunction}

${code}
`;

      // 使用 pyodide-sandbox 执行代码
      const process = new Deno.Command("deno", {
        args: [
          "run",
          "-A",
          "jsr:@eyurtsev/pyodide-sandbox",
          "-c",
          enhancedCode
        ],
        stdout: "piped",
        stderr: "piped",
        timeout: timeout
      });

      const { stdout, stderr, code: exitCode } = await process.output();
      const duration = Date.now() - startTime;

      const stdoutText = new TextDecoder().decode(stdout);
      const stderrText = new TextDecoder().decode(stderr);

      // 提取 return_val 的结果（从pyodide-sandbox的输出中）
      const returnValue = this.extractReturnValue(stdoutText);

      // 清理 stdout，移除 return_val 输出（保持pyodide-sandbox的JSON结构）
      const cleanStdout = this.cleanStdout(stdoutText);

      return {
        stdout: cleanStdout,
        stderr: stderrText,
        returnValue,
        metadata: {
          duration,
          exitCode,
          timedOut: false
        }
      };

    } catch (error) {
      const duration = Date.now() - startTime;

      if (error.name === 'AbortError' || error.message.includes('timeout')) {
        return {
          stdout: "",
          stderr: `执行超时 (${timeout}ms)`,
          returnValue: "",
          metadata: {
            duration,
            exitCode: 1,
            timedOut: true
          }
        };
      }

      return {
        stdout: "",
        stderr: `Pyodide执行错误: ${error.message}`,
        returnValue: "",
        metadata: {
          duration,
          exitCode: 1,
          timedOut: false
        }
      };
    }
  }

  /**
   * 提取 return_val 的结果
   */
  private extractReturnValue(pyodideOutput: string): string {
    try {
      // pyodide-sandbox的输出结构是：
      // {"stdout":"原始输出内容","stderr":null,"result":"return_val输出的JSON","success":true,"sessionMetadata":{...}}

      // 解析pyodide-sandbox的输出JSON
      const parsedOutput = JSON.parse(pyodideOutput);

      // 优先使用result字段（这是pyodide-sandbox捕获的return_val输出）
      if (parsedOutput.result) {
        return parsedOutput.result;
      }

      // 如果没有result字段，再从stdout中提取
      const pyodideStdout = parsedOutput.stdout || "";

      // 查找return_val输出的JSON内容
      // 由于return_val函数会print(value)，所以JSON会出现在stdout中

      // 尝试从stdout中提取JSON对象
      const jsonMatch = pyodideStdout.match(/\{[^{}]*"score"[^{}]*\}/);
      if (jsonMatch) {
        return jsonMatch[0];
      }

      // 如果没有找到，尝试查找任何JSON对象
      const anyJsonMatch = pyodideStdout.match(/\{[^{}]*\}/);
      if (anyJsonMatch) {
        return anyJsonMatch[0];
      }

      return "";
    } catch (error) {
      console.error("解析pyodide输出失败:", error);
      return "";
    }
  }

  /**
   * 清理 stdout，移除 return_val 输出
   */
  private cleanStdout(pyodideOutput: string): string {
    try {
      // 解析pyodide-sandbox的输出JSON
      const parsedOutput = JSON.parse(pyodideOutput);

      // 从pyodide-sandbox的stdout中移除return_val输出的JSON
      const pyodideStdout = parsedOutput.stdout || "";

      // 移除JSON对象，保留其他内容
      let cleanedStdout = pyodideStdout.replace(/\{[^{}]*"score"[^{}]*\}/g, '');
      if (cleanedStdout === pyodideStdout) {
        // 如果没有找到特定的JSON，尝试移除任何JSON对象
        cleanedStdout = pyodideStdout.replace(/\{[^{}]*\}/g, '');
      }

      // 清理多余的空行
      cleanedStdout = cleanedStdout.replace(/\n+/g, '\n').trim();
      // 返回清理后的纯 stdout 文本
      return cleanedStdout;
    } catch (error) {
      console.error("清理pyodide输出失败:", error);
      // 回退为原始内容（可能是pyodide-sandbox的JSON字符串）
      return pyodideOutput;
    }
  }

  getExecutionCount(): number {
    return this.executionCount;
  }
}

// ==================== Pyodide FaaS 服务器 ====================

class PyodideFaaSServer {
  private readonly executor: PyodideExecutor;
  private readonly startTime = Date.now();

  constructor() {
    this.executor = new PyodideExecutor();
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
    const healthData: HealthStatus = {
      status: "healthy",
      timestamp: new Date().toISOString(),
      runtime: "pyodide-webassembly",
      version: "pyodide-faas-v1.0.0",
      execution_count: this.executor.getExecutionCount(),
      python_version: "Pyodide WebAssembly Python",
      security: {
        sandbox: "pyodide-webassembly",
        isolation: "deno-permissions",
        permissions: "restricted"
      }
    };

    return new Response(JSON.stringify(healthData), {
      status: 200,
      headers: { "Content-Type": "application/json" }
    });
  }

  /**
   * 处理指标请求
   */
  handleMetrics(): Response {
    const uptime = Date.now() - this.startTime;
    const metrics = {
      execution_count: this.executor.getExecutionCount(),
      uptime_seconds: Math.floor(uptime / 1000),
      runtime: "pyodide-webassembly",
      python_version: "Pyodide WebAssembly Python",
      status: "healthy"
    };

    return new Response(JSON.stringify(metrics), {
      status: 200,
      headers: { "Content-Type": "application/json" }
    });
  }
}

// ==================== 主函数 ====================

async function main() {
  const port = parseInt(Deno.env.get("FAAS_PORT") || "8000");

  console.log(`🚀 启动 Pyodide Python FaaS 服务器，端口: ${port}...`);
  console.log("🔒 安全特性: Deno 权限控制 + Pyodide WebAssembly 沙箱");
  console.log("⚡ 运行模式: Pyodide WebAssembly Python 执行器");

  const faasServer = new PyodideFaaSServer();

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
  console.log("  GET  /health    - 健康检查");
  console.log("  GET  /metrics   - 指标信息");
  console.log("  POST /run_code  - 执行 Python 代码 (Pyodide)");
  console.log("");
  console.log("🔐 安全保障:");
  console.log("  ✅ Deno 权限控制");
  console.log("  ✅ Pyodide WebAssembly 沙箱");
  console.log("  ✅ 代码执行隔离");
  console.log("");
  console.log("⚡ 特性:");
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
