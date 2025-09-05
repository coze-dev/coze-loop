#!/usr/bin/env deno run --allow-all

/**
 * coze-loop-faas 服务器
 * 提供代码执行服务，支持 JavaScript/TypeScript 和 Python
 */

interface ExecutionRequest {
  language: string;
  code: string;
  input?: any;
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
}

class FaaSServer {
  private readonly workspace: string;
  private readonly defaultTimeout: number;

  constructor() {
    this.workspace = Deno.env.get("FAAS_WORKSPACE") || "/tmp/faas-workspace";
    this.defaultTimeout = parseInt(Deno.env.get("FAAS_TIMEOUT") || "30000");
  }

  /**
   * 处理代码执行请求
   */
  async handleRunCode(request: Request): Promise<Response> {
    try {
      const body: ExecutionRequest = await request.json();
      const { language, code, input, timeout = this.defaultTimeout } = body;

      // 参数验证
      if (!language || !code) {
        return new Response(
          JSON.stringify({ error: "Missing required parameters: language, code" }),
          { status: 400, headers: { "Content-Type": "application/json" } }
        );
      }

      // 支持的语言检查
      if (!["javascript", "typescript", "python"].includes(language.toLowerCase())) {
        return new Response(
          JSON.stringify({ error: "Unsupported language. Supported: javascript, typescript, python" }),
          { status: 400, headers: { "Content-Type": "application/json" } }
        );
      }

      console.log(`Executing ${language} code, timeout: ${timeout}ms`);

      // 创建临时文件
      const tempFile = await this.createTempFile(language.toLowerCase(), code, input);

      try {
        // 执行代码
        const result = await this.executeCode(tempFile, timeout);

        // 返回结果
        const response: ApiResponse = {
          output: {
            stdout: result.stdout,
            stderr: result.stderr,
            ret_val: result.returnValue
          }
        };

        return new Response(JSON.stringify(response), {
          status: 200,
          headers: { "Content-Type": "application/json" }
        });

      } finally {
        // 清理临时文件
        await this.cleanup(tempFile);
      }

    } catch (error) {
      console.error("Error handling run_code request:", error);
      const errorMessage = error instanceof Error ? error.message : String(error);
      return new Response(
        JSON.stringify({ error: "Internal server error", details: errorMessage }),
        { status: 500, headers: { "Content-Type": "application/json" } }
      );
    }
  }

  /**
   * 创建临时文件
   */
  private async createTempFile(language: string, code: string, input?: any): Promise<string> {
    const timestamp = Date.now();
    const randomId = Math.random().toString(36).substr(2, 9);
    
    let fileContent = '';
    let extension = '';
    
    switch (language) {
      case 'javascript':
      case 'typescript':
        extension = '.ts';
        // 包装用户代码，捕获输出和返回值
        fileContent = `
const originalLog = console.log;
const originalError = console.error;
let stdout = '';
let stderr = '';

console.log = (...args) => {
  stdout += args.join(' ') + '\\n';
  originalLog(...args);
};

console.error = (...args) => {
  stderr += args.join(' ') + '\\n';
  originalError(...args);
};

try {
  // 用户输入数据
  const input = ${JSON.stringify(input || {})};
  
  // 用户代码执行函数
  const userFunction = () => {
    ${code}
  };
  
  // 执行用户代码并捕获返回值
  const result = userFunction();
  
  // 输出结果
  console.log(JSON.stringify({
    stdout: stdout,
    stderr: stderr,
    ret_val: JSON.stringify(result)
  }));
} catch (error) {
  console.error(JSON.stringify({
    stdout: stdout,
    stderr: stderr + error.message,
    ret_val: null
  }));
}
        `;
        break;
        
      case 'python':
        extension = '.py';
        fileContent = `
import sys
import json
import io
from contextlib import redirect_stdout, redirect_stderr

# 捕获标准输出和错误输出
stdout_capture = io.StringIO()
stderr_capture = io.StringIO()

try:
    with redirect_stdout(stdout_capture), redirect_stderr(stderr_capture):
        # 用户输入数据
        input_data = ${JSON.stringify(input || {})}
        
        # 用户代码
        ${code}
        
        # 如果用户代码定义了返回值变量，捕获它
        ret_val = locals().get('result', None)
    
    # 输出结果
    result = {
        "stdout": stdout_capture.getvalue(),
        "stderr": stderr_capture.getvalue(),
        "ret_val": json.dumps(ret_val) if ret_val is not None else ""
    }
    print(json.dumps(result))
    
except Exception as e:
    result = {
        "stdout": stdout_capture.getvalue(),
        "stderr": stderr_capture.getvalue() + str(e),
        "ret_val": ""
    }
    print(json.dumps(result))
        `;
        break;
    }
    
    const tempFile = `${this.workspace}/temp_${timestamp}_${randomId}${extension}`;
    await Deno.writeTextFile(tempFile, fileContent);
    return tempFile;
  }

  /**
   * 执行代码
   */
  private async executeCode(tempFile: string, timeout: number): Promise<ExecutionResult> {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), timeout);
    
    try {
      let command: Deno.Command;
      
      if (tempFile.endsWith('.ts') || tempFile.endsWith('.js')) {
        // 执行 TypeScript/JavaScript
        command = new Deno.Command("deno", {
          args: ["run", "--allow-all", tempFile],
          stdout: "piped",
          stderr: "piped",
          signal: controller.signal,
        });
      } else if (tempFile.endsWith('.py')) {
        // 执行 Python
        command = new Deno.Command("python3", {
          args: [tempFile],
          stdout: "piped", 
          stderr: "piped",
          signal: controller.signal,
        });
      } else {
        throw new Error(`Unsupported file extension: ${tempFile}`);
      }
      
      const { code: exitCode, stdout, stderr } = await command.output();
      
      const stdoutText = new TextDecoder().decode(stdout);
      const stderrText = new TextDecoder().decode(stderr);
      
      // 解析执行结果
      if (exitCode === 0 && stdoutText.trim()) {
        try {
          // 尝试解析 JSON 格式的结果
          const result = JSON.parse(stdoutText.trim());
          return {
            stdout: result.stdout || "",
            stderr: result.stderr || stderrText,
            returnValue: result.ret_val || ""
          };
        } catch {
          // 如果不是 JSON 格式，直接返回原始输出
          return {
            stdout: stdoutText,
            stderr: stderrText,
            returnValue: ""
          };
        }
      } else {
        return {
          stdout: stdoutText,
          stderr: stderrText,
          returnValue: ""
        };
      }
    } catch (error) {
      if (error instanceof Error && error.name === 'AbortError') {
        throw new Error(`Code execution timeout after ${timeout}ms`);
      }
      throw error;
    } finally {
      clearTimeout(timeoutId);
    }
  }

  /**
   * 清理临时文件
   */
  private async cleanup(tempFile: string): Promise<void> {
    try {
      await Deno.remove(tempFile);
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : String(error);
      console.warn(`Failed to cleanup temp file ${tempFile}:`, errorMessage);
    }
  }

  /**
   * 健康检查
   */
  handleHealth(): Response {
    return new Response("OK", { 
      status: 200,
      headers: { "Content-Type": "text/plain" }
    });
  }
}

// 启动服务器
async function main() {
  const port = parseInt(Deno.env.get("FAAS_PORT") || "8000");
  const faasServer = new FaaSServer();

  console.log(`Starting FaaS server on port ${port}...`);
  console.log(`Workspace: ${Deno.env.get("FAAS_WORKSPACE")}`);
  console.log(`Default timeout: ${Deno.env.get("FAAS_TIMEOUT")}ms`);

  const server = Deno.serve({
    port: port,
    hostname: "0.0.0.0"
  }, async (request: Request) => {
    const url = new URL(request.url);
    const method = request.method;

    console.log(`${method} ${url.pathname}`);

    // 健康检查接口
    if (url.pathname === "/health") {
      return faasServer.handleHealth();
    }

    // 代码执行接口
    if (url.pathname === "/run_code" && method === "POST") {
      return await faasServer.handleRunCode(request);
    }

    // 404
    return new Response("Not Found", { 
      status: 404,
      headers: { "Content-Type": "text/plain" }
    });
  });

  console.log(`FaaS server started successfully on http://0.0.0.0:${port}`);
  console.log("Available endpoints:");
  console.log("  GET  /health    - Health check");
  console.log("  POST /run_code  - Execute code");
}

// 错误处理
if (import.meta.main) {
  try {
    await main();
  } catch (error) {
    console.error("Failed to start FaaS server:", error);
    Deno.exit(1);
  }
}