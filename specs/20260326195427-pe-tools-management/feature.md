# PE-管理后台-Tools管理

## 注意事项

- 只关注服务端实现(backend)，前端(frontend)忽略（前后端分离开发模式）
- 服务端研发规范在 constitution/constitution.md 里，方案和实现必须严格 follow
- 本次需求相关的模块为 modules/prompt 模块

## 需求描述

新增 tool 这个实体，实现对应新增、详情、列表、提交版本、版本列表、保存草稿的功能。

### 补充信息

- 由于是前后端分开开发，所以需要两边先事先提前商量好 IDL，当前 IDL 代码已经变更完毕（可以和 git 仓库上 tag 为 PK_FEAT_COMMON_TOOL_START 的提交节点做 diff 看到完整的变更状态）
  - 但是，kitex gen 还未生成，需要使用 upgrade-idl 这个 skill 来生成
- 需要新增一个 batchGetTools 接口，入参是 id+version 列表对，version 为空则默认查最新版本。参考方法：BatchGetPromptByPromptKey（也可以看 idl 里的定义）

## Domain

```go
package entity

import "time"

type Tool struct {
    ID          int64
    SpaceID     int64
    ToolBasic  *ToolBasic
    ToolCommit *ToolCommit
}

type ToolBasic struct {
    Name                   string
    Description            string
    LatestCommittedVersion string
    CreatedAt              time.Time
    CreatedBy              string
    UpdatedAt              time.Time
    UpdatedBy              string
}

type ToolCommit struct {
    ToolDetail  *ToolDetail
    CommitInfo  *CommitInfo
}

type CommitInfo struct {
    Version     string
    BaseVersion string
    Description string
    CommittedBy string
}

const (
    PublicDraftVersion = "$PublicDraft"
)

func (v CommitInfo) IsPublicDraft() bool {
    return v.Version == PublicDraftVersion
}

type ToolDetail struct {
    Content string
}
```

## DB

```sql
CREATE TABLE IF NOT EXISTS `tool_basic` (
    `id`                 bigint unsigned                   NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    `space_id`           bigint unsigned                   NOT NULL COMMENT '空间ID',
    `name`               varchar(128) COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '名称',
    `description`        varchar(1024) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '描述',
    `latest_committed_version` varchar(128) COLLATE utf8mb4_bin NULL DEFAULT '' COMMENT '最新版本',
    `created_by`         varchar(128) COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '创建人',
    `updated_by`         varchar(128) COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '更新人',
    `created_at`         datetime                          NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`         datetime                          NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`         bigint                            NOT NULL DEFAULT '0' COMMENT '删除时间',
    PRIMARY KEY (`id`),
    KEY `idx_spaceid_name_delat` (`space_id`, `name`, `deleted_at`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='工具主体';

CREATE TABLE IF NOT EXISTS `tool_commit` (
    `id`               bigint unsigned                         NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    `space_id`         bigint unsigned                         NOT NULL COMMENT '空间ID',
    `tool_id`          bigint unsigned                         NOT NULL COMMENT 'Tool ID',
    `content`          longtext COLLATE utf8mb4_general_ci COMMENT '工具内容',
    `version`          varchar(128) COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '版本',
    `base_version`     varchar(128) COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '来源版本',
    `committed_by`     varchar(128) COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '提交人',
    `description`      text COLLATE utf8mb4_general_ci COMMENT '提交版本描述',
    `created_at`       datetime                                NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`       datetime                                NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_tool_version` (`tool_id`, `version`),
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='工具版本';
```

## IDL 接口

- 创建接口：CreateTool
- 详情接口：GetToolDetail
- 列表接口：ListTools
- 提交版本：CommitToolDraft
- 版本列表：ListToolCommit
- 保存草稿：SaveToolDetail
- 批量获取：BatchGetTools
