package main

import (
	"context"
	"encoding/json"
	"fmt"
	"keel.local/dev/internal/flows"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func controlTask(ctx context.Context, dir string, task flows.Task) flows.Result {
	out := flows.Result{ID: task.ID, State: "FAILED", Exit: -1}
	var input struct {
		Action  string `json:"action"`
		Task    string `json:"task_id"`
		Service string `json:"service"`
	}
	if json.Unmarshal([]byte(task.Command), &input) != nil {
		out.Stderr = "Invalid control request"
		return out
	}
	if task.Kind == "process.control" {
		if input.Action == "cancel" {
			taskRegistry.Lock()
			cancel := taskRegistry.active[input.Task]
			taskRegistry.Unlock()
			if cancel == nil {
				out.Stderr = "No active owned task"
				return out
			}
			cancel()
			out.Stdout = "Cancellation requested; inspect the original task outcome"
		} else if input.Action == "query" {
			b, e := os.ReadFile(filepath.Join(dir, flows.Digest([]byte(input.Task))+".json"))
			if e != nil {
				out.Stderr = "Unknown task; no side-effect claim"
				return out
			}
			out.Stdout = string(b)
		} else {
			out.Stderr = "Unsupported process action"
			return out
		}
		out.State = "SUCCESS"
		out.Exit = 0
		return out
	}
	allowed := false
	for _, name := range strings.Split(os.Getenv("KEEL_AGENT_ALLOWED_SERVICES"), ",") {
		if name != "" && input.Service == name {
			allowed = true
		}
	}
	if !allowed || !containsControlAction(input.Action) || strings.HasPrefix(input.Service, "-") {
		out.Stderr = "User service/action not allowlisted"
		return out
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "systemctl", "--user", input.Action, input.Service)
	cmd.Env = []string{"PATH=/usr/bin:/bin", fmt.Sprintf("XDG_RUNTIME_DIR=/run/user/%d", os.Getuid())}
	buffer := &limitWriter{}
	cmd.Stdout = buffer
	cmd.Stderr = buffer
	e := cmd.Run()
	out.Stdout = buffer.b.String()
	if e != nil {
		out.Stderr = "User-service action failed"
		return out
	}
	out.State = "SUCCESS"
	out.Exit = 0
	return out
}
func containsControlAction(action string) bool {
	return action == "status" || action == "restart" || action == "stop" || action == "start"
}
