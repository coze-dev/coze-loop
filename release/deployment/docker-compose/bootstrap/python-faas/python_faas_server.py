#!/usr/bin/env python3
"""
专用Python FaaS服务器
专注于Python代码执行，提供统一的/run_code接口
"""

import json
import os
import time
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse
import threading
from contextlib import redirect_stdout, redirect_stderr
import io
import sys

class PythonExecutor:
    """Python代码执行器"""
    
    def __init__(self):
        self.execution_count = 0
        self.return_val_output = None
    
    def execute_python(self, code, timeout=30000):
        """执行Python代码"""
        self.execution_count += 1
        
        try:
            # 1. 预执行语法检查
            syntax_valid, syntax_error = self._check_syntax(code)
            if not syntax_valid:
                return {
                    "stdout": "",
                    "stderr": f"python syntax error: {syntax_error}",
                    "returnValue": ""
                }
            
            stdout_capture = io.StringIO()
            stderr_capture = io.StringIO()
            self.return_val_output = None
            
            # 2. 创建一个新的命名空间执行代码
            namespace = {
                '__builtins__': __builtins__,
                'return_val': self._capture_return_val,
                '_return_val_output': None  # 初始化全局变量
            }
            
            with redirect_stdout(stdout_capture), redirect_stderr(stderr_capture):
                exec(code, namespace, namespace)
            
            # 检查是否通过全局变量设置了返回值
            return_value = namespace.get('_return_val_output')
            if return_value is None:
                return_value = self.return_val_output
            
            return {
                "stdout": stdout_capture.getvalue(),
                "stderr": stderr_capture.getvalue(),
                "returnValue": return_value if return_value is not None else ""
            }
        except Exception as e:
            return {
                "stdout": stdout_capture.getvalue() if 'stdout_capture' in locals() else "",
                "stderr": (stderr_capture.getvalue() if 'stderr_capture' in locals() else "") + str(e),
                "returnValue": ""
            }
    
    def _check_syntax(self, code):
        """检查Python代码语法"""
        import ast
        try:
            ast.parse(code)
            return True, None
        except SyntaxError as e:
            error_msg = f"{e.msg} ({e.filename if e.filename else '<string>'}, line {e.lineno})"
            return False, error_msg
        except Exception as e:
            return False, str(e)
    
    def _capture_return_val(self, value):
        """捕获return_val函数的输出"""
        self.return_val_output = value

class PythonFaaSHandler(BaseHTTPRequestHandler):
    def __init__(self, *args, executor=None, **kwargs):
        self.executor = executor
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
        health_data = {
            "status": "healthy",
            "timestamp": time.strftime("%Y-%m-%dT%H:%M:%S.%fZ"),
            "language": "python",
            "version": "python-faas-v1.0.0",
            "execution_count": self.executor.execution_count
        }
        
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(json.dumps(health_data).encode())
    
    def handle_metrics(self):
        metrics = {
            "language": "python",
            "execution_count": self.executor.execution_count,
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
            
            language = request_data.get('language', '').lower()
            code = request_data.get('code')
            timeout = request_data.get('timeout', 30000)
            
            # 参数验证
            if not code:
                self.send_response(400)
                self.send_header('Content-Type', 'application/json')
                self.end_headers()
                self.wfile.write(json.dumps({
                    "error": "Missing required parameter: code"
                }).encode())
                return
            
            # 语言检查 - 只支持Python
            if language and language not in ["python", "py"]:
                self.send_response(400)
                self.send_header('Content-Type', 'application/json')
                self.end_headers()
                self.wfile.write(json.dumps({
                    "error": "This service only supports Python code execution"
                }).encode())
                return
            
            print(f"执行Python代码，超时: {timeout}ms")
            print(f"代码预览: {code[:200]}...")  # 添加代码预览日志
            
            # 执行Python代码
            start_time = time.time()
            result = self.executor.execute_python(code, timeout)
            duration = int((time.time() - start_time) * 1000)
            
            # 添加详细的执行日志
            print(f"执行完成 - stdout长度: {len(result['stdout'])}, stderr长度: {len(result['stderr'])}, ret_val长度: {len(result['returnValue'])}")
            
            # 返回结果
            response = {
                "output": {
                    "stdout": result["stdout"],
                    "stderr": result["stderr"],
                    "ret_val": result["returnValue"]
                },
                "metadata": {
                    "language": "python",
                    "duration": duration,
                    "status": "success"
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

def create_handler(executor):
    def handler(*args, **kwargs):
        PythonFaaSHandler(*args, executor=executor, **kwargs)
    return handler

def main():
    port = int(os.environ.get("FAAS_PORT", "8000"))
    
    print(f"启动Python FaaS服务器，端口: {port}...")
    print(f"工作空间: {os.environ.get('FAAS_WORKSPACE', '/tmp/faas-workspace')}")
    print(f"默认超时: {os.environ.get('FAAS_TIMEOUT', '30000')}ms")
    print("专用语言: Python")
    
    # 创建Python执行器
    executor = PythonExecutor()
    
    # 创建HTTP服务器
    handler = create_handler(executor)
    httpd = HTTPServer(('0.0.0.0', port), handler)
    
    print(f"Python FaaS服务器启动成功: http://0.0.0.0:{port}")
    print("可用端点:")
    print("  GET  /health    - 健康检查")
    print("  GET  /metrics   - 指标信息")
    print("  POST /run_code  - 执行Python代码")
    
    try:
        httpd.serve_forever()
    except KeyboardInterrupt:
        print("\n正在关闭服务器...")
        httpd.shutdown()

if __name__ == "__main__":
    main()