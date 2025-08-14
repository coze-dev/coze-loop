![Image](https://p9-arcosite.byteimg.com/tos-cn-i-goo7wpa0wc/11faa43b83754c089d2ec953306d3e63~tplv-goo7wpa0wc-image.image)

<div align="center">
<a href="#what-can-coze-loop-do">Coze Loop</a> •
<a href="#feature-list">Feature list</a> •
<a href="#quick-start">Quick start</a> •
<a href="#developer-guide">Developer guide</a>
</p>
<p>
  <img alt="License" src="https://img.shields.io/badge/license-apache2.0-blue.svg">
  <img alt="Go Version" src="https://img.shields.io/badge/go-%3E%3D%201.23.4-blue">
</p>

English | [中文](README.cn.md)

</div>

## What is Coze Loop

[Coze Loop](https://www.coze.cn/loop) is a developer-oriented, platform-level solution focused on the development and operation of AI agents. It addresses various challenges faced during the AI agent development process, providing full lifecycle management capabilities from development, debugging, evaluation, to monitoring.

Based on the commercial version, Coze Loop introduces an open-source edition that offers developers free access to core foundational feature modules. By sharing its core technology framework in an open-source model, developers can customize and extend according to business needs, facilitating community co-construction, sharing, and exchange, helping developers participate in AI agent exploration and practice with zero barriers.

## What can Coze Loop do?
Coze Loop helps developers develop and operate AI Agent more efficiently by providing full lifecycle management capabilities. Whether it is prompt engineering, AI Agent evaluation, or monitoring and optimization after deployment, Coze Loop provides powerful tools and intelligent support, greatly simplifying the development process of AI Agents and enhancing their operational performance and stability.

* **Prompt development**: The Prompt development module of Coze Loop provides developers with end-to-end support for writing, debugging, optimizing, and version management. Through a visual Playground, it enables real-time interactive testing of prompts, allowing developers to intuitively compare the output of different LLMs.
* **Evaluation**: The Coze Loop evaluation module provides developers with systematic evaluation capabilities, enabling automated multi-dimensional testing of prompts and Coze agents' output, such as accuracy, conciseness, compliance, and more.
* **Observability**: Coze Loop provides developers with observability for the entire execution process, fully recording every stage from user input to AI output, including key stages such as prompt parsing, model invocation, and tool execution, and automatically capturing intermediate results and exceptions.

## Feature list
| **Feature** | **Functional points** |
| --- | --- |
| Prompt debugging | * Playground debugging and comparison <br> * Prompt version management |
| Evaluation | * Manage evaluation sets <br> Management evaluator <br> * Manage experiments |
| Observation | SDK trace reporting <br> * Trace data observation |
| Model | Support integration with OpenAI, Volcengine Ark, and other models |
## Quick Start
> Refer to [Quick Start](https://github.com/coze-dev/coze-loop/wiki/2.-Quickstart) to learn in detail how to install and deploy the latest version of Coze Loop.

### Deployment method 1: Docker deployment (Docker Compose)
> Please install and start Docker Engine before you start.

Procedure:

1. Clone the source code.
   Run the following command to obtain the latest version of the Coze Loop source code.
   ```Bash
   # Clone the code
   git clone https://github.com/coze-dev/coze-loop.git
   
   # Enter the coze-loop directory
   cd coze-loop 
   ```

2. Configure a model.
   1. Enter the `coze-loop` directory.
   2. Edit the file `release/deployment/docker-compose/conf/model_config.yaml`.
   3. Modify the api_key and model fields. Take Volcengine Ark as an example:
      * api_key: Volcengine Ark API Key. Users in China can refer to the [Volcengine Ark documentation](https://www.volcengine.com/docs/82379/1541594), while users outside China can refer to the [BytePlus ModelArk documentation](https://docs.byteplus.com/en/docs/ModelArk/1361424?utm_source=github&utm_medium=readme&utm_campaign=coze_open_source).
      * model: The Endpoint ID of the Volcengine Ark model access point. Users within China can refer to [the Volcengine Ark documentation](https://www.volcengine.com/docs/82379/1099522); users outside China can refer to [the BytePlus ModelArk documentation](https://docs.byteplus.com/en/docs/ModelArk/1099522?utm_source=github&utm_medium=readme&utm_campaign=coze_open_source).
3. Start the service.
   Run the following commands to quickly deploy the open-source version of Coze Loop using Docker Compose.
   ```Bash
   # Start the service (default: development mode)
   # Run in the coze-loop/ directory
   make compose-up 
   ```

4. Access the Coze Loop open-source version through your browser `http://localhost:8082`.

### Deployment method 2: Kubernetes deployment using Helm Chart

> * The Kubernetes cluster has been prepared, the Nginx Ingress add-ons have been enabled, and the Kubectl and Helm tools have been installed.
> * To quickly try it out locally, you can deploy a Kubernetes cluster using Minikube. For detailed steps, refer to [Quick Start](https://github.com/coze-dev/coze-loop/wiki/2.-Quickstart).

Procedure:

1. Run the following command to obtain the Helm Chart package.
   ```Bash
   helm pull oci://docker.io/cozedev/coze-loop --version 1.0.0-helm
   tar -zxvf coze-loop-1.0.0-helm.tgz && cd coze-loop && rm -f ../coze-loop-1.0.0-helm.tgz
   ```

2. Configure a model.
   Go to the `coze-loop` directory and edit the `release/deployment/docker-compose/conf/model_config.yaml` file. Configure the following fields, using Volcengine Ark as an example:
   * api_key: Volcengine Ark API Key. Users in mainland China can refer to the [Volcengine Ark documentation](https://www.volcengine.com/docs/82379/1541594), while users outside mainland China can refer to the [BytePlus ModelArk documentation](https://docs.byteplus.com/en/docs/ModelArk/1361424?utm_source=github&utm_medium=readme&utm_campaign=coze_open_source).
   * model: The Endpoint ID of the Volcengine Ark model access point. Users in China can refer to the [Volcengine Ark documentation](https://www.volcengine.com/docs/82379/1099522), while users outside China can refer to the [BytePlus ModelArk documentation](https://docs.byteplus.com/en/docs/ModelArk/1099522?utm_source=github&utm_medium=readme&utm_campaign=coze_open_source).
3. Configure Ingress rules.
   Ingress is used to expose services to external networks. You need to configure the `templates/ingress.yaml` file in the project directory according to the actual cluster situation, manually modify parameters such as ingressClassName, and configure elements such as class, instance, host, and IP allocation.
4. Deploy and start the service.
   Execute the following commands to quickly deploy the open-source version of Coze Loop using Helm.
   ```Bash
   # Run in the coze-loop/ directory
   make helm-up
   # After the service deployment is complete, check the status of the cluster pods
   make helm-pod
   # Check the service startup logs. If both the app and nginx are running normally, the deployment is successful
   make helm-logf-app
   make helm-logf-nginx
   ```

5. Access the Coze Loop open source edition via a browser.
   The access domain name and URL depend on the domain name and URL assigned to your cluster.
6. Start customizing your Coze Loop project.
   Refer to the examples in the `examples/` directory. Modify `values.yaml` to override the default settings. After making changes, rerun `make helm-up` for the changes to take effect.

## Use the Coze Loop open source version

* [Prompt development and debugging](https://loop.coze.cn/open/docs/cozeloop/create-prompt): Coze Loop provides a complete prompt development workflow.
* [Evaluation](https://loop.coze.cn/open/docs/cozeloop/evaluation-quick-start): The evaluation functionality of Coze Loop provides standard evaluation data management, an automated evaluation engine, and comprehensive statistics on experimental results.
* [Trace reporting and query](https://loop.coze.cn/open/docs/cozeloop/trace_integrate): Coze Loop supports automatic reporting of traces from prompt debugging sessions created on the platform, enabling real-time tracking of each trace.
* [Open-source Edition usage of the Coze Loop SDK](https://github.com/coze-dev/coze-loop/wiki/8.-Open-source-edition-uses-CozeLoop-SDK): The Coze Loop SDK in three languages is suitable for both commercial and open-source editions. For the Open-source Edition, developers only need to modify some parameter configurations during initialization.

## Developer guide

* [System architecture](https://github.com/coze-dev/coze-loop/wiki/3.-Architecture): Learn about the technical architecture and core components of Coze Loop Open-source Edition.
* [Startup mode](https://github.com/coze-dev/coze-loop/wiki/4.-Service-startup-modes): When installing and deploying Coze Loop Open-source Edition, the default development mode allows backend file modifications without requiring service redeployment.
* [Model configuration](https://github.com/coze-dev/coze-loop/wiki/5.-Model-configuration): Coze Loop Open-source Edition supports various LLM models through the Eino framework. Refer to this document to view the supported model list and learn how to configure models.
* [Code development and testing](https://github.com/coze-dev/coze-loop/wiki/6.-Code-development-and-testing): Learn how to perform secondary development and testing based on Coze Loop Open-source Edition.
* [Fault troubleshooting](https://github.com/coze-dev/coze-loop/wiki/7.-Troubleshooting): Learn how to check container status and system logs.

## License

This project uses the Apache 2.0 license. For more details, please refer to the [LICENSE](LICENSE) file.

## Community Contributions

We welcome community contributions. For contribution guidelines, please refer to [CONTRIBUTING](CONTRIBUTING.md) and [Code of conduct](CODE_OF_CONDUCT.md). We look forward to your contributions!

## Security and Privacy

If you identify potential security issues in this project or believe you may have found one, please notify Bytedance's security team via our [Security Center](https://security.bytedance.com/src) or [Vulnerability Report Email](sec@bytedance.com).

Please **do not** create public GitHub Issues.

## Join the Community

We are committed to building an open and friendly developer community. All developers interested in AI Agent development are welcome to join us!

### Issue Reports & Feature Requests
To efficiently track and resolve issues while ensuring transparency and collaboration, we recommend participating through:
- **GitHub Issues**: [Submit bug reports or feature requests](https://github.com/coze-dev/coze-loop/issues)
- **Pull Requests**: [Contribute code or documentation improvements](https://github.com/coze-dev/coze-loop/pulls)

### Technical Discussion & Communication
Join our technical discussion groups to share experiences with other developers and stay updated with the latest project developments:

* Lark Group Chat: Scan the QR code below on the Lark mobile app to join the Coze Loop technical discussion group.

![Image](https://p9-arcosite.byteimg.com/tos-cn-i-goo7wpa0wc/818dd6ec45d24041873ca101681186c1~tplv-goo7wpa0wc-image.image)

* Discord Server: [Coze Community](https://discord.gg/a6YtkysB)

* Telegram Group: [Coze](https://t.me/+pP9CkPnomDA0Mjgx)

## Acknowledgments
Thanks to all developers and community members who contributed to the Coze Loop project Special thanks:

* LLM integration support provided by the [Eino](https://github.com/cloudwego/eino) framework team
* High-performance frameworks developed by the [CloudWeGo](https://www.cloudwego.io) team
* All users who participated in testing and feedback

