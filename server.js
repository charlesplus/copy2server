#!/usr/bin/env node
const crypto = require('crypto');
const fs = require('fs');
const http = require('http');
const path = require('path');
const { URL } = require('url');

const DEFAULT_CONFIG = {
  addr: ':8282',
  uploadDir: 'uploads',
  maxUploadMB: 512,
  maxStorageGB: 5,
  retentionDays: 15,
  indexPath: 'index.html'
};

const MIME_BY_EXT = {
  '.apng': 'image/apng',
  '.avif': 'image/avif',
  '.gif': 'image/gif',
  '.htm': 'text/html; charset=utf-8',
  '.html': 'text/html; charset=utf-8',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.json': 'application/json; charset=utf-8',
  '.pdf': 'application/pdf',
  '.png': 'image/png',
  '.svg': 'image/svg+xml',
  '.txt': 'text/plain; charset=utf-8',
  '.webp': 'image/webp',
  '.zip': 'application/zip'
};

function loadConfig() {
  const cfg = { ...DEFAULT_CONFIG };
  const configPath = (process.env.CONFIG || 'server.config.json').trim() || 'server.config.json';
  if (fs.existsSync(configPath)) {
    Object.assign(cfg, JSON.parse(fs.readFileSync(configPath, 'utf8')));
  }

  cfg.addr = (process.env.ADDR || cfg.addr || DEFAULT_CONFIG.addr).trim() || DEFAULT_CONFIG.addr;
  cfg.uploadDir = (process.env.UPLOAD_DIR || cfg.uploadDir || DEFAULT_CONFIG.uploadDir).trim() || DEFAULT_CONFIG.uploadDir;
  cfg.indexPath = (process.env.INDEX_HTML || cfg.indexPath || DEFAULT_CONFIG.indexPath).trim() || DEFAULT_CONFIG.indexPath;
  cfg.maxUploadMB = envInt('MAX_UPLOAD_MB', cfg.maxUploadMB, DEFAULT_CONFIG.maxUploadMB);
  cfg.maxStorageGB = envFloat('MAX_STORAGE_GB', cfg.maxStorageGB, DEFAULT_CONFIG.maxStorageGB);
  cfg.retentionDays = envInt('RETENTION_DAYS', cfg.retentionDays, DEFAULT_CONFIG.retentionDays);

  if (cfg.maxUploadMB <= 0) cfg.maxUploadMB = DEFAULT_CONFIG.maxUploadMB;
  if (cfg.maxStorageGB <= 0) cfg.maxStorageGB = DEFAULT_CONFIG.maxStorageGB;
  if (cfg.retentionDays <= 0) cfg.retentionDays = DEFAULT_CONFIG.retentionDays;
  return cfg;
}

function envInt(name, value, fallback) {
  const parsed = Number.parseInt(process.env[name] ?? value, 10);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function envFloat(name, value, fallback) {
  const parsed = Number.parseFloat(process.env[name] ?? value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function parseAddr(addr) {
  addr = String(addr || ':8282').trim();
  if (addr.startsWith(':')) return { host: '0.0.0.0', port: Number.parseInt(addr.slice(1), 10) };
  const index = addr.lastIndexOf(':');
  if (index !== -1) return { host: addr.slice(0, index), port: Number.parseInt(addr.slice(index + 1), 10) };
  return { host: '0.0.0.0', port: Number.parseInt(addr, 10) };
}

class App {
  constructor(cfg) {
    this.cfg = cfg;
    this.uploadDirAbs = path.resolve(cfg.uploadDir);
    this.maxUpload = Number(cfg.maxUploadMB) * 1024 * 1024;
    this.maxStorage = Number(cfg.maxStorageGB) * 1024 * 1024 * 1024;
    this.retentionMs = Number(cfg.retentionDays) * 24 * 60 * 60 * 1000;
    this.cleanupRunning = false;
    fs.mkdirSync(this.uploadDirAbs, { recursive: true });
  }

  renderIndex() {
    return fs.readFileSync(this.cfg.indexPath, 'utf8')
      .replaceAll('{{.MaxUploadMB}}', String(this.cfg.maxUploadMB))
      .replaceAll('{{.UploadDir}}', path.basename(this.cfg.uploadDir) || this.cfg.uploadDir)
      .replaceAll('{{.RetentionDays}}', String(this.cfg.retentionDays));
  }

  listFiles() {
    return fs.readdirSync(this.uploadDirAbs, { withFileTypes: true })
      .filter((entry) => entry.isFile())
      .map((entry) => this.fileInfo(entry.name))
      .sort((a, b) => b.modifiedAt.localeCompare(a.modifiedAt));
  }

  fileInfo(name) {
    name = safeExistingName(name);
    const filePath = path.join(this.uploadDirAbs, name);
    const stat = fs.statSync(filePath);
    const type = mimeType(name);
    return {
      name,
      size: stat.size,
      modifiedAt: stat.mtime.toISOString(),
      serverPath: filePath,
      downloadUrl: '/download/' + encodeURIComponent(name),
      isImage: type.startsWith('image/')
    };
  }

  scheduleCleanup() {
    if (this.cleanupRunning) return;
    this.cleanupRunning = true;
    setImmediate(() => {
      try {
        this.cleanupFiles();
      } catch (error) {
        console.error(`cleanup failed: ${error.message}`);
      } finally {
        this.cleanupRunning = false;
      }
    });
  }

  cleanupFiles() {
    const cutoff = Date.now() - this.retentionMs;
    for (const file of this.cleanupFileList()) {
      if (file.mtimeMs < cutoff) {
        try {
          fs.unlinkSync(file.path);
        } catch (error) {
          if (error.code !== 'ENOENT') console.error(`cleanup remove ${file.path}: ${error.message}`);
        }
      }
    }

    const files = this.cleanupFileList();
    let total = files.reduce((sum, file) => sum + file.size, 0);
    if (total <= this.maxStorage) return;
    files.sort((a, b) => a.mtimeMs - b.mtimeMs);
    for (const file of files) {
      if (total <= this.maxStorage) break;
      try {
        fs.unlinkSync(file.path);
        total -= file.size;
      } catch (error) {
        if (error.code === 'ENOENT') total -= file.size;
        else console.error(`cleanup remove ${file.path}: ${error.message}`);
      }
    }
  }

  cleanupFileList() {
    const files = [];
    for (const entry of fs.readdirSync(this.uploadDirAbs, { withFileTypes: true })) {
      if (!entry.isFile()) continue;
      const filePath = path.join(this.uploadDirAbs, entry.name);
      try {
        const stat = fs.statSync(filePath);
        files.push({ path: filePath, size: stat.size, mtimeMs: stat.mtimeMs });
      } catch (error) {
        console.error(`cleanup stat ${filePath}: ${error.message}`);
      }
    }
    return files;
  }
}

const cfg = loadConfig();
const app = new App(cfg);

const server = http.createServer((req, res) => {
  const requestUrl = new URL(req.url, 'http://127.0.0.1');
  if (req.method === 'GET' && requestUrl.pathname === '/') {
    send(res, 200, app.renderIndex(), 'text/html; charset=utf-8');
    return;
  }
  if (req.method === 'GET' && requestUrl.pathname === '/api/files') {
    sendJson(res, 200, { files: app.listFiles() });
    return;
  }
  if (req.method === 'GET' && requestUrl.pathname.startsWith('/download/')) {
    handleDownload(res, decodeURIComponent(requestUrl.pathname.slice('/download/'.length)));
    return;
  }
  if (req.method === 'POST' && requestUrl.pathname === '/api/upload') {
    handleUpload(req, res);
    return;
  }
  send(res, 404, 'not found', 'text/plain; charset=utf-8');
});

function handleUpload(req, res) {
  const contentType = req.headers['content-type'] || '';
  if (!contentType.includes('multipart/form-data')) {
    sendError(res, 400, '请使用 multipart/form-data 上传文件');
    req.resume();
    return;
  }

  const length = Number.parseInt(req.headers['content-length'] || '0', 10);
  if (!length) {
    sendError(res, 400, '没有找到可上传的文件');
    req.resume();
    return;
  }
  if (length > app.maxUpload) {
    sendError(res, 413, '上传文件超过大小限制');
    req.resume();
    return;
  }

  const chunks = [];
  let total = 0;
  let rejected = false;
  req.on('data', (chunk) => {
    total += chunk.length;
    if (total > app.maxUpload) {
      rejected = true;
      sendError(res, 413, '上传文件超过大小限制');
      req.destroy();
      return;
    }
    chunks.push(chunk);
  });
  req.on('end', () => {
    if (rejected) return;
    const savedPaths = [];
    try {
      const parts = parseMultipart(Buffer.concat(chunks), contentType);
      const files = [];
      for (const part of parts) {
        if (part.name !== 'file') continue;
        if (part.data.length > app.maxStorage) {
          sendError(res, 413, '上传文件超过存储上限');
          return;
        }
        const filename = part.filename || filenameFromContentType(part.contentType);
        const savedName = uniqueFilename(filename);
        const dst = path.join(app.uploadDirAbs, savedName);
        fs.writeFileSync(dst, part.data, { flag: 'wx' });
        savedPaths.push(dst);
        files.push(app.fileInfo(savedName));
      }
      if (!files.length) {
        sendError(res, 400, '没有找到可上传的文件');
        return;
      }
      app.scheduleCleanup();
      sendJson(res, 201, { files });
    } catch (error) {
      for (const filePath of savedPaths) {
        try { fs.unlinkSync(filePath); } catch (_) {}
      }
      sendError(res, 400, error.message || '读取上传数据失败');
    }
  });
  req.on('error', () => {
    if (!res.headersSent) sendError(res, 400, '读取上传数据失败');
  });
}

function handleDownload(res, rawName) {
  let name;
  try {
    name = safeExistingName(rawName);
  } catch (_) {
    send(res, 404, 'not found', 'text/plain; charset=utf-8');
    return;
  }
  const filePath = path.join(app.uploadDirAbs, name);
  if (!filePath.startsWith(app.uploadDirAbs + path.sep) || !fs.existsSync(filePath) || !fs.statSync(filePath).isFile()) {
    send(res, 404, 'not found', 'text/plain; charset=utf-8');
    return;
  }
  res.writeHead(200, {
    'Content-Type': mimeType(name),
    'Content-Length': fs.statSync(filePath).size,
    'Content-Disposition': `attachment; filename*=UTF-8''${encodeURIComponent(name)}`
  });
  fs.createReadStream(filePath).pipe(res);
}

function parseMultipart(body, contentType) {
  const match = contentType.match(/boundary=(?:"([^"]+)"|([^;]+))/i);
  if (!match) throw new Error('请使用 multipart/form-data 上传文件');
  const boundary = Buffer.from('--' + (match[1] || match[2]));
  const parts = [];
  let pos = body.indexOf(boundary);
  if (pos === -1) return parts;

  while (pos !== -1) {
    pos += boundary.length;
    if (body.subarray(pos, pos + 2).toString() === '--') break;
    if (body.subarray(pos, pos + 2).toString() === '\r\n') pos += 2;

    const headerEnd = body.indexOf(Buffer.from('\r\n\r\n'), pos);
    if (headerEnd === -1) break;
    const headers = parseHeaders(body.subarray(pos, headerEnd).toString('utf8'));
    const dataStart = headerEnd + 4;
    const next = body.indexOf(Buffer.from('\r\n--' + (match[1] || match[2])), dataStart);
    if (next === -1) break;

    const disposition = headers['content-disposition'] || '';
    const params = parseDispositionParams(disposition);
    parts.push({
      name: params.name || '',
      filename: params.filename || '',
      contentType: headers['content-type'] || 'application/octet-stream',
      data: body.subarray(dataStart, next)
    });
    pos = next + 2;
  }
  return parts;
}

function parseHeaders(text) {
  const headers = {};
  for (const line of text.split('\r\n')) {
    const index = line.indexOf(':');
    if (index === -1) continue;
    headers[line.slice(0, index).trim().toLowerCase()] = line.slice(index + 1).trim();
  }
  return headers;
}

function parseDispositionParams(value) {
  const params = {};
  for (const part of value.split(';').slice(1)) {
    const index = part.indexOf('=');
    if (index === -1) continue;
    const key = part.slice(0, index).trim().toLowerCase();
    let val = part.slice(index + 1).trim();
    if (val.startsWith('"') && val.endsWith('"')) val = val.slice(1, -1).replace(/\\"/g, '"');
    params[key] = val;
  }
  return params;
}

function uniqueFilename(original) {
  const base = sanitizeFilename(original);
  const ext = path.extname(base);
  const stem = path.basename(base, ext) || 'file';
  const stamp = new Date().toISOString().replace(/[-:]/g, '').slice(0, 15).replace('T', '-');
  return `${stamp}-${crypto.randomBytes(4).toString('hex')}-${stem}${ext}`;
}

function sanitizeFilename(name) {
  name = path.posix.basename(String(name || '').trim());
  const rawExt = path.extname(name);
  const ext = sanitizeExtension(rawExt);
  const stem = path.basename(name, rawExt).replaceAll(' ', '-');
  let clean = '';
  for (const ch of stem) {
    if (/^[A-Za-z0-9._-]$/.test(ch)) clean += ch;
  }
  clean = clean.replace(/^[._-]+|[._-]+$/g, '') || 'file';
  if (clean.length + ext.length > 120) clean = clean.slice(0, Math.max(1, 120 - ext.length));
  return clean + ext;
}

function sanitizeExtension(ext) {
  if (!ext.startsWith('.') || ext.length > 20) return '';
  const clean = ext.slice(1).replace(/[^A-Za-z0-9]/g, '').toLowerCase();
  return clean ? '.' + clean : '';
}

function safeExistingName(name) {
  const clean = path.posix.basename(String(name || '').trim());
  if (!clean || clean === '.' || clean === '/' || clean.includes('/') || clean.includes('\\')) {
    throw new Error('invalid filename');
  }
  return clean;
}

function filenameFromContentType(contentType) {
  const ext = {
    'image/png': '.png',
    'image/jpeg': '.jpg',
    'image/gif': '.gif',
    'image/webp': '.webp'
  }[contentType] || '';
  return 'copy2server' + ext;
}

function mimeType(name) {
  return MIME_BY_EXT[path.extname(name).toLowerCase()] || 'application/octet-stream';
}

function sendJson(res, status, value) {
  send(res, status, JSON.stringify(value), 'application/json; charset=utf-8');
}

function sendError(res, status, message) {
  sendJson(res, status, { error: message });
}

function send(res, status, body, contentType) {
  const payload = Buffer.isBuffer(body) ? body : Buffer.from(String(body));
  res.writeHead(status, { 'Content-Type': contentType, 'Content-Length': payload.length });
  res.end(payload);
}

const { host, port } = parseAddr(cfg.addr);
server.listen(port, host, () => {
  console.log(`copy2server listening on ${cfg.addr}`);
  console.log(`uploads: ${app.uploadDirAbs}`);
});
