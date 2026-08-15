# copy2server

copy2server 是一个轻量工具，用来把本地文件、截图和剪贴板内容快速变成服务器上的文件路径。

它提供两种上传入口：

- Web UI：支持拖拽、文件选择和粘贴上传。
- 原生 CLI：支持快速上传剪贴板或指定文件，适合绑定全局快捷键。

核心流程很简单：复制本地内容，运行 `copy2server`，然后直接粘贴返回的服务器路径。

## 为什么需要 CLI？

CLI 是 copy2server 最高频、最快的使用方式。它可以上传当前剪贴板内容，不需要打开浏览器，也不需要切换窗口。

典型场景：

- 在 Finder 或 Windows Explorer 里复制文件，按快捷键，然后粘贴服务器路径。
- 截图后按快捷键，然后粘贴上传后的图片路径。
- 复制一段文本，按快捷键，得到上传后的 `.txt` 文件路径。
- 在终端或脚本里上传文件，并拿到 stdout 输出的服务器路径。

Web UI 更适合可视化上传和文件管理；CLI 更适合高频、瞬时、一键式上传。

## 功能

- 浏览器选择文件上传。
- 拖拽文件到页面上传。
- 在网页中粘贴截图或剪贴板图片上传。
- 通过 CLI 上传剪贴板内容。
- 通过 CLI 上传指定文件。
- 上传后在 stdout 打印服务器文件路径。
- 默认把服务器文件路径写回剪贴板。
- 在 Web UI 中查看已上传文件。
- 下载已上传文件。
- 在 Web UI 中复制服务器文件路径。
- 限制单次上传大小。
- 限制上传目录总存储大小，默认 `5 GB`。
- 上传成功后清理旧文件，默认保留 `15` 天。

## 服务端

服务端可以使用 Go、Python 3 或 Node.js 运行。Python 和 Node.js 版本只使用标准库，不需要 `pip install` 或 `npm install`。

三种服务端实现共用 `server.config.json` 和 `index.html`。

```bash
# Go
go run .

# Python 3
python3 server.py

# Node.js
node server.js
```

默认监听 `:8282`，表示监听所有网卡的 8282 端口。上传文件默认保存在 `uploads/`。

如果只允许本机访问，可以设置：

```json
{
  "addr": "127.0.0.1:8282"
}
```

## CLI 二进制

预编译 CLI 二进制可以放到任意 `PATH` 目录下使用。

- macOS Apple Silicon：`dist/copy2server-darwin-arm64`
- macOS Intel：`dist/copy2server-darwin-amd64`
- Windows x64：`dist/copy2server-windows-amd64.exe`
- Windows ARM64：`dist/copy2server-windows-arm64.exe`

macOS 示例：

```bash
chmod +x dist/copy2server-darwin-arm64
./dist/copy2server-darwin-arm64 --server http://127.0.0.1:8282 ./image.png
```

Windows 示例：

```powershell
.\dist\copy2server-windows-amd64.exe --server http://127.0.0.1:8282 .\image.png
```

也可以把二进制重命名为 `copy2server` 或 `copy2server.exe`，命令会更短。

## CLI 用法

```bash
# 上传可读取的剪贴板内容，并把返回路径写回剪贴板
copy2server

# 上传单个文件
copy2server ./image.png

# 上传多个文件
copy2server image.png notes.txt

# 临时指定服务端地址
copy2server --server http://127.0.0.1:8282 ./image.png

# 只打印返回路径，不修改剪贴板
copy2server --no-copy ./image.png

# 为剪贴板派生的上传内容指定文件名
copy2server --name note.txt
```

开发时可以直接运行：

```bash
go run ./cmd/copy2server --no-copy ./image.png
```

CLI 服务端地址优先级：

1. `--server`
2. `COPY2SERVER_URL`
3. `client.config.json` 中的 `serverUrl`
4. `http://127.0.0.1:8282`

## 全局快捷键

CLI 最推荐的使用方式是绑定全局快捷键。

推荐流程：

1. 复制一个文件、截图、图片或文本。
2. 按下配置好的快捷键。
3. 粘贴已经写回剪贴板的服务器路径。

可用工具：

- macOS：Shortcuts、Automator、Raycast、Alfred、Keyboard Maestro。
- Windows：AutoHotkey、PowerToys、Windows 快捷方式热键。
- Linux：桌面环境快捷键、`sxhkd`、自定义脚本。

Windows AutoHotkey 示例：

```ahk
^!u::RunWait, D:\Tools\copy2server.exe,, Hide
```

macOS 快捷指令中可以调用：

```bash
/usr/local/bin/copy2server
```

## 剪贴板支持

剪贴板能力取决于操作系统向命令行程序暴露了哪些格式。

当前支持：

- 纯文本。
- CLI 参数中显式传入的文件路径。
- 从 Finder 或 Windows Explorer 复制的文件引用。
- 截图和剪贴板图片。
- 部分富文本和二进制剪贴板格式，前提是平台能暴露可读取数据。

平台说明：

- macOS：使用 `pbpaste`/`pbcopy` 处理文本，通过 `osascript`/AppKit 读取 Finder 文件和截图，`pngpaste` 可作为图片读取 fallback。
- Windows：通过 Win32 `CF_HDROP` 读取 Explorer 复制的文件，通过 Win32 `CF_UNICODETEXT` 写回路径，并使用 Windows 剪贴板 API 处理截图。
- Linux：使用系统可用的剪贴板工具，例如 `wl-paste`、`wl-copy`、`xclip` 或 `xsel`。

## 配置

服务端配置写在 `server.config.json`：

```json
{
  "addr": ":8282",
  "uploadDir": "uploads",
  "maxUploadMB": 512,
  "maxStorageGB": 5,
  "retentionDays": 15,
  "indexPath": "index.html"
}
```

CLI 配置写在 `client.config.json`：

```json
{
  "serverUrl": "http://127.0.0.1:8282"
}
```

`addr` 是服务端监听地址，`serverUrl` 是 CLI 上传请求访问的服务端地址。

远程使用时，服务端通常可以保持：

```json
{
  "addr": ":8282"
}
```

然后把客户端配置成真实服务器地址：

```json
{
  "serverUrl": "http://192.168.1.10:8282"
}
```

环境变量：

- `CONFIG`：服务端配置路径，默认 `server.config.json`
- `ADDR`：服务端监听地址，例如 `:8282` 或 `127.0.0.1:8282`
- `UPLOAD_DIR`：上传文件保存目录
- `MAX_UPLOAD_MB`：单次上传大小上限，单位 MB
- `MAX_STORAGE_GB`：上传目录总容量上限，单位 GB，默认 `5`
- `RETENTION_DAYS`：文件保留天数，默认 `15`
- `INDEX_HTML`：Web UI 模板路径，默认 `index.html`
- `COPY2SERVER_CONFIG`：CLI 配置路径，默认 `client.config.json`
- `COPY2SERVER_URL`：CLI 服务端地址覆盖项

示例：

```bash
ADDR=:8282 UPLOAD_DIR=uploads MAX_UPLOAD_MB=512 MAX_STORAGE_GB=5 RETENTION_DAYS=15 python3 server.py
COPY2SERVER_URL=http://192.168.1.10:8282 copy2server ./image.png
```

## 清理行为

清理动作只会在成功写入上传文件之后触发。服务启动时不会单独清理，服务空闲运行时也不会周期性清理。

清理顺序：

1. 删除超过 `RETENTION_DAYS` 的普通文件。
2. 如果上传目录总大小仍超过 `MAX_STORAGE_GB`，按修改时间从旧到新删除普通文件，直到低于容量上限。

如果单个上传文件本身大于存储容量上限，服务端会拒绝该上传。

## 编译 CLI

```bash
mkdir -p dist
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o dist/copy2server-darwin-arm64 ./cmd/copy2server
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o dist/copy2server-darwin-amd64 ./cmd/copy2server
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o dist/copy2server-windows-amd64.exe ./cmd/copy2server
CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -o dist/copy2server-windows-arm64.exe ./cmd/copy2server
```

## OpenSpec

本仓库使用 OpenSpec 管理产品和研发变更。

重要路径：

- `openspec/config.yaml`：项目上下文和工作流规则。
- `openspec/specs/`：已经接受的系统规格。
- `openspec/changes/`：提案中或进行中的变更。
- `.agents/skills/`：`openspec init --tools codex` 生成的 Codex OpenSpec 工作流技能。

常用命令：

```bash
openspec new change <change-name>
openspec status --change <change-name>
openspec validate --all --strict
```

## 定位

copy2server 不是完整网盘，也不打算替代云存储。它是一个小工具，用于把本地内容快速投递到服务器，并拿到一个可复用的服务器路径。

适合场景：

- 个人或团队内部工具。
- 局域网文件临时共享。
- 截图、文件、文本快速交接。
- 脚本化上传流程。

如果要暴露到可信网络之外，建议增加认证和网络访问控制。
