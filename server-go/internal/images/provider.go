package images

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/QWiseDev/Img2Gallery/server-go/internal/config"
)

const requestTimeout = 600 * time.Second

type ProviderClient struct {
	cfg    config.Config
	client *http.Client
}

type imageResponse struct {
	Data []struct {
		B64JSON string `json:"b64_json"`
		URL     string `json:"url"`
	} `json:"data"`
}

func NewProviderClient(cfg config.Config) *ProviderClient {
	return &ProviderClient{cfg: cfg, client: newProviderHTTPClient()}
}

func (c *ProviderClient) GenerateAndStore(prompt string, provider Provider) (string, error) {
	if err := validateProvider(provider); err != nil {
		return "", err
	}
	payload := map[string]any{"model": provider.Model, "prompt": prompt, "n": 1, "response_format": "b64_json"}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, strings.TrimRight(provider.APIBase, "/")+"/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+provider.APIKey)
	response, err := c.do(req, "生图接口")
	if err != nil {
		return "", err
	}
	imageBytes, suffix, err := c.extractImage(response)
	if err != nil {
		return "", err
	}
	return c.storeImageBytes(imageBytes, suffix)
}

func (c *ProviderClient) EditAndStore(prompt, sourceImagePath string, provider Provider) (string, error) {
	if err := validateProvider(provider); err != nil {
		return "", err
	}
	sourcePath, err := c.resolveSourceImage(sourceImagePath)
	if err != nil {
		return "", err
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("model", provider.Model)
	_ = writer.WriteField("prompt", prompt)
	_ = writer.WriteField("n", "1")
	if err := addFilePart(writer, "image", sourcePath); err != nil {
		return "", err
	}
	_ = writer.Close()
	req, _ := http.NewRequest(http.MethodPost, strings.TrimRight(provider.APIBase, "/")+"/v1/images/edits", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+provider.APIKey)
	response, err := c.do(req, "图片编辑接口")
	if err != nil {
		return "", err
	}
	imageBytes, suffix, err := c.extractImage(response)
	if err != nil {
		return "", err
	}
	return c.storeImageBytes(imageBytes, suffix)
}

func (c *ProviderClient) do(req *http.Request, label string) (*imageResponse, error) {
	var lastErr error
	for attempt := 1; attempt <= providerAttempts; attempt++ {
		if attempt > 1 {
			if err := resetRequestBody(req); err != nil {
				return nil, err
			}
			waitProviderRetry(req, attempt)
		}
		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = errors.New(label + "请求失败：" + err.Error())
			if shouldRetryProviderError(err, attempt) {
				continue
			}
			return nil, lastErr
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			raw, _ := io.ReadAll(io.LimitReader(resp.Body, providerErrorBodyLimit))
			_ = resp.Body.Close()
			lastErr = providerStatusError(label, resp.Status, raw)
			if shouldRetryProviderStatus(resp.StatusCode, attempt) {
				continue
			}
			return nil, lastErr
		}
		raw, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, errors.New(label + "返回读取失败")
		}
		var payload imageResponse
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, errors.New(label + "返回格式错误")
		}
		return &payload, nil
	}
	return nil, lastErr
}

func (c *ProviderClient) extractImage(payload *imageResponse) ([]byte, string, error) {
	if payload == nil || len(payload.Data) == 0 {
		return nil, "", errors.New("生图接口未返回图片数据")
	}
	first := payload.Data[0]
	if first.B64JSON != "" {
		data, err := base64.StdEncoding.DecodeString(first.B64JSON)
		if err != nil {
			return nil, "", errors.New("生图接口返回图片解码失败")
		}
		return data, ".png", nil
	}
	if first.URL != "" {
		return c.downloadImage(first.URL)
	}
	return nil, "", errors.New("不支持的生图接口返回格式")
}

func (c *ProviderClient) downloadImage(url string) ([]byte, string, error) {
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, "", errors.New("下载生成图片失败：" + err.Error())
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	suffix := suffixFromContentType(resp.Header.Get("Content-Type"))
	if suffix == "" {
		suffix = filepath.Ext(url)
	}
	if suffix == "" {
		suffix = ".png"
	}
	return data, suffix, nil
}

func (c *ProviderClient) storeImageBytes(imageBytes []byte, suffix string) (string, error) {
	if err := os.MkdirAll(c.cfg.ImageStorageDir, 0o755); err != nil {
		return "", err
	}
	filename := randomHex(12) + suffix
	path := filepath.Join(c.cfg.ImageStorageDir, filename)
	return filename, os.WriteFile(path, imageBytes, 0o644)
}

func (c *ProviderClient) resolveSourceImage(sourceImagePath string) (string, error) {
	if sourceImagePath == "" {
		return "", errors.New("图片编辑任务缺少原图")
	}
	root, _ := filepath.Abs(c.cfg.ImageStorageDir)
	path, _ := filepath.Abs(filepath.Join(c.cfg.ImageStorageDir, sourceImagePath))
	if root != path && !strings.HasPrefix(path, strings.TrimRight(root, string(os.PathSeparator))+string(os.PathSeparator)) {
		return "", errors.New("图片编辑任务原图路径无效")
	}
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return "", errors.New("图片编辑任务原图不存在")
	}
	return path, nil
}

func validateProvider(provider Provider) error {
	if provider.ProviderType != "openai_compatible" {
		return errors.New("当前提供商类型暂未支持")
	}
	if provider.APIBase == "" {
		return errors.New("未配置当前模型提供商 API 地址")
	}
	if provider.APIKey == "" {
		return errors.New("未配置当前模型提供商 API Key")
	}
	return nil
}

func addFilePart(writer *multipart.Writer, field, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	part, err := writer.CreateFormFile(field, filepath.Base(path))
	if err != nil {
		return err
	}
	_, err = io.Copy(part, file)
	return err
}

func suffixFromContentType(contentType string) string {
	switch strings.Split(contentType, ";")[0] {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		return ""
	}
}

func randomHex(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))[:size]
	}
	return hex.EncodeToString(buf)
}
