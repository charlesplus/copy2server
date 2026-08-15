# copy2server

copy2server is a lightweight tool for turning local files, screenshots, and clipboard content into server-side file paths.

It provides two ways to upload:

- A web UI for drag-and-drop, file picker, and paste-to-upload workflows.
- A native CLI for instant clipboard/file uploads, designed to be bound to a global hotkey.

The main workflow is simple: copy something locally, run `copy2server`, then paste the returned server path anywhere.

## Why CLI?

The CLI is the fastest way to use copy2server. It lets you upload the current clipboard without opening a browser or switching apps.

Typical workflows:

- Copy a file in Finder or Windows Explorer, press a hotkey, then paste the server path.
- Take a screenshot, press a hotkey, then paste the uploaded image path.
- Copy text, press a hotkey, then paste a server path to the uploaded `.txt` file.
- Upload a file from a terminal or script and capture the returned path.

The web UI is useful for visual upload and file management. The CLI is best for high-frequency, one-action uploads.

## Features

- Upload files from the browser UI.
- Drag and drop files into the web page.
- Paste screenshots or clipboard images into the web page.
- Upload clipboard content from the CLI.
- Upload explicit files from the CLI.
- Print uploaded server paths to stdout.
- Copy uploaded server paths back to the clipboard by default.
- List uploaded files in the web UI.
- Download uploaded files.
- Copy server paths from the web UI.
- Limit individual upload size.
- Limit total upload storage size, default `5 GB`.
- Clean old files after successful uploads, default retention `15 days`.

## Server

The server can run with Go, Python 3, or Node.js. Python and Node.js versions use only standard libraries, so no `pip install` or `npm install` is required.

All server implementations use `server.config.json` and `index.html`.

```bash
# Go
go run .

# Python 3
python3 server.py

# Node.js
node server.js
```

By default, the server listens on `:8282`, which means all network interfaces on port `8282`. Uploaded files are stored in `uploads/`.

To listen only on localhost, set:

```json
{
  "addr": "127.0.0.1:8282"
}
```

## CLI Binaries

Prebuilt CLI binaries can be placed anywhere on your `PATH`.

- macOS Apple Silicon: `dist/copy2server-darwin-arm64`
- macOS Intel: `dist/copy2server-darwin-amd64`
- Windows x64: `dist/copy2server-windows-amd64.exe`
- Windows ARM64: `dist/copy2server-windows-arm64.exe`

macOS example:

```bash
chmod +x dist/copy2server-darwin-arm64
./dist/copy2server-darwin-arm64 --server http://127.0.0.1:8282 ./image.png
```

Windows example:

```powershell
.\dist\copy2server-windows-amd64.exe --server http://127.0.0.1:8282 .\image.png
```

You can rename the binary to `copy2server` or `copy2server.exe` for shorter commands.

## CLI Usage

```bash
# Upload readable clipboard content and copy the returned path back to clipboard
copy2server

# Upload one file
copy2server ./image.png

# Upload multiple files
copy2server image.png notes.txt

# Upload with an explicit server URL
copy2server --server http://127.0.0.1:8282 ./image.png

# Print the returned path without changing the clipboard
copy2server --no-copy ./image.png

# Set a filename for clipboard-derived uploads
copy2server --name note.txt
```

For development:

```bash
go run ./cmd/copy2server --no-copy ./image.png
```

CLI server URL priority:

1. `--server`
2. `COPY2SERVER_URL`
3. `client.config.json` field `serverUrl`
4. `http://127.0.0.1:8282`

## Global Hotkeys

The best way to use the CLI is to bind it to a global hotkey.

Recommended pattern:

1. Copy a file, screenshot, image, or text.
2. Press your configured hotkey.
3. Paste the server path that was written back to the clipboard.

Suggested tools:

- macOS: Shortcuts, Automator, Raycast, Alfred, Keyboard Maestro.
- Windows: AutoHotkey, PowerToys, Windows shortcut hotkeys.
- Linux: desktop environment keyboard shortcuts, `sxhkd`, custom scripts.

Example AutoHotkey binding on Windows:

```ahk
^!u::RunWait, D:\Tools\copy2server.exe,, Hide
```

Example macOS shell command for a shortcut:

```bash
/usr/local/bin/copy2server
```

## Clipboard Support

Clipboard formats depend on what each operating system exposes to command-line apps.

Supported paths:

- Plain text.
- Explicit file paths passed as CLI arguments.
- File references copied from Finder or Windows Explorer.
- Screenshots and clipboard images.
- Some rich text and binary clipboard formats when the platform exposes readable data.

Platform notes:

- macOS: uses `pbpaste`/`pbcopy` for text, `osascript`/AppKit for Finder files and screenshots, and `pngpaste` as an optional image fallback.
- Windows: reads Explorer-copied files via native Win32 `CF_HDROP`, writes returned paths via native `CF_UNICODETEXT`, and uses Windows clipboard APIs for screenshots.
- Linux: uses available desktop clipboard tools such as `wl-paste`, `wl-copy`, `xclip`, or `xsel`.

## Configuration

Server configuration lives in `server.config.json`:

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

CLI configuration lives in `client.config.json`:

```json
{
  "serverUrl": "http://127.0.0.1:8282"
}
```

`addr` is where the server listens. `serverUrl` is where the CLI sends upload requests.

For remote use, the server can usually keep:

```json
{
  "addr": ":8282"
}
```

Then configure the client with the actual server address:

```json
{
  "serverUrl": "http://192.168.1.10:8282"
}
```

Environment variables:

- `CONFIG`: server config path, default `server.config.json`
- `ADDR`: server listen address, for example `:8282` or `127.0.0.1:8282`
- `UPLOAD_DIR`: upload storage directory
- `MAX_UPLOAD_MB`: per-request upload limit in MB
- `MAX_STORAGE_GB`: upload directory storage quota in GB, default `5`
- `RETENTION_DAYS`: retention period in days, default `15`
- `INDEX_HTML`: web UI template path, default `index.html`
- `COPY2SERVER_CONFIG`: CLI config path, default `client.config.json`
- `COPY2SERVER_URL`: CLI server URL override

Examples:

```bash
ADDR=:8282 UPLOAD_DIR=uploads MAX_UPLOAD_MB=512 MAX_STORAGE_GB=5 RETENTION_DAYS=15 python3 server.py
COPY2SERVER_URL=http://192.168.1.10:8282 copy2server ./image.png
```

## Cleanup Behavior

Cleanup runs after successful upload writes. It does not run just because the server starts or sits idle.

Cleanup order:

1. Delete ordinary files older than `RETENTION_DAYS`.
2. If total upload storage is still above `MAX_STORAGE_GB`, delete ordinary files from oldest to newest until the directory is under the quota.

A single uploaded file larger than the storage quota is rejected.

## Build CLI

```bash
mkdir -p dist
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o dist/copy2server-darwin-arm64 ./cmd/copy2server
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o dist/copy2server-darwin-amd64 ./cmd/copy2server
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o dist/copy2server-windows-amd64.exe ./cmd/copy2server
CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -o dist/copy2server-windows-arm64.exe ./cmd/copy2server
```

## OpenSpec

This repository uses OpenSpec to manage product and engineering changes.

Important paths:

- `openspec/config.yaml`: project context and workflow rules.
- `openspec/specs/`: accepted system specifications.
- `openspec/changes/`: proposed and in-progress changes.
- `.agents/skills/`: Codex OpenSpec workflow skills generated by `openspec init --tools codex`.

Useful commands:

```bash
openspec new change <change-name>
openspec status --change <change-name>
openspec validate --all --strict
```

## Scope

copy2server is intentionally not a full cloud drive. It is a small utility for quickly moving local content to a server and getting a reusable server path.

Best fit:

- Personal or team internal tools.
- Local network sharing.
- Quick screenshot/file/text handoff.
- Scripted upload workflows.

If exposed beyond a trusted network, add authentication and network-level access control.
