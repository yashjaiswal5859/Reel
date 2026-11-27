package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"reel/internal/media"
	"reel/internal/storage"
	"reel/internal/temporal/workflows"
)

// JobService abstracts workflow orchestration for HTTP handlers.
type JobService interface {
	StartVideoJob(ctx context.Context, input workflows.VideoJobInput) error
	GetJobStatus(ctx context.Context, jobID string) (*media.JobStatus, error)
	CancelJob(ctx context.Context, jobID string) error
}

// TemporalJobService implements JobService using the Temporal Go SDK.
type TemporalJobService struct {
	client    client.Client
	taskQueue string
}

// NewTemporalJobService creates a Temporal-backed JobService.
func NewTemporalJobService(c client.Client, taskQueue string) *TemporalJobService {
	return &TemporalJobService{client: c, taskQueue: taskQueue}
}

func (s *TemporalJobService) StartVideoJob(ctx context.Context, input workflows.VideoJobInput) error {
	opts := client.StartWorkflowOptions{
		ID:        input.JobID,
		TaskQueue: s.taskQueue,
	}
	_, err := s.client.ExecuteWorkflow(ctx, opts, workflows.VideoProcessingWorkflow, input)
	return err
}

func (s *TemporalJobService) GetJobStatus(ctx context.Context, jobID string) (*media.JobStatus, error) {
	desc, err := s.client.DescribeWorkflowExecution(ctx, jobID, "")
	if err != nil {
		return nil, err
	}

	wfStatus := desc.WorkflowExecutionInfo.Status
	switch wfStatus {
	case enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED:
		var res media.JobStatus
		if err := s.client.GetWorkflow(ctx, jobID, "").Get(ctx, &res); err != nil {
			return nil, err
		}
		return &res, nil
	case enumspb.WORKFLOW_EXECUTION_STATUS_CANCELED, enumspb.WORKFLOW_EXECUTION_STATUS_TERMINATED:
		return &media.JobStatus{
			JobID: jobID,
			Stage: media.StageCancelled,
		}, nil
	case enumspb.WORKFLOW_EXECUTION_STATUS_FAILED, enumspb.WORKFLOW_EXECUTION_STATUS_TIMED_OUT:
		return &media.JobStatus{
			JobID: jobID,
			Stage: media.StageFailed,
			Error: wfStatus.String(),
		}, nil
	default:
		resp, err := s.client.QueryWorkflow(ctx, jobID, "", workflows.QueryJobStatus)
		if err != nil {
			return &media.JobStatus{
				JobID: jobID,
				Stage: media.StagePreprocessing,
			}, nil
		}
		var status media.JobStatus
		if err := resp.Get(&status); err != nil {
			return nil, err
		}
		return &status, nil
	}
}

func (s *TemporalJobService) CancelJob(ctx context.Context, jobID string) error {
	return s.client.CancelWorkflow(ctx, jobID, "")
}

type jobHandler struct {
	storage        storage.Strategy
	jobs           JobService
	maxFileSizeMB  int64
}

func (h *jobHandler) upload(c *gin.Context) {
	if h.storage == nil || h.jobs == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "job service not configured"})
		return
	}

	// Enforce upload size limit at the HTTP reader level before any buffering.
	if h.maxFileSizeMB > 0 {
		maxBytes := h.maxFileSizeMB * 1024 * 1024
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
	}

	mr, err := c.Request.MultipartReader()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expected multipart/form-data request"})
		return
	}

	var uploadedKey string
	var jobID string

	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{
					"error": fmt.Sprintf("file exceeds maximum allowed size of %d MB", h.maxFileSizeMB),
				})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("read multipart: %v", err)})
			return
		}

		if part.FormName() != "video" {
			_ = part.Close()
			continue
		}

		filename := part.FileName()
		if err := media.ValidateExtension(filename); err != nil {
			_ = part.Close()
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Sniff only the first 512 bytes for container magic bytes without buffering the video.
		header := make([]byte, 512)
		n, readErr := io.ReadFull(part, header)
		if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
			_ = part.Close()
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read video header"})
			return
		}
		if err := media.ValidateVideoHeader(header[:n]); err != nil {
			_ = part.Close()
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		jobID, err = newJobID()
		if err != nil {
			_ = part.Close()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate job id"})
			return
		}

		ext := strings.ToLower(filepath.Ext(filename))
		uploadedKey = fmt.Sprintf("jobs/%s/input%s", jobID, ext)
		stream := io.MultiReader(bytes.NewReader(header[:n]), part)

		contentType := part.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		if err := h.storage.Put(c.Request.Context(), uploadedKey, stream, -1, contentType); err != nil {
			_ = part.Close()
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				_ = h.storage.Delete(context.Background(), uploadedKey)
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{
					"error": fmt.Sprintf("file exceeds maximum allowed size of %d MB", h.maxFileSizeMB),
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("store video: %v", err)})
			return
		}
		_ = part.Close()
		break
	}

	if uploadedKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing 'video' file field"})
		return
	}

	if err := h.jobs.StartVideoJob(c.Request.Context(), workflows.VideoJobInput{
		JobID:    jobID,
		InputKey: uploadedKey,
	}); err != nil {
		// Clean up orphaned upload on workflow dispatch failure.
		_ = h.storage.Delete(context.Background(), uploadedKey)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("start workflow: %v", err)})
		return
	}

	c.JSON(http.StatusAccepted, media.JobStatus{
		JobID:    jobID,
		Stage:    media.StageUploaded,
		Progress: 5,
		InputKey: uploadedKey,
	})
}

func (h *jobHandler) getStatus(c *gin.Context) {
	if h.jobs == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "job service not configured"})
		return
	}
	jobID := c.Param("id")
	status, err := h.jobs.GetJobStatus(c.Request.Context(), jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("job %q not found: %v", jobID, err)})
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h *jobHandler) cancel(c *gin.Context) {
	if h.jobs == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "job service not configured"})
		return
	}
	jobID := c.Param("id")
	if err := h.jobs.CancelJob(c.Request.Context(), jobID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("cancel job: %v", err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"job_id": jobID, "status": "cancelling"})
}

func newJobID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "job-" + hex.EncodeToString(b), nil
}
