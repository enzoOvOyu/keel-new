package ai

import (
	"embed"
	"errors"
)

//go:embed prompts/*.txt
var prompts embed.FS

func Prompt(task string) (string, error) {
	if task != "sheet" && task != "field" && task != "relation" {
		return "", errors.New("UNKNOWN_PROMPT")
	}
	b, e := prompts.ReadFile("prompts/" + task + "-v1.txt")
	return string(b), e
}
