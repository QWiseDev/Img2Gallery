package images

import (
	"path/filepath"
	"testing"

	"github.com/QWiseDev/Img2Gallery/server-go/internal/config"
	"github.com/QWiseDev/Img2Gallery/server-go/internal/db"
)

func TestNormalizeParams(t *testing.T) {
	compression := 140
	params := NormalizeParams(GenerationParams{
		Size:              "1536x1024",
		Quality:           "high",
		OutputFormat:      "webp",
		OutputCompression: &compression,
		Moderation:        "low",
	})

	if params.Size != "1536x1024" {
		t.Fatalf("size = %q, want 1536x1024", params.Size)
	}
	if params.Quality != "high" {
		t.Fatalf("quality = %q, want high", params.Quality)
	}
	if params.OutputFormat != "webp" {
		t.Fatalf("output format = %q, want webp", params.OutputFormat)
	}
	if params.OutputCompression == nil || *params.OutputCompression != 100 {
		t.Fatalf("compression = %v, want 100", params.OutputCompression)
	}
	if params.Moderation != "low" {
		t.Fatalf("moderation = %q, want low", params.Moderation)
	}
}

func TestNormalizeParamsDefaultsInvalidValues(t *testing.T) {
	compression := 82
	params := NormalizeParams(GenerationParams{
		Size:              "999x13",
		Quality:           "ultra",
		OutputFormat:      "png",
		OutputCompression: &compression,
		Moderation:        "strict",
	})

	if params.Size != "auto" {
		t.Fatalf("size = %q, want auto", params.Size)
	}
	if params.Quality != "auto" {
		t.Fatalf("quality = %q, want auto", params.Quality)
	}
	if params.OutputFormat != "png" {
		t.Fatalf("output format = %q, want png", params.OutputFormat)
	}
	if params.OutputCompression != nil {
		t.Fatalf("compression = %v, want nil", params.OutputCompression)
	}
	if params.Moderation != "auto" {
		t.Fatalf("moderation = %q, want auto", params.Moderation)
	}
}

func TestRepositoryReturnsTypedImageDTO(t *testing.T) {
	repo := newTestRepository(t)
	userID := insertImageTestUser(t, repo)

	imageID, err := repo.AddImage(userID, "paint a quiet neon city", "queued", "127.0.0.1", "generate", "", GenerationParams{Size: "1536x1024"})
	if err != nil {
		t.Fatalf("AddImage returned error: %v", err)
	}

	image, found, err := repo.GetImage(int(imageID), userID)
	if err != nil {
		t.Fatalf("GetImage returned error: %v", err)
	}
	if !found {
		t.Fatalf("expected image to be found")
	}
	assertImageDTO(t, image, userID)

	images, err := repo.ListUserImages(userID, 10, 0)
	if err != nil {
		t.Fatalf("ListUserImages returned error: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("len(images) = %d, want 1", len(images))
	}
	assertImageDTO(t, images[0], userID)
}

func TestRepositoryRequiresImageParamsRows(t *testing.T) {
	repo := newTestRepository(t)
	userID := insertImageTestUser(t, repo)
	if _, err := repo.db.Exec(`
		INSERT INTO images (user_id, prompt, status)
		VALUES (?, 'legacy image without params', 'ready')
	`, userID); err != nil {
		t.Fatalf("insert legacy image returned error: %v", err)
	}

	images, err := repo.ListUserImages(userID, 10, 0)
	if err != nil {
		t.Fatalf("ListUserImages returned error: %v", err)
	}
	if len(images) != 0 {
		t.Fatalf("len(images) = %d, want 0 before database upgrade", len(images))
	}
}

func assertImageDTO(t *testing.T, image Image, userID int) {
	t.Helper()
	if image.ID == 0 {
		t.Fatalf("image ID was not set")
	}
	if image.Prompt != "paint a quiet neon city" {
		t.Fatalf("prompt = %q", image.Prompt)
	}
	if image.Status != "queued" {
		t.Fatalf("status = %q, want queued", image.Status)
	}
	if image.Author.ID != userID {
		t.Fatalf("author ID = %d, want %d", image.Author.ID, userID)
	}
	if image.Params.Size != "1536x1024" {
		t.Fatalf("params size = %q, want 1536x1024", image.Params.Size)
	}
}

func newTestRepository(t *testing.T) *Repository {
	t.Helper()
	cfg := config.Config{
		DatabasePath:    filepath.Join(t.TempDir(), "app.db"),
		ImageStorageDir: t.TempDir(),
		AppTimezone:     "Asia/Shanghai",
	}
	database, err := db.Open(cfg)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := db.Init(database, cfg); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	return NewRepository(database, cfg)
}

func insertImageTestUser(t *testing.T, repo *Repository) int {
	t.Helper()
	res, err := repo.db.Exec(`
		INSERT INTO users (username, display_name, password_hash, avatar_color)
		VALUES ('alice', 'Alice', 'salt$hash', '#14b8a6')
	`)
	if err != nil {
		t.Fatalf("insert user returned error: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId returned error: %v", err)
	}
	return int(id)
}
