package entity

import "time"

// EvalInput 优化后的评估输入结构
type EvalInput struct {
	Run     RunData   `json:"run"`
	History []RunData `json:"history,omitempty"`
}

// RunData 运行数据
type RunData struct {
	Input           Content `json:"input"`
	Output          Content `json:"output"`
	ReferenceOutput Content `json:"reference_output"`
}

// Content 内容结构
type Content struct {
	ContentType string     `json:"content_type"`
	Text        string     `json:"text,omitempty"`
	Image       *ImageInfo `json:"image,omitempty"`
	Audio       *AudioInfo `json:"audio,omitempty"`
	MultiPart   []Content  `json:"multi_part,omitempty"`
}

// ImageInfo 图片信息
type ImageInfo struct {
	URL    string `json:"url,omitempty"`
	Base64 string `json:"base64,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// AudioInfo 音频信息
type AudioInfo struct {
	URL      string        `json:"url,omitempty"`
	Base64   string        `json:"base64,omitempty"`
	Duration time.Duration `json:"duration,omitempty"`
	Format   string        `json:"format,omitempty"`
}

// EvalOutput 评估输出结构
type EvalOutput struct {
	Score  float64 `json:"score"`
	Reason string  `json:"reason"`
}
