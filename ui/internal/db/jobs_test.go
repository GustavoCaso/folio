package db_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/db"
)

var content = []byte{'a', 'b', 'c'}

func TestCreateAndGetJob(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	job, err := database.CreateJob(context.Background(), "book.pdf", content, "req-123")
	if err != nil {
		t.Fatal(err)
	}
	if job.ID == "" {
		t.Fatal("expected non-empty job ID")
	}
	if job.Status != "PENDING" {
		t.Fatalf("expected PENDING, got %s", job.Status)
	}
	if job.RequestID != "req-123" {
		t.Fatalf("expected request_id req-123, got %q", job.RequestID)
	}
	if !reflect.DeepEqual(job.Content, content) {
		t.Fatalf("expected content %s, got %q", content, job.Content)
	}

	got, err := database.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Filename != "book.pdf" {
		t.Fatalf("expected book.pdf, got %s", got.Filename)
	}
	if got.RequestID != "req-123" {
		t.Fatalf("expected request_id req-123 after read, got %q", got.RequestID)
	}
	if !reflect.DeepEqual(got.Content, content) {
		t.Fatalf("expected content %s, got %q", content, job.Content)
	}
}

func TestUpdateJobStatus(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	job, err := database.CreateJob(context.Background(), "book.pdf", content, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.UpdateJobStatus(context.Background(), job.ID, "PROCESSING"); err != nil {
		t.Fatal(err)
	}

	got, _ := database.GetJob(context.Background(), job.ID)
	if got.Status != "PROCESSING" {
		t.Fatalf("expected PROCESSING, got %s", got.Status)
	}
}

func TestUpdateReadingProgress(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	job, _ := database.CreateJob(context.Background(), "book.pdf", content, "")
	if err := database.UpdateReadingProgress(context.Background(), job.ID, "paragraph-42"); err != nil {
		t.Fatal(err)
	}

	got, _ := database.GetJob(context.Background(), job.ID)
	if got.ReadingProgress != "paragraph-42" {
		t.Fatalf("expected paragraph-42, got %q", got.ReadingProgress)
	}
}

func TestGetPendingJobs(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	pending1, _ := database.CreateJob(ctx, "a.pdf", content, "")
	pending2, _ := database.CreateJob(ctx, "b.pdf", content, "")
	processing, _ := database.CreateJob(ctx, "c.pdf", content, "")
	done, _ := database.CreateJob(ctx, "d.pdf", content, "")
	failed, _ := database.CreateJob(ctx, "e.pdf", content, "")

	if err := database.UpdateJobStatus(ctx, processing.ID, "PROCESSING"); err != nil {
		t.Fatal(err)
	}
	if err := database.MarkJobDone(ctx, done.ID, "/data/d.md"); err != nil {
		t.Fatal(err)
	}
	if err := database.MarkJobFailed(ctx, failed.ID, "boom"); err != nil {
		t.Fatal(err)
	}

	jobs, err := database.GetPendingJobs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 pending jobs, got %d", len(jobs))
	}

	got := map[string]bool{}
	for _, j := range jobs {
		if j.Status != "PENDING" {
			t.Errorf("expected PENDING, got %s for %s", j.Status, j.ID)
		}
		if len(j.Content) != 0 {
			t.Errorf("expected content to not be populated, got %v for %s", j.Content, j.ID)
		}
		got[j.ID] = true
	}
	if !got[pending1.ID] || !got[pending2.ID] {
		t.Fatalf("missing pending jobs in result: %v", got)
	}
}

func TestGetPendingJobsEmpty(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	jobs, err := database.GetPendingJobs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 0 {
		t.Fatalf("expected 0 pending jobs, got %d", len(jobs))
	}
}

func TestMarkJobDone(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	job, _ := database.CreateJob(context.Background(), "book.pdf", content, "")
	if len(job.Content) == 0 {
		t.Fatalf("expected content to not be empty, got %v", job.Content)
	}

	if err := database.MarkJobDone(context.Background(), job.ID, "/data/book.md"); err != nil {
		t.Fatal(err)
	}

	got, _ := database.GetJob(context.Background(), job.ID)
	if got.Status != "DONE" {
		t.Fatalf("expected DONE, got %s", got.Status)
	}
	if got.OutputPath != "/data/book.md" {
		t.Fatalf("expected /data/book.md, got %s", got.OutputPath)
	}

	if len(got.Content) != 0 {
		t.Fatalf("expected empty content, got %v", got.Content)
	}
}

func TestDeleteJob(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	job, err := database.CreateJob(ctx, "test.pdf", content, "req-1")
	if err != nil {
		t.Fatal(err)
	}

	if err := database.DeleteJob(ctx, job.ID); err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}

	_, err = database.GetJob(ctx, job.ID)
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestRetryJob(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	job, err := database.CreateJob(ctx, "test.pdf", content, "req-1")
	if err != nil {
		t.Fatal(err)
	}

	if job.RetryCount != 0 {
		t.Fatalf("job must retry count to 0 after CreateJob, got: %d", job.RetryCount)
	}

	err = database.MarkJobFailed(ctx, job.ID, "testing retry logic")
	if err != nil {
		t.Fatal(err)
	}

	job, err = database.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}

	if job.Status != "FAILED" {
		t.Fatalf("job must have failed status after MarkJobFailed, got: %s", job.Status)
	}

	err = database.RetryJob(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}

	job, err = database.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}

	if job.Status != "PENDING" {
		t.Fatalf("job must have pending status after RetryJob, got: %s", job.Status)
	}

	if job.RetryCount != 1 {
		t.Fatalf("job must retry count to 1 after RetryJob, got: %d", job.RetryCount)
	}
}
