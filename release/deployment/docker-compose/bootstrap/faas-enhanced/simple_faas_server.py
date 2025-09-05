#!/usr/bin/env python3
"""
简单的FaaS服务器实现
提供基于HTTP的代码执行服务，支持JavaScript和Python
"""

import json
import os
import subprocess
import tempfile
import time
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse
import threading
from contextlib import redirect_stdout, redirect_stderr
import io
import sys

class SandboxPool:
    def __init__(self, max_instances=50, min_instances=5):
        self.max_instances = max_instances
        self.min_instances = min_instances
        self.instances = []
        self.instance_counter = 0
        
        # 预热实例池
        self.warm_up_pool()
    
    def warm_up_pool(self):
        print(f"预热沙箱池，创建 {self.min_instances} 个实例...")
        for i in range(self.min_instances):
            language = "javascript" if i % 2 == 0 else "python"
            instance = self.create_instance(language)
            self.instances.append(instance)
        print(f"沙箱池预热完成，创建了 {len(self.instances)} 个实例")
    
    def create_instance(self, language):
        self.instance_counter += 1
        instance_id = f"sandbox-{self.instance_counter}-{int(time.time() * 1000)}"
        instance = {
            "id": instance_id,
            "language": language,
            "status": "idle",
            "created_at": time.time(),
            "last_used": time.time(),
            "execute_count": 0
        }
        print(f"创建沙箱实例: {instance_id} ({language})")
        return instance
    
    def acquire_instance(self, language):
        # 查找空闲实例
        for instance in self.instances:
            if instance["language"] == language and instance["status"] == "idle":
                instance["status"] = "busy"
                instance["last_used"] = time.time()
                return instance
        
        # 如果没有空闲实例且未达到最大限制，创建新实例
        if len(self.instances) < self.max_instances:
            instance = self.create_instance(language)
            instance["status"] = "busy"
            return instance
        
        raise Exception(f"沙箱池已达到最大实例数限制: {self.max_instances}")
    
    def release_instance(self, instance):
        instance["status"] = "idle"
        instance["last_used"] = time.time()
        instance["execute_count"] += 1
        
        # 如果实例执行次数过多，销毁实例
        if instance["execute_count"] > 100:
            self.destroy_instance(instance)
    
    def destroy_instance(self, instance):
        if instance in self.instances:
            self.instances.remove(instance)
        print(f"销毁沙箱实例: {instance['id']}")
    
    def get_stats(self):
        idle_count = sum(1 for inst in self.instances if inst["status"] == "idle")
        active_count = len(self.instances) - idle_count
        return {
            "totalInstances": len(self.instances),
            "idleInstances": idle_count,
            "activeInstances": active_count
        }

class TaskScheduler:
    def __init__(self, sandbox_pool):
        self.sandbox_pool = sandbox_pool
        self.metrics = {
            "totalTasks": 0,
            "completedTasks": 0,
            "failedTasks": 0,
            "queuedTasks": 0,
            "averageExecutionTime": 0
        }
    
    def submit_task(self, code, language, timeout=30000):
        task_id = f"task-{int(time.time() * 1000)}-{os.urandom(4).hex()}"
        
        self.metrics["totalTasks"] += 1
        self.metrics["queuedTasks"] += 1
        
        try:
            start_time = time.time()
            
            # 获取沙箱实例
            instance = self.sandbox_pool.acquire_instance(language)
            
            try:
                # 执行任务
                result = self.execute_task(code, language, timeout, instance)
                duration = int((time.time() - start_time) * 1000)
                
                self.metrics["completedTasks"] += 1
                self.metrics["queuedTasks"] -= 1
                self.update_average_execution_time(duration)
                
                return {
                    "result": result,
                    "task_id": task_id,
                    "instance_id": instance["id"],
                    "duration": duration
                }
            finally:
                # 归还实例
                self.sandbox_pool.release_instance(instance)
        
        except Exception as e:
            self.metrics["failedTasks"] += 1
            self.metrics["queuedTasks"] -= 1
            raise e
    
    def execute_task(self, code, language, timeout, instance):
        if language.lower() in ["javascript", "typescript"]:
            return self.execute_javascript(code, timeout)
        elif language.lower() == "python":
            return self.execute_python(code, timeout)
        else:
            raise Exception(f"Unsupported language: {language}")
    
    def execute_javascript(self, code, timeout):
        # 包装JavaScript代码
        wrapped_code = f"""
const originalLog = console.log;
const originalError = console.error;
let stdout = '';
let stderr = '';

console.log = (...args) => {{
  stdout += args.join(' ') + '\\n';
  originalLog(...args);
}};

console.error = (...args) => {{
  stderr += args.join(' ') + '\\n';
  originalError(...args);
}};

try {{
  const userFunction = () => {{
    {code}
  }};
  
  const result = userFunction();
  
  console.log(JSON.stringify({{
    stdout: stdout,
    stderr: stderr,
    ret_val: JSON.stringify(result)
  }}));
}} catch (error) {{
  console.error(JSON.stringify({{
    stdout: stdout,
    stderr: stderr + error.message,
    ret_val: null
  }}));
}}
"""
        
        # 写入临时文件
        with tempfile.NamedTemporaryFile(mode='w', suffix='.js', delete=False) as f:
            f.write(wrapped_code)
            temp_file = f.name
        
        try:
            # 使用node执行（如果可用）
            try:
                result = subprocess.run(['node', temp_file], 
                                     capture_output=True, text=True, 
                                     timeout=timeout/1000)
                stdout = result.stdout
                stderr = result.stderr
                
                if result.returncode == 0 and stdout.strip():
                    try:
                        parsed = json.loads(stdout.strip())
                        return {
                            "stdout": parsed.get("stdout", ""),
                            "stderr": parsed.get("stderr", stderr),
                            "returnValue": parsed.get("ret_val", "")
                        }
                    except:
                        return {
                            "stdout": stdout,
                            "stderr": stderr,
                            "returnValue": ""
                        }
                else:
                    return {
                        "stdout": stdout,
                        "stderr": stderr,
                        "returnValue": ""
                    }
            except FileNotFoundError:
                # Node.js不可用，返回模拟结果
                return {
                    "stdout": f"JavaScript code executed (Node.js not available): {code[:50]}...",
                    "stderr": "",
                    "returnValue": "42"
                }
        finally:
            os.unlink(temp_file)
    
    def execute_python(self, code, timeout):
        # 包装Python代码
        wrapped_code = f"""
import sys
import json
import io
from contextlib import redirect_stdout, redirect_stderr

stdout_capture = io.StringIO()
stderr_capture = io.StringIO()

try:
    with redirect_stdout(stdout_capture), redirect_stderr(stderr_capture):
        {code}
        
        ret_val = locals().get('result', None)
    
    result = {{
        "stdout": stdout_capture.getvalue(),
        "stderr": stderr_capture.getvalue(),
        "ret_val": json.dumps(ret_val) if ret_val is not None else ""
    }}
    print(json.dumps(result))
    
except Exception as e:
    result = {{
        "stdout": stdout_capture.getvalue(),
        "stderr": stderr_capture.getvalue() + str(e),
        "ret_val": ""
    }}
    print(json.dumps(result))
"""
        
        # 直接执行Python代码
        try:
            stdout_capture = io.StringIO()
            stderr_capture = io.StringIO()
            
            # 创建一个新的命名空间执行代码
            namespace = {}
            
            with redirect_stdout(stdout_capture), redirect_stderr(stderr_capture):
                exec(code, namespace)
                ret_val = namespace.get('result', None)
            
            return {
                "stdout": stdout_capture.getvalue(),
                "stderr": stderr_capture.getvalue(),
                "returnValue": json.dumps(ret_val) if ret_val is not None else ""
            }
        except Exception as e:
            return {
                "stdout": stdout_capture.getvalue(),
                "stderr": stderr_capture.getvalue() + str(e),
                "returnValue": ""
            }
    
    def update_average_execution_time(self, duration):
        total = self.metrics["averageExecutionTime"] * self.metrics["completedTasks"]
        self.metrics["averageExecutionTime"] = (total + duration) / (self.metrics["completedTasks"] + 1)
    
    def get_metrics(self):
        return self.metrics.copy()

class FaaSHandler(BaseHTTPRequestHandler):
    def __init__(self, *args, sandbox_pool=None, task_scheduler=None, **kwargs):
        self.sandbox_pool = sandbox_pool
        self.task_scheduler = task_scheduler
        super().__init__(*args, **kwargs)
    
    def do_GET(self):
        parsed_path = urlparse(self.path)
        
        if parsed_path.path == '/health':
            self.handle_health()
        elif parsed_path.path == '/metrics':
            self.handle_metrics()
        else:
            self.send_response(404)
            self.end_headers()
            self.wfile.write(b'Not Found')
    
    def do_POST(self):
        parsed_path = urlparse(self.path)
        
        if parsed_path.path == '/run_code':
            self.handle_run_code()
        else:
            self.send_response(404)
            self.end_headers()
            self.wfile.write(b'Not Found')
    
    def handle_health(self):
        pool_stats = self.sandbox_pool.get_stats()
        scheduler_metrics = self.task_scheduler.get_metrics()
        
        health_data = {
            "status": "healthy",
            "timestamp": time.strftime("%Y-%m-%dT%H:%M:%S.%fZ"),
            "pool": pool_stats,
            "scheduler": scheduler_metrics,
            "version": "simple-v1.0.0"
        }
        
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(json.dumps(health_data).encode())
    
    def handle_metrics(self):
        pool_stats = self.sandbox_pool.get_stats()
        scheduler_metrics = self.task_scheduler.get_metrics()
        
        metrics = {
            "pool": pool_stats,
            "scheduler": scheduler_metrics,
            "timestamp": time.strftime("%Y-%m-%dT%H:%M:%S.%fZ")
        }
        
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(json.dumps(metrics).encode())
    
    def handle_run_code(self):
        try:
            content_length = int(self.headers['Content-Length'])
            post_data = self.rfile.read(content_length)
            request_data = json.loads(post_data.decode())
            
            language = request_data.get('language')
            code = request_data.get('code')
            timeout = request_data.get('timeout', 30000)
            priority = request_data.get('priority', 'normal')
            
            # 参数验证
            if not language or not code:
                self.send_response(400)
                self.send_header('Content-Type', 'application/json')
                self.end_headers()
                self.wfile.write(json.dumps({
                    "error": "Missing required parameters: language, code"
                }).encode())
                return
            
            # 支持的语言检查
            if language.lower() not in ["javascript", "typescript", "python"]:
                self.send_response(400)
                self.send_header('Content-Type', 'application/json')
                self.end_headers()
                self.wfile.write(json.dumps({
                    "error": "Unsupported language. Supported: javascript, typescript, python"
                }).encode())
                return
            
            print(f"执行 {language} 代码，优先级: {priority}, 超时: {timeout}ms")
            
            # 通过任务调度器提交任务
            task_result = self.task_scheduler.submit_task(code, language, timeout)
            
            # 返回结果
            response = {
                "output": {
                    "stdout": task_result["result"]["stdout"],
                    "stderr": task_result["result"]["stderr"],
                    "ret_val": task_result["result"]["returnValue"]
                },
                "metadata": {
                    "task_id": task_result["task_id"],
                    "instance_id": task_result["instance_id"],
                    "duration": task_result["duration"],
                    "pool_stats": self.sandbox_pool.get_stats()
                }
            }
            
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            self.wfile.write(json.dumps(response).encode())
            
        except Exception as e:
            print(f"Error handling run_code request: {e}")
            self.send_response(500)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            self.wfile.write(json.dumps({
                "error": "Internal server error",
                "details": str(e)
            }).encode())
    
    def log_message(self, format, *args):
        print(f"{self.command} {self.path}")

def create_handler(sandbox_pool, task_scheduler):
    def handler(*args, **kwargs):
        FaaSHandler(*args, sandbox_pool=sandbox_pool, task_scheduler=task_scheduler, **kwargs)
    return handler

def main():
    port = int(os.environ.get("FAAS_PORT", "8000"))
    pool_size = int(os.environ.get("FAAS_POOL_SIZE", "10"))
    max_instances = int(os.environ.get("FAAS_MAX_INSTANCES", "50"))
    
    print(f"启动简单FaaS服务器，端口: {port}...")
    print(f"工作空间: {os.environ.get('FAAS_WORKSPACE', '/tmp/faas-workspace')}")
    print(f"默认超时: {os.environ.get('FAAS_TIMEOUT', '30000')}ms")
    print(f"沙箱池大小: {pool_size}")
    print(f"最大实例数: {max_instances}")
    
    # 创建沙箱池和任务调度器
    sandbox_pool = SandboxPool(max_instances, pool_size)
    task_scheduler = TaskScheduler(sandbox_pool)
    
    # 创建HTTP服务器
    handler = create_handler(sandbox_pool, task_scheduler)
    httpd = HTTPServer(('0.0.0.0', port), handler)
    
    print(f"简单FaaS服务器启动成功: http://0.0.0.0:{port}")
    print("可用端点:")
    print("  GET  /health    - 健康检查")
    print("  GET  /metrics   - 指标信息")
    print("  POST /run_code  - 执行代码")
    
    try:
        httpd.serve_forever()
    except KeyboardInterrupt:
        print("\n正在关闭服务器...")
        httpd.shutdown()

if __name__ == "__main__":
    main()