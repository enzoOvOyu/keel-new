package flows

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
)

// The OSS engine uses a platform credential. Keep its initial surface to orchestration;
// target commands and credentials must remain behind the scoped Job Service.
var sourceExpressions = regexp.MustCompile(`(?s)\{\{(.*?)\}\}`)
var enginePrivate = regexp.MustCompile(`\b(envs|secret|render|read|kv)\b|[A-Za-z_]\w*\s*\(`)

func ValidatePolicy(source, namespace string) error {
	if strings.Contains(source, "PRIVATE KEY-----") {
		return problem(422, "FLOW_USE_CONTROLLED_CREDENTIAL_REFERENCE")
	}
	for _, m := range sourceExpressions.FindAllStringSubmatch(source, -1) {
		if enginePrivate.MatchString(strings.ReplaceAll(m[1], "fromJson(", "(")) {
			return problem(422, "FLOW_ENGINE_PRIVATE_CONTEXT_FORBIDDEN")
		}
	}
	doc, e := FlowDocument(source)
	if e != nil {
		return e
	}
	labels, _ := doc["labels"].([]any)
	for _, label := range labels {
		m, _ := label.(map[string]any)
		if m["key"] == "keel_binding" {
			return problem(422, "FLOW_RESERVED_LABEL")
		}
	}
	inputs, _ := doc["inputs"].([]any)
	controlled := false
	for _, input := range inputs {
		m, _ := input.(map[string]any)
		if m["id"] == "ticket" {
			controlled = true
		}
	}
	if controlled {
		triggers, _ := doc["triggers"].([]any)
		for _, trigger := range triggers {
			m, _ := trigger.(map[string]any)
			if m["disabled"] != true {
				return problem(422, "TARGET_TRIGGER_REQUIRES_SCOPED_JOB_ADAPTER")
			}
		}
	}
	var walk func(any) error
	walk = func(value any) error {
		switch v := value.(type) {
		case []any:
			for _, child := range v {
				if e := walk(child); e != nil {
					return e
				}
			}
		case map[string]any:
			if plugin, ok := v["type"].(string); ok && strings.HasPrefix(plugin, "io.kestra.") {
				allowed := strings.HasPrefix(plugin, "io.kestra.plugin.core.flow.") || contains([]string{"io.kestra.plugin.core.log.Log", "io.kestra.plugin.core.debug.Return", "io.kestra.plugin.core.output.OutputValues", "io.kestra.plugin.core.trigger.Schedule", "io.kestra.plugin.core.trigger.Webhook", "io.kestra.plugin.core.trigger.Flow", "io.kestra.plugin.core.http.Request"}, plugin)
				if !allowed {
					return problem(422, "FLOW_PLUGIN_REQUIRES_SCOPED_ADAPTER")
				}
				if strings.HasSuffix(plugin, ".Subflow") && fmt.Sprint(v["namespace"]) != namespace {
					return problem(422, "SUBFLOW_PROJECT_SCOPE_DENIED")
				}
				if plugin == "io.kestra.plugin.core.http.Request" {
					base := strings.TrimRight(os.Getenv("KEEL_WORKER_CALLBACK_URL"), "/")
					if base == "" || (fmt.Sprint(v["uri"]) != base+"/api/v1/flows/internal/dispatch" && fmt.Sprint(v["uri"]) != base+"/api/v1/flows/internal/results") {
						return problem(422, "FLOW_HTTP_MUST_USE_JOB_SERVICE")
					}
				}
			}
			for _, child := range v {
				if e := walk(child); e != nil {
					return e
				}
			}
		}
		return nil
	}
	return walk(doc)
}

var switchReadOnlyCharacters = regexp.MustCompile(`^[A-Za-z0-9_./:= -]+$`)

func SwitchReadOnly(command string) bool {
	seen := false
	for _, line := range strings.Split(command, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !switchReadOnlyCharacters.MatchString(line) || !(line == "nv show" || strings.HasPrefix(line, "nv show ") || line == "show" || strings.HasPrefix(line, "show ")) {
			return false
		}
		seen = true
	}
	return seen
}

func ValidateTaskParameters(kind, command string) error {
	if kind != "file.upload" && kind != "file.download" {
		return nil
	}
	var in struct {
		Path string `json:"path"`
		Data string `json:"data_base64"`
		SHA  string `json:"sha256"`
	}
	if json.Unmarshal([]byte(command), &in) != nil || in.Path == "" || path.IsAbs(in.Path) || strings.Contains(in.Path, "..") || strings.Contains(in.Path, "\\") {
		return problem(422, "FILE_TASK_PARAMETERS_INVALID")
	}
	if kind == "file.upload" {
		data, e := base64.StdEncoding.DecodeString(in.Data)
		if e != nil || len(data) > 1<<20 || hash(string(data)) != in.SHA {
			return problem(422, "FILE_TASK_DIGEST_INVALID")
		}
	}
	return nil
}
