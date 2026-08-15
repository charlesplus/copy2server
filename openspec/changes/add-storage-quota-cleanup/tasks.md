## 1. Configuration

- [x] 1.1 在 `server.config.json` 中新增默认 `maxStorageGB: 5`。
- [x] 1.2 在 Go 服务端配置结构中新增 `maxStorageGB`，并支持 `MAX_STORAGE_GB` 覆盖。
- [x] 1.3 在 Python3 服务端配置读取中新增 `maxStorageGB`，并支持 `MAX_STORAGE_GB` 覆盖。
- [x] 1.4 在 Node.js 服务端配置读取中新增 `maxStorageGB`，并支持 `MAX_STORAGE_GB` 覆盖。
- [x] 1.5 对缺失、零值、负数和非数字配置统一回退到默认 `5G`。

## 2. Cleanup Semantics

- [x] 2.1 为三套服务端实现上传目录普通文件总大小统计，跳过目录和不可读取条目。
- [x] 2.2 将清理逻辑整理为单个动作，先处理超过 `retentionDays` 的文件，再处理超过 `maxStorageGB` 的容量上限。
- [x] 2.3 实现按修改时间从旧到新删除普通文件，直到总大小低于等于容量上限或没有可删除文件。
- [x] 2.4 为清理任务增加非重入保护，避免同一上传目录并发清理。
- [x] 2.5 移除本次变更中对启动时清理和周期性清理的实现要求。

## 3. Upload Integration

- [x] 3.1 在 Go 服务端上传成功并生成响应数据后异步触发清理。
- [x] 3.2 在 Python3 服务端上传成功并生成响应数据后异步触发清理。
- [x] 3.3 在 Node.js 服务端上传成功并生成响应数据后异步触发清理。
- [x] 3.4 在三套服务端中拒绝单个文件大于 `maxStorageGB` 的上传，并返回 JSON `error`。
- [x] 3.5 确保上传响应不等待异步清理完成。

## 4. Tests

- [x] 4.1 增加 Go 单元测试，覆盖 `maxStorageGB` 默认值、环境变量覆盖和非法值回退。
- [x] 4.2 增加 Go 单元测试，覆盖成功写入后触发 retention 清理和容量清理。
- [x] 4.3 增加 Go 单元测试，覆盖容量清理按最旧文件优先删除。
- [x] 4.4 增加 Go 单元测试或集成测试，覆盖超大单文件上传被拒绝。
- [x] 4.5 为 Python3 服务端增加可执行 smoke test，覆盖成功写入后容量上限清理。
- [x] 4.6 为 Node.js 服务端增加可执行 smoke test，覆盖成功写入后容量上限清理。

## 5. Verification and Documentation

- [x] 5.1 运行 `go test ./...`。
- [x] 5.2 运行 `python3 -m py_compile server.py`。
- [x] 5.3 运行 `node --check server.js`。
- [x] 5.4 分别对 Go、Python3、Node.js 服务端运行上传成功后异步清理 smoke test。
- [x] 5.5 运行 `openspec validate add-storage-quota-cleanup --strict`。
- [x] 5.6 更新 README，说明 `maxStorageGB`、`MAX_STORAGE_GB`、默认 `5G` 和成功写入后异步清理行为。
