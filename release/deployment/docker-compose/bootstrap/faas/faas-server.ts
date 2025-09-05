#!/usr/bin/env deno run --allow-all

/**
 * coze-loop-faas 服务器
 * 提供代码执行服务，支持 JavaScript/TypeScript 和 Python (基于 Pyodide)
 */

import { loadPyodide } from "npm:pyodide";

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
  private pyodide: any = null;
  private pyodideInitialized = false;

  constructor() {
    this.workspace = Deno.env.get("FAAS_WORKSPACE") || "/tmp/faas-workspace";
    this.defaultTimeout = parseInt(Deno.env.get("FAAS_TIMEOUT") || "30000");
    
    // 异步初始化 Pyodide
    this.initializePyodide();
  }

  /**
   * 初始化 Pyodide
   */
  private async initializePyodide() {
    try {
      console.log("正在初始化 Pyodide...");
      this.pyodide = await loadPyodide();
      
      // 加载 micropip 用于包管理
      await this.pyodide.loadPackage("micropip");
      
      this.pyodideInitialized = true;
      console.log("Pyodide 初始化完成");
    } catch (error) {
      console.error("Pyodide 初始化失败:", error);
      this.pyodideInitialized = false;
    }
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
        const result = await this.executeCode(tempFile, timeout, language.toLowerCase(), code);

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
        // 清理临时文件（仅对非 Pyodide 执行）
        if (tempFile !== 'pyodide-execution') {
          await this.cleanup(tempFile);
        }
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
        // Python 代码将通过 Pyodide 执行，不需要创建临时文件
        return 'pyodide-execution';
        break;
    }
    
    const tempFile = `${this.workspace}/temp_${timestamp}_${randomId}${extension}`;
    await Deno.writeTextFile(tempFile, fileContent);
    return tempFile;
  }

  /**
   * 执行代码
   */
  private async executeCode(tempFile: string, timeout: number, language?: string, code?: string): Promise<ExecutionResult> {
    // 如果是 Python 代码，使用 Pyodide 执行
    if (tempFile === 'pyodide-execution' && language === 'python') {
      return await this.executePythonWithPyodide(code!, timeout);
    }
    
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
   * 使用 Pyodide 执行 Python 代码
   */
  private async executePythonWithPyodide(code: string, timeout: number): Promise<ExecutionResult> {
    if (!this.pyodideInitialized || !this.pyodide) {
      throw new Error("Pyodide 未初始化，无法执行 Python 代码");
    }

    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), timeout);
    
    try {
      // 先设置捕获环境
      this.pyodide.runPython(`
import sys
import json
from io import StringIO

# 捕获标准输出
old_stdout = sys.stdout
sys.stdout = stdout_capture = StringIO()

# 捕获标准错误
old_stderr = sys.stderr  
sys.stderr = stderr_capture = StringIO()

ret_val = None
error_msg = ""
      `);

      // 执行用户代码
      try {
        this.pyodide.runPython(code);
        
        // 尝试获取 result 变量
        this.pyodide.runPython(`
if 'result' in locals():
    ret_val = result
        `);
      } catch (execError) {
        // 记录执行错误
        this.pyodide.runPython(`
error_msg = "${String(execError).replace(/"/g, '\\"')}"
        `);
      }

      // 恢复输出并获取结果
      const result = this.pyodide.runPython(`
# 恢复标准输出和错误
sys.stdout = old_stdout
sys.stderr = old_stderr

# 返回结果
{
    "stdout": stdout_capture.getvalue(),
    "stderr": stderr_capture.getvalue() + error_msg,
    "ret_val": ret_val
}
      `);
      
      // 解析结果
      const parsedResult = typeof result === 'object' ? result : JSON.parse(result);
      
      return {
        stdout: parsedResult.stdout || "",
        stderr: parsedResult.stderr || "",
        returnValue: parsedResult.ret_val ? JSON.stringify(parsedResult.ret_val) : ""
      };
      
    } catch (error) {
      if (controller.signal.aborted) {
        throw new Error(`Python code execution timeout after ${timeout}ms`);
      }
      
      return {
        stdout: "",
        stderr: error instanceof Error ? error.message : String(error),
        returnValue: ""
      };
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
    const healthData = {
      status: "healthy",
      timestamp: new Date().toISOString(),
      pyodide: {
        initialized: this.pyodideInitialized,
        available: this.pyodide !== null
      },
      version: "faas-v1.0.0-pyodide"
    };
    
    return new Response(JSON.stringify(healthData), { 
      status: 200,
      headers: { "Content-Type": "application/json" }
    });
  }
}

// 启动服务器
async function main() {
  const port = parseInt(Deno.env.get("FAAS_PORT") || "8000");
  const faasServer = new FaaSServer();

  console.log(`Starting FaaS server with Pyodide support on port ${port}...`);
  console.log(`Workspace: ${Deno.env.get("FAAS_WORKSPACE")}`);
  console.log(`Default timeout: ${Deno.env.get("FAAS_TIMEOUT")}ms`);
  console.log("Python execution: Powered by Pyodide");

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

  console.log(`FaaS server with Pyodide started successfully on http://0.0.0.0:${port}`);
  console.log("Available endpoints:");
  console.log("  GET  /health    - Health check");
  console.log("  POST /run_code  - Execute code (JS/TS/Python via Pyodide)");
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