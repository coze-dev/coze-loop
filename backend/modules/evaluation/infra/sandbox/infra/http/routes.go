package http

import (
	"code.byted.org/flowdevops/cozeloop/backend/modules/evaluation/sandbox/application"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/sirupsen/logrus"
)

// RegisterSandboxRoutes 注册沙箱路由
func RegisterSandboxRoutes(h *server.Hertz, sandboxApp *application.SandboxApp, logger *logrus.Logger) {
	handler := NewSandboxHandler(sandboxApp, logger)

	// 沙箱API路由组
	sandboxGroup := h.Group("/api/v1/sandbox")
	{
		// 执行代码
		sandboxGroup.POST("/run", handler.RunCode)

		// 验证代码
		sandboxGroup.POST("/validate", handler.ValidateCode)

		// 健康检查
		sandboxGroup.GET("/health", handler.GetHealthStatus)
	}
}