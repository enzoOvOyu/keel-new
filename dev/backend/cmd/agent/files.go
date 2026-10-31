package main

import (
	"encoding/base64"
	"encoding/json"
	"keel.local/dev/internal/flows"
	"os"
	"path/filepath"
	"strings"
)

// File tasks operate only below the agent-owned files directory, never arbitrary host paths.
func fileTask(dir string, t flows.Task) flows.Result {
	out := flows.Result{ID: t.ID, State: "FAILED", Exit: -1}
	var input struct {
		Path string `json:"path"`
		Data string `json:"data_base64"`
		SHA  string `json:"sha256"`
	}
	if json.Unmarshal([]byte(t.Command), &input) != nil || input.Path == "" || filepath.IsAbs(input.Path) || strings.Contains(input.Path, "..") {
		out.Stderr = "Invalid relative file path"
		return out
	}
	root := filepath.Join(dir, "files")
	if e := os.MkdirAll(root, 0700); e != nil {
		out.Stderr = "File root unavailable"
		return out
	}
	target := filepath.Join(root, filepath.Clean(input.Path))
	current := root
	for _, part := range strings.Split(filepath.Clean(input.Path), string(filepath.Separator)) {
		current = filepath.Join(current, part)
		if info, e := os.Lstat(current); e == nil && info.Mode()&os.ModeSymlink != 0 {
			out.Stderr = "Symlink paths forbidden"
			return out
		}
	}
	if t.Kind == "file.upload" {
		data, e := base64.StdEncoding.DecodeString(input.Data)
		if e != nil || len(data) > 1<<20 || flows.Digest(data) != input.SHA {
			out.Stderr = "File digest/size invalid"
			return out
		}
		if e = os.MkdirAll(filepath.Dir(target), 0700); e != nil {
			out.Stderr = "File directory unavailable"
			return out
		}
		tmp, e := os.CreateTemp(filepath.Dir(target), ".upload-")
		if e != nil {
			out.Stderr = "File staging failed"
			return out
		}
		name := tmp.Name()
		defer os.Remove(name)
		tmp.Chmod(0600)
		_, e = tmp.Write(data)
		closeErr := tmp.Close()
		if e != nil || closeErr != nil || os.Rename(name, target) != nil {
			out.Stderr = "Atomic file save failed"
			return out
		}
		out.Stdout = flows.Digest(data)
	} else {
		info, e := os.Stat(target)
		if e != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
			out.Stderr = "File unavailable or too large"
			return out
		}
		data, e := os.ReadFile(target)
		if e != nil {
			out.Stderr = "File read failed"
			return out
		}
		out.Stdout = base64.StdEncoding.EncodeToString(data)
	}
	out.State = "SUCCESS"
	out.Exit = 0
	return out
}
