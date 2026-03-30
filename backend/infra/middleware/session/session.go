// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"os"
	"time"

	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

const (
	SessionKey     = "session_key"
	SessionExpires = 7 * 24 * time.Hour

	defaultHMACSecret = "openloop-session-hmac-key"
	envSessionHMACKey = "COZE_LOOP_SESSION_HMAC_KEY"
)

// Signing key: prefer `COZE_LOOP_SESSION_HMAC_KEY` from environment; fall back to the default.
var hmacSecret []byte

func init() {
	if v := os.Getenv(envSessionHMACKey); v != "" {
		hmacSecret = []byte(v)
	} else {
		hmacSecret = []byte(defaultHMACSecret)
	}
}

type Session struct {
	UserID string

	SessionID int64
	CreatedAt time.Time
	ExpiresAt time.Time
}

//go:generate mockgen -destination=mocks/session_service.go -package=mock_session . ISessionService
type ISessionService interface {
	ValidateSession(ctx context.Context, sessionID string) (*Session, error)
	GenerateSessionKey(ctx context.Context, session *Session) (string, error)
}

type sessionServiceImpl struct{}

func NewSessionService() ISessionService {
	return &sessionServiceImpl{}
}

func (s sessionServiceImpl) GenerateSessionKey(ctx context.Context, session *Session) (string, error) {
	// 设置会话的创建时间和过期时间
	session.CreatedAt = time.Now()
	session.ExpiresAt = time.Now().Add(SessionExpires)

	// 序列化会话数据
	sessionData, err := json.Marshal(session)
	if err != nil {
		return "", err
	}

	// 计算HMAC签名以确保完整性
	h := hmac.New(sha256.New, hmacSecret)
	h.Write(sessionData)
	signature := h.Sum(nil)

	// 组合会话数据和签名
	finalData := append(sessionData, signature...)

	// Base64编码最终结果
	return base64.RawURLEncoding.EncodeToString(finalData), nil
}

func (s sessionServiceImpl) ValidateSession(ctx context.Context, sessionID string) (*Session, error) {
	logs.CtxDebug(ctx, "sessionID: %s", sessionID)

	// 解码会话数据
	data, err := base64.RawURLEncoding.DecodeString(sessionID)
	if err != nil {
		return nil, errorx.New("invalid session format: %w, data:%s", err, sessionID)
	}

	// 确保数据长够长，至少包含会话数据和签名
	if len(data) < 32 { // 简单检查，实际应该更严格
		return nil, errorx.New("session data too short")
	}

	// 分离会话数据和签名
	sessionData := data[:len(data)-32] // 假设签名是32字节
	signature := data[len(data)-32:]

	// 验证签名
	h := hmac.New(sha256.New, hmacSecret)
	h.Write(sessionData)
	expectedSignature := h.Sum(nil)

	if !hmac.Equal(signature, expectedSignature) {
		return nil, errorx.New("invalid session signature")
	}

	// 解析会话数据
	var session Session
	if err := json.Unmarshal(sessionData, &session); err != nil {
		return nil, errorx.New("invalid session data: %w", err)
	}

	// 检查会话是否过期
	if time.Now().After(session.ExpiresAt) {
		return nil, errorx.New("session expired")
	}

	return &session, nil
}
