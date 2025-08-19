#!/usr/bin/env -S deno run --allow-all

/**
 * 简化版 Deno Pyodide 运行时
 * 基于 pyodide_runner.py 的核心功能，直接使用 JSR @langchain/pyodide-sandbox
 */

/// <reference types="https://deno.land/x/deno@v1.37.0/lib/deno.d.ts" />

// ============================================================================
// 类型定义
// ============================================================================

interface SandboxConfig {
  // 基本权限配置（兼容 pyodide_runner.py）
  allow_env?: string[] | boolean;
  allow_read?: string[] | boolean;
  allow_write?: string[] | boolean;
  allow_net?: string[] | boolean;
  allow_run?: string[] | boolean;
  allow_ffi?: string[] | boolean;
  node_modules_dir?: string;
  memory_limit_mb?: number;
  timeout_seconds?: number;
}

interface ExecutionRequest {
  config?: SandboxConfig;
  code: string;
  params?: Record<string, any>;
}

interface ExecutionResult {
  success: boolean;
  result?: any;
  stdout?: string;
  stderr?: string;
  execution_time: number;
  sandbox_error?: string;
  status: "success" | "error";
}

// ============================================================================
// 直接使用 Pyodide 执行器
// ============================================================================

class DirectPyodideExecutor {
  /**
   * 执行代码 - 直接使用 JSR @langchain/pyodide-sandbox
   */
  async execute(request: ExecutionRequest): Promise<ExecutionResult> {
    const startTime = performance.now();
    
    try {
      // 验证请求
      if (!request.code || typeof request.code !== 'string') {
        return this.createErrorResult("代码不能为空", startTime);
      }

      const config = request.config || {};
      
      // 动态导入 pyodide-sandbox
      const { runPython } = await import("jsr:@langchain/pyodide-sandbox@0.0.4");
      
      if (!runPython || typeof runPython !== 'function') {
        throw new Error("无法找到 runPython 函数");
      }
      
      // 包装代码以支持 main 函数和参数
      const wrappedCode = this.wrapCode(request.code, request.params);
      
      // 执行代码
      const result = await runPython(wrappedCode, {
        timeout: (config.timeout_seconds || 30) * 1000, // 转换为毫秒
        // 其他配置选项根据实际 API 调整
      });
      
      const executionTime = (performance.now() - startTime) / 1000;
      
      // 解析结果
      if (result && result.success) {
        // 检查结果中是否包含错误（score为0表示执行失败）
        let parsedResult = result.result;
        if (result.jsonResult) {
          try {
            parsedResult = JSON.parse(result.jsonResult);
          } catch (e) {
            // 静默处理解析错误
            parsedResult = result.result;
          }
        }
        
        // 如果 score 为 0，认为是执行失败
        if (parsedResult && typeof parsedResult === 'object' && parsedResult.score === 0) {
          return {
            success: false,
            result: parsedResult,
            stdout: Array.isArray(result.stdout) ? result.stdout.join('\n') : result.stdout,
            stderr: Array.isArray(result.stderr) ? result.stderr.join('\n') : result.stderr,
            execution_time: executionTime,
            status: "error" as const,
            sandbox_error: parsedResult.reason || "执行失败"
          };
        }
        
        return {
          success: true,
          result: parsedResult,
          stdout: Array.isArray(result.stdout) ? result.stdout.join('\n') : result.stdout,
          stderr: Array.isArray(result.stderr) ? result.stderr.join('\n') : result.stderr,
          execution_time: executionTime,
          status: "success" as const,
          sandbox_error: undefined
        };
      } else {
        // 检查是否有错误信息
        const errorMessage = result?.error || "执行失败";
        return {
          success: false,
          result: undefined,
          stdout: Array.isArray(result?.stdout) ? result.stdout.join('\n') : result?.stdout,
          stderr: Array.isArray(result?.stderr) ? result.stderr.join('\n') : result?.stderr || errorMessage,
          execution_time: executionTime,
          status: "error" as const,
          sandbox_error: errorMessage
        };
      }

    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : String(error);
      return this.createErrorResult(errorMessage, startTime);
    }
  }

  /**
   * 包装代码以支持 main 函数和参数
   * 基于 pyodide_runner.py 的代码包装逻辑
   */
  private wrapCode(code: string, params?: Record<string, any>): string {
    const prefix = `
import json
import sys
import asyncio

class Args:
    def __init__(self, params):
        self.params = params or {}
        # 为了向后兼容，将参数作为属性
        for key, value in self.params.items():
            if isinstance(key, str) and key.isidentifier():
                setattr(self, key, value)

class Output(dict):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        # 确保有基本的输出结构
        if 'score' not in self:
            self['score'] = 1.0
        if 'reason' not in self:
            self['reason'] = '执行完成'

# 初始化参数
args = {}
`;

    const suffix = `

# 执行主函数
result = None
try:
    if 'main' in globals() and callable(main):
        import inspect
        if inspect.iscoroutinefunction(main):
            # 如果是异步函数，使用 asyncio.run
            result = asyncio.run(main(Args(args)))
        else:
            # 如果是同步函数，直接调用
            result = main(Args(args))
    else:
        # 如果没有main函数，直接执行代码块的结果
        result = None
        
    # 确保结果是可序列化的
    if result is None:
        result = Output(score=1.0, reason='执行完成')
    elif not isinstance(result, (dict, list, str, int, float, bool)):
        result = Output(score=1.0, reason=f'执行完成，结果类型: {type(result).__name__}')
        
except Exception as e:
    import traceback
    error_msg = f"{type(e).__name__}: {str(e)}"
    traceback.print_exc()
    print(error_msg, file=sys.stderr)
    result = Output(score=0.0, reason=f'执行失败: {error_msg}')

# 输出结果
result
`;

    if (params && Object.keys(params).length > 0) {
      // 将 JavaScript 的 null 转换为 Python 的 None
      const pythonParams = JSON.stringify(params).replace(/null/g, 'None');
      return prefix + `args = ${pythonParams}\n` + code + suffix;
    } else {
      return prefix + code + suffix;
    }
  }



  /**
   * 创建错误结果
   */
  private createErrorResult(errorMessage: string, startTime: number): ExecutionResult {
    const executionTime = (performance.now() - startTime) / 1000;
    
    return {
      success: false,
      status: "error" as const,
      stderr: errorMessage,
      execution_time: executionTime,
      sandbox_error: errorMessage
    };
  }
}

// ============================================================================
// 主程序入口
// ============================================================================

/**
 * 处理管道通信
 * 模拟 runner.go 的管道通信机制
 */
async function handlePipeComm(): Promise<void> {
  try {
    // 从标准输入读取请求
    const reader = Deno.stdin.readable.getReader();
    const chunks: Uint8Array[] = [];
    
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      chunks.push(value);
    }
    
    // 合并所有数据块
    const totalLength = chunks.reduce((sum, chunk) => sum + chunk.length, 0);
    const combined = new Uint8Array(totalLength);
    let offset = 0;
    for (const chunk of chunks) {
      combined.set(chunk, offset);
      offset += chunk.length;
    }
    
    // 解析请求
    const requestText = new TextDecoder().decode(combined);
    const request: ExecutionRequest = JSON.parse(requestText);
    
    // 执行代码
    const executor = new DirectPyodideExecutor();
    const result = await executor.execute(request);
    
    // 输出结果
    const resultText = JSON.stringify(result, null, 0);
    await Deno.stdout.write(new TextEncoder().encode(resultText));
    
  } catch (error) {
    const errorMessage = error instanceof Error ? error.message : String(error);
    const errorResult: ExecutionResult = {
      success: false,
      status: "error",
      stderr: errorMessage,
      execution_time: 0,
      sandbox_error: `管道通信错误: ${errorMessage}`
    };
    
    const resultText = JSON.stringify(errorResult, null, 0);
    await Deno.stdout.write(new TextEncoder().encode(resultText));
  }
}

// 启动主程序
if (import.meta.main) {
  await handlePipeComm();
}