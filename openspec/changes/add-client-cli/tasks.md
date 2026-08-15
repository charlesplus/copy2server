## 1. Project Structure and Configuration

- [x] 1.1 新增独立的 Go CLI 命令入口，同时保留现有服务端入口。
- [x] 1.2 扩展配置解析，支持可选 `serverUrl`，且不改变服务端 `addr` 行为。
- [x] 1.3 实现 CLI 服务端 URL 优先级：`--server`、`COPY2SERVER_URL`、`client.config.json.serverUrl`、`http://127.0.0.1:8282`。
- [x] 1.4 更新 `server.config.json`、`client.config.json` 和 README 示例，说明 CLI 配置方式。

## 2. CLI Argument and Input Handling

- [x] 2.1 增加 CLI 参数：`--file`、`--server`、`--copy`、`--no-copy`、`--name`、`--timeout` 和帮助输出。
- [x] 2.2 实现位置参数文件上传，并确保显式文件优先于剪贴板输入。
- [x] 2.3 上传前校验显式文件路径；路径不可读时失败，并且不发起上传请求。
- [x] 2.4 规范 CLI 输出，确保每个上传后的服务器路径占 stdout 一行。

## 3. Clipboard Integration

- [x] 3.1 定义内部剪贴板接口，返回上传候选项，并支持纯文本写回。
- [x] 3.2 实现纯文本剪贴板读取和上传候选项生成。
- [x] 3.3 实现剪贴板文件引用支持，处理可读取的本地文件。
- [x] 3.4 实现剪贴板图片支持，处理操作系统暴露的图片数据。
- [x] 3.5 实现可读取富文本和二进制剪贴板 fallback，并提供明确的不支持格式错误。
- [x] 3.6 实现默认剪贴板写回：仅在上传成功后写入换行分隔的服务器路径。

## 4. Upload Client

- [x] 4.1 实现向 `<serverUrl>/api/upload` 发起 multipart 上传，字段名使用 `file`。
- [x] 4.2 解析上传响应，并将 `files[].serverPath` 提供给 stdout 和剪贴板写回。
- [x] 4.3 保留服务端 JSON `error` 响应，并在 CLI stderr 中展示。
- [x] 4.4 支持多个显式文件和多个剪贴板派生上传候选项；可行时使用单次请求上传。
- [x] 4.5 确保上传失败时以非零状态退出，并保持剪贴板不变。

## 5. Server Compatibility

- [x] 5.1 验证 Go 服务端仍通过同一个 `POST /api/upload` 契约接受浏览器上传和 CLI multipart 上传。
- [x] 5.2 验证 Python3 服务端无需新增 pip 依赖即可接受 CLI multipart 上传。
- [x] 5.3 验证 Node.js 服务端无需新增 npm 依赖即可接受 CLI multipart 上传。
- [x] 5.4 新增或更新 smoke-test 脚本/辅助工具，覆盖 CLI 对三套服务端运行时的上传行为。

## 6. Verification and Documentation

- [x] 6.1 增加 Go 单元测试，覆盖 CLI 配置优先级、参数解析、输出格式和上传响应解析。
- [x] 6.2 增加测试或测试接缝，覆盖剪贴板成功、不支持剪贴板内容、上传失败不写回剪贴板等行为。
- [x] 6.3 运行 `go test ./...`。
- [x] 6.4 运行 `python3 -m py_compile server.py`。
- [x] 6.5 运行 `node --check server.js`。
- [x] 6.6 运行 OpenSpec 校验：`openspec validate add-client-cli --strict`。
- [x] 6.7 更新 README，加入剪贴板上传、显式文件上传、服务端 URL 覆盖、禁用剪贴板写回等 CLI 使用示例。
