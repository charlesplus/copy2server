#!/usr/bin/env python3
import json
import mimetypes
import os
import posixpath
import secrets
import shutil
import string
import sys
import threading
import time
from datetime import datetime, timezone
from email import policy
from email.parser import BytesParser
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import quote, unquote, urlparse

DEFAULT_CONFIG = {
    "addr": ":8282",
    "uploadDir": "uploads",
    "maxUploadMB": 512,
    "maxStorageGB": 5,
    "retentionDays": 15,
    "indexPath": "index.html",
}


def load_config():
    cfg = dict(DEFAULT_CONFIG)
    config_path = os.environ.get("CONFIG", "server.config.json").strip() or "server.config.json"
    path = Path(config_path)
    if path.exists():
        with path.open("r", encoding="utf-8") as fh:
            cfg.update(json.load(fh))

    cfg["addr"] = os.environ.get("ADDR", str(cfg.get("addr") or DEFAULT_CONFIG["addr"])).strip() or DEFAULT_CONFIG["addr"]
    cfg["uploadDir"] = os.environ.get("UPLOAD_DIR", str(cfg.get("uploadDir") or DEFAULT_CONFIG["uploadDir"])).strip() or DEFAULT_CONFIG["uploadDir"]
    cfg["indexPath"] = os.environ.get("INDEX_HTML", str(cfg.get("indexPath") or DEFAULT_CONFIG["indexPath"])).strip() or DEFAULT_CONFIG["indexPath"]
    cfg["maxUploadMB"] = env_int("MAX_UPLOAD_MB", cfg.get("maxUploadMB"), DEFAULT_CONFIG["maxUploadMB"])
    cfg["maxStorageGB"] = env_float("MAX_STORAGE_GB", cfg.get("maxStorageGB"), DEFAULT_CONFIG["maxStorageGB"])
    cfg["retentionDays"] = env_int("RETENTION_DAYS", cfg.get("retentionDays"), DEFAULT_CONFIG["retentionDays"])

    if cfg["maxUploadMB"] <= 0:
        cfg["maxUploadMB"] = DEFAULT_CONFIG["maxUploadMB"]
    if cfg["maxStorageGB"] <= 0:
        cfg["maxStorageGB"] = DEFAULT_CONFIG["maxStorageGB"]
    if cfg["retentionDays"] <= 0:
        cfg["retentionDays"] = DEFAULT_CONFIG["retentionDays"]
    return cfg


def env_int(name, value, fallback):
    raw = os.environ.get(name, value)
    try:
        return int(raw)
    except (TypeError, ValueError):
        return fallback


def env_float(name, value, fallback):
    raw = os.environ.get(name, value)
    try:
        return float(raw)
    except (TypeError, ValueError):
        return fallback


def parse_addr(addr):
    addr = str(addr or ":8282").strip()
    if addr.startswith(":"):
        return "", int(addr[1:])
    if ":" in addr:
        host, port = addr.rsplit(":", 1)
        return host, int(port)
    return "", int(addr)


class App:
    def __init__(self, cfg):
        self.cfg = cfg
        self.upload_dir = Path(cfg["uploadDir"]).resolve()
        self.upload_dir.mkdir(parents=True, exist_ok=True)
        self.max_upload = int(cfg["maxUploadMB"]) * 1024 * 1024
        self.max_storage = int(float(cfg["maxStorageGB"]) * 1024 * 1024 * 1024)
        self.retention_seconds = int(cfg["retentionDays"]) * 24 * 60 * 60
        self.index_path = Path(cfg["indexPath"])
        self.cleanup_lock = threading.Lock()

    def render_index(self):
        html = self.index_path.read_text(encoding="utf-8")
        values = {
            "{{.MaxUploadMB}}": str(self.cfg["maxUploadMB"]),
            "{{.UploadDir}}": Path(self.cfg["uploadDir"]).name or self.cfg["uploadDir"],
            "{{.RetentionDays}}": str(self.cfg["retentionDays"]),
        }
        for key, value in values.items():
            html = html.replace(key, value)
        return html.encode("utf-8")

    def list_files(self):
        files = []
        for entry in self.upload_dir.iterdir():
            if entry.is_file():
                try:
                    files.append(self.file_info(entry.name))
                except OSError as exc:
                    print(f"skip file {entry.name!r}: {exc}", file=sys.stderr)
        files.sort(key=lambda item: item["modifiedAt"], reverse=True)
        return files

    def file_info(self, name):
        name = safe_existing_name(name)
        path = self.upload_dir / name
        stat = path.stat()
        mime_type, _ = mimetypes.guess_type(name)
        return {
            "name": name,
            "size": stat.st_size,
            "modifiedAt": datetime.fromtimestamp(stat.st_mtime, timezone.utc).isoformat().replace("+00:00", "Z"),
            "serverPath": str(path),
            "downloadUrl": "/download/" + quote(name),
            "isImage": bool(mime_type and mime_type.startswith("image/")),
        }

    def schedule_cleanup(self):
        if not self.cleanup_lock.acquire(blocking=False):
            return
        thread = threading.Thread(target=self._cleanup_worker, daemon=True)
        thread.start()

    def _cleanup_worker(self):
        try:
            self.cleanup_files()
        finally:
            self.cleanup_lock.release()

    def cleanup_files(self):
        cutoff = time.time() - self.retention_seconds
        for file in self.cleanup_file_list():
            if file["mtime"] < cutoff:
                try:
                    file["path"].unlink()
                except FileNotFoundError:
                    pass
                except OSError as exc:
                    print(f"cleanup remove {file['path']}: {exc}", file=sys.stderr)

        files = self.cleanup_file_list()
        total = sum(file["size"] for file in files)
        if total <= self.max_storage:
            return
        files.sort(key=lambda file: file["mtime"])
        for file in files:
            if total <= self.max_storage:
                break
            try:
                file["path"].unlink()
                total -= file["size"]
            except FileNotFoundError:
                total -= file["size"]
            except OSError as exc:
                print(f"cleanup remove {file['path']}: {exc}", file=sys.stderr)

    def cleanup_file_list(self):
        files = []
        for entry in self.upload_dir.iterdir():
            try:
                if not entry.is_file():
                    continue
                stat = entry.stat()
            except OSError as exc:
                print(f"cleanup stat {entry}: {exc}", file=sys.stderr)
                continue
            files.append({"path": entry, "size": stat.st_size, "mtime": stat.st_mtime})
        return files


class Handler(BaseHTTPRequestHandler):
    server_version = "copy2server-python/1.0"

    @property
    def app(self):
        return self.server.app

    def do_GET(self):
        path = urlparse(self.path).path
        if path == "/":
            self.write_bytes(HTTPStatus.OK, self.app.render_index(), "text/html; charset=utf-8")
            return
        if path == "/api/files":
            self.write_json(HTTPStatus.OK, {"files": self.app.list_files()})
            return
        if path.startswith("/download/"):
            self.handle_download(path)
            return
        self.send_error(HTTPStatus.NOT_FOUND)

    def do_POST(self):
        path = urlparse(self.path).path
        if path != "/api/upload":
            self.send_error(HTTPStatus.NOT_FOUND)
            return
        self.handle_upload()

    def handle_upload(self):
        content_type = self.headers.get("Content-Type", "")
        if "multipart/form-data" not in content_type:
            self.write_error(HTTPStatus.BAD_REQUEST, "请使用 multipart/form-data 上传文件")
            return

        try:
            length = int(self.headers.get("Content-Length", "0"))
        except ValueError:
            length = 0
        if length <= 0:
            self.write_error(HTTPStatus.BAD_REQUEST, "没有找到可上传的文件")
            return
        if length > self.app.max_upload:
            self.write_error(HTTPStatus.REQUEST_ENTITY_TOO_LARGE, "上传文件超过大小限制")
            return

        body = self.rfile.read(length)
        msg = BytesParser(policy=policy.default).parsebytes(
            b"Content-Type: " + content_type.encode("utf-8") + b"\r\nMIME-Version: 1.0\r\n\r\n" + body
        )
        if not msg.is_multipart():
            self.write_error(HTTPStatus.BAD_REQUEST, "请使用 multipart/form-data 上传文件")
            return

        files = []
        saved_paths = []
        try:
            for part in msg.iter_parts():
                if part.get_content_disposition() != "form-data":
                    continue
                if part.get_param("name", header="content-disposition") != "file":
                    continue
                filename = part.get_filename() or filename_from_content_type(part.get_content_type())
                payload = part.get_payload(decode=True)
                if payload is None:
                    payload = str(part.get_payload()).encode("utf-8")
                if len(payload) > self.app.max_storage:
                    self.write_error(HTTPStatus.REQUEST_ENTITY_TOO_LARGE, "上传文件超过存储上限")
                    return
                saved_name = unique_filename(filename)
                dst = self.app.upload_dir / saved_name
                with dst.open("xb") as fh:
                    fh.write(payload)
                saved_paths.append(dst)
                files.append(self.app.file_info(saved_name))
        except OSError:
            for path in saved_paths:
                try:
                    path.unlink()
                except OSError:
                    pass
            self.write_error(HTTPStatus.INTERNAL_SERVER_ERROR, "保存文件失败")
            return

        if not files:
            self.write_error(HTTPStatus.BAD_REQUEST, "没有找到可上传的文件")
            return
        self.app.schedule_cleanup()
        self.write_json(HTTPStatus.CREATED, {"files": files})

    def handle_download(self, path):
        try:
            name = safe_existing_name(unquote(path.removeprefix("/download/")))
        except ValueError:
            self.send_error(HTTPStatus.NOT_FOUND)
            return
        file_path = self.app.upload_dir / name
        if not file_path.is_file():
            self.send_error(HTTPStatus.NOT_FOUND)
            return
        self.send_response(HTTPStatus.OK)
        self.send_header("Content-Type", mimetypes.guess_type(name)[0] or "application/octet-stream")
        self.send_header("Content-Length", str(file_path.stat().st_size))
        self.send_header("Content-Disposition", f"attachment; filename={quote(name)}")
        self.end_headers()
        with file_path.open("rb") as fh:
            shutil.copyfileobj(fh, self.wfile)

    def write_json(self, status, value):
        self.write_bytes(status, json.dumps(value, ensure_ascii=False).encode("utf-8"), "application/json; charset=utf-8")

    def write_error(self, status, message):
        self.write_json(status, {"error": message})

    def write_bytes(self, status, payload, content_type):
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def log_message(self, fmt, *args):
        print(f"{self.address_string()} - {fmt % args}")


def unique_filename(original):
    base = sanitize_filename(original)
    stem, ext = os.path.splitext(base)
    stem = stem or "file"
    stamp = datetime.now(timezone.utc).strftime("%Y%m%d-%H%M%S")
    return f"{stamp}-{secrets.token_hex(4)}-{stem}{ext}"


def sanitize_filename(name):
    name = posixpath.basename(str(name or "").strip())
    ext = sanitize_extension(os.path.splitext(name)[1])
    stem = os.path.splitext(name)[0].replace(" ", "-")
    allowed = string.ascii_letters + string.digits + ".-_"
    clean = "".join(ch for ch in stem if ch in allowed).strip(".-_") or "file"
    if len(clean) + len(ext) > 120:
        clean = clean[: max(1, 120 - len(ext))]
    return clean + ext


def sanitize_extension(ext):
    if not ext.startswith(".") or len(ext) > 20:
        return ""
    clean = "".join(ch for ch in ext[1:] if ch in string.ascii_letters + string.digits).lower()
    return "." + clean if clean else ""


def safe_existing_name(name):
    clean = posixpath.basename(str(name or "").strip())
    if clean in ("", ".", "/") or "/" in clean or "\\" in clean:
        raise ValueError("invalid filename")
    return clean


def filename_from_content_type(content_type):
    ext = mimetypes.guess_extension(content_type or "") or ""
    return "copy2server" + ext


def main():
    cfg = load_config()
    app = App(cfg)
    host, port = parse_addr(cfg["addr"])
    server = ThreadingHTTPServer((host, port), Handler)
    server.app = app
    print(f"copy2server listening on {cfg['addr']}")
    print(f"uploads: {app.upload_dir}")
    server.serve_forever()


if __name__ == "__main__":
    main()
