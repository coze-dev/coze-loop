package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"code.byted.org/flowdevops/cozeloop/backend/modules/evaluation/sandbox/application"
	sandboxHttp "code.byted.org/flowdevops/cozeloop/backend/modules/evaluation/sandbox/infra/http"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/sirupsen/logrus"
)

func main() {
	// 初始化日志
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.JSONFormatter{})

	logger.Info("启动代码评估器沙箱环境Demo")

	// 创建沙箱应用
	sandboxApp, err := application.NewSandboxApp(logger)
	if err != nil {
		logger.WithError(err).Fatal("初始化沙箱应用失败")
	}

	// 创建HTTP服务器
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	h := server.Default(server.WithHostPorts(":" + port))

	// 注册路由
	sandboxHttp.RegisterSandboxRoutes(h, sandboxApp, logger)

	// 添加基础路由
	h.GET("/", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(200, map[string]interface{}{
			"service": "Code Evaluator Sandbox",
			"version": "1.0.0",
			"status":  "running",
		})
	})

	// 启动服务器
	go func() {
		logger.Infof("HTTP服务器启动在端口 :%s", port)
		h.Spin()
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("正在关闭服务器...")

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := sandboxApp.Shutdown(); err != nil {
		logger.WithError(err).Error("沙箱应用关闭失败")
	}

	h.Shutdown(ctx)

	logger.Info("服务器已关闭")
}