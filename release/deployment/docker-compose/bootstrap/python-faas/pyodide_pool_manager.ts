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

      // 预加载 Pyodide，提升首次执行速度
      try {
        await this.preloadPyodide(processId);
      } catch (e) {
        console.warn(`⚠️  预加载失败但不影响进程创建: ${processId}: ${e}`);
      }

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
  }      /**
    * 预处理代码，处理换行符和特殊字符问题
    */
   private preprocessCode(code: string, processId?: string): string {
     try {
       const originalCode = code;
       console.log(`🔍 [${processId || 'unknown'}] 开始预处理代码，长度: ${code.length}`);

       // 仅在字符串字面量内部，将实际控制字符转义为可见序列，避免 Python 源码语法错误
       const escapeControlsInLiterals = (src: string): string => {
         // 处理双引号字符串
         let out = src.replace(/"([^"\\]|\\.)*"/gs, (m) => {
           const inner = m.slice(1, -1)
             .replace(/\n/g, "\\n")
             .replace(/\r/g, "\\r")
             .replace(/\t/g, "\\t");
           return `"${inner}"`;
         });
         // 处理单引号字符串
         out = out.replace(/'([^'\\]|\\.)*'/gs, (m) => {
           const inner = m.slice(1, -1)
             .replace(/\n/g, "\\n")
             .replace(/\r/g, "\\r")
             .replace(/\t/g, "\\t");
           return `'${inner}'`;
         });
         return out;
       };

       const processedCode = escapeControlsInLiterals(originalCode);

       if (originalCode !== processedCode) {
         console.log(`🔧 [${processId || 'unknown'}] 已对字符串字面量进行控制字符转义处理`);
         console.log(`📊 [${processId || 'unknown'}] 预处理统计: 原始长度=${originalCode.length}, 处理后长度=${processedCode.length}`);
       } else {
         console.log(`ℹ️ [${processId || 'unknown'}] 代码无需预处理`);
       }

       return processedCode;
     } catch (error) {
       console.error(`❌ [${processId || 'unknown'}] 代码预处理失败:`, error);
       return code;
     }
   }


  /**
   * 使用pyodide-sandbox执行代码
   */
  private async executeWithPyodideSandbox(code: string, timeout: number, processId: string): Promise<ExecutionResult> {
    const startTime = Date.now();

    try {
      console.log(`🚀 [${processId}] 开始执行Python代码，超时: ${timeout}ms`);

      // 预处理代码（仅做 JSON 层转义归一化）
      const processedCode = this.preprocessCode(code, processId);

      console.log(`📝 [${processId}] 预处理完成，写入临时文件并调用pyodide-sandbox`);

      // 将代码写入临时文件，避免 -c 方式带来的转义与长度问题
      const tmpFile = await Deno.makeTempFile({ prefix: `pyodide-${processId}-`, suffix: ".py" });
      await Deno.writeTextFile(tmpFile, processedCode);
      console.log(`🗂️ [${processId}] 临时代码文件: ${tmpFile}`);
      console.log(`🧾 [${processId}] 代码预览(前400字):\n${processedCode.slice(0, 400)}`);

      const importMap = Deno.env.get("PYODIDE_IMPORT_MAP") || "/tmp/faas-workspace/vendor/import_map.json";
      const process = new Deno.Command("deno", {
        args: [
          "run",
          "-A",
          `--import-map=${importMap}`,
          "/tmp/faas-workspace/vendor/jsr.io/@eyurtsev/pyodide-sandbox/0.0.3/main.ts",
          "-f",
          tmpFile
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

      console.log(`⏱️ [${processId}] pyodide-sandbox执行完成，耗时: ${duration}ms，退出码: ${exitCode}`);

      // 使用UTF-8解码，并处理可能的编码错误
      const stdoutText = new TextDecoder('utf-8', { fatal: false }).decode(stdout);
      const stderrText = new TextDecoder('utf-8', { fatal: false }).decode(stderr);

      console.log(`📤 [${processId}] 原始stdout长度: ${stdoutText.length}`);
      console.log(`📤 [${processId}] 原始stderr长度: ${stderrText.length}`);

      if (stderrText) {
        console.log(`⚠️ [${processId}] stderr内容: ${stderrText.substring(0, 200)}${stderrText.length > 200 ? '...' : ''}`);
      }

      // 提取 return_val 的结果
      const returnValue = this.extractReturnValue(stdoutText);
      const cleanStdout = this.cleanStdout(stdoutText);

      console.log(`🔍 [${processId}] 提取的返回值长度: ${returnValue.length}`);
      console.log(`🔍 [${processId}] 清理后的stdout长度: ${cleanStdout.length}`);

      if (returnValue) {
        console.log(`✅ [${processId}] 成功提取返回值: ${returnValue.substring(0, 100)}${returnValue.length > 100 ? '...' : ''}`);
      } else {
        console.log(`❌ [${processId}] 未能提取到返回值`);
        console.log(`🔍 [${processId}] 原始stdout内容: ${stdoutText.substring(0, 500)}${stdoutText.length > 500 ? '...' : ''}`);
      }

      const keepTmp = Deno.env.get("FAAS_KEEP_TMP") === "1";
      const shouldDeleteTmp = !keepTmp && exitCode === 0 && (!stderrText || stderrText.length === 0);
      if (shouldDeleteTmp) {
        try {
          await Deno.remove(tmpFile);
          console.log(`🧽 [${processId}] 已清理临时文件`);
        } catch (e) {
          console.warn(`⚠️ [${processId}] 清理临时文件失败: ${e}`);
        }
      } else {
        console.log(`🗂️ [${processId}] 保留临时代码文件用于排查: ${tmpFile} (FAAS_KEEP_TMP=${keepTmp ? '1' : '0'}, exit=${exitCode}, stderr_len=${stderrText?.length || 0})`);
      }

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
      console.error(`❌ [${processId}] pyodide-sandbox执行异常:`, error);

      if ((error as any).name === 'AbortError' || (error as any).message?.includes('timeout')) {
        console.log(`⏰ [${processId}] 执行超时 (${timeout}ms)`);
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

      // 失败分支：不要尝试删除临时文件，便于排查
      console.warn(`🧾 [${processId}] 发生异常，保留临时代码文件: ${typeof tmpFile !== 'undefined' ? tmpFile : '(未创建)'}`);

      return {
        stdout: "",
        stderr: `Pyodide执行错误: ${(error as any).message}`,
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
   * 预加载 Pyodide（与 K8s 行为保持一致）
   */
  private async preloadPyodide(processId: string): Promise<void> {
    console.log(`⏳ [${processId}] 预加载Pyodide...`);

    try {
      const importMap = Deno.env.get("PYODIDE_IMPORT_MAP") || "/tmp/faas-workspace/vendor/import_map.json";
      const workspaceDir = Deno.env.get("FAAS_WORKSPACE") || "/tmp/faas-workspace";

      const preloadTestFile = `${workspaceDir}/preload_test_${processId}.py`;
      await Deno.writeTextFile(preloadTestFile, "print('preload test')");

      const preloadCommand = new Deno.Command("deno", {
        args: [
          "run",
          "-A",
          `--import-map=${importMap}`,
          "/tmp/faas-workspace/vendor/jsr.io/@eyurtsev/pyodide-sandbox/0.0.3/main.ts",
          "-f",
          preloadTestFile
        ],
        stdout: "piped",
        stderr: "piped",
        timeout: 30000,
        env: {
          "PYTHONIOENCODING": "utf-8",
          "LANG": "en_US.UTF-8",
          "LC_ALL": "en_US.UTF-8"
        }
      });

      const { stderr, code: exitCode } = await preloadCommand.output();

      try {
        await Deno.remove(preloadTestFile);
      } catch (e) {
        console.warn(`⚠️ [${processId}] 清理预加载测试文件失败: ${e}`);
      }

      if (exitCode === 0) {
        console.log(`✅ [${processId}] Pyodide预加载成功`);
      } else {
        const stderrText = new TextDecoder('utf-8', { fatal: false }).decode(stderr);
        console.warn(`⚠️ [${processId}] Pyodide预加载完成但有警告: ${stderrText}`);
      }

    } catch (error) {
      console.error(`❌ [${processId}] Pyodide预加载失败:`, error);
      // 抛出让调用方决定是否忽略
      throw error;
    }
  }


  /**
   * 提取return_val的结果
   */
  private extractReturnValue(output: string): string {
    try {
      console.log(`🔍 开始提取返回值，输出长度: ${output.length}`);

      // 首先尝试解析pyodide-sandbox的输出JSON
      const parsedOutput = JSON.parse(output);
      console.log(`📋 成功解析pyodide-sandbox输出JSON`);

      // 优先使用result字段（这是pyodide-sandbox捕获的return_val输出）
      if (parsedOutput.result) {
        console.log(`✅ 找到result字段: ${typeof parsedOutput.result}`);
        // 如果result是字符串，尝试解析为JSON
        if (typeof parsedOutput.result === 'string') {
          try {
            // 解析JSON字符串，然后重新序列化以去除多余的转义
            const parsedResult = JSON.parse(parsedOutput.result);
            const result = JSON.stringify(parsedResult, null, 0);
            console.log(`🎯 从result字段提取到JSON返回值: ${result.substring(0, 100)}${result.length > 100 ? '...' : ''}`);
            return result;
          } catch {
            // 如果解析失败，直接返回原始字符串
            console.log(`📝 从result字段提取到字符串返回值: ${parsedOutput.result.substring(0, 100)}${parsedOutput.result.length > 100 ? '...' : ''}`);
            return parsedOutput.result;
          }
        }
        console.log(`📊 从result字段提取到非字符串返回值: ${parsedOutput.result}`);
        return parsedOutput.result;
      }

      // 如果没有result字段，从stdout中提取
      const pyodideStdout = parsedOutput.stdout || "";
      console.log(`📤 从stdout中提取，长度: ${pyodideStdout.length}`);

      // 首先尝试提取特殊标记格式的return_val
      const specialMarkerMatch = pyodideStdout.match(/__COZE_RETURN_VAL_START__\s*\n?(.*?)\s*\n?__COZE_RETURN_VAL_END__/s);
      if (specialMarkerMatch) {
        const returnVal = specialMarkerMatch[1].trim();
        console.log(`🎯 找到特殊标记格式返回值: ${returnVal.substring(0, 100)}${returnVal.length > 100 ? '...' : ''}`);
        try {
          // 尝试解析为JSON，如果是JSON则重新序列化
          const parsed = JSON.parse(returnVal);
          const result = JSON.stringify(parsed, null, 0);
          console.log(`✅ 特殊标记格式JSON解析成功: ${result.substring(0, 100)}${result.length > 100 ? '...' : ''}`);
          return result;
        } catch {
          // 如果不是JSON，直接返回
          console.log(`📝 特殊标记格式非JSON返回值: ${returnVal}`);
          return returnVal;
        }
      }

      // 查找return_val输出的JSON内容（改进正则表达式以处理复杂内容）
      const jsonMatch = pyodideStdout.match(/\{[^{}]*(?:"score"[^{}]*)*\}/);
      if (jsonMatch) {
        console.log(`🎯 找到JSON格式返回值: ${jsonMatch[0].substring(0, 100)}${jsonMatch[0].length > 100 ? '...' : ''}`);
        return jsonMatch[0];
      }

      // 如果没有找到特定的JSON，尝试查找任何JSON对象
      const anyJsonMatch = pyodideStdout.match(/\{[^{}]*\}/);
      if (anyJsonMatch) {
        console.log(`🎯 找到通用JSON格式返回值: ${anyJsonMatch[0].substring(0, 100)}${anyJsonMatch[0].length > 100 ? '...' : ''}`);
        return anyJsonMatch[0];
      }

      console.log(`❌ 未找到任何返回值格式`);
      return "";
    } catch (error) {
      console.log(`⚠️ JSON解析失败，尝试直接提取: ${error.message}`);
      // 如果JSON解析失败，尝试直接从原始输出中提取
      try {
        // 首先尝试特殊标记格式
        const specialMarkerMatch = output.match(/__COZE_RETURN_VAL_START__\s*\n?(.*?)\s*\n?__COZE_RETURN_VAL_END__/s);
        if (specialMarkerMatch) {
          const returnVal = specialMarkerMatch[1].trim();
          console.log(`🎯 直接提取特殊标记格式返回值: ${returnVal.substring(0, 100)}${returnVal.length > 100 ? '...' : ''}`);
          try {
            const parsed = JSON.parse(returnVal);
            const result = JSON.stringify(parsed, null, 0);
            console.log(`✅ 直接提取JSON解析成功: ${result.substring(0, 100)}${result.length > 100 ? '...' : ''}`);
            return result;
          } catch {
            console.log(`📝 直接提取非JSON返回值: ${returnVal}`);
            return returnVal;
          }
        }

        // 改进的JSON匹配，处理复杂内容
        const jsonMatch = output.match(/\{[^{}]*(?:"score"[^{}]*)*\}/);
        if (jsonMatch) {
          console.log(`🎯 直接提取JSON格式返回值: ${jsonMatch[0].substring(0, 100)}${jsonMatch[0].length > 100 ? '...' : ''}`);
          return jsonMatch[0];
        }

        const anyJsonMatch = output.match(/\{[^{}]*\}/);
        if (anyJsonMatch) {
          console.log(`🎯 直接提取通用JSON格式返回值: ${anyJsonMatch[0].substring(0, 100)}${anyJsonMatch[0].length > 100 ? '...' : ''}`);
          return anyJsonMatch[0];
        }
      } catch (fallbackError) {
        console.error("❌ 解析输出失败:", error);
        console.error("❌ 回退解析也失败:", fallbackError);
      }

      console.log(`❌ 所有提取方法都失败，返回空字符串`);
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
