package asr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"time"
)

var streamBufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 32*1024)
		return &b
	},
}

// openAICompatClient implements Strategy for OpenAI-compatible /audio/transcriptions APIs
// (shared by Groq Whisper and OpenAI Whisper) using streaming multipart uploads via io.Pipe.
type openAICompatClient struct {
	providerName string
	endpoint     string
	apiKey       string
	defaultModel string
	httpClient   *http.Client
}

func newOpenAICompatClient(providerName, endpoint, apiKey, defaultModel string, httpClient *http.Client) (*openAICompatClient, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("%s api key is required", providerName)
	}
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 5 * time.Minute,
			Transport: &http.Transport{
				MaxIdleConns:        50,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     90 * time.Second,
			},
		}
	}
	return &openAICompatClient{
		providerName: providerName,
		endpoint:     endpoint,
		apiKey:       apiKey,
		defaultModel: defaultModel,
		httpClient:   httpClient,
	}, nil
}

func (c *openAICompatClient) Name() string {
	return c.providerName
}

type verboseJSONResponse struct {
	Language string  `json:"language"`
	Duration float64 `json:"duration"`
	Text     string  `json:"text"`
	Words    []struct {
		Word  string  `json:"word"`
		Start float64 `json:"start"`
		End   float64 `json:"end"`
	} `json:"words"`
	Segments []struct {
		Text  string  `json:"text"`
		Start float64 `json:"start"`
		End   float64 `json:"end"`
	} `json:"segments"`
}

func (c *openAICompatClient) Transcribe(ctx context.Context, req TranscribeRequest) (*TranscribeResponse, error) {
	if req.Audio == nil {
		return nil, errors.New("audio reader is required")
	}

	model := req.Model
	if model == "" {
		model = c.defaultModel
	}
	filename := req.Filename
	if filename == "" {
		filename = "audio.wav"
	}

	// Stream multipart form body directly through io.Pipe to avoid buffering large audio in RAM.
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	contentType := mw.FormDataContentType()

	errCh := make(chan error, 1)
	go func() {
		defer pw.Close()
		if err := mw.WriteField("model", model); err != nil {
			_ = pw.CloseWithError(err)
			errCh <- err
			return
		}
		if err := mw.WriteField("response_format", "verbose_json"); err != nil {
			_ = pw.CloseWithError(err)
			errCh <- err
			return
		}
		if err := mw.WriteField("timestamp_granularities[]", "word"); err != nil {
			_ = pw.CloseWithError(err)
			errCh <- err
			return
		}
		if err := mw.WriteField("timestamp_granularities[]", "segment"); err != nil {
			_ = pw.CloseWithError(err)
			errCh <- err
			return
		}
		if req.Language != "" {
			if err := mw.WriteField("language", req.Language); err != nil {
				_ = pw.CloseWithError(err)
				errCh <- err
				return
			}
		}

		fw, err := mw.CreateFormFile("file", filename)
		if err != nil {
			_ = pw.CloseWithError(err)
			errCh <- err
			return
		}

		bufPtr := streamBufPool.Get().(*[]byte)
		_, copyErr := io.CopyBuffer(fw, req.Audio, *bufPtr)
		streamBufPool.Put(bufPtr)
		if copyErr != nil {
			_ = pw.CloseWithError(copyErr)
			errCh <- copyErr
			return
		}

		if err := mw.Close(); err != nil {
			_ = pw.CloseWithError(err)
			errCh <- err
			return
		}
		errCh <- nil
	}()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, pr)
	if err != nil {
		_ = pr.Close()
		return nil, fmt.Errorf("create asr request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", contentType)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%s request failed: %w", c.providerName, err)
	}
	defer resp.Body.Close()

	// Drain pipe writer status
	if pipeErr := <-errCh; pipeErr != nil {
		return nil, fmt.Errorf("stream multipart audio: %w", pipeErr)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read %s response: %w", c.providerName, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s api error (%d): %s", c.providerName, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed verboseJSONResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", c.providerName, err)
	}

	words := make([]Word, 0, len(parsed.Words))
	for _, w := range parsed.Words {
		txt := strings.TrimSpace(w.Word)
		if txt == "" {
			continue
		}
		words = append(words, Word{
			Text:  txt,
			Start: w.Start,
			End:   w.End,
		})
	}

	NormalizeWordTimestamps(words)
	segments := GroupWordsIntoSentences(words)
	if len(segments) == 0 && strings.TrimSpace(parsed.Text) != "" {
		// Fallback if provider returned segments without word-level granularity.
		for _, seg := range parsed.Segments {
			segments = append(segments, Segment{
				Text:  strings.TrimSpace(seg.Text),
				Start: seg.Start,
				End:   seg.End,
			})
		}
	}

	return &TranscribeResponse{
		Provider: c.providerName,
		Model:    model,
		Language: parsed.Language,
		Text:     strings.TrimSpace(parsed.Text),
		Segments: segments,
		Duration: parsed.Duration,
	}, nil
}
