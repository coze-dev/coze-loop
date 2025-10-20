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

      return {
        stdout: stdoutText,
        stderr: stderrText,
        returnValue: ""
      };
    } finally {
      clearTimeout(timeoutId);
    }
  }

  private async cleanup(tempFile: string): Promise<void> {
    try {
      await Deno.remove(tempFile);
    } catch {
      // 忽略清理错误
    }
  }

  getExecutionCount(): number {
    return this.executionCount;
  }
}

// ==================== JavaScript FaaS 服务器 ====================

class JavaScriptFaaSServer {
  private readonly executor: JavaScriptExecutor;
  private readonly startTime = Date.now();

  constructor() {
    this.executor = new JavaScriptExecutor();
  }

  async start(): Promise<void> {
    const port = parseInt(Deno.env.get("FAAS_PORT") || "8000");
    console.log(`🚀 JavaScript FaaS server starting on port ${port}...`);

    const handler = this.createHandler();
    const server = Deno.serve({ port, handler });

    console.log(`✅ JavaScript FaaS server started on port ${port}`);
    await server.finished;
  }

  private createHandler(): (request: Request) => Promise<Response> {
    return async (request: Request) => {
      const url = new URL(request.url);
      const path = url.pathname;
      const method = request.method;

      try {
        if (method === "GET" && path === "/health") {
          return this.handleHealthCheck();
        }

        if (method === "POST" && path === "/run_code") {
          return this.handleRunCode(request);
        }

        return new Response("Not Found", { status: 404 });
      } catch (error) {
        console.error("❌ 请求处理错误:", error);
        return new Response(
          JSON.stringify({ error: "Internal server error", details: String(error) }),
          { 
            status: 500,
            headers: { "Content-Type": "application/json" }
          }
        );
      }
    };
  }

  private async handleHealthCheck(): Promise<Response> {
    const uptime = Date.now() - this.startTime;
    const healthStatus = {
      status: "healthy",
      timestamp: new Date().toISOString(),
      uptime: uptime,
      runtime: "deno",
      version: Deno.version.deno,
      execution_count: this.executor.getExecutionCount(),
      language: "javascript"
    };

    return new Response(JSON.stringify(healthStatus), {
      headers: { "Content-Type": "application/json" }
    });
  }

  private async handleRunCode(request: Request): Promise<Response> {
    const startTime = Date.now();

    try {
      const body = await request.json() as ExecutionRequest;
      const { code, language = "javascript", timeout = 30000 } = body;

      if (!code) {
        return new Response(
          JSON.stringify({ error: "Code is required" }),
          { status: 400, headers: { "Content-Type": "application/json" } }
        );
      }

      console.log(`🚀 执行 ${language} 代码，超时: ${timeout}ms`);

      const result = await this.executor.executeJavaScript(code, timeout);
      const duration = Date.now() - startTime;

      const response: ApiResponse = {
        output: {
          stdout: result.stdout,
          stderr: result.stderr,
          ret_val: result.returnValue
        },
        metadata: {
          language: language,
          duration: duration,
          status: "completed"
        }
      };

      console.log(`✅ 执行完成，耗时: ${duration}ms`);

      return new Response(JSON.stringify(response), {
        headers: { "Content-Type": "application/json" }
      });

    } catch (error) {
      const duration = Date.now() - startTime;
      console.error(`❌ 执行失败，耗时: ${duration}ms，错误:`, error);

      return new Response(
        JSON.stringify({
          error: "Execution failed",
          details: String(error),
          output: {
            stdout: "",
            stderr: String(error),
            ret_val: ""
          },
          metadata: {
            language: "javascript",
            duration: duration,
            status: "failed"
          }
        }),
        { 
          status: 500,
          headers: { "Content-Type": "application/json" }
        }
      );
    }
  }
}

// ==================== 主程序 ====================

if (import.meta.main) {
  const server = new JavaScriptFaaSServer();
  await server.start();
}