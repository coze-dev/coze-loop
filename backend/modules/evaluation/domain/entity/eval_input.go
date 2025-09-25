package entity

import "time"

// EvalInput 优化后的评估输入结构
type EvalInput struct {
	Run     RunData   `json:"run"`
	History []RunData `json:"history,omitempty"`
}

// RunData 运行数据
type RunData struct {
	Input           EvalContent `json:"input"`
	Output          EvalContent `json:"output"`
	ReferenceOutput EvalContent `json:"reference_output"`
}

// EvalContent 评估内容结构
type EvalContent struct {
	ContentType string          `json:"content_type"`
	Text        string          `json:"text,omitempty"`
	Image       *EvalImageInfo  `json:"image,omitempty"`
	Audio       *EvalAudioInfo  `json:"audio,omitempty"`
	MultiPart   []EvalContent   `json:"multi_part,omitempty"`
}

// EvalImageInfo 图片信息
type EvalImageInfo struct {
	URL    string `json:"url,omitempty"`
	Base64 string `json:"base64,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// EvalAudioInfo 音频信息
type EvalAudioInfo struct {
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