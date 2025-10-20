#!/usr/bin/env deno run --allow-all

/**
 * 专用JavaScript FaaS服务器
 * 专注于JavaScript/TypeScript代码执行，提供统一的/run_code接口
 */

interface ExecutionRequest {
  language?: string;
  code: string;
  timeout?: number;
}

interface ExecutionResult {
  stdout: string;
  stderr: string;
  returnValue: string;
}

interface ApiResponse {
  output: {
    stdout: string;
    stderr: string;
    ret_val: string;
  };
  metadata?: {
    language: string;
    duration: number;
    status: string;
  };
}

class JavaScriptExecutor {
  private executionCount = 0;

  async executeJavaScript(code: string, timeout = 30000): Promise<ExecutionResult> {
    this.executionCount++;

    // 保留用户代码原样，避免移除由后端注入的 return_val 实现
    const processedCode = code;
    // 将用户代码写入独立临时文件，避免任何模板拼接/转义问题
    const userCodeFile = await this.createUserCodeFile(processedCode);

    // 直接构造包装代码，不使用模板字符串的嵌套
    // 不再添加return_val函数定义，使用runtime中提供的实现
    const wrappedLines: string[] = [];
    wrappedLines.push("let userStdout = '';");
    wrappedLines.push("let userStderr = '';");
    wrappedLines.push("let returnValue = '';");
    wrappedLines.push("");
    wrappedLines.push("const originalLog = console.log;");
    wrappedLines.push("const originalError = console.error;");
    wrappedLines.push("");
    wrappedLines.push("console.log = (...args) => {");
    wrappedLines.push("  userStdout += args.join(' ') + \"\\n\";");
    wrappedLines.push("};");
    wrappedLines.push("");
    wrappedLines.push("console.error = (...args) => {");
    wrappedLines.push("  userStderr += args.join(' ') + \"\\n\";");
    wrappedLines.push("};");
    wrappedLines.push("");
    wrappedLines.push("try {");
    wrappedLines.push("  const __userCode = await Deno.readTextFile(" + JSON.stringify(userCodeFile) + ");");
    wrappedLines.push("  (new Function('__code', 'return (function(){ \"use strict\"; return eval(__code); })();'))(__userCode);");
    wrappedLines.push("");
    wrappedLines.push("  if (!returnValue && userStdout.trim()) {");
    wrappedLines.push("    const lines = userStdout.trim().split('\\n');");
    wrappedLines.push("    for (let i = lines.length - 1; i >= 0; i--) {");
    wrappedLines.push("      const line = lines[i].trim();");
    wrappedLines.push("      if (line.startsWith('{') && line.endsWith('}')) {");
    wrappedLines.push("        try {");
    wrappedLines.push("          JSON.parse(line);");
    wrappedLines.push("          returnValue = line;");
    wrappedLines.push("          lines.splice(i, 1);");
    wrappedLines.push("          userStdout = lines.join('\\n');");
    wrappedLines.push("          break;");
    wrappedLines.push("        } catch (_) {");
    wrappedLines.push("        }");
    wrappedLines.push("      }");
    wrappedLines.push("    }");
    wrappedLines.push("  }");
    wrappedLines.push("");
    wrappedLines.push("  originalLog(JSON.stringify({ stdout: userStdout, stderr: userStderr, ret_val: returnValue }));");
    wrappedLines.push("} catch (error) {");
    wrappedLines.push("  const msg = (error && error.stack) ? String(error.stack) : String((error && error.message) || error);");
    wrappedLines.push("  originalLog(JSON.stringify({ stdout: userStdout, stderr: userStderr + msg + \"\\n\", ret_val: '' }));");
    wrappedLines.push("}");
    const wrappedCode = wrappedLines.join('\n');

    const tempFile = await this.createTempFile(wrappedCode);

    try {
      return await this.executeCode(tempFile, timeout);
    } finally {
      await this.cleanup(tempFile);
    }
  }

  private async createTempFile(code: string): Promise<string> {
    const timestamp = Date.now();
    const randomId = Math.random().toString(36).substr(2, 9);
    const tempFile = `/tmp/faas-workspace/temp_${timestamp}_${randomId}.js`;

    await Deno.writeTextFile(tempFile, code);
    return tempFile;
  }

  private async createUserCodeFile(code: string): Promise<string> {
    const timestamp = Date.now();
    const randomId = Math.random().toString(36).substr(2, 9);
    const userFile = `/tmp/faas-workspace/user_${timestamp}_${randomId}.js`;
    await Deno.writeTextFile(userFile, code);
    return userFile;
  }

  private async executeCode(tempFile: string, timeout: number): Promise<ExecutionResult> {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), timeout);

    try {
      const command = new Deno.Command("deno", {
        args: ["run", "--allow-all", "--quiet", tempFile],
        stdout: "piped",
        stderr: "piped",
        signal: controller.signal,
      });

      const { code: exitCode, stdout, stderr } = await command.output();

      const stdoutText = new TextDecoder().decode(stdout);
      const stderrText = new TextDecoder().decode(stderr);

      if (exitCode === 0 && stdoutText.trim()) {
        // 按行分割，找到最后一个有效的JSON行
        const lines = stdoutText.trim().split('\n');
        for (let i = lines.length - 1; i >= 0; i--) {
          const line = lines[i].trim();
          if (line.startsWith('{') && line.endsWith('}')) {
            try {
              const result = JSON.parse(line);
              return {
                stdout: result.stdout || "",
                stderr: result.stderr || stderrText,
                returnValue: result.ret_val || ""
              };
            } catch {
              continue; // 尝试上一行
            }
          }
        }
      }

      // 回退逻辑：直接返回所有输出
      return {
        stdout: stdoutText,
        stderr: stderrText,
        returnValue: ""
      };
    } catch (error) {
      if (error instanceof Error && error.name === 'AbortError') {
        throw new Error(`Code execution timeout after ${timeout}ms`);
      }
      throw error;
    } finally {
      clearTimeout(timeoutId);
    }
  }



  private async cleanup(tempFile: string): Promise<void> {
    try {
      await Deno.remove(tempFile);
    } catch (error) {
      console.warn(`Failed to cleanup temp file ${tempFile}:`, error);
    }
  }

  getExecutionCount(): number {
    return this.executionCount;
  }
}

class JavaScriptFaaSServer {
  private readonly executor: JavaScriptExecutor;

  constructor() {
    this.executor = new JavaScriptExecutor();
  }

  async handleRunCode(request: Request): Promise<Response> {
    try {
      const body: ExecutionRequest = await request.json();
      const {
        language,
        code,
        timeout = 30000
      } = body;

      if (!code) {
        return new Response(
          JSON.stringify({ error: "Missing required parameter: code" }),
          { status: 400, headers: { "Content-Type": "application/json" } }
        );
      }

      // 语言检查 - 只支持JavaScript/TypeScript
      if (language && !["javascript", "js", "typescript", "ts"].includes(language.toLowerCase())) {
        return new Response(
          JSON.stringify({ error: "This service only supports JavaScript/TypeScript code execution" }),
          { status: 400, headers: { "Content-Type": "application/json" } }
        );
      }

      console.log(`执行JavaScript代码，超时: ${timeout}ms`);

      const startTime = Date.now();
      const result = await this.executor.executeJavaScript(code, timeout);
      const duration = Date.now() - startTime;

      const response: ApiResponse = {
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
          language: "javascript",
          duration,
          status: "success"
        }
      };

      return new Response(JSON.stringify(response), {
        status: 200,
        headers: { "Content-Type": "application/json" }
      });

    } catch (error) {
      console.error("Error handling run_code request:", error);
      const errorMessage = error instanceof Error ? error.message : String(error);
      return new Response(
        JSON.stringify({ error: "Internal server error", details: errorMessage }),
        { status: 500, headers: { "Content-Type": "application/json" } }
      );
    }
  }

  handleHealth(): Response {
    const healthData = {
      status: "healthy",
      timestamp: new Date().toISOString(),
      language: "javascript",
      version: "js-faas-v1.0.0",
      execution_count: this.executor.getExecutionCount()
    };

    return new Response(JSON.stringify(healthData), {
      status: 200,
      headers: { "Content-Type": "application/json" }
    });
  }

  handleMetrics(): Response {
    const metrics = {
      language: "javascript",
      execution_count: this.executor.getExecutionCount(),
      timestamp: new Date().toISOString()
    };

    return new Response(JSON.stringify(metrics), {
      status: 200,
      headers: { "Content-Type": "application/json" }
    });
  }
}

async function main() {
  const port = parseInt(Deno.env.get("FAAS_PORT") || "8000");
  const faasServer = new JavaScriptFaaSServer();

  console.log(`启动JavaScript FaaS服务器，端口: ${port}...`);
  console.log(`工作空间: ${Deno.env.get("FAAS_WORKSPACE") || "/tmp/faas-workspace"}`);
  console.log(`默认超时: ${Deno.env.get("FAAS_TIMEOUT") || "30000"}ms`);
  console.log("专用语言: JavaScript/TypeScript");

  // 确保工作空间目录存在
  const workspace = Deno.env.get("FAAS_WORKSPACE") || "/tmp/faas-workspace";
  try {
    await Deno.mkdir(workspace, { recursive: true });
  } catch (error) {
    if (!(error instanceof Deno.errors.AlreadyExists)) {
      console.warn(`Failed to create workspace directory: ${error}`);
    }
  }

  const server = Deno.serve({
    port: port,
    hostname: "0.0.0.0"
  }, async (request: Request) => {
    const url = new URL(request.url);
    const method = request.method;

    console.log(`${method} ${url.pathname}`);

    if (url.pathname === "/health") {
      return faasServer.handleHealth();
    }

    if (url.pathname === "/metrics") {
      return faasServer.handleMetrics();
    }

    if (url.pathname === "/run_code" && method === "POST") {
      return await faasServer.handleRunCode(request);
    }

    return new Response("Not Found", {
      status: 404,
      headers: { "Content-Type": "text/plain" }
    });
  });

  console.log(`JavaScript FaaS服务器启动成功: http://0.0.0.0:${port}`);
  console.log("可用端点:");
  console.log("  GET  /health    - 健康检查");
  console.log("  GET  /metrics   - 指标信息");
  console.log("  POST /run_code  - 执行JavaScript/TypeScript代码");
}

if (import.meta.main) {
  try {
    await main();
  } catch (error) {
    console.error("启动JavaScript FaaS服务器失败:", error);
    Deno.exit(1);
  }
}
