// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type CustomRPCServer struct {
	// 应用ID
	ID int64
	// DTO使用，不存数据库
	Name string `json:"-"`
	// DTO使用，不存数据库
	Description string `json:"-"`
	// 注意以下信息会存储到DB，也就是说实验创建时以下内容就确定了，运行时直接从评测DB中获取，而不是实时从app模块拉
	ServerName string
	// 接入协议
	AccessProtocol AccessProtocol
	Regions        []Region
	Cluster        string
	// 执行http信息
	InvokeHTTPInfo *HTTPInfo
	// 异步执行http信息，如果用户选了异步就传入这个字段
	AsyncInvokeHTTPInfo *HTTPInfo
	// 是否需要搜索对象
	NeedSearchTarget *bool
	// 搜索对象http信息
	SearchHTTPInfo *HTTPInfo
	// 搜索对象返回的信息
	CustomEvalTarget *CustomEvalTarget
	// 是否异步
	IsAsync *bool
}

type HTTPInfo struct {
	Method HTTPMethod
	Path   string
	// ms，默认5000，最大800,000
	Timeout *int64
}

type CustomEvalTarget struct {
	// 唯一键，平台不消费，仅做透传
	ID *string
	// 名称，平台用于展示在对象搜索下拉列表
	Name *string
	// 头像url，平台用于展示在对象搜索下拉列表
	AvatarURL *string
	// 扩展字段，目前主要存储旧版协议response中的额外字段：object_type(旧版ID)、object_meta、space_id
	Ext map[string]string
}

type Region = string

const (
	RegionBOE  = "boe"
	RegionCN   = "cn"
	RegionI18N = "i18n"
)

type AccessProtocol = string

const (
	AccessProtocolRPC         = "rpc"
	AccessProtocolRPCOld      = "rpc_old"
	AccessProtocolFaasHTTP    = "faas_http"
	AccessProtocolFaasHTTPOld = "faas_http_old"
)

type HTTPMethod = string

const (
	HTTPMethodGet  = "get"
	HTTPMethodPost = "post"
)
