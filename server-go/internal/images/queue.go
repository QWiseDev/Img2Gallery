package images

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

const jobTimeout = 720 * time.Second

type Queue struct {
	repo     *Repository
	provider *ProviderClient
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	running  map[int]struct{}
}

func NewQueue(repo *Repository, provider *ProviderClient) *Queue {
	ctx, cancel := context.WithCancel(context.Background())
	return &Queue{repo: repo, provider: provider, ctx: ctx, cancel: cancel, running: map[int]struct{}{}}
}

func (q *Queue) Start() {
	_ = q.repo.ResetRunningJobs()
	go q.loop()
}

func (q *Queue) Stop() {
	q.cancel()
}

func (q *Queue) Events(ctx context.Context, imageID, viewerID int) <-chan string {
	ch := make(chan string)
	go func() {
		defer close(ch)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			image, ok, _ := q.repo.GetImage(imageID, viewerID)
			if !ok {
				ch <- sse(map[string]any{"status": "missing", "position": nil, "queue": q.repo.QueueCounts()})
				return
			}
			delete(image, "source_image_path")
			status, _ := image["status"].(string)
			ch <- sse(map[string]any{"status": status, "position": q.repo.QueuePosition(imageID), "queue": q.repo.QueueCounts(), "image": image})
			if status == "ready" || status == "failed" {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return ch
}

func (q *Queue) loop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		q.dispatch()
		select {
		case <-q.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (q *Queue) dispatch() {
	available := q.repo.GetConcurrency() - q.runningCount()
	if available <= 0 {
		return
	}
	ids, err := q.repo.NextQueuedJobs(available)
	if err != nil {
		return
	}
	for _, id := range ids {
		if q.markRunning(id) {
			go q.process(id)
		}
	}
}

func (q *Queue) process(imageID int) {
	defer q.unmarkRunning(imageID)
	provider, ok, err := q.repo.ActiveProvider()
	if err != nil || !ok {
		_ = q.repo.MarkFailed(imageID, "未配置可用模型提供商")
		return
	}
	_ = q.repo.MarkRunning(imageID, provider)
	image, ok, _ := q.repo.GetImage(imageID, 0)
	if !ok {
		return
	}
	path, err := q.runJob(image, provider)
	if err != nil {
		_ = q.repo.MarkFailed(imageID, err.Error())
		return
	}
	_ = q.repo.MarkReady(imageID, path)
}

func (q *Queue) runJob(image map[string]any, provider Provider) (string, error) {
	ctx, cancel := context.WithTimeout(q.ctx, jobTimeout)
	defer cancel()
	done := make(chan result, 1)
	go func() {
		prompt, _ := image["prompt"].(string)
		params := paramsFromImage(image)
		if image["task_type"] == "edit" {
			source, _ := image["source_image_path"].(string)
			path, err := q.provider.EditAndStore(prompt, source, params, provider)
			done <- result{path: path, err: err}
			return
		}
		path, err := q.provider.GenerateAndStore(prompt, params, provider)
		done <- result{path: path, err: err}
	}()
	select {
	case <-ctx.Done():
		return "", context.Cause(ctx)
	case res := <-done:
		return res.path, res.err
	}
}

func paramsFromImage(image map[string]any) GenerationParams {
	raw, _ := image["params"].(map[string]any)
	var params GenerationParams
	if raw == nil {
		return NormalizeParams(params)
	}
	params.Size, _ = raw["size"].(string)
	params.Quality, _ = raw["quality"].(string)
	params.OutputFormat, _ = raw["output_format"].(string)
	params.Moderation, _ = raw["moderation"].(string)
	if value, ok := raw["output_compression"].(int); ok {
		params.OutputCompression = &value
	} else if value, ok := raw["output_compression"].(float64); ok {
		intValue := int(value)
		params.OutputCompression = &intValue
	}
	return NormalizeParams(params)
}

func (q *Queue) runningCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.running)
}

func (q *Queue) markRunning(id int) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, ok := q.running[id]; ok {
		return false
	}
	q.running[id] = struct{}{}
	return true
}

func (q *Queue) unmarkRunning(id int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.running, id)
}

type result struct {
	path string
	err  error
}

func sse(payload map[string]any) string {
	raw, _ := json.Marshal(payload)
	return "data: " + string(raw) + "\n\n"
}
