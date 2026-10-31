package main

import (
	"context"
	"keel.local/dev/internal/flows"
	"testing"
	"time"
)

func TestTaskJournalAndDigest(t *testing.T) {
	dir := t.TempDir()
	task := flows.Task{ID: "synthetic-task", Kind: "bash", Command: "printf 'synthetic agent result'", Timeout: 5}
	task.SHA256 = flows.Digest([]byte(task.Command))
	first := execute(context.Background(), dir, task)
	if first.State != "SUCCESS" || first.Stdout != "synthetic agent result" {
		t.Fatal(first)
	}
	same := execute(context.Background(), dir, task)
	if same.Stdout != first.Stdout || same.Exit != 0 {
		t.Fatal("duplicate task changed result")
	}
	task.Command = "exit 99"
	task.SHA256 = flows.Digest([]byte(task.Command))
	again := execute(context.Background(), dir, task)
	if again.State != "FAILED" || again.Stderr != "Task ID reused with different content" {
		t.Fatal("duplicate task reran")
	}
	task.ID = "bad-digest"
	task.SHA256 = "wrong"
	if r := execute(context.Background(), dir, task); r.State != "FAILED" {
		t.Fatal("digest bypassed")
	}
}
func TestFileBoundary(t *testing.T) {
	task := flows.Task{ID: "synthetic-file", Kind: "file.upload", Command: `{"path":"../../escape","data_base64":"","sha256":""}`}
	if fileTask(t.TempDir(), task).State != "FAILED" {
		t.Fatal("path escape allowed")
	}
}

func TestTaskStreamsOutputBeforeExit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := "printf stream-before-exit; sleep 0.1; printf final-output"
	task := flows.Task{ID: "synthetic-stream", Kind: "bash", Command: command, SHA256: flows.Digest([]byte(command)), Timeout: 3}
	chunks := make(chan string, 8)
	done := make(chan flows.Result, 1)
	go func() {
		done <- execute(ctx, t.TempDir(), task, func(stream, chunk string) {
			if stream == "stdout" {
				chunks <- chunk
			}
		})
	}()
	select {
	case first := <-chunks:
		if first != "stream-before-exit" {
			t.Fatal(first)
		}
	case <-time.After(time.Second):
		t.Fatal("no live output")
	}
	result := <-done
	if result.State != "SUCCESS" || result.Stdout != "stream-before-exitfinal-output" {
		t.Fatal(result)
	}
}
