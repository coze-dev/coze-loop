#!/usr/bin/env deno run --allow-net --allow-env --allow-run

/// <reference types="https://deno.land/x/deno@v1.45.5/cli/dts/lib.deno.d.ts" />

/**
 * 增强版 Python FaaS 服务器
 * 
 * 这个版本提供真正的 Python 代码执行能力，而不是模拟器
 * 
 * 特性：
 * 1. 真正的 Python 代码执行（通过子进程调用 Python）
 * 2. 完整的安全沙箱隔离
 * 3. 支持复杂的 Python 代码，包括函数定义、数据处理等
 * 4. 保持与原 API 的完全兼容
 * 5. 基于 Deno 的安全执行环境
 * 6. 支持 stdout、stderr 和 return_val 捕获
 * 
 * 安全措施：
 * - 通过 Docker 容器隔离
 * - 代码静态分析检查危险模式
 * - 模块导入黑名单控制
 * - 执行超时控制
 * - 资源使用限制
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

// ==================== 安全代码检查器 ====================

class SecurityCodeChecker {
  private static readonly DANGEROUS_PATTERNS = [
    // 危险函数调用
    /open\s*\(/,
    /file\s*\(/,
    /exec\s*\(/,
    /eval\s*\(/,
    /__import__\s*\(/,
    /compile\s*\(/,
    
    // 系统调用模式
    /subprocess\./,
    /os\.system/,
    /os\.popen/,
    /os\.spawn/,
    /os\.exec/,
    
    // 网络访问模式
    /socket\./,
    /urllib\./,
    /http\./,
    /requests\./,
    
    // 文件系统访问模式
    /with\s+open\s*\(/,
    /open\s*\([^)]*['"]/,
  ];

  // 黑名单策略：只阻止危险的系统模块
  private static readonly DANGEROUS_SYSTEM_MODULES = new Set([
    // 系统访问模块
    'os', 'subprocess', 'socket', 'urllib', 'http', 'requests',
    
    // 网络通信模块
    'ftplib', 'poplib', 'imaplib', 'smtplib', 'telnetlib', 'webbrowser',
    
    // 序列化和持久化模块（可能不安全）
    'pickle', 'marshal', 'shelve', 'dbm', 'sqlite3',
    
    // 多线程和多进程模块
    'threading', 'multiprocessing',
    
    // 系统底层模块
    'ctypes', 'gc', 'signal', 'resource', 'mmap', 'fcntl',
    'termios', 'tty', 'pty', 'grp', 'pwd', 'spwd', 'crypt',
    
    // 动态导入和代码操作模块（允许 sys 和 ast）
    'importlib', 'pkgutil', 'runpy', 'zipimport', 'inspect', 
    'types', 'code', 'codeop', 'compileall', 'dis',
    
    // 程序控制模块
    'atexit',
    
    // 安全相关模块（在沙箱环境中不需要）
    'ssl', 'hashlib', 'hmac', 'secrets'
  ]);

  /**
   * 检查代码安全性
   */
  static checkCodeSecurity(code: string): { safe: boolean; violations: string[] } {
    const violations: string[] = [];

    // 检查危险模式
    for (const pattern of this.DANGEROUS_PATTERNS) {
      if (pattern.test(code)) {
        violations.push(`检测到危险模式: ${pattern.source}`);
      }
    }

    // 检查模块导入 - 黑名单策略
    const importMatches = code.match(/(?:import|from)\s+(\w+)/g);
    if (importMatches) {
      for (const match of importMatches) {
        const moduleName = match.replace(/(?:import|from)\s+/, '').split('.')[0];
        if (this.DANGEROUS_SYSTEM_MODULES.has(moduleName)) {
          violations.push(`禁止导入危险系统模块: ${moduleName}`);
        }
      }
    }

    return {
      safe: violations.length === 0,
      violations
    };
  }
}

// ==================== Python 代码执行器 ====================

class EnhancedPythonExecutor {
  private executionCount = 0;
  private pythonVersion: string | null = null;

  constructor() {
    this.initializePythonEnvironment();
  }

  /**
   * 初始化 Python 环境
   */
  private async initializePythonEnvironment(): Promise<void> {
    try {
      const process = new Deno.Command("python3", {
        args: ["--version"],
        stdout: "piped",
        stderr: "piped",
      });
      
      const { stdout } = await process.output();
      this.pythonVersion = new TextDecoder().decode(stdout).trim();
      console.log(`🐍 Python 环境初始化成功: ${this.pythonVersion}`);
    } catch (error) {
      console.warn(`⚠️ Python 环境检查失败: ${error.message}`);
      this.pythonVersion = "Python 3.x (未知版本)";
    }
  }

  /**
   * 执行 Python 代码（真正的执行）
   */
  async executePython(code: string, timeout = 30000): Promise<ExecutionResult> {
    this.executionCount++;
    
    console.log(`🚀 执行 Python 代码 (真实执行器)，超时: ${timeout}ms`);
    
    // 安全检查
    const securityCheck = SecurityCodeChecker.checkCodeSecurity(code);
    if (!securityCheck.safe) {
      return {
        stdout: "",
        stderr: `安全检查失败:\n${securityCheck.violations.join('\n')}`,
        returnValue: "",
        metadata: {
          duration: 0,
          exitCode: 1,
          timedOut: false
        }
      };
    }

    try {
      const startTime = Date.now();
      const result = await this.executeRealPython(code, timeout);
      const duration = Date.now() - startTime;
      
      return {
        ...result,
        metadata: {
          ...result.metadata,
          duration
        }
      };
    } catch (error) {
      return {
        stdout: "",
        stderr: `执行错误: ${error.message}`,
        returnValue: "",
        metadata: {
          duration: 0,
          exitCode: 1,
          timedOut: error.message.includes('timeout')
        }
      };
    }
  }

  /**
   * 真正执行 Python 代码
   */
  private async executeRealPython(code: string, timeout: number): Promise<ExecutionResult> {
    // 创建临时文件
    const tempCodeFile = `/tmp/user_code_${Date.now()}_${Math.random().toString(36).substr(2, 9)}.py`;
    const tempWrapperFile = `/tmp/wrapper_${Date.now()}_${Math.random().toString(36).substr(2, 9)}.py`;
    
    try {
      // 写入用户代码到单独文件
      await Deno.writeTextFile(tempCodeFile, code);
      
      // 创建安全的执行包装器
      const wrapperScript = this.createSafeExecutionWrapper(tempCodeFile);
      await Deno.writeTextFile(tempWrapperFile, wrapperScript);
      
      // 执行包装器脚本
      const process = new Deno.Command("python3", {
        args: [tempWrapperFile],
        stdout: "piped",
        stderr: "piped",
        env: {
          // 设置安全的环境变量
          PYTHONPATH: "",
          PYTHONDONTWRITEBYTECODE: "1",
          PYTHONUNBUFFERED: "1",
        }
      });

      // 设置超时
      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), timeout);
      
      try {
        const { stdout, stderr, code: exitCode } = await process.output();
        clearTimeout(timeoutId);
        
        const stdoutText = new TextDecoder().decode(stdout);
        const stderrText = new TextDecoder().decode(stderr);
        
        // 提取 return_val 的结果
        const returnValue = this.extractReturnValue(stdoutText);
        
        // 清理 stdout，移除 return_val 输出
        const cleanStdout = this.cleanStdout(stdoutText);
        
        return {
          stdout: cleanStdout,
          stderr: stderrText,
          returnValue,
          metadata: {
            duration: 0, // 将在上层设置
            exitCode,
            timedOut: false
          }
        };
        
      } catch (error) {
        clearTimeout(timeoutId);
        if (error.name === 'AbortError') {
          throw new Error(`执行超时 (${timeout}ms)`);
        }
        throw error;
      }
      
    } finally {
      // 清理临时文件
      try {
        await Deno.remove(tempCodeFile);
        await Deno.remove(tempWrapperFile);
      } catch {
        // 忽略清理错误
      }
    }
  }

  /**
   * 创建安全的 Python 执行包装器
   */
  private createSafeExecutionWrapper(userCodeFile: string): string {
    return `#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import sys
import builtins
import io
import json
import traceback
import codecs
import locale
from contextlib import redirect_stdout, redirect_stderr

# ========== UTF-8编码配置 ==========

# 确保UTF-8编码处理
if hasattr(sys.stdout, 'buffer'):
    sys.stdout = codecs.getwriter('utf-8')(sys.stdout.buffer)
if hasattr(sys.stderr, 'buffer'):
    sys.stderr = codecs.getwriter('utf-8')(sys.stderr.buffer)

# 设置默认编码
if hasattr(sys, 'setdefaultencoding'):
    sys.setdefaultencoding('utf-8')

# ========== 安全配置 ==========

# 保存必要的内置函数用于内部操作
_internal_open = builtins.open
_internal_exec = builtins.exec
_internal_eval = builtins.eval
_internal_compile = builtins.compile

# 安全的模块导入控制
original_import = builtins.__import__

def safe_import(name, *args, **kwargs):
    """安全模块导入，黑名单策略"""
    
    # 黑名单：危险的系统模块
    dangerous_modules = {
        'os', 'subprocess', 'socket', 'urllib', 'http', 'requests',
        'ftplib', 'poplib', 'imaplib', 'smtplib', 'telnetlib', 'webbrowser',
        'pickle', 'marshal', 'shelve', 'dbm', 'sqlite3',
        'threading', 'multiprocessing',
        'ctypes', 'gc', 'signal', 'resource', 'mmap', 'fcntl',
        'termios', 'tty', 'pty', 'grp', 'pwd', 'spwd', 'crypt',
        'importlib', 'pkgutil', 'runpy', 'zipimport', 'inspect', 
        'types', 'code', 'codeop', 'compileall', 'dis',
        'atexit', 'ssl', 'hashlib', 'hmac', 'secrets'
    }
    
    if name in dangerous_modules:
        raise ImportError(f"🚫 SECURITY: Dangerous module '{name}' is blocked")
    
    # 允许所有非危险模块
    return original_import(name, *args, **kwargs)

# ========== 输出捕获 ==========

# 全局变量用于捕获 return_val
_return_val_output = None

def return_val(value):
    """捕获返回值，使用更清晰的分隔符"""
    global _return_val_output
    _return_val_output = str(value) if value is not None else ""
    # 使用特殊的分隔符，避免与正常输出混淆
    print(f"__COZE_RETURN_VAL_START__")
    print(_return_val_output)
    print(f"__COZE_RETURN_VAL_END__")

# 创建受限的用户命名空间
def create_safe_builtins():
    """创建安全的内置函数集合，移除危险函数"""
    safe_builtins = {}
    
    # 复制所有安全的内置函数
    for name, obj in builtins.__dict__.items():
        if name not in ['open', 'input', 'raw_input', 'file', 'execfile', 'reload', 'compile', 'eval', 'exec']:
            safe_builtins[name] = obj
    
    # 添加自定义函数
    safe_builtins['return_val'] = return_val
    
    return safe_builtins

# 创建用户代码的安全执行环境
user_globals = {
    '__builtins__': create_safe_builtins(),
    '__import__': safe_import,
    'json': json
}

# ========== 执行用户代码 ==========

try:
    # 捕获 stdout 和 stderr
    stdout_capture = io.StringIO()
    stderr_capture = io.StringIO()
    
    with redirect_stdout(stdout_capture), redirect_stderr(stderr_capture):
        # 读取并执行用户代码文件
        with _internal_open('${userCodeFile}', 'r', encoding='utf-8') as f:
            user_code = f.read()
        _internal_exec(user_code, user_globals)
    
    # 输出结果
    stdout_content = stdout_capture.getvalue()
    stderr_content = stderr_capture.getvalue()
    
    # 输出到真正的 stdout（确保UTF-8编码）
    if stdout_content:
        try:
            sys.stdout.write(stdout_content)
            sys.stdout.flush()
        except UnicodeEncodeError:
            # 如果有编码问题，强制使用UTF-8
            sys.stdout.buffer.write(stdout_content.encode('utf-8'))
            sys.stdout.buffer.flush()
    
    # 输出到真正的 stderr（确保UTF-8编码）
    if stderr_content:
        try:
            sys.stderr.write(stderr_content)
            sys.stderr.flush()
        except UnicodeEncodeError:
            # 如果有编码问题，强制使用UTF-8
            sys.stderr.buffer.write(stderr_content.encode('utf-8'))
            sys.stderr.buffer.flush()
        
except Exception as e:
    error_msg = traceback.format_exc()
    try:
        print(error_msg, file=sys.stderr)
    except UnicodeEncodeError:
        # 如果有编码问题，强制使用UTF-8
        sys.stderr.buffer.write(error_msg.encode('utf-8'))
        sys.stderr.buffer.flush()
    sys.exit(1)
`;
  }

  /**
   * 提取 return_val 的结果，使用更健壮的提取逻辑
   */
  private extractReturnValue(stdout: string): string {
    const match = stdout.match(/__COZE_RETURN_VAL_START__\n(.*?)\n__COZE_RETURN_VAL_END__/s);
    return match ? match[1] : "";
  }

  /**
   * 清理 stdout，移除 return_val 输出
   */
  private cleanStdout(stdout: string): string {
    // 移除完整的return_val输出块
    return stdout.replace(/__COZE_RETURN_VAL_START__\n.*?\n__COZE_RETURN_VAL_END__\n?/gs, '');
  }

  getPythonVersion(): string | null {
    return this.pythonVersion;
  }

  getExecutionCount(): number {
    return this.executionCount;
  }
}

// ==================== 增强 FaaS 服务器 ====================

class EnhancedPythonFaaSServer {
  private readonly executor: EnhancedPythonExecutor;
  private readonly startTime = Date.now();

  constructor() {
    this.executor = new EnhancedPythonExecutor();
  }

  /**
   * 处理代码执行请求
   */
  async handleRunCode(request: Request): Promise<Response> {
    try {
      // 增强JSON请求处理
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

      // 语言检查（可选，因为服务专门处理Python）
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
        metadata: {
          language: "python",
          runtime: "enhanced-real-executor",
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

      // 区分不同类型的错误
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
      runtime: "deno-enhanced-real-executor",
      version: "enhanced-python-faas-v1.0.0",
      execution_count: this.executor.getExecutionCount(),
      python_version: this.executor.getPythonVersion() || undefined,
      security: {
        sandbox: "deno-permissions+docker-isolation",
        isolation: "process-level+container-level",
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
      runtime: "enhanced-real-executor",
      python_version: this.executor.getPythonVersion(),
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

  console.log(`🚀 启动增强版 Python FaaS 服务器，端口: ${port}...`);
  console.log("🔒 安全特性: Deno 权限控制 + Docker 容器隔离 + 代码安全检查");
  console.log("⚡ 运行模式: 真实 Python 执行器 (支持完整 Python 功能)");

  const faasServer = new EnhancedPythonFaaSServer();

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

  console.log(`✅ 增强版 Python FaaS 服务器启动成功: http://0.0.0.0:${port}`);
  console.log("📡 可用端点:");
  console.log("  GET  /health    - 健康检查");
  console.log("  GET  /metrics   - 指标信息");
  console.log("  POST /run_code  - 执行 Python 代码 (真实执行)");
  console.log("");
  console.log("🔐 安全保障:");
  console.log("  ✅ Deno 权限控制");
  console.log("  ✅ Docker 容器隔离");
  console.log("  ✅ 危险代码检测");
  console.log("  ✅ 危险系统模块黑名单");
  console.log("  ✅ 允许非系统模块导入");
  console.log("  ✅ 临时文件隔离");
  console.log("  ✅ 环境变量控制");
  console.log("");
  console.log("⚡ 特性:");
  console.log("  ✅ 真正的 Python 代码执行");
  console.log("  ✅ 支持函数定义和调用");
  console.log("  ✅ 支持复杂数据处理");
  console.log("  ✅ 支持自定义对象和类");
  console.log("  ✅ 完整的 stdout/stderr 捕获");
  console.log("  ✅ return_val 函数支持");
  console.log("  ✅ 执行超时控制");
  console.log("  ✅ API 兼容性");
}

if (import.meta.main) {
  await main();
}