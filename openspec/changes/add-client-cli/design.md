## Context

服务端已经提供稳定的 multipart 上传 API：`POST /api/upload`，并返回 `files[].serverPath`。新的 CLI 是这个 API 的客户端，不替代浏览器 UI，也不替代 Go/Python3/Node.js 服务端实现。

本项目有两个会影响方案的约束：CLI 应优先作为 Go 二进制交付；Python3/Node.js 服务端运行时仍必须不依赖 pip/npm 包安装。

## Goals / Non-Goals

**Goals:**

- 构建一个 Go CLI，向已有 copy2server 服务端上传显式文件或可读取的剪贴板内容。
- 保持 stdout 对脚本友好：始终打印上传后得到的 `serverPath`。
- 只在上传成功后默认把服务器路径写回剪贴板。
- 通过现有 multipart `file` 字段契约，保持 Go、Python3、Node.js 服务端实现兼容。
- 将 CLI 服务端访问 URL 配置与服务端监听地址配置分开。

**Non-Goals:**

- 本次不提供 Python 或 Node.js 版本 CLI。
- 本次不提供认证、列表、下载、删除或服务端管理命令。
- 本次不重新设计服务端 API。
- 当平台不暴露可读取数据时，不保证能提取操作系统私有剪贴板格式。

## Decisions

### Decision: Add a Separate Go CLI Entrypoint

创建独立的 Go 客户端命令，例如 `cmd/copy2server/`，同时保留现有服务端入口。必要时，可以把配置解析、上传响应解析、默认文件名等共享逻辑移动到小型内部包中。

考虑过的替代方案：

- 在现有服务端二进制上增加 CLI 参数：拒绝。服务端和客户端生命周期不同，默认行为会变得含糊。
- 用 Python 或 Node.js 实现 CLI：拒绝。当前明确要求第一优先交付 Go 二进制。

### Decision: Use Existing Multipart Upload Contract

CLI 向 `<serverUrl>/api/upload` 发送 `multipart/form-data`，字段名使用 `file`，然后解析现有 JSON 响应。这样不需要新增 CLI 专用接口，也能让 Go、Python3、Node.js 三套服务端保持同一个上传契约。

考虑过的替代方案：

- 新增 `/api/cli/upload`：拒绝。它会重复现有行为，并增加三套服务端的同步成本。
- 直接发送原始字节流：拒绝。它需要新的服务端 API 和额外元数据规则。

### Decision: Treat Clipboard Handling as an Adapter Layer

剪贴板读写放在独立接口后面，接口返回一个或多个上传候选项：文件名、已知内容类型，以及文件路径或字节数据。平台相关实现可以通过 build tags 或少量平台文件隔离。

上传层不关心输入来源是文件参数、文件引用、图片字节、富文本、二进制数据还是纯文本。

考虑过的替代方案：

- 只调用平台命令：读取文本较简单，但图片和文件引用场景脆弱。
- 要求用户安装外部剪贴板工具：不作为默认路径；在原生 API 不可用时可以作为可选 fallback。

### Decision: Default to Copying Server Paths Back to Clipboard

上传成功后，stdout 输出服务器路径，剪贴板写回同样的文本。多文件上传时，两者都使用换行分隔路径。上传失败时，CLI 不得修改剪贴板。

考虑过的替代方案：

- 只打印不复制：拒绝。工具的核心工作流是把本地内容变成可粘贴的服务器路径。
- 只复制不打印：拒绝。脚本场景需要 stdout。

### Decision: Add `serverUrl` for Client Configuration

`addr` 继续表示服务端监听地址。`serverUrl` 表示 CLI 使用的客户端访问 URL。优先级为：`--server`、`COPY2SERVER_URL`、`client.config.json` 的 `serverUrl`、`http://127.0.0.1:8282`。

考虑过的替代方案：

- 复用 `addr` 作为 URL：拒绝。`:8282` 是合法监听地址，但不是完整客户端 URL。
- 每次都要求传 `--server`：拒绝。本地默认使用应该保持一个命令即可完成。

## Risks / Trade-offs

- [风险] 跨平台剪贴板 API 差异很大。→ 缓解：隔离剪贴板 provider，尽可能保证文本支持，并对不支持或私有格式给出明确错误。
- [风险] 新增 Go 剪贴板依赖会增加二进制复杂度。→ 缓解：依赖只限定在 Go CLI；不为 Python3/Node.js 服务端引入 pip/npm 依赖。
- [风险] 三套服务端 multipart 解析差异可能破坏 CLI 兼容性。→ 缓解：增加 smoke test，用 CLI 或等价 multipart 客户端分别验证 Go、Python3、Node.js 服务端。
- [风险] 部分失败时写回剪贴板可能破坏用户原剪贴板内容。→ 缓解：只在所有请求上传成功并拿到服务器路径后写回。
- [风险] 多文件和多剪贴板项可能让输出不明确。→ 缓解：规定 stdout 和剪贴板写回都使用换行分隔路径。

## Migration Plan

1. 在保留所有现有配置项的同时，新增 `client.config.json`，并在其中放置 `serverUrl`。
2. 新增 Go CLI 命令和共享客户端配置逻辑。
3. 在稳定剪贴板接口后，逐步加入平台剪贴板适配器。
4. 验证现有浏览器工作流保持不变。
5. 分别验证 CLI 上传能兼容 Go、Python3、Node.js 服务端运行时。

回滚方式直接：移除 CLI 命令和 `serverUrl` 使用即可。现有服务端配置和浏览器上传行为保持兼容。
