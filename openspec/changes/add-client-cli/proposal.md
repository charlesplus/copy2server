## Why

copy2server 目前需要通过浏览器上传本地剪贴板内容或文件，这对终端工作流来说不够直接。新增 Go 客户端 CLI 后，用户可以用一个命令把当前剪贴板内容或指定文件上传到服务器，并得到服务器端文件路径；构建出二进制后，目标机器也不需要 Go 环境。

## What Changes

- 新增一个基于 Go 的客户端 CLI 二进制，用于向已有 copy2server 服务端上传内容。
- CLI 默认命令读取系统剪贴板，上传可读取内容，打印返回的服务器路径，并把这些路径写回剪贴板。
- 支持通过位置参数和 `--file` 显式上传文件路径。
- 支持平台实现能够读取的所有剪贴板格式，包括文本、文件引用、图片、富文本和可读取的二进制数据。
- 新增 CLI 服务端 URL 配置，来源包括 `--server`、`COPY2SERVER_URL`、`client.config.json.serverUrl` 和本地默认值。
- 保留现有服务端上传 API，并明确它对非浏览器客户端的契约。
- 保持现有 Go、Python3、Node.js 服务端实现与 CLI 上传行为兼容。

Non-goals:

- 本次不新增 Python 或 Node.js 版本的 CLI。
- 本次不新增认证、远程删除、文件列表等命令。
- 本次不要求修改浏览器 UI 工作流。
- 不承诺提取操作系统没有暴露给 CLI 的专有剪贴板格式。

## Capabilities

### New Capabilities

- `client-cli`：命令行上传工作流、参数、输出、退出行为和剪贴板路径写回。
- `clipboard-integration`：跨平台剪贴板读写行为和格式处理预期。

### Modified Capabilities

- `runtime-configuration`：新增面向客户端的 `serverUrl` 配置和 CLI 覆盖优先级。
- `web-file-transfer`：明确 `POST /api/upload` 支持非浏览器 multipart 客户端，并返回 CLI 使用的服务器路径。

## Impact

- 代码：新增 Go CLI 入口或包，同时保留现有 Go 服务端入口；必要时调整共享配置加载逻辑。
- API：不破坏 `POST /api/upload`；现有响应结构会成为 CLI 依赖的契约。
- 配置：新增 `client.config.json`，包含可选 `serverUrl`；现有服务端配置项保持向后兼容。
- 依赖：Go CLI 可能需要平台剪贴板支持。任何新增 Go 依赖都必须在设计中说明，并且不能影响 Python3/Node.js 服务端的无包管理器依赖运行要求。
- 兼容性：Go、Python3、Node.js 服务端实现都必须继续接受 CLI 的 multipart 上传请求形态。Python3 和 Node.js 服务端仍必须无需 pip/npm 安装即可运行。
