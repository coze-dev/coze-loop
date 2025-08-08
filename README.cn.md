![Image](https://p9-arcosite.byteimg.com/tos-cn-i-goo7wpa0wc/11faa43b83754c089d2ec953306d3e63~tplv-goo7wpa0wc-image.image)

<div align="center">
<p>
  <a href="#什么是-coze-loop">Coze Loop</a> •
  <a href="#功能清单">功能清单</a> •
  <a href="#快速开始">快速开始</a> •
  <a href="#开发指南">开发指南</a>
</p>
<p>
  <img alt="License" src="https://img.shields.io/badge/license-apache2.0-blue.svg">
  <img alt="Go Version" src="https://img.shields.io/badge/go-%3E%3D%201.23.4-blue">
</p>

[English](README.md) | 中文

</div>

## 什么是 Coze Loop

[Coze Loop ](https://www.coze.cn/loop) 是一个面向开发者，专注于 AI Agent 开发与运维的平台级解决方案。 它可以解决 AI Agent 开发过程中面临的各种挑战，提供从开发、调试、评估、到监控的全生命周期管理能力。

Coze Loop 在商业化版本的基础上，推出开源版免费对开发者开放核心基础功能模块，以开源模式共享核心技术框架，开发者可根据业务需求定制与扩展，便于社区共建、分享交流，助力开发者零门槛参与 AI Agent 的探索与实践。

## Coze Loop 能做什么？
Coze Loop 通过提供全生命周期的管理能力，帮助开发者更高效地开发和运维 AI Agent。无论是提示词工程、AI Agent 评测，还是上线后的监控与调优，Coze Loop 都提供了强大的工具和智能化的支持，极大地简化了 AI Agent 的开发流程，提升了 AI Agent 的运行效果和稳定性。

* **Prompt 开发**：Coze Loop 的 Prompt 开发模块为开发者提供了从编写、调试、优化到版本管理的全流程支持，通过可视化 Playground 实现 Prompt 的实时交互测试，让开发者能够直观比较不同大语言模型的输出效果。
* **评测**：Coze Loop 评测模块为开发者提供系统化的评测能力，能够对 Prompt 和扣子智能体的输出效果进行多维度自动化检测，例如准确性、简洁性和合规性等。
* **观测**：Coze Loop 为开发者提供了全链路执行过程的可视化观测能力，完整记录从用户输入到 AI 输出的每个处理环节，包括 Prompt 解析、模型调用和工具执行等关键节点，并自动捕获中间结果和异常状态。

## 功能清单

| **功能** | **功能点** |
| --- | --- |
| Prompt 调试 | * Playground 调试、对比 <br> * Prompt 版本管理 |
| 评测 | * 管理评测集 <br> * 管理评估器 <br> * 管理实验 |
| 观测 | * SDK 上报 Trace <br> * Trace 数据观测 |
| 模型 | 支持接入 OpenAI、火山方舟等模型 |

## 快速开始
> 参考[快速开始](https://github.com/coze-dev/coze-loop/wiki/2.-%E5%BF%AB%E9%80%9F%E5%BC%80%E5%A7%8B)，详细了解如何安装部署 Coze Loop 最新版本。
环境要求：
* 提前安装 Docker、Docker Compose，并启动 Docker 服务
操作步骤：
1. 获取源码。执行以下命令，获取 Coze Loop 最新版本的源码。
   ```Bash
   # 克隆代码
   git clone https://github.com/coze-dev/coze-loop.git
   # 进入Coze Loop目录下
   cd coze-loop
   ```
2. 配置模型。进入目录 `conf/default/app/runtime/`，编辑文件 `model_config.yaml`，修改 api_key 和 model 字段。以火山方舟为例：
   * api_key：火山方舟 API Key。中国境内用户参考[火山方舟文档](https://www.volcengine.com/docs/82379/1541594)；非中国境内的用户可参考[BytePlus ModelArk 文档](https://docs.byteplus.com/en/docs/ModelArk/1361424?utm_source=github&utm_medium=readme&utm_campaign=coze_open_source)。
   * model：火山方舟模型接入点的 Endpoint ID。中国境内用户参考[火山方舟文档](https://www.volcengine.com/docs/82379/1099522)；非中国境内的用户可参考[BytePlus ModelArk 文档](https://docs.byteplus.com/en/docs/ModelArk/1099522?utm_source=github&utm_medium=readme&utm_campaign=coze_open_source)。
3. 启动服务。执行以下命令，使用 Docker Compose 快速部署 Coze Loop 开源版。
   ```Bash
   # 启动服务，默认为开发模式
   docker compose up --build
   ```
4. 通过浏览器访问 `http://localhost:8082`，访问 Coze Loop 开源版。

## 使用 Coze Loop 开源版

* [Prompt 开发与调试](https://loop.coze.cn/open/docs/cozeloop/create-prompt)：Coze Loop 提供了完整的提示词开发流程。
* [评测](https://loop.coze.cn/open/docs/cozeloop/evaluation-quick-start)：Coze Loop 的评测功能提供标准评测数据管理、自动化评估引擎和综合的实验结果统计。
* [Trace 上报与查询](https://loop.coze.cn/open/docs/cozeloop/trace_integrate)：Coze Loop 支持对平台上创建的 Prompt 调试的 Trace 自动上报，实时追踪每一条 Trace 数据。
* [开源版使用Coze Loop SDK](https://github.com/coze-dev/coze-loop/wiki/8.-%E5%BC%80%E6%BA%90%E7%89%88%E4%BD%BF%E7%94%A8-CozeLoop-SDK)：Coze Loop 三个语言的 [SDK](https://loop.coze.cn/open/docs/cozeloop/sdk) 均适用于商业版和开源版。对于开源版，开发者只需要初始化时修改部分参数配置。

## 开发指南

* [系统架构](https://github.com/coze-dev/coze-loop/wiki/3.-%E7%B3%BB%E7%BB%9F%E6%9E%B6%E6%9E%84)：了解Coze Loop 开源版的技术架构与核心组件。
* [启动模式](https://github.com/coze-dev/coze-loop/wiki/4.-%E6%9C%8D%E5%8A%A1%E5%90%AF%E5%8A%A8%E6%A8%A1%E5%BC%8F)：安装部署Coze Loop 开源版时，默认使用开发模式，此模式下修改后端文件无需重新部署服务。
* [模型配置](https://github.com/coze-dev/coze-loop/wiki/5.-%E6%A8%A1%E5%9E%8B%E9%85%8D%E7%BD%AE)：Coze Loop 开源版通过 Eino 框架支持多种 LLM 模型，参考此文档查看支持的模型列表，了解如何配置模型。
* [代码开发与测试](https://github.com/coze-dev/coze-loop/wiki/6.-%E4%BB%A3%E7%A0%81%E5%BC%80%E5%8F%91%E4%B8%8E%E6%B5%8B%E8%AF%95)：了解如何基于Coze Loop 开源版进行二次开发与测试。
* [故障排查](https://github.com/coze-dev/coze-loop/wiki/7.-%E6%95%85%E9%9A%9C%E6%8E%92%E6%9F%A5)：了解如何查看容器状态、系统日志。

## License

本项目采用 Apache 2.0 许可证。详情请参阅 [LICENSE](LICENSE) 文件。

## 社区贡献
我们欢迎社区贡献，贡献指南参见 [CONTRIBUTING](CONTRIBUTING.md) 和 [Code of conduct](CODE_OF_CONDUCT.md)，期待您的贡献！

## 安全与隐私

如果你在该项目中发现潜在的安全问题，或你认为可能发现了安全问题，请通过我们的[安全中心](https://security.bytedance.com/src) 或[漏洞报告邮箱](sec@bytedance.com)通知字节跳动安全团队。
请**不要**创建公开的 GitHub Issue。

## 加入社区

我们致力于构建一个开放、友好的开发者社区，欢迎所有对 AI Agent 开发感兴趣的开发者加入我们！

### 问题反馈与功能建议
为了更高效地跟踪和解决问题，保证信息透明和便于协同，我们推荐通过以下方式参与：
- **GitHub Issues**：[提交 Bug 报告或功能请求](https://github.com/coze-dev/coze-loop/issues)
- **Pull Requests**：[贡献代码或文档改进](https://github.com/coze-dev/coze-loop/pulls)

### 技术交流与讨论
加入我们的技术交流群，与其他开发者分享经验、获取项目最新动态：

* 飞书群聊：飞书移动端扫描以下二维码，加入Coze Loop 技术交流群。

![Image](https://p9-arcosite.byteimg.com/tos-cn-i-goo7wpa0wc/818dd6ec45d24041873ca101681186c1~tplv-goo7wpa0wc-image.image)

* Discord 服务器：[Coze Community](https://discord.gg/a6YtkysB)

* Telegram 群组：[Coze](https://t.me/+pP9CkPnomDA0Mjgx)

## 致谢
感谢所有为 Coze Loop 项目做出贡献的开发者和社区成员。特别感谢：

* [Eino](https://github.com/cloudwego/eino) 框架团队提供的 LLM 集成支持
* [CloudWeGo](https://www.cloudwego.io) 团队开发的高性能框架
* 所有参与测试和反馈的用户
