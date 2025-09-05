// Deno JavaScript/TypeScript Runner
// 用于在Deno环境中执行JavaScript/TypeScript代码

// 配置接口
interface SandboxConfig {
  allow_env?: boolean | string[];
  allow_read?: boolean | string[];
  allow_write?: boolean | string[];
  allow_net?: boolean | string[];
  allow_run?: boolean | string[];
  allow_ffi?: boolean;
  memory_limit_mb?: number;
  timeout_seconds?: number;
}

// 执行请求接口
interface ExecutionRequest {
  config?: SandboxConfig;
  code: string;
  language: string;
  params?: Record<string, any>;
}

// 执行结果接口
interface ExecutionResult {
  success: boolean;
  result?: any;
  stdout?: string;
  stderr?: string;
  execution_time: number;
  sandbox_error?: string;
  status: string;
}

// 执行JavaScript/TypeScript代码
async function executeJavaScriptCode(request: ExecutionRequest): Promise<ExecutionResult> {
  const startTime = Date.now();
  let stdout = "";
  let stderr = "";
  
  try {
    // 设置超时
    const timeout = (request.config?.timeout_seconds || 30) * 1000;
    const timeoutPromise = new Promise((_, reject) => {
      setTimeout(() => reject(new Error("执行超时")), timeout);
    });
    
    // 捕获console输出
    const originalConsoleLog = console.log;
    const originalConsoleError = console.error;
    
    console.log = (...args: any[]) => {
      stdout += args.map(arg => String(arg)).join(' ') + '\n';
    };
    
    console.error = (...args: any[]) => {
      stderr += args.map(arg => String(arg)).join(' ') + '\n';
    };
    
    try {
      // 准备执行环境
      const executionPromise = (async () => {
        // 设置参数
        const params = request.params || {};
        
        // 包装代码以支持评估
        const wrappedCode = `
// 设置参数
${Object.entries(params).map(([key, value]) => 
  `const ${key} = ${JSON.stringify(value)};`
).join('\n')}

// 用户代码
try {
  ${request.code}
  
  // 如果没有显式输出，尝试获取最后的表达式结果
  if (typeof result === 'undefined' && typeof score === 'undefined') {
    console.log(JSON.stringify({score: 1.0, reason: "代码执行成功"}));
  } else if (typeof score !== 'undefined') {
    console.log(JSON.stringify({score: score, reason: typeof reason !== 'undefined' ? reason : ""}));
  } else if (typeof result !== 'undefined') {
    console.log(JSON.stringify({score: 1.0, reason: String(result)}));
  }
} catch (error) {
  console.error("执行错误:", error.message);
  console.log(JSON.stringify({score: 0.0, reason: "执行错误: " + error.message}));
}
`;
        
        // 执行代码
        const result = eval(wrappedCode);
        return result;
      })();
      
      // 等待执行完成或超时
      await Promise.race([executionPromise, timeoutPromise]);
      
    } finally {
      // 恢复console
      console.log = originalConsoleLog;
      console.error = originalConsoleError;
    }
    
    const executionTime = (Date.now() - startTime) / 1000;
    
    // 解析输出中的结果
    let parsedResult: any = {};
    const lines = stdout.split('\n');
    
    // 查找最后一行的JSON输出
    for (let i = lines.length - 1; i >= 0; i--) {
      const line = lines[i].trim();
      if (line.startsWith('{') && line.endsWith('}')) {
        try {
          parsedResult = JSON.parse(line);
          break;
        } catch (e) {
          // 忽略解析错误
        }
      }
    }
    
    // 如果没有找到JSON输出，使用默认结果
    if (!parsedResult.score) {
      parsedResult = {
        score: 1.0,
        reason: "代码执行完成"
      };
    }
    
    return {
      success: true,
      result: parsedResult,
      stdout: stdout,
      stderr: stderr,
      execution_time: executionTime,
      status: "success"
    };
    
  } catch (error: any) {
    const executionTime = (Date.now() - startTime) / 1000;
    
    return {
      success: false,
      stdout: stdout,
      stderr: stderr,
      execution_time: executionTime,
      sandbox_error: error.message || "未知错误",
      status: "error"
    };
  }
}

// 主函数
async function main() {
  try {
    // 从stdin读取请求
    const decoder = new TextDecoder();
    const input = new Uint8Array(1024 * 1024); // 1MB buffer
    let totalBytes = 0;
    
    // 读取所有输入数据
    while (true) {
      const n = await Deno.stdin.read(input.subarray(totalBytes));
      if (n === null) break;
      totalBytes += n;
    }
    
    const requestText = decoder.decode(input.subarray(0, totalBytes));
    const request: ExecutionRequest = JSON.parse(requestText);
    
    // 执行代码
    const result = await executeJavaScriptCode(request);
    
    // 输出结果
    console.log(JSON.stringify(result));
    
  } catch (error: any) {
    // 输出错误结果
    const errorResult: ExecutionResult = {
      success: false,
      execution_time: 0,
      sandbox_error: `Runner错误: ${error.message}`,
      status: "error"
    };
    
    console.log(JSON.stringify(errorResult));
  }
}

// 运行主函数
if (import.meta.main) {
  main().catch((error) => {
    console.error("Fatal error:", error);
    Deno.exit(1);
  });
}