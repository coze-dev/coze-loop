#!/usr/bin/env deno run --allow-all

/**
 * coze-loop-faas-enhanced 增强版服务器
 * 提供基于沙箱池和任务调度的高并发代码执行服务
 * 支持 JavaScript/TypeScript 和 Python
 */

interface ExecutionRequest {
  language: string;
  code: string;
  input?: any;
  timeout?: number;
  priority?: "low" | "normal" | "high" | "urgent";
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
    task_id: string;
    instance_id: string;
    duration: number;
    pool_stats: any;
  };
}

interface SandboxInstance {
  id: string;
  language: string;
  status: "idle" | "busy" | "error";
  createdAt: number;
  lastUsed: number;
  executeCount: number;
}

interface TaskMetrics {
  totalTasks: number;
  completedTasks: number;
  failedTasks: number;
  queuedTasks: number;
  averageExecutionTime: number;
}

class SandboxPool {
  private instances: Map<string, SandboxInstance> = new Map();
  private idleInstances: SandboxInstance[] = [];
  private readonly maxInstances: number;
  private readonly minInstances: number;
  private instanceCounter = 0;

  constructor(maxInstances = 50, minInstances = 5) {
    this.maxInstances = maxInstances;
    this.minInstances = minInstances;
    
    // 预热实例池
    this.warmUpPool();
  }

  private async warmUpPool() {
    console.log(`预热沙箱池，创建 ${this.minInstances} 个实例...`);
    
    for (let i = 0; i < this.minInstances; i++) {
      const language = i % 2 === 0 ? "javascript" : "python";
      const instance = this.createInstance(language);
      this.idleInstances.push(instance);
    }
    
    console.log(`沙箱池预热完成，创建了 ${this.idleInstances.length} 个实例`);
  }

  private createInstance(language: string): SandboxInstance {
    const id = `sandbox-${++this.instanceCounter}-${Date.now()}`;
    const instance: SandboxInstance = {
      id,
      language,
      status: "idle",
      createdAt: Date.now(),
      lastUsed: Date.now(),
      executeCount: 0,
    };
    
    this.instances.set(id, instance);
    console.log(`创建沙箱实例: ${id} (${language})`);
    
    return instance;
  }

  async acquireInstance(language: string): Promise<SandboxInstance> {
    // 尝试从空闲池获取匹配语言的实例
    const idleIndex = this.idleInstances.findIndex(
      inst => inst.language === language && inst.status === "idle"
    );
    
    if (idleIndex >= 0) {
      const instance = this.idleInstances.splice(idleIndex, 1)[0];
      instance.status = "busy";
      instance.lastUsed = Date.now();
      return instance;
    }
    
    // 如果没有空闲实例且未达到最大限制，创建新实例
    if (this.instances.size < this.maxInstances) {
      const instance = this.createInstance(language);
      instance.status = "busy";
      return instance;
    }
    
    throw new Error(`沙箱池已达到最大实例数限制: ${this.maxInstances}`);
  }

  releaseInstance(instance: SandboxInstance) {
    instance.status = "idle";
    instance.lastUsed = Date.now();
    instance.executeCount++;
    
    // 如果实例执行次数过多，销毁实例
    if (instance.executeCount > 100) {
      this.destroyInstance(instance);
      return;
    }
    
    // 放回空闲池
    this.idleInstances.push(instance);
    
    // 定期清理超时的空闲实例
    this.cleanupIdleInstances();
  }

  private destroyInstance(instance: SandboxInstance) {
    this.instances.delete(instance.id);
    const idleIndex = this.idleInstances.indexOf(instance);
    if (idleIndex >= 0) {
      this.idleInstances.splice(idleIndex, 1);
    }
    console.log(`销毁沙箱实例: ${instance.id}`);
  }

  private cleanupIdleInstances() {
    const now = Date.now();
    const timeout = 5 * 60 * 1000; // 5分钟超时
    
    this.idleInstances = this.idleInstances.filter(instance => {
      if (now - instance.lastUsed > timeout) {
        this.destroyInstance(instance);
        return false;
      }
      return true;
    });
  }

  getStats() {
    return {
      totalInstances: this.instances.size,
      idleInstances: this.idleInstances.length,
      activeInstances: this.instances.size - this.idleInstances.length,
    };
  }
}

class TaskScheduler {
  private taskQueue: Array<{
    task: any;
    priority: number;
    resolve: (value: any) => void;
    reject: (reason: any) => void;
  }> = [];
  
  private workers: Worker[] = [];
  private readonly maxWorkers: number;
  private metrics: TaskMetrics = {
    totalTasks: 0,
    completedTasks: 0,
    failedTasks: 0,
    queuedTasks: 0,
    averageExecutionTime: 0,
  };

  constructor(private sandboxPool: SandboxPool, maxWorkers = 10) {
    this.maxWorkers = maxWorkers;
    this.startWorkers();
  }

  private startWorkers() {
    // 这里简化实现，实际应该使用Web Workers或类似机制
    console.log(`启动 ${this.maxWorkers} 个任务处理协程`);
  }

  async submitTask(
    code: string,
    language: string,
    priority: "low" | "normal" | "high" | "urgent" = "normal",
    timeout = 30000
  ): Promise<{ result: ExecutionResult; taskId: string; instanceId: string; duration: number }> {
    const taskId = `task-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
    const priorityValue = this.getPriorityValue(priority);
    
    this.metrics.totalTasks++;
    this.metrics.queuedTasks++;
    
    return new Promise(async (resolve, reject) => {
      try {
        const startTime = Date.now();
        
        // 获取沙箱实例
        const instance = await this.sandboxPool.acquireInstance(language);
        
        try {
          // 执行任务
          const result = await this.executeTask(code, language, timeout, instance);
          const duration = Date.now() - startTime;
          
          this.metrics.completedTasks++;
          this.metrics.queuedTasks--;
          this.updateAverageExecutionTime(duration);
          
          resolve({
            result,
            taskId,
            instanceId: instance.id,
            duration,
          });
        } finally {
          // 归还实例
          this.sandboxPool.releaseInstance(instance);
        }
      } catch (error) {
        this.metrics.failedTasks++;
        this.metrics.queuedTasks--;
        reject(error);
      }
    });
  }

  private getPriorityValue(priority: string): number {
    switch (priority) {
      case "urgent": return 4;
      case "high": return 3;
      case "normal": return 2;
      case "low": return 1;
      default: return 2;
    }
  }

  private async executeTask(
    code: string,
    language: string,
    timeout: number,
    instance: SandboxInstance
  ): Promise<ExecutionResult> {
    // 创建临时文件
    const tempFile = await this.createTempFile(language, code);
    
    try {
      // 执行代码
      return await this.executeCode(tempFile, timeout);
    } finally {
      // 清理临时文件
      await this.cleanup(tempFile);
    }
  }

  private async createTempFile(language: string, code: string): Promise<string> {
    const timestamp = Date.now();
    const randomId = Math.random().toString(36).substr(2, 9);
    
    let fileContent = '';
    let extension = '';
    
    switch (language.toLowerCase()) {
      case 'javascript':
      case 'typescript':
        extension = '.ts';
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
  const userFunction = () => {
    ${code}
  };
  
  const result = userFunction();
  
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

stdout_capture = io.StringIO()
stderr_capture = io.StringIO()

try:
    with redirect_stdout(stdout_capture), redirect_stderr(stderr_capture):
        ${code}
        
        ret_val = locals().get('result', None)
    
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
    
    const tempFile = `/tmp/faas-workspace/temp_${timestamp}_${randomId}${extension}`;
    await Deno.writeTextFile(tempFile, fileContent);
    return tempFile;
  }

  private async executeCode(tempFile: string, timeout: number): Promise<ExecutionResult> {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), timeout);
    
    try {
      let command: Deno.Command;
      
      if (tempFile.endsWith('.ts') || tempFile.endsWith('.js')) {
        command = new Deno.Command("deno", {
          args: ["run", "--allow-all", "--quiet", tempFile],
          stdout: "piped",
          stderr: "piped",
          signal: controller.signal,
        });
      } else if (tempFile.endsWith('.py')) {
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
      
      if (exitCode === 0 && stdoutText.trim()) {
        try {
          const result = JSON.parse(stdoutText.trim());
          return {
            stdout: result.stdout || "",
            stderr: result.stderr || stderrText,
            returnValue: result.ret_val || ""
          };
        } catch {
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

  private async cleanup(tempFile: string): Promise<void> {
    try {
      await Deno.remove(tempFile);
    } catch (error) {
      console.warn(`Failed to cleanup temp file ${tempFile}:`, error);
    }
  }

  private updateAverageExecutionTime(duration: number) {
    const total = this.metrics.averageExecutionTime * this.metrics.completedTasks;
    this.metrics.averageExecutionTime = (total + duration) / (this.metrics.completedTasks + 1);
  }

  getMetrics(): TaskMetrics {
    return { ...this.metrics };
  }
}

class EnhancedFaaSServer {
  private readonly workspace: string;
  private readonly defaultTimeout: number;
  private readonly sandboxPool: SandboxPool;
  private readonly taskScheduler: TaskScheduler;

  constructor() {
    this.workspace = Deno.env.get("FAAS_WORKSPACE") || "/tmp/faas-workspace";
    this.defaultTimeout = parseInt(Deno.env.get("FAAS_TIMEOUT") || "30000");
    
    const poolSize = parseInt(Deno.env.get("FAAS_POOL_SIZE") || "10");
    const maxInstances = parseInt(Deno.env.get("FAAS_MAX_INSTANCES") || "50");
    const workerCount = parseInt(Deno.env.get("FAAS_WORKER_COUNT") || "10");
    
    this.sandboxPool = new SandboxPool(maxInstances, poolSize);
    this.taskScheduler = new TaskScheduler(this.sandboxPool, workerCount);
  }

  /**
   * 处理代码执行请求
   */
  async handleRunCode(request: Request): Promise<Response> {
    try {
      const body: ExecutionRequest = await request.json();
      const { 
        language, 
        code, 
        input, 
        timeout = this.defaultTimeout,
        priority = "normal"
      } = body;

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

      console.log(`执行 ${language} 代码，优先级: ${priority}, 超时: ${timeout}ms`);

      // 通过任务调度器提交任务
      const taskResult = await this.taskScheduler.submitTask(code, language, priority, timeout);

      // 返回结果
      const response: ApiResponse = {
        output: {
          stdout: taskResult.result.stdout,
          stderr: taskResult.result.stderr,
          ret_val: taskResult.result.returnValue
        },
        metadata: {
          task_id: taskResult.taskId,
          instance_id: taskResult.instanceId,
          duration: taskResult.duration,
          pool_stats: this.sandboxPool.getStats()
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

  /**
   * 健康检查
   */
  handleHealth(): Response {
    const poolStats = this.sandboxPool.getStats();
    const schedulerMetrics = this.taskScheduler.getMetrics();
    
    const healthData = {
      status: "healthy",
      timestamp: new Date().toISOString(),
      pool: poolStats,
      scheduler: schedulerMetrics,
      version: "enhanced-v1.0.0"
    };
    
    return new Response(JSON.stringify(healthData), { 
      status: 200,
      headers: { "Content-Type": "application/json" }
    });
  }

  /**
   * 获取指标信息
   */
  handleMetrics(): Response {
    const poolStats = this.sandboxPool.getStats();
    const schedulerMetrics = this.taskScheduler.getMetrics();
    
    const metrics = {
      pool: poolStats,
      scheduler: schedulerMetrics,
      timestamp: new Date().toISOString()
    };
    
    return new Response(JSON.stringify(metrics), {
      status: 200,
      headers: { "Content-Type": "application/json" }
    });
  }
}

// 启动服务器
async function main() {
  const port = parseInt(Deno.env.get("FAAS_PORT") || "8000");
  const faasServer = new EnhancedFaaSServer();

  console.log(`启动增强版FaaS服务器，端口: ${port}...`);
  console.log(`工作空间: ${Deno.env.get("FAAS_WORKSPACE")}`);
  console.log(`默认超时: ${Deno.env.get("FAAS_TIMEOUT")}ms`);
  console.log(`沙箱池大小: ${Deno.env.get("FAAS_POOL_SIZE")}`);
  console.log(`最大实例数: ${Deno.env.get("FAAS_MAX_INSTANCES")}`);
  console.log(`工作协程数: ${Deno.env.get("FAAS_WORKER_COUNT")}`);

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

    // 指标接口
    if (url.pathname === "/metrics") {
      return faasServer.handleMetrics();
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

  console.log(`增强版FaaS服务器启动成功: http://0.0.0.0:${port}`);
  console.log("可用端点:");
  console.log("  GET  /health    - 健康检查");
  console.log("  GET  /metrics   - 指标信息");
  console.log("  POST /run_code  - 执行代码");
}

// 错误处理
if (import.meta.main) {
  try {
    await main();
  } catch (error) {
    console.error("启动增强版FaaS服务器失败:", error);
    Deno.exit(1);
  }
}