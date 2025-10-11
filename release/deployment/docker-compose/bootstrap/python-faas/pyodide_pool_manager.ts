#!/usr/bin/env deno run --allow-net --allow-env --allow-read --allow-write --allow-run

/// <reference types="https://deno.land/x/deno@v1.45.5/cli/dts/lib.deno.d.ts" />
// @deno-types="https://deno.land/x/deno@v1.45.5/cli/dts/lib.deno.d.ts"

/**
 * Pyodide 进程池管理器
 *
 * 实现基于进程池的Python代码执行优化
 * 通过预启动的deno进程和Pyodide预加载来提升执行速度
 */

// ==================== 类型定义 ====================

interface PoolConfig {
  minSize: number;
  maxSize: number;
  idleTimeout: number;
  maxExecutionTime: number;
  preloadTimeout: number;
}

interface PooledProcess {
  id: string;
  isReady: boolean;
  isBusy: boolean;
  lastUsed: number;
  executionCount: number;
}

interface ExecutionRequest {
  id: string;
  code: string;
  timeout: number;
  resolve: (result: any) => void;
  reject: (error: any) => void;
  startTime: number;
}

interface ExecutionResult {
  stdout: string;
  stderr: string;
  returnValue: string;
  metadata: {
    duration: number;
    exitCode: number;
    timedOut: boolean;
    processId: string;
  };
}

// ==================== 进程池管理器 ====================

class PyodidePoolManager {
  private config: PoolConfig;
  private processes: Map<string, PooledProcess> = new Map();
  private availableProcesses: Set<string> = new Set();
  private busyProcesses: Set<string> = new Set();
  private pendingRequests: ExecutionRequest[] = [];
  private nextProcessId = 1;
  private isShuttingDown = false;
  private cleanupInterval: number | null = null;

  constructor(config: Partial<PoolConfig> = {}) {
    this.config = {
      minSize: config.minSize || 2,
      maxSize: config.maxSize || 8,
      idleTimeout: config.idleTimeout || 300000, // 5分钟
      maxExecutionTime: config.maxExecutionTime || 30000, // 30秒
      preloadTimeout: config.preloadTimeout || 60000, // 1分钟
    };

    console.log(`🏊 初始化Pyodide进程池: min=${this.config.minSize}, max=${this.config.maxSize}`);
  }

  /**
   * 启动进程池
   */
  async start(): Promise<void> {
    console.log("🚀 启动Pyodide进程池...");

    // 启动最小数量的进程
    const initPromises: Promise<PooledProcess>[] = [];
    for (let i = 0; i < this.config.minSize; i++) {
      initPromises.push(this.createProcess());
    }

    await Promise.all(initPromises);

    // 启动清理任务
    this.startCleanupTask();

    console.log(`✅ 进程池启动完成，当前进程数: ${this.processes.size}`);
  }

  /**
   * 创建新的进程槽位
   */
  private async createProcess(): Promise<PooledProcess> {
    const processId = `pyodide-${this.nextProcessId++}`;

    console.log(`🔧 创建新进程槽位: ${processId}`);

    try {
      const pooledProcess: PooledProcess = {
        id: processId,
        isReady: true,
        isBusy: false,
        lastUsed: Date.now(),
        executionCount: 0
      };

      this.processes.set(processId, pooledProcess);
      this.availableProcesses.add(processId);

      console.log(`✅ 进程槽位创建成功: ${processId}`);
      return pooledProcess;

    } catch (error) {
      console.error(`❌ 创建进程槽位失败: ${processId}`, error);
      throw error;
    }
  }


  /**
   * 执行Python代码
   */
  async executePython(code: string, timeout = 30000): Promise<ExecutionResult> {
    if (this.isShuttingDown) {
      throw new Error("进程池正在关闭");
    }

    const requestId = `req-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;

    return new Promise((resolve, reject) => {
      const request: ExecutionRequest = {
        id: requestId,
        code,
        timeout,
        resolve,
        reject,
        startTime: Date.now()
      };

      this.pendingRequests.push(request);
      this.processRequest();
    });
  }

  /**
   * 处理执行请求
   */
  private async processRequest(): Promise<void> {
    if (this.pendingRequests.length === 0) return;

    // 尝试获取可用进程
    let processId = this.getAvailableProcess();

    // 如果没有可用进程且未达到最大数量，创建新进程
    if (!processId && this.processes.size < this.config.maxSize) {
      try {
        const newProcess = await this.createProcess();
        processId = newProcess.id;
      } catch (error) {
        console.error("创建新进程失败:", error);
      }
    }

    if (!processId) {
      // 没有可用进程，等待
      return;
    }

    const request = this.pendingRequests.shift();
    if (!request) return;

    await this.executeWithProcess(processId, request);

    // 继续处理其他请求
    if (this.pendingRequests.length > 0) {
      setTimeout(() => this.processRequest(), 0);
    }
  }

  /**
   * 获取可用进程
   */
  private getAvailableProcess(): string | null {
    for (const processId of this.availableProcesses) {
      const process = this.processes.get(processId);
      if (process && process.isReady && !process.isBusy) {
        return processId;
      }
    }
    return null;
  }

  /**
   * 使用指定进程执行代码
   */
  private async executeWithProcess(processId: string, request: ExecutionRequest): Promise<void> {
    const process = this.processes.get(processId);
    if (!process) {
      request.reject(new Error(`进程不存在: ${processId}`));
      return;
    }

    // 标记进程为忙碌
    process.isBusy = true;
    process.lastUsed = Date.now();
    this.availableProcesses.delete(processId);
    this.busyProcesses.add(processId);

    console.log(`⚡ 使用进程执行代码: ${processId}, 请求: ${request.id}`);

    try {
      // 注入return_val函数
      const enhancedCode = `
def return_val(value):
    """return_val函数实现 - 只输出JSON内容到stdout最后一行"""
    if value is None:
        ret_val = ""
    else:
        ret_val = str(value)
    print(ret_val)

${request.code}
`;

      // 使用pyodide-sandbox执行代码（每次都是新的进程调用，但进程池保持活跃）
      const result = await this.executeWithPyodideSandbox(enhancedCode, request.timeout, processId);

      process.executionCount++;
      request.resolve(result);

    } catch (error) {
      console.error(`❌ 执行失败: ${processId}`, error);
      request.reject(error);
    } finally {
      this.releaseProcess(processId);
    }
  }

   /**
    * 预处理代码，处理换行符和特殊字符问题
    */
   private preprocessCode(code: string, processId?: string): string {
     try {
       let processedCode = code;

       // 处理JSON层面的双重转义
       processedCode = processedCode.replace(/\\\\n/g, '\\n');  // \\n -> \n (JSON转义)
       processedCode = processedCode.replace(/\\\\t/g, '\\t');  // \\t -> \t (JSON转义)
       processedCode = processedCode.replace(/\\\\r/g, '\\r');  // \\r -> \r (JSON转义)
       processedCode = processedCode.replace(/\\\\"/g, '\\"');  // \\" -> \" (JSON转义)
       processedCode = processedCode.replace(/\\\\\\\\/g, '\\\\'); // \\\\ -> \\ (JSON转义)

       // 关键修复：处理Python字符串字面量中的转义序列
       // 将 "\\na" 转换为 "\na"，这样Python会正确解释为换行符+字母a
       processedCode = processedCode.replace(/"\\\\n/g, '"\n');  // "\\n -> "\n (实际换行符)
       processedCode = processedCode.replace(/"\\\\t/g, '"\t');  // "\\t -> "\t (实际制表符)
       processedCode = processedCode.replace(/"\\\\r/g, '"\r');  // "\\r -> "\r (实际回车符)
       processedCode = processedCode.replace(/"\\\\"/g, '"\""');  // "\\" -> "\" (实际双引号)
       processedCode = processedCode.replace(/"\\\\\\\\/g, '"\\'); // "\\\\ -> "\ (实际反斜杠)

       // 额外处理：确保所有字符串字面量中的转义序列都被正确处理
       processedCode = processedCode.replace(/"\\n/g, '"\n');  // "\\n -> "\n (实际换行符)
       processedCode = processedCode.replace(/"\\t/g, '"\t');  // "\\t -> "\t (实际制表符)
       processedCode = processedCode.replace(/"\\r/g, '"\r');  // "\\r -> "\r (实际回车符)
       processedCode = processedCode.replace(/"\\"/g, '"\""');  // "\\" -> "\" (实际双引号)

       // 检查是否处理了转义字符
       if (code.includes('\\\\n') && processedCode.includes('\\n')) {
         console.log(`✅ 已处理转义字符: ${processId || 'unknown'}`);
       }

       // 记录处理前后的差异（仅用于调试）
       if (code !== processedCode) {
         console.log(`🔧 代码预处理完成: ${processId || 'unknown'}, 处理了转义字符`);
       }

       return processedCode;
     } catch (error) {
       console.error(`❌ 代码预处理失败: ${processId || 'unknown'}`, error);
       // 如果预处理失败，返回原始代码
       return code;
     }
   }

  /**
   * 使用pyodide-sandbox执行代码
   */
  private async executeWithPyodideSandbox(code: string, timeout: number, processId: string): Promise<ExecutionResult> {
    const startTime = Date.now();

    try {
      // 预处理代码，处理Unicode字符问题
      const processedCode = this.preprocessCode(code, processId);

      // 使用 pyodide-sandbox 执行代码
      // 确保代码正确编码，处理特殊字符
      const process = new Deno.Command("deno", {
        args: [
          "run",
          "-A",
          "jsr:@eyurtsev/pyodide-sandbox",
          "-c",
          processedCode
        ],
        stdout: "piped",
        stderr: "piped",
        timeout: timeout,
        env: {
          "PYTHONIOENCODING": "utf-8",
          "LANG": "en_US.UTF-8",
          "LC_ALL": "en_US.UTF-8"
        }
      });

      const { stdout, stderr, code: exitCode } = await process.output();
      const duration = Date.now() - startTime;

      // 使用UTF-8解码，并处理可能的编码错误
      const stdoutText = new TextDecoder('utf-8', { fatal: false }).decode(stdout);
      const stderrText = new TextDecoder('utf-8', { fatal: false }).decode(stderr);

      // 提取 return_val 的结果
      const returnValue = this.extractReturnValue(stdoutText);
      const cleanStdout = this.cleanStdout(stdoutText);

      return {
        stdout: cleanStdout,
        stderr: stderrText,
        returnValue,
        metadata: {
          duration,
          exitCode,
          timedOut: false,
          processId
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
            timedOut: true,
            processId
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
          timedOut: false,
          processId
        }
      };
    }
  }


  /**
   * 提取return_val的结果
   */
  private extractReturnValue(output: string): string {
    try {
      // 首先尝试解析pyodide-sandbox的输出JSON
      const parsedOutput = JSON.parse(output);

      // 优先使用result字段（这是pyodide-sandbox捕获的return_val输出）
      if (parsedOutput.result) {
        // 如果result是字符串，尝试解析为JSON
        if (typeof parsedOutput.result === 'string') {
          try {
            // 解析JSON字符串，然后重新序列化以去除多余的转义
            const parsedResult = JSON.parse(parsedOutput.result);
            return JSON.stringify(parsedResult, null, 0);
          } catch {
            // 如果解析失败，直接返回原始字符串
            return parsedOutput.result;
          }
        }
        return parsedOutput.result;
      }

      // 如果没有result字段，从stdout中提取
      const pyodideStdout = parsedOutput.stdout || "";

      // 首先尝试提取特殊标记格式的return_val
      const specialMarkerMatch = pyodideStdout.match(/__COZE_RETURN_VAL_START__\s*\n?(.*?)\s*\n?__COZE_RETURN_VAL_END__/s);
      if (specialMarkerMatch) {
        const returnVal = specialMarkerMatch[1].trim();
        try {
          // 尝试解析为JSON，如果是JSON则重新序列化
          const parsed = JSON.parse(returnVal);
          return JSON.stringify(parsed, null, 0);
        } catch {
          // 如果不是JSON，直接返回
          return returnVal;
        }
      }

      // 查找return_val输出的JSON内容（改进正则表达式以处理复杂内容）
      const jsonMatch = pyodideStdout.match(/\{[^{}]*(?:"score"[^{}]*)*\}/);
      if (jsonMatch) {
        return jsonMatch[0];
      }

      // 如果没有找到特定的JSON，尝试查找任何JSON对象
      const anyJsonMatch = pyodideStdout.match(/\{[^{}]*\}/);
      if (anyJsonMatch) {
        return anyJsonMatch[0];
      }

      return "";
    } catch (error) {
      // 如果JSON解析失败，尝试直接从原始输出中提取
      try {
        // 首先尝试特殊标记格式
        const specialMarkerMatch = output.match(/__COZE_RETURN_VAL_START__\s*\n?(.*?)\s*\n?__COZE_RETURN_VAL_END__/s);
        if (specialMarkerMatch) {
          const returnVal = specialMarkerMatch[1].trim();
          try {
            const parsed = JSON.parse(returnVal);
            return JSON.stringify(parsed, null, 0);
          } catch {
            return returnVal;
          }
        }

        // 改进的JSON匹配，处理复杂内容
        const jsonMatch = output.match(/\{[^{}]*(?:"score"[^{}]*)*\}/);
        if (jsonMatch) {
          return jsonMatch[0];
        }

        const anyJsonMatch = output.match(/\{[^{}]*\}/);
        if (anyJsonMatch) {
          return anyJsonMatch[0];
        }
      } catch (fallbackError) {
        console.error("解析输出失败:", error);
        console.error("回退解析也失败:", fallbackError);
      }

      return "";
    }
  }

  /**
   * 清理stdout
   */
  private cleanStdout(output: string): string {
    try {
      // 首先尝试解析pyodide-sandbox的输出JSON
      const parsedOutput = JSON.parse(output);

      // 从pyodide-sandbox的stdout中移除return_val输出的JSON
      const pyodideStdout = parsedOutput.stdout || "";

      // 首先移除特殊标记格式的return_val输出
      let cleaned = pyodideStdout.replace(/__COZE_RETURN_VAL_START__\s*\n?.*?\s*\n?__COZE_RETURN_VAL_END__/gs, '');

      // 移除JSON对象，保留其他内容（改进正则表达式以处理复杂内容）
      cleaned = cleaned.replace(/\{[^{}]*(?:"score"[^{}]*)*\}/g, '');
      if (cleaned === pyodideStdout) {
        // 如果没有找到特定的JSON，尝试移除任何JSON对象
        cleaned = pyodideStdout.replace(/\{[^{}]*\}/g, '');
      }

      // 清理多余的空行
      cleaned = cleaned.replace(/\n+/g, '\n').trim();

      // 返回清理后的纯stdout文本
      return cleaned;
    } catch (error) {
      // 如果JSON解析失败，尝试直接从原始输出中清理
      try {
        // 首先移除特殊标记格式
        let cleaned = output.replace(/__COZE_RETURN_VAL_START__\s*\n?.*?\s*\n?__COZE_RETURN_VAL_END__/gs, '');

        cleaned = cleaned.replace(/\{[^{}]*(?:"score"[^{}]*)*\}/g, '');
        if (cleaned === output) {
          cleaned = output.replace(/\{[^{}]*\}/g, '');
        }
        cleaned = cleaned.replace(/\n+/g, '\n').trim();
        return cleaned;
      } catch (fallbackError) {
        console.error("清理输出失败:", error);
        console.error("回退清理也失败:", fallbackError);
        // 回退为原始内容（可能是pyodide-sandbox的JSON字符串）
        return output;
      }
    }
  }

  /**
   * 释放进程
   */
  private releaseProcess(processId: string): void {
    const process = this.processes.get(processId);
    if (!process) return;

    process.isBusy = false;
    this.busyProcesses.delete(processId);
    this.availableProcesses.add(processId);

    console.log(`🔄 释放进程: ${processId}`);
  }

  /**
   * 启动清理任务
   */
  private startCleanupTask(): void {
    this.cleanupInterval = setInterval(() => {
      this.cleanupIdleProcesses();
    }, 60000); // 每分钟清理一次
  }

  /**
   * 清理空闲进程
   */
  private cleanupIdleProcesses(): void {
    const now = Date.now();
    const toRemove: string[] = [];

    for (const [processId, process] of this.processes) {
      if (!process.isBusy &&
          now - process.lastUsed > this.config.idleTimeout &&
          this.processes.size > this.config.minSize) {
        toRemove.push(processId);
      }
    }

    for (const processId of toRemove) {
      this.destroyProcess(processId);
    }

    if (toRemove.length > 0) {
      console.log(`🧹 清理空闲进程: ${toRemove.join(', ')}`);
    }
  }

  /**
   * 销毁进程槽位
   */
  private destroyProcess(processId: string): void {
    const process = this.processes.get(processId);
    if (!process) return;

    try {
      this.processes.delete(processId);
      this.availableProcesses.delete(processId);
      this.busyProcesses.delete(processId);

      console.log(`🗑️ 销毁进程槽位: ${processId}`);
    } catch (error) {
      console.error(`销毁进程槽位失败: ${processId}`, error);
    }
  }

  /**
   * 获取池状态
   */
  getPoolStatus() {
    return {
      totalProcesses: this.processes.size,
      availableProcesses: this.availableProcesses.size,
      busyProcesses: this.busyProcesses.size,
      pendingRequests: this.pendingRequests.length,
      config: this.config
    };
  }

  /**
   * 关闭进程池
   */
  async shutdown(): Promise<void> {
    console.log("🛑 关闭进程池...");
    this.isShuttingDown = true;

    if (this.cleanupInterval) {
      clearInterval(this.cleanupInterval);
    }

    // 拒绝所有待处理请求
    for (const request of this.pendingRequests) {
      request.reject(new Error("进程池正在关闭"));
    }
    this.pendingRequests = [];

    // 销毁所有进程
    const destroyPromises = Array.from(this.processes.keys()).map(processId =>
      this.destroyProcess(processId)
    );

    await Promise.all(destroyPromises);

    console.log("✅ 进程池关闭完成");
  }
}

export { PyodidePoolManager, type PoolConfig, type ExecutionResult };
