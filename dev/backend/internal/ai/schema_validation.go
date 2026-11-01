package ai

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
)

func validateExtractionSchema(ctx context.Context, raw []byte) error {
	return validateEmbeddedSchema(ctx, raw, "prompts/validate_extraction.py", "AI_EXTRACTION_SCHEMA_INVALID")
}
func validateEngineeringSchema(ctx context.Context, raw []byte) error {
	return validateEmbeddedSchema(ctx, raw, "prompts/validate_engineering.py", "AI_ENGINEERING_SCHEMA_INVALID")
}
func validateEmbeddedSchema(ctx context.Context, raw []byte, scriptName, errorCode string) error {
	script, e := prompts.ReadFile(scriptName)
	if e != nil {
		return errors.New("AI_SCHEMA_VALIDATOR_UNAVAILABLE")
	}
	python := os.Getenv("KEEL_BOM_PYTHON")
	if python == "" {
		python = "python3"
	}
	command := exec.CommandContext(ctx, python, "-c", string(script))
	command.Stdin = bytes.NewReader(raw)
	var diagnostic bytes.Buffer
	command.Stdout = &diagnostic
	if err := command.Run(); err != nil {
		if diagnostic.Len() > 0 {
			return errors.New(errorCode + ": " + diagnostic.String())
		}
		return errors.New("AI_SCHEMA_VALIDATOR_UNAVAILABLE")
	}
	return nil
}
