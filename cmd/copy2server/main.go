package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	defaultServerURL    = "http://127.0.0.1:8282"
	defaultClientConfig = "client.config.json"
)

type config struct {
	ServerURL string `json:"serverUrl"`
}

type options struct {
	files   []string
	server  string
	copy    bool
	name    string
	timeout time.Duration
}

type fileFlags []string

func (f *fileFlags) String() string { return strings.Join(*f, ",") }
func (f *fileFlags) Set(value string) error {
	*f = append(*f, value)
	return nil
}

type uploadCandidate struct {
	filename    string
	contentType string
	path        string
	data        []byte
}

type uploadResponse struct {
	Files []struct {
		ServerPath string `json:"serverPath"`
	} `json:"files"`
	Error string `json:"error"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	opts, err := parseOptions(args)
	if err != nil {
		return err
	}
	serverURL, err := resolveServerURL(opts.server)
	if err != nil {
		return err
	}

	candidates, err := candidatesFromOptions(opts)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return errors.New("没有找到可上传的内容")
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.timeout)
	defer cancel()
	paths, err := upload(ctx, serverURL, candidates)
	if err != nil {
		return err
	}
	output := strings.Join(paths, "\n")
	fmt.Fprintln(stdout, output)
	if opts.copy {
		if err := writeClipboard(ctx, output); err != nil {
			return fmt.Errorf("上传成功，但写回剪贴板失败: %w", err)
		}
	}
	return nil
}

func parseOptions(args []string) (options, error) {
	opts := options{copy: true, timeout: 30 * time.Second}
	fs := flag.NewFlagSet("copy2server", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var files fileFlags
	fs.Var(&files, "file", "上传指定文件，可重复")
	fs.StringVar(&opts.server, "server", "", "copy2server 服务端 URL")
	fs.BoolVar(&opts.copy, "copy", true, "上传成功后写回剪贴板")
	fs.BoolFunc("no-copy", "上传成功后不写回剪贴板", func(string) error {
		opts.copy = false
		return nil
	})
	fs.StringVar(&opts.name, "name", "", "剪贴板内容上传时使用的文件名")
	fs.DurationVar(&opts.timeout, "timeout", 30*time.Second, "上传超时时间")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: copy2server [options] [files...]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return opts, err
		}
		return opts, fmt.Errorf("参数错误: %w", err)
	}
	opts.files = append(opts.files, files...)
	opts.files = append(opts.files, fs.Args()...)
	if opts.timeout <= 0 {
		opts.timeout = 30 * time.Second
	}
	return opts, nil
}

func resolveServerURL(flagValue string) (string, error) {
	server := strings.TrimSpace(flagValue)
	if server == "" {
		server = strings.TrimSpace(os.Getenv("COPY2SERVER_URL"))
	}
	if server == "" {
		cfg, _ := loadCLIConfig(env("COPY2SERVER_CONFIG", defaultClientConfig))
		server = strings.TrimSpace(cfg.ServerURL)
	}
	if server == "" {
		server = defaultServerURL
	}
	server = strings.TrimRight(server, "/")
	parsed, err := url.Parse(server)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("无效 server URL: %s", server)
	}
	return server, nil
}

func env(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func loadCLIConfig(path string) (config, error) {
	var cfg config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func candidatesFromOptions(opts options) ([]uploadCandidate, error) {
	if len(opts.files) > 0 {
		candidates := make([]uploadCandidate, 0, len(opts.files))
		for _, file := range opts.files {
			candidate, err := candidateFromFile(file)
			if err != nil {
				return nil, err
			}
			candidates = append(candidates, candidate)
		}
		return candidates, nil
	}
	return readClipboardCandidates(opts.name)
}

func candidateFromFile(file string) (uploadCandidate, error) {
	info, err := os.Stat(file)
	if err != nil {
		return uploadCandidate{}, fmt.Errorf("读取文件失败 %q: %w", file, err)
	}
	if info.IsDir() {
		return uploadCandidate{}, fmt.Errorf("不是可上传文件: %s", file)
	}
	return uploadCandidate{filename: filepath.Base(file), path: file}, nil
}

func upload(ctx context.Context, serverURL string, candidates []uploadCandidate) ([]string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, candidate := range candidates {
		filename := candidate.filename
		if filename == "" {
			filename = "copy2server.bin"
		}
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			return nil, err
		}
		if candidate.path != "" {
			file, err := os.Open(candidate.path)
			if err != nil {
				return nil, fmt.Errorf("打开文件失败 %q: %w", candidate.path, err)
			}
			_, copyErr := io.Copy(part, file)
			closeErr := file.Close()
			if copyErr != nil {
				return nil, copyErr
			}
			if closeErr != nil {
				return nil, closeErr
			}
			continue
		}
		if _, err := part.Write(candidate.data); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, serverURL+"/api/upload", &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var parsed uploadResponse
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return nil, fmt.Errorf("解析服务端响应失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if parsed.Error != "" {
			return nil, errors.New(parsed.Error)
		}
		return nil, fmt.Errorf("上传失败: HTTP %d", resp.StatusCode)
	}
	paths := make([]string, 0, len(parsed.Files))
	for _, file := range parsed.Files {
		if file.ServerPath != "" {
			paths = append(paths, file.ServerPath)
		}
	}
	if len(paths) == 0 {
		return nil, errors.New("服务端响应缺少 serverPath")
	}
	return paths, nil
}

func readClipboardCandidates(name string) ([]uploadCandidate, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if candidates, err := readRichClipboard(ctx, name); err == nil && len(candidates) > 0 {
		return candidates, nil
	}
	text, err := readClipboardText(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("剪贴板没有可上传内容")
	}
	if files := existingFilesFromText(text); len(files) > 0 {
		candidates := make([]uploadCandidate, 0, len(files))
		for _, file := range files {
			candidate, err := candidateFromFile(file)
			if err != nil {
				return nil, err
			}
			candidates = append(candidates, candidate)
		}
		return candidates, nil
	}
	filename := name
	if filename == "" {
		filename = "clipboard.txt"
	}
	return []uploadCandidate{{filename: filename, contentType: "text/plain; charset=utf-8", data: []byte(text)}}, nil
}

func readRichClipboard(ctx context.Context, name string) ([]uploadCandidate, error) {
	switch runtime.GOOS {
	case "darwin":
		if candidates, err := readMacClipboardFiles(ctx); err == nil && len(candidates) > 0 {
			return candidates, nil
		}
		if candidate, err := readMacClipboardImage(ctx, name); err == nil {
			return []uploadCandidate{candidate}, nil
		}
		if _, err := exec.LookPath("pngpaste"); err == nil {
			filename := name
			if filename == "" {
				filename = "clipboard.png"
			}
			data, err := runCommand(ctx, "pngpaste", "-")
			if err == nil && len(data) > 0 {
				return []uploadCandidate{{filename: filename, contentType: "image/png", data: data}}, nil
			}
		}
	case "windows":
		if candidates, err := readWindowsClipboardFiles(ctx); err == nil && len(candidates) > 0 {
			return candidates, nil
		}
		if candidate, err := readWindowsClipboardImage(ctx, name); err == nil {
			return []uploadCandidate{candidate}, nil
		}
	case "linux":
		if _, err := exec.LookPath("wl-paste"); err == nil {
			typesRaw, err := runCommand(ctx, "wl-paste", "--list-types")
			if err == nil {
				for _, mimeType := range []string{"image/png", "text/html", "text/rtf", "application/octet-stream"} {
					if !strings.Contains(string(typesRaw), mimeType) {
						continue
					}
					data, err := runCommand(ctx, "wl-paste", "--type", mimeType)
					if err == nil && len(data) > 0 {
						filename := name
						if filename == "" {
							filename = filenameForMime(mimeType)
						}
						return []uploadCandidate{{filename: filename, contentType: mimeType, data: data}}, nil
					}
				}
			}
		}
	}
	return nil, errors.New("no readable rich clipboard data")
}

func readMacClipboardFiles(ctx context.Context) ([]uploadCandidate, error) {
	if _, err := exec.LookPath("osascript"); err != nil {
		return nil, err
	}
	data, err := runCommand(ctx, "osascript", "-e", macFileURLsScript)
	if err != nil {
		return nil, err
	}
	paths := splitClipboardPaths(string(data))
	if len(paths) == 0 {
		return nil, errors.New("no file references in clipboard")
	}
	candidates := make([]uploadCandidate, 0, len(paths))
	for _, file := range paths {
		candidate, err := candidateFromFile(file)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func readMacClipboardImage(ctx context.Context, name string) (uploadCandidate, error) {
	if _, err := exec.LookPath("osascript"); err != nil {
		return uploadCandidate{}, err
	}
	tmp, err := os.CreateTemp("", "copy2server-clipboard-*.png")
	if err != nil {
		return uploadCandidate{}, err
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return uploadCandidate{}, err
	}
	defer os.Remove(tmpPath)
	if _, err := runCommand(ctx, "osascript", "-e", macImageScript, tmpPath); err != nil {
		return uploadCandidate{}, err
	}
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return uploadCandidate{}, err
	}
	if len(data) == 0 {
		return uploadCandidate{}, errors.New("no image data in clipboard")
	}
	filename := name
	if filename == "" {
		filename = "clipboard.png"
	}
	return uploadCandidate{filename: filename, contentType: "image/png", data: data}, nil
}

func splitClipboardPaths(text string) []string {
	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		paths = append(paths, line)
	}
	return paths
}

const macFileURLsScript = `
use framework "Foundation"
use framework "AppKit"
use scripting additions
set pb to current application's NSPasteboard's generalPasteboard()
set urls to pb's readObjectsForClasses:{current application's NSURL} options:(missing value)
if urls = missing value then return ""
set paths to {}
repeat with u in urls
	if (u's isFileURL()) as boolean then set end of paths to (u's |path|()) as text
end repeat
set AppleScript's text item delimiters to linefeed
return paths as text
`

const macImageScript = `
use framework "Foundation"
use framework "AppKit"
use scripting additions
on run argv
	set outPath to item 1 of argv
	set pb to current application's NSPasteboard's generalPasteboard()
	set img to current application's NSImage's alloc()'s initWithPasteboard:pb
	if img = missing value then error "no image"
	set tiffData to img's TIFFRepresentation()
	if tiffData = missing value then error "no tiff data"
	set bitmap to current application's NSBitmapImageRep's imageRepWithData:tiffData
	if bitmap = missing value then error "no bitmap data"
	set pngData to bitmap's representationUsingType:4 |properties|:(current application's NSDictionary's dictionary())
	if pngData = missing value then error "no png data"
	pngData's writeToFile:outPath atomically:true
end run
`

func readWindowsClipboardImage(ctx context.Context, name string) (uploadCandidate, error) {
	tmp, err := os.CreateTemp("", "copy2server-clipboard-*.png")
	if err != nil {
		return uploadCandidate{}, err
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return uploadCandidate{}, err
	}
	defer os.Remove(tmpPath)
	command := strings.Replace(windowsImageScript, "__COPY2SERVER_OUTPUT_PATH__", powerShellQuote(tmpPath), 1)
	if _, err := runCommand(ctx, "powershell", "-NoProfile", "-STA", "-Command", command); err != nil {
		return uploadCandidate{}, err
	}
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return uploadCandidate{}, err
	}
	if len(data) == 0 {
		return uploadCandidate{}, errors.New("no image data in clipboard")
	}
	filename := name
	if filename == "" {
		filename = "clipboard.png"
	}
	return uploadCandidate{filename: filename, contentType: "image/png", data: data}, nil
}

const windowsImageScript = `
$Path = __COPY2SERVER_OUTPUT_PATH__
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$image = [System.Windows.Forms.Clipboard]::GetImage()
if ($null -eq $image) { exit 1 }
try {
  $image.Save($Path, [System.Drawing.Imaging.ImageFormat]::Png)
} finally {
  $image.Dispose()
}
`

func powerShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func readClipboardText(ctx context.Context) (string, error) {
	switch runtime.GOOS {
	case "darwin":
		data, err := runCommand(ctx, "pbpaste")
		return string(data), err
	case "windows":
		data, err := runCommand(ctx, "powershell", "-NoProfile", "-Command", "Get-Clipboard -Raw")
		return strings.ReplaceAll(string(data), "\r\n", "\n"), err
	default:
		for _, command := range [][]string{{"wl-paste"}, {"xclip", "-selection", "clipboard", "-out"}, {"xsel", "--clipboard", "--output"}} {
			if _, err := exec.LookPath(command[0]); err != nil {
				continue
			}
			data, err := runCommand(ctx, command[0], command[1:]...)
			if err == nil {
				return string(data), nil
			}
		}
	}
	return "", errors.New("无法读取剪贴板")
}

func writeClipboard(ctx context.Context, text string) error {
	switch runtime.GOOS {
	case "darwin":
		return pipeCommand(ctx, text, "pbcopy")
	case "windows":
		return writeWindowsClipboardText(ctx, text)
	default:
		for _, command := range [][]string{{"wl-copy"}, {"xclip", "-selection", "clipboard"}, {"xsel", "--clipboard", "--input"}} {
			if _, err := exec.LookPath(command[0]); err != nil {
				continue
			}
			if err := pipeCommand(ctx, text, command[0], command[1:]...); err == nil {
				return nil
			}
		}
	}
	return errors.New("无法写入剪贴板")
}

func runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Output()
}

func pipeCommand(ctx context.Context, input, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = strings.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return err
	}
	return nil
}

func existingFilesFromText(text string) []string {
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		line = strings.Trim(strings.TrimSpace(line), "\"'")
		if strings.HasPrefix(line, "file://") {
			if parsed, err := url.Parse(line); err == nil {
				line = parsed.Path
			}
		}
		if line == "" {
			continue
		}
		if info, err := os.Stat(line); err == nil && !info.IsDir() {
			files = append(files, line)
		}
	}
	return files
}

func filenameForMime(mimeType string) string {
	switch mimeType {
	case "image/png":
		return "clipboard.png"
	case "text/html":
		return "clipboard.html"
	case "text/rtf":
		return "clipboard.rtf"
	default:
		return "clipboard.bin"
	}
}
