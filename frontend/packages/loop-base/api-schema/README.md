# @cozeloop/api-schema

Coze Loop 的 API Schema 定义包。

## 安装

```json
{
  "dependencies": {
    "@cozeloop/api-schema": "workspace:*"
  }
}
```

```bash
rush update
```

## 更新 API Schema

当后端 API 发生变更时，运行以下命令更新：

```bash
rushx update
```

### 指定分支

默认从 `main` 分支拉取 IDL 定义。如需从其他分支更新，可以修改 [package.json](./package.json) 中 `prethrift` 脚本的 `--branch` 参数：

```json
{
  "scripts": {
    "prethrift": "bash ./scripts/download-thrift.sh --branch=your-branch"
  }
}
```

然后执行 `rushx update` 即可从指定分支拉取最新的 API Schema。

## License

Apache-2.0
