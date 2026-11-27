package activities

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"go.temporal.io/sdk/temporal"
	"reel/internal/ai/imagegen"
	"reel/internal/storage"
	"reel/internal/story"
)

// ImageActivities implements Temporal activities for character reference and scene image generation.
type ImageActivities struct {
	Storage        storage.Strategy
	ImageGen       imagegen.Strategy
	MaxConcurrency int
}

// ImageGenInput identifies the job and storyboard to illustrate.
type ImageGenInput struct {
	JobID         string `json:"job_id"`
	StoryboardKey string `json:"storyboard_key"`
	Width         int    `json:"width,omitempty"`
	Height        int    `json:"height,omitempty"`
}

// ImageGenOutput summarizes generated image assets.
type ImageGenOutput struct {
	StoryboardKey string   `json:"storyboard_key"`
	GeneratedKeys []string `json:"generated_keys"`
}

func (a *ImageActivities) concurrency() int {
	if a.MaxConcurrency <= 0 {
		return 3
	}
	return a.MaxConcurrency
}

// GenerateCharacterRefs generates reference sheets for each character in the Character Bible.
func (a *ImageActivities) GenerateCharacterRefs(ctx context.Context, input ImageGenInput) (*ImageGenOutput, error) {
	if a.ImageGen == nil {
		return nil, temporal.NewNonRetryableApplicationError("imagegen provider not configured", "ConfigError", nil)
	}

	doc, err := a.loadStoryboard(ctx, input.StoryboardKey)
	if err != nil {
		return nil, err
	}

	chars := doc.Storyboard.CharacterBible
	if len(chars) == 0 {
		return &ImageGenOutput{StoryboardKey: input.StoryboardKey}, nil
	}

	keys := make([]string, len(chars))
	sem := make(chan struct{}, a.concurrency())
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	var firstErr error
	var errOnce sync.Once

	for i := range chars {
		if chars[i].ReferenceImageKey != "" {
			keys[i] = chars[i].ReferenceImageKey
			continue
		}
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-subCtx.Done():
				return
			}

			prompt, neg := story.BuildCharacterSheetPrompt(chars[idx], doc.Storyboard.Style)
			resp, err := a.ImageGen.Generate(subCtx, imagegen.GenerateRequest{
				Prompt:         prompt,
				NegativePrompt: neg,
				Width:          1024,
				Height:         1024,
			})
			if err != nil {
				errOnce.Do(func() {
					firstErr = fmt.Errorf("character %s image: %w", chars[idx].ID, err)
					cancel()
				})
				return
			}

			key := fmt.Sprintf("jobs/%s/characters/%s.png", input.JobID, chars[idx].ID)
			ct := resp.ContentType
			if ct == "" {
				ct = "image/png"
			}
			if err := a.Storage.Put(subCtx, key, bytes.NewReader(resp.ImageBytes), int64(len(resp.ImageBytes)), ct); err != nil {
				errOnce.Do(func() {
					firstErr = fmt.Errorf("store character %s image: %w", chars[idx].ID, err)
					cancel()
				})
				return
			}
			chars[idx].ReferenceImageKey = key
			keys[idx] = key
		}(i)
	}

	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}

	if err := a.saveStoryboard(ctx, input.StoryboardKey, doc); err != nil {
		return nil, err
	}

	return &ImageGenOutput{
		StoryboardKey: input.StoryboardKey,
		GeneratedKeys: keys,
	}, nil
}

// GenerateSceneImages generates illustrations for each storyboard scene with bounded parallelism.
func (a *ImageActivities) GenerateSceneImages(ctx context.Context, input ImageGenInput) (*ImageGenOutput, error) {
	if a.ImageGen == nil {
		return nil, temporal.NewNonRetryableApplicationError("imagegen provider not configured", "ConfigError", nil)
	}

	doc, err := a.loadStoryboard(ctx, input.StoryboardKey)
	if err != nil {
		return nil, err
	}

	scenes := doc.Storyboard.Scenes
	if len(scenes) == 0 {
		return &ImageGenOutput{StoryboardKey: input.StoryboardKey}, nil
	}

	w, h := input.Width, input.Height
	if w <= 0 || h <= 0 {
		w, h = 1024, 1024
	}

	// Pre-resolve character reference URLs when storage supports HTTP presigned URLs.
	charRefURLs := make(map[string]string, len(doc.Storyboard.CharacterBible))
	for _, c := range doc.Storyboard.CharacterBible {
		if c.ReferenceImageKey != "" {
			if u, err := a.Storage.URL(ctx, c.ReferenceImageKey, 30*time.Minute); err == nil && strings.HasPrefix(u, "http") {
				charRefURLs[c.ID] = u
			}
		}
	}

	keys := make([]string, len(scenes))
	sem := make(chan struct{}, a.concurrency())
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	var firstErr error
	var errOnce sync.Once

	for i := range scenes {
		if scenes[i].ImageKey != "" {
			keys[i] = scenes[i].ImageKey
			continue
		}
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-subCtx.Done():
				return
			}

			prompt, neg := story.BuildScenePrompt(scenes[idx], doc.Storyboard.CharacterBible, doc.Storyboard.Style)
			var refs []string
			for _, cid := range scenes[idx].CharacterIDs {
				if u, ok := charRefURLs[cid]; ok {
					refs = append(refs, u)
				}
			}

			resp, err := a.ImageGen.Generate(subCtx, imagegen.GenerateRequest{
				Prompt:         prompt,
				NegativePrompt: neg,
				Width:          w,
				Height:         h,
				ReferenceURLs:  refs,
			})
			if err != nil {
				errOnce.Do(func() {
					firstErr = fmt.Errorf("scene %s image: %w", scenes[idx].SceneID, err)
					cancel()
				})
				return
			}

			key := fmt.Sprintf("jobs/%s/scenes/%s.png", input.JobID, scenes[idx].SceneID)
			ct := resp.ContentType
			if ct == "" {
				ct = "image/png"
			}
			if err := a.Storage.Put(subCtx, key, bytes.NewReader(resp.ImageBytes), int64(len(resp.ImageBytes)), ct); err != nil {
				errOnce.Do(func() {
					firstErr = fmt.Errorf("store scene %s image: %w", scenes[idx].SceneID, err)
					cancel()
				})
				return
			}
			scenes[idx].Prompt = prompt
			scenes[idx].ImageKey = key
			keys[idx] = key
		}(i)
	}

	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}

	if err := a.saveStoryboard(ctx, input.StoryboardKey, doc); err != nil {
		return nil, err
	}

	return &ImageGenOutput{
		StoryboardKey: input.StoryboardKey,
		GeneratedKeys: keys,
	}, nil
}

func (a *ImageActivities) loadStoryboard(ctx context.Context, key string) (*StoredStoryboard, error) {
	rc, err := a.Storage.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("get storyboard %s: %w", key, err)
	}
	defer rc.Close()

	raw, err := io.ReadAll(io.LimitReader(rc, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read storyboard: %w", err)
	}

	var doc StoredStoryboard
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("decode storyboard: %w", err)
	}
	return &doc, nil
}

func (a *ImageActivities) saveStoryboard(ctx context.Context, key string, doc *StoredStoryboard) error {
	payload, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal storyboard: %w", err)
	}
	return a.Storage.Put(ctx, key, bytes.NewReader(payload), int64(len(payload)), "application/json")
}
