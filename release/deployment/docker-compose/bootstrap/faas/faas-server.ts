#!/usr/bin/env deno run --allow-all

/**
 * coze-loop-faas 统一FaaS服务器
 * 通过配置参数控制基础模式和增强模式
 * 支持 JavaScript/TypeScript 和 Python (基于 Pyodide)
 */

import { loadPyodide } from "npm:pyodide";

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
    task_id?: string;
    instance_id?: string;
    duration?: number;
    pool_stats?: any;
    mode?: string;
  };
}

interface UnifiedConfig {
  // 基础配置
  port: number;
  workspace: string;
  timeout: number;
  
  // 增强功能开关
  enableSandboxPool: boolean;
  enableTaskScheduler: boolean;
  enableMetrics: boolean;
  
  // 池配置
  poolSize: number;
  maxInstances: number;
  workerCount: number;
  
  // 运行模式
  mode: "basic" | "enhanced";
}

interface SandboxInstance {
  id: string;
  language: string;
  status: "idle" | "busy" | "error";
  createdAt: number;
  lastUsed: number;
  executeCount: number;
  pyodide?: any;
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
  private enabled: boolean;

  constructor(maxInstances = 50, minInstances = 5, enabled = true) {
    this.maxInstances = maxInstances;
    this.minInstances = minInstances;
    this.enabled = enabled;
    
    if (this.enabled) {
      this.warmUpPool();
    }
  }

  private async warmUpPool() {
    if (!this.enabled) return;
    
    console.log(`预热沙箱池，创建 ${this.minInstances} 个实例...`);
    
    const createPromises = [];
    for (let i = 0; i < this.minInstances; i++) {
      const language = i % 2 === 0 ? "javascript" : "python";
      createPromises.push(this.createInstance(language));
    }
    
    const instances = await Promise.all(createPromises);
    this.idleInstances.push(...instances);
    
    console.log(`沙箱池预热完成，创建了 ${this.idleInstances.length} 个实例`);
  }

  private async createInstance(language: string): Promise<SandboxInstance> {
    const id = `sandbox-${++this.instanceCounter}-${Date.now()}`;
    const instance: SandboxInstance = {
      id,
      language,
      status: "idle",
      createdAt: Date.now(),
      lastUsed: Date.now(),
      executeCount: 0,
    };
    
    if (language === "python") {
      try {
        console.log(`为实例 ${id} 初始化 Pyodide...`);
        instance.pyodide = await loadPyodide();
        await instance.pyodide.loadPackage("micropip");
        console.log(`实例 ${id} Pyodide 初始化完成`);
      } catch (error) {
        console.error(`实例 ${id} Pyodide 初始化失败:`, error);
        instance.status = "error";
      }
    }
    
    this.instances.set(id, instance);
    console.log(`创建沙箱实例: ${id} (${language})`);
    
    return instance;
  }

  async acquireInstance(language: string): Promise<SandboxInstance> {
    if (!this.enabled) {
      // 基础模式：创建临时实例
      return await this.createTemporaryInstance(language);
    }

    const idleIndex = this.idleInstances.findIndex(
      inst => inst.language === language && inst.status === "idle"
    );
    
    if (idleIndex >= 0) {
      const instance = this.idleInstances.splice(idleIndex, 1)[0];
      instance.status = "busy";
      instance.lastUsed = Date.now();
      return instance;
    }
    
    if (this.instances.size < this.maxInstances) {
      const instance = await this.createInstance(language);
      instance.status = "busy";
      return instance;
    }
    
    throw new Error(`沙箱池已达到最大实例数限制: ${this.maxInstances}`);
  }

  private async createTemporaryInstance(language: string): Promise<SandboxInstance> {
    const id = `temp-${Date.now()}`;
    const instance: SandboxInstance = {
      id,
      language,
      status: "busy",
      createdAt: Date.now(),
      lastUsed: Date.now(),
      executeCount: 0,
    };

    if (language === "python") {
      try {
        instance.pyodide = await loadPyodide();
        await instance.pyodide.loadPackage("micropip");
      } catch (error) {
        console.error(`临时实例 ${id} Pyodide 初始化失败:`, error);
        instance.status = "error";
      }
    }

    return instance;
  }

  releaseInstance(instance: SandboxInstance) {
    if (!this.enabled) {
      // 基础模式：直接销毁临时实例
      return;
    }

    instance.status = "idle";
    instance.lastUsed = Date.now();
    instance.executeCount++;
    
    if (instance.executeCount > 100) {
      this.destroyInstance(instance);
      return;
    }
    
    this.idleInstances.push(instance);
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
    const timeout = 5 * 60 * 1000;
    
    this.idleInstances = this.idleInstances.filter(instance => {
      if (now - instance.lastUsed > timeout) {
        this.destroyInstance(instance);
        return false;
      }
      return true;
    });
  }

  getStats() {
    if (!this.enabled) {
      return {
        mode: "basic",
        totalInstances: 0,
        idleInstances: 0,
        activeInstances: 0,
      };
    }

    return {
      mode: "enhanced",
      totalInstances: this.instances.size,
      idleInstances: this.idleInstances.length,
      activeInstances: this.instances.size - this.idleInstances.length,
    };
  }
}

class TaskScheduler {
  private metrics: TaskMetrics = {
    totalTasks: 0,
    completedTasks: 0,
    failedTasks: 0,
    queuedTasks: 0,
    averageExecutionTime: 0,
  };
  private enabled: boolean;

  constructor(private sandboxPool: SandboxPool, enabled = true) {
    this.enabled = enabled;
  }

  async submitTask(
    code: string,
    language: string,
    priority: "low" | "normal" | "high" | "urgent" = "normal",
    timeout = 30000
  ): Promise<{ result: ExecutionResult; taskId: string; instanceId: string; duration: number }> {
    const taskId = `task-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
    
    if (this.enabled) {
      this.metrics.totalTasks++;
      this.metrics.queuedTasks++;
    }
    
    try {
      const startTime = Date.now();
      
      const instance = await this.sandboxPool.acquireInstance(language);
      
      try {
        const result = await this.executeTask(code, language, timeout, instance);
        const duration = Date.now() - startTime;
        
        if (this.enabled) {
          this.metrics.completedTasks++;
          this.metrics.queuedTasks--;
          this.updateAverageExecutionTime(duration);
        }
        
        return {
          result,
          taskId,
          instanceId: instance.id,
          duration,
        };
      } finally {
        this.sandboxPool.releaseInstance(instance);
      }
    } catch (error) {
      if (this.enabled) {
        this.metrics.failedTasks++;
        this.metrics.queuedTasks--;
      }
      throw error;
    }
  }

  private async executeTask(
    code: string,
    language: string,
    timeout: number,
    instance: SandboxInstance
  ): Promise<ExecutionResult> {
    const tempFile = await this.createTempFile(language, code);
    
    try {
      return await this.executeCode(tempFile, timeout, code, instance);
    } finally {
      if (tempFile !== 'pyodide-execution') {
        await this.cleanup(tempFile);
      }
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
        return 'pyodide-execution';
    }
    
    const tempFile = `/tmp/faas-workspace/temp_${timestamp}_${randomId}${extension}`;
    await Deno.writeTextFile(tempFile, fileContent);
    return tempFile;
  }

  private async executeCode(tempFile: string, timeout: number, code?: string, instance?: SandboxInstance): Promise<ExecutionResult> {
    if (tempFile === 'pyodide-execution' && instance?.pyodide) {
      return await this.executePythonWithPyodide(code!, timeout, instance.pyodide);
    }
    
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

  private async executePythonWithPyodide(code: string, timeout: number, pyodide: any): Promise<ExecutionResult> {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), timeout);
    
    try {
      pyodide.runPython(`
import sys
import json
from io import StringIO

old_stdout = sys.stdout
sys.stdout = stdout_capture = StringIO()

old_stderr = sys.stderr  
sys.stderr = stderr_capture = StringIO()

ret_val = None
error_msg = ""
      `);

      try {
        pyodide.runPython(code);
        
        pyodide.runPython(`
if 'result' in locals():
    ret_val = result
        `);
      } catch (execError) {
        pyodide.runPython(`
error_msg = "${String(execError).replace(/"/g, '\\"')}"
        `);
      }

      const result = pyodide.runPython(`
sys.stdout = old_stdout
sys.stderr = old_stderr

{
    "stdout": stdout_capture.getvalue(),
    "stderr": stderr_capture.getvalue() + error_msg,
    "ret_val": ret_val
}
      `);
      
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

  private async cleanup(tempFile: string): Promise<void> {
    try {
      await Deno.remove(tempFile);
    } catch (error) {
      console.warn(`Failed to cleanup temp file ${tempFile}:`, error);
    }
  }

  private updateAverageExecutionTime(duration: number) {
    if (!this.enabled) return;
    
    const total = this.metrics.averageExecutionTime * this.metrics.completedTasks;
    this.metrics.averageExecutionTime = (total + duration) / (this.metrics.completedTasks + 1);
  }

  getMetrics(): TaskMetrics {
    return { ...this.metrics };
  }
}

class UnifiedFaaSServer {
  private readonly config: UnifiedConfig;
  private readonly sandboxPool: SandboxPool;
  private readonly taskScheduler: TaskScheduler;

  constructor() {
    // 解析配置
    this.config = this.parseConfig();
    
    // 初始化组件
    this.sandboxPool = new SandboxPool(
      this.config.maxInstances,
      this.config.poolSize,
      this.config.enableSandboxPool
    );
    
    this.taskScheduler = new TaskScheduler(
      this.sandboxPool,
      this.config.enableTaskScheduler
    );
  }

  private parseConfig(): UnifiedConfig {
    const mode = Deno.env.get("FAAS_MODE") || "enhanced";
    const enableEnhanced = mode === "enhanced";

    return {
      port: parseInt(Deno.env.get("FAAS_PORT") || "8000"),
      workspace: Deno.env.get("FAAS_WORKSPACE") || "/tmp/faas-workspace",
      timeout: parseInt(Deno.env.get("FAAS_TIMEOUT") || "30000"),
      
      // 增强功能开关
      enableSandboxPool: enableEnhanced && (Deno.env.get("FAAS_ENABLE_POOL") !== "false"),
      enableTaskScheduler: enableEnhanced && (Deno.env.get("FAAS_ENABLE_SCHEDULER") !== "false"),
      enableMetrics: enableEnhanced && (Deno.env.get("FAAS_ENABLE_METRICS") !== "false"),
      
      // 池配置
      poolSize: parseInt(Deno.env.get("FAAS_POOL_SIZE") || "10"),
      maxInstances: parseInt(Deno.env.get("FAAS_MAX_INSTANCES") || "50"),
      workerCount: parseInt(Deno.env.get("FAAS_WORKER_COUNT") || "10"),
      
      mode: mode as "basic" | "enhanced",
    };
  }

  async handleRunCode(request: Request): Promise<Response> {
    try {
      const body: ExecutionRequest = await request.json();
      const { 
        language, 
        code, 
        input, 
        timeout = this.config.timeout,
        priority = "normal"
      } = body;

      if (!language || !code) {
        return new Response(
          JSON.stringify({ error: "Missing required parameters: language, code" }),
          { status: 400, headers: { "Content-Type": "application/json" } }
        );
      }

      if (!["javascript", "typescript", "python"].includes(language.toLowerCase())) {
        return new Response(
          JSON.stringify({ error: "Unsupported language. Supported: javascript, typescript, python" }),
          { status: 400, headers: { "Content-Type": "application/json" } }
        );
      }

      console.log(`执行 ${language} 代码，模式: ${this.config.mode}, 超时: ${timeout}ms`);

      const taskResult = await this.taskScheduler.submitTask(code, language, priority, timeout);

      const response: ApiResponse = {
        output: {
          stdout: taskResult.result.stdout,
          stderr: taskResult.result.stderr,
          ret_val: taskResult.result.returnValue
        }
      };

      // 增强模式返回元数据
      if (this.config.mode === "enhanced") {
        response.metadata = {
          task_id: taskResult.taskId,
          instance_id: taskResult.instanceId,
          duration: taskResult.duration,
          pool_stats: this.sandboxPool.getStats(),
          mode: this.config.mode
        };
      }

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
    const poolStats = this.sandboxPool.getStats();
    
    const healthData: any = {
      status: "healthy",
      timestamp: new Date().toISOString(),
      mode: this.config.mode,
      version: "v1.0.0"
    };

    if (this.config.mode === "enhanced") {
      healthData.pool = poolStats;
      if (this.config.enableMetrics) {
        healthData.scheduler = this.taskScheduler.getMetrics();
      }
    }
    
    return new Response(JSON.stringify(healthData), { 
      status: 200,
      headers: { "Content-Type": "application/json" }
    });
  }

  handleMetrics(): Response {
    if (!this.config.enableMetrics) {
      return new Response(
        JSON.stringify({ error: "Metrics disabled in current mode" }),
        { status: 404, headers: { "Content-Type": "application/json" } }
      );
    }

    const poolStats = this.sandboxPool.getStats();
    const schedulerMetrics = this.taskScheduler.getMetrics();
    
    const metrics = {
      pool: poolStats,
      scheduler: schedulerMetrics,
      config: {
        mode: this.config.mode,
        enableSandboxPool: this.config.enableSandboxPool,
        enableTaskScheduler: this.config.enableTaskScheduler,
        poolSize: this.config.poolSize,
        maxInstances: this.config.maxInstances
      },
      timestamp: new Date().toISOString()
    };
    
    return new Response(JSON.stringify(metrics), {
      status: 200,
      headers: { "Content-Type": "application/json" }
    });
  }
}

async function main() {
  const faasServer = new UnifiedFaaSServer();
  const config = faasServer['config']; // 访问私有属性

  console.log(`启动FaaS服务器，模式: ${config.mode}，端口: ${config.port}...`);
  console.log(`工作空间: ${config.workspace}`);
  console.log(`默认超时: ${config.timeout}ms`);
  
  if (config.mode === "enhanced") {
    console.log(`沙箱池: ${config.enableSandboxPool ? '启用' : '禁用'} (大小: ${config.poolSize}, 最大: ${config.maxInstances})`);
    console.log(`任务调度: ${config.enableTaskScheduler ? '启用' : '禁用'}`);
    console.log(`指标监控: ${config.enableMetrics ? '启用' : '禁用'}`);
  } else {
    console.log("基础模式: 简单代码执行，无池管理");
  }
  
  console.log("Python执行: 基于Pyodide沙箱");

  const server = Deno.serve({
    port: config.port,
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

  console.log(`FaaS服务器启动成功: http://0.0.0.0:${config.port}`);
  console.log("可用端点:");
  console.log("  GET  /health    - 健康检查");
  if (config.enableMetrics) {
    console.log("  GET  /metrics   - 指标信息");
  }
  console.log("  POST /run_code  - 执行代码");
}

if (import.meta.main) {
  try {
    await main();
  } catch (error) {
    console.error("启动FaaS服务器失败:", error);
    Deno.exit(1);
  }
}