package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultAddr          = ":8282"
	defaultConfigPath    = "server.config.json"
	defaultUploadDir     = "uploads"
	defaultMaxUploadMB   = int64(512)
	defaultMaxStorageGB  = float64(5)
	defaultRetentionDays = 15
	defaultIndexPath     = "index.html"
)

type config struct {
	Addr          string  `json:"addr"`
	UploadDir     string  `json:"uploadDir"`
	MaxUploadMB   int64   `json:"maxUploadMB"`
	MaxStorageGB  float64 `json:"maxStorageGB"`
	RetentionDays int     `json:"retentionDays"`
	IndexPath     string  `json:"indexPath"`
}

type app struct {
	cfg          config
	uploadDirAbs string
	maxUpload    int64
	maxStorage   int64
	retention    time.Duration
	index        *template.Template
	cleanupMu    sync.Mutex
}

type uploadedFile struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	ModifiedAt  string `json:"modifiedAt"`
	ServerPath  string `json:"serverPath"`
	DownloadURL string `json:"downloadUrl"`
	IsImage     bool   `json:"isImage"`
}

type uploadResponse struct {
	Files []uploadedFile `json:"files"`
}

type cleanupFile struct {
	name    string
	path    string
	size    int64
	modTime time.Time
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	uploadDirAbs, err := filepath.Abs(cfg.UploadDir)
	if err != nil {
		log.Fatalf("resolve upload dir: %v", err)
	}
	if err := os.MkdirAll(uploadDirAbs, 0755); err != nil {
		log.Fatalf("create upload dir: %v", err)
	}

	index, err := template.ParseFiles(cfg.IndexPath)
	if err != nil {
		log.Fatalf("load index template %q: %v", cfg.IndexPath, err)
	}

	a := &app{
		cfg:          cfg,
		uploadDirAbs: uploadDirAbs,
		maxUpload:    cfg.MaxUploadMB * 1024 * 1024,
		maxStorage:   int64(cfg.MaxStorageGB * 1024 * 1024 * 1024),
		retention:    time.Duration(cfg.RetentionDays) * 24 * time.Hour,
		index:        index,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", a.handleIndex)
	mux.HandleFunc("GET /api/files", a.handleFiles)
	mux.HandleFunc("POST /api/upload", a.handleUpload)
	mux.HandleFunc("GET /download/", a.handleDownload)

	log.Printf("copy2server listening on %s", cfg.Addr)
	log.Printf("uploads: %s", uploadDirAbs)
	log.Fatal(http.ListenAndServe(cfg.Addr, mux))
}

func loadConfig() (config, error) {
	cfg := defaultConfig()

	configPath := env("CONFIG", defaultConfigPath)
	data, err := os.ReadFile(configPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return cfg, err
	}
	if err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, err
		}
	}

	cfg.Addr = env("ADDR", cfg.Addr)
	cfg.UploadDir = env("UPLOAD_DIR", cfg.UploadDir)
	cfg.IndexPath = env("INDEX_HTML", cfg.IndexPath)
	cfg.MaxUploadMB = envInt64("MAX_UPLOAD_MB", cfg.MaxUploadMB)
	cfg.MaxStorageGB = envFloat64("MAX_STORAGE_GB", cfg.MaxStorageGB)
	cfg.RetentionDays = int(envInt64("RETENTION_DAYS", int64(cfg.RetentionDays)))

	return normalizeConfig(cfg), nil
}

func defaultConfig() config {
	return config{
		Addr:          defaultAddr,
		UploadDir:     defaultUploadDir,
		MaxUploadMB:   defaultMaxUploadMB,
		MaxStorageGB:  defaultMaxStorageGB,
		RetentionDays: defaultRetentionDays,
		IndexPath:     defaultIndexPath,
	}
}

func normalizeConfig(cfg config) config {
	if strings.TrimSpace(cfg.Addr) == "" {
		cfg.Addr = defaultAddr
	}
	if strings.TrimSpace(cfg.UploadDir) == "" {
		cfg.UploadDir = defaultUploadDir
	}
	if strings.TrimSpace(cfg.IndexPath) == "" {
		cfg.IndexPath = defaultIndexPath
	}
	if cfg.MaxUploadMB <= 0 {
		cfg.MaxUploadMB = defaultMaxUploadMB
	}
	if cfg.MaxStorageGB <= 0 {
		cfg.MaxStorageGB = defaultMaxStorageGB
	}
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = defaultRetentionDays
	}
	return cfg
}

func (a *app) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := struct {
		MaxUploadMB   int64
		UploadDir     string
		RetentionDays int
	}{
		MaxUploadMB:   a.cfg.MaxUploadMB,
		UploadDir:     filepath.Base(a.cfg.UploadDir),
		RetentionDays: a.cfg.RetentionDays,
	}
	if err := a.index.Execute(w, data); err != nil {
		log.Printf("render index: %v", err)
	}
}

func (a *app) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, a.maxUpload)
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "请使用 multipart/form-data 上传文件")
		return
	}

	var files []uploadedFile
	buf := make([]byte, 64*1024)
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "读取上传数据失败")
			return
		}
		if part.FormName() != "file" {
			part.Close()
			continue
		}

		filename := part.FileName()
		if filename == "" {
			filename = filenameFromContentType(part.Header.Get("Content-Type"))
		}
		savedName, err := uniqueFilename(filename)
		if err != nil {
			part.Close()
			writeError(w, http.StatusInternalServerError, "生成文件名失败")
			return
		}
		dstPath := filepath.Join(a.uploadDirAbs, savedName)
		dst, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			part.Close()
			writeError(w, http.StatusInternalServerError, "创建文件失败")
			return
		}

		written, copyErr := copyWithLimit(dst, part, buf, a.maxStorage)
		closeErr := dst.Close()
		part.Close()
		if copyErr != nil || closeErr != nil {
			_ = os.Remove(dstPath)
			if errors.Is(copyErr, errFileExceedsStorageQuota) || written > a.maxStorage {
				writeError(w, http.StatusRequestEntityTooLarge, "上传文件超过存储上限")
				return
			}
			writeError(w, http.StatusInternalServerError, "保存文件失败")
			return
		}

		file, err := a.fileInfo(savedName)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "读取文件信息失败")
			return
		}
		files = append(files, file)
	}

	if len(files) == 0 {
		writeError(w, http.StatusBadRequest, "没有找到可上传的文件")
		return
	}
	a.scheduleCleanup()
	writeJSON(w, http.StatusCreated, uploadResponse{Files: files})
}

var errFileExceedsStorageQuota = errors.New("file exceeds storage quota")

func copyWithLimit(dst io.Writer, src io.Reader, buf []byte, maxBytes int64) (int64, error) {
	var written int64
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			written += int64(nr)
			if written > maxBytes {
				return written, errFileExceedsStorageQuota
			}
			nw, ew := dst.Write(buf[:nr])
			if ew != nil {
				return written, ew
			}
			if nw != nr {
				return written, io.ErrShortWrite
			}
		}
		if er != nil {
			if errors.Is(er, io.EOF) {
				return written, nil
			}
			return written, er
		}
	}
}

func (a *app) handleFiles(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(a.uploadDirAbs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取文件列表失败")
		return
	}

	files := make([]uploadedFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		file, err := a.fileInfo(entry.Name())
		if err != nil {
			log.Printf("skip file %q: %v", entry.Name(), err)
			continue
		}
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModifiedAt > files[j].ModifiedAt
	})

	writeJSON(w, http.StatusOK, struct {
		Files []uploadedFile `json:"files"`
	}{Files: files})
}

func (a *app) handleDownload(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/download/")
	name, err := safeExistingName(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	path := filepath.Join(a.uploadDirAbs, name)
	if !strings.HasPrefix(path, a.uploadDirAbs+string(os.PathSeparator)) {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, path)
}

func (a *app) fileInfo(name string) (uploadedFile, error) {
	name, err := safeExistingName(name)
	if err != nil {
		return uploadedFile{}, err
	}
	path := filepath.Join(a.uploadDirAbs, name)
	info, err := os.Stat(path)
	if err != nil {
		return uploadedFile{}, err
	}
	if info.IsDir() {
		return uploadedFile{}, fmt.Errorf("%s is directory", name)
	}

	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(name)))
	return uploadedFile{
		Name:        name,
		Size:        info.Size(),
		ModifiedAt:  info.ModTime().UTC().Format(time.RFC3339),
		ServerPath:  path,
		DownloadURL: "/download/" + url.PathEscape(name),
		IsImage:     strings.HasPrefix(contentType, "image/"),
	}, nil
}

func (a *app) scheduleCleanup() {
	if !a.cleanupMu.TryLock() {
		return
	}
	go func() {
		defer a.cleanupMu.Unlock()
		if err := a.cleanupFiles(); err != nil {
			log.Printf("cleanup failed: %v", err)
		}
	}()
}

func (a *app) cleanupFiles() error {
	cutoff := time.Now().Add(-a.retention)
	files, err := a.cleanupFileList()
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.modTime.Before(cutoff) {
			if err := os.Remove(file.path); err != nil && !errors.Is(err, os.ErrNotExist) {
				log.Printf("cleanup remove %q: %v", file.path, err)
			}
		}
	}

	files, err = a.cleanupFileList()
	if err != nil {
		return err
	}
	var total int64
	for _, file := range files {
		total += file.size
	}
	if total <= a.maxStorage {
		return nil
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})
	for _, file := range files {
		if total <= a.maxStorage {
			break
		}
		if err := os.Remove(file.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("cleanup remove %q: %v", file.path, err)
			continue
		}
		total -= file.size
	}
	return nil
}

func (a *app) cleanupFileList() ([]cleanupFile, error) {
	entries, err := os.ReadDir(a.uploadDirAbs)
	if err != nil {
		return nil, err
	}
	files := make([]cleanupFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			log.Printf("cleanup stat %q: %v", entry.Name(), err)
			continue
		}
		files = append(files, cleanupFile{
			name:    entry.Name(),
			path:    filepath.Join(a.uploadDirAbs, entry.Name()),
			size:    info.Size(),
			modTime: info.ModTime(),
		})
	}
	return files, nil
}

func uniqueFilename(original string) (string, error) {
	base := sanitizeFilename(original)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if stem == "" {
		stem = "file"
	}

	random := make([]byte, 4)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s-%s%s", time.Now().UTC().Format("20060102-150405"), hex.EncodeToString(random), stem, ext), nil
}

func sanitizeFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	ext := sanitizeExtension(filepath.Ext(name))
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	stem = strings.ReplaceAll(stem, " ", "-")

	var b strings.Builder
	for _, r := range stem {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.', r == '-', r == '_':
			b.WriteRune(r)
		}
	}

	cleanStem := strings.Trim(b.String(), ".-_")
	if cleanStem == "" {
		cleanStem = "file"
	}

	if len(cleanStem)+len(ext) > 120 {
		limit := 120 - len(ext)
		if limit < 1 {
			limit = 1
		}
		cleanStem = cleanStem[:min(len(cleanStem), limit)]
	}
	return cleanStem + ext
}

func sanitizeExtension(ext string) string {
	if len(ext) > 20 || !strings.HasPrefix(ext, ".") {
		return ""
	}
	var b strings.Builder
	b.WriteByte('.')
	for _, r := range ext[1:] {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}
	if b.Len() == 1 {
		return ""
	}
	return strings.ToLower(b.String())
}

func safeExistingName(name string) (string, error) {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == string(os.PathSeparator) || name == "" || strings.Contains(name, string(os.PathSeparator)) {
		return "", errors.New("invalid filename")
	}
	return name, nil
}

func filenameFromContentType(contentType string) string {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "file"
	}
	exts, err := mime.ExtensionsByType(mediaType)
	if err != nil || len(exts) == 0 {
		return "file"
	}
	return "copy2server" + exts[0]
}

func envInt64(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func envFloat64(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, struct {
		Error string `json:"error"`
	}{Error: message})
}
