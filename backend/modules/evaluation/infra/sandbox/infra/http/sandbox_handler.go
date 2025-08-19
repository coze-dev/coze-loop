package http

import (
	"context"
	"net/http"

	"code.byted.org/flowdevops/cozeloop/backend/modules/evaluation/sandbox/application"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/sirupsen/logrus"
)

// ValidateCodeRequest 验证代码请求
type ValidateCodeRequest struct {
	Code     string `json:"code" binding:"required"`
	Language string `json:"language" binding:"required"`
}

// ValidateCodeResponse 验证代码响应
type ValidateCodeResponse struct {
	Success bool `json:"success"`
	Valid   bool `json:"valid"`
}

// SandboxHandler 沙箱HTTP处理器
type SandboxHandler struct {
	sandboxApp *application.SandboxApp
	logger     *logrus.Logger
}

// NewSandboxHandler 创建沙箱HTTP处理器
func NewSandboxHandler(sandboxApp *application.SandboxApp, logger *logrus.Logger) *SandboxHandler {
	return &SandboxHandler{
		sandboxApp: sandboxApp,
		logger:     logger,
	}
}

// RunCode 执行代码接口
func (h *SandboxHandler) RunCode(ctx context.Context, c *app.RequestContext) {
	var req application.ExecuteCodeRequest
	if err := c.BindAndValidate(&req); err != nil {
		h.logger.WithError(err).Error("请求参数绑定失败")
		c.JSON(http.StatusBadRequest, utils.H{
			"success": false,
			"error":   "请求参数错误: " + err.Error(),
		})
		return
	}

	// 执行代码
	resp, err := h.sandboxApp.RunCode(ctx, &req)
	if err != nil {
		h.logger.WithError(err).Error("代码执行失败")
		c.JSON(http.StatusInternalServerError, utils.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ValidateCode 验证代码接口
func (h *SandboxHandler) ValidateCode(ctx context.Context, c *app.RequestContext) {
	var req ValidateCodeRequest
	if err := c.BindAndValidate(&req); err != nil {
		h.logger.WithError(err).Error("请求参数绑定失败")
		c.JSON(http.StatusBadRequest, utils.H{
			"success": false,
			"error":   "请求参数错误: " + err.Error(),
		})
		return
	}

	// 验证代码
	isValid := h.sandboxApp.ValidateCode(ctx, req.Code, req.Language)

	c.JSON(http.StatusOK, ValidateCodeResponse{
		Success: true,
		Valid:   isValid,
	})
}

// GetHealthStatus 获取健康状态

// GetHealthStatus 获取健康状态
func (h *SandboxHandler) GetHealthStatus(ctx context.Context, c *app.RequestContext) {
	status := h.sandboxApp.GetHealthStatus(ctx)

	var httpStatus int
	switch status.Status {
	case "healthy":
		httpStatus = http.StatusOK
	case "degraded":
		httpStatus = http.StatusPartialContent
	default:
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, status)
}