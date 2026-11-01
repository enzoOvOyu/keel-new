package engineering

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"io"
	"keel.local/dev/internal/bom"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Image is trusted operator configuration, never supplied by a workbook/model.
// The process receives only its immutable input and a fresh output directory.
type LibreOfficeProvider struct{ Image string }

func (p LibreOfficeProvider) Capabilities(ctx context.Context) (CalculationCapabilities, error) {
	caps := CalculationCapabilities{ID: "libreoffice-isolated", Version: "unavailable", Available: false, Formulas: []string{"LibreOffice Calc supported OOXML functions"}, SupportsNamedRanges: true, SupportsExternalLinks: false}
	if p.Image == "" {
		return caps, nil
	}
	c, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	data, e := exec.CommandContext(c, "docker", "run", "--rm", "--network", "none", "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--tmpfs", "/tmp:rw,nosuid,nodev,size=64m", "--user", strconv.Itoa(os.Getuid())+":"+strconv.Itoa(os.Getgid()), "--env", "HOME=/tmp", p.Image, "--version").Output()
	if e != nil {
		return caps, fmt.Errorf("CALC_PROVIDER_UNAVAILABLE")
	}
	var v struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &v) != nil || v.Version == "" {
		return caps, fmt.Errorf("CALC_PROVIDER_UNAVAILABLE")
	}
	caps.Available = true
	caps.Version = v.Version
	return caps, nil
}

func (p LibreOfficeProvider) Calculate(ctx context.Context, q SpreadsheetCalculationRequest) (CalculationRun, error) {
	run := CalculationRun{ID: uuid.NewString(), ProjectID: q.Source.ProjectID, ModuleInstanceID: q.Revision.ModuleInstanceID, RevisionID: q.Revision.ID, SourceSHA256: q.Source.SHA256, InputDigest: Digest([]any{q.Source.SHA256, q.RequestedCells, q.Parameters}), ProviderID: "libreoffice-isolated", ProviderVersion: "unavailable", Status: "running", Compatibility: "unknown", Inputs: q.Parameters, Outputs: []NamedValue{}, Formulas: []FormulaEvidence{}, Errors: []CalculationError{}, StartedAt: time.Now().UTC()}
	finish := func(code string) (CalculationRun, error) {
		now := time.Now().UTC()
		run.FinishedAt = &now
		if code != "" {
			run.Status = "failed"
			run.Errors = append(run.Errors, CalculationError{Code: code, Message: code})
			return run, fmt.Errorf("%s", code)
		}
		return run, nil
	}
	caps, e := p.Capabilities(ctx)
	if e != nil || !caps.Available {
		return finish("CALC_PROVIDER_UNAVAILABLE")
	}
	run.ProviderVersion = caps.Version
	if len(q.Parameters) > 0 {
		return finish("WORKBOOK_PARAMETER_BINDINGS_REQUIRED")
	}
	stream, e := q.OpenSource(ctx)
	if e != nil {
		return finish("CALC_SOURCE_UNAVAILABLE")
	}
	defer stream.Close()
	data, e := io.ReadAll(io.LimitReader(stream, 20*1024*1024+1))
	if e != nil || len(data) > 20*1024*1024 {
		return finish("CALC_SOURCE_SIZE_LIMIT")
	}
	hash := sha256.Sum256(data)
	if hex.EncodeToString(hash[:]) != q.Source.SHA256 {
		return finish("CALC_SOURCE_HASH_MISMATCH")
	}
	book, e := excelize.OpenReader(bytes.NewReader(data))
	if e != nil {
		return finish("CALC_SOURCE_INVALID")
	}
	defer book.Close()
	wb, e := bom.Parse(data)
	if e != nil {
		return finish("CALC_SOURCE_INVALID")
	}
	run.Formulas = FormulaGraph(wb, book.GetDefinedName())
	requested := map[string]bool{}
	for _, c := range q.RequestedCells {
		requested[c.SheetID+"!"+c.Address] = true
	}
	if len(requested) > 0 {
		keep := []FormulaEvidence{}
		for _, f := range run.Formulas {
			if requested[f.Cell.SheetID+"!"+f.Cell.Address] {
				keep = append(keep, f)
			}
		}
		if len(keep) != len(requested) {
			return finish("CALC_FORMULA_CELL_NOT_FOUND")
		}
		run.Formulas = keep
	}
	dir, e := os.MkdirTemp("", "keel-calc-")
	if e != nil {
		return finish("CALC_STORAGE_FAILED")
	}
	defer os.RemoveAll(dir)
	input := filepath.Join(dir, "source.xlsx")
	output := filepath.Join(dir, "output")
	if os.WriteFile(input, data, 0400) != nil || os.Mkdir(output, 0700) != nil {
		return finish("CALC_STORAGE_FAILED")
	}
	name := "keel-calc-" + run.ID
	// Removing by this generated name also terminates the sandbox after timeout;
	// cancelling a docker CLI alone would leave its container alive.
	defer func() {
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = exec.CommandContext(c, "docker", "rm", "-f", name).Run()
	}()
	c, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	args := []string{"run", "--rm", "--name", name, "--network", "none", "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--pids-limit", "128", "--memory", "1g", "--cpus", "2", "--tmpfs", "/tmp:rw,nosuid,nodev,size=256m", "--user", strconv.Itoa(os.Getuid()) + ":" + strconv.Itoa(os.Getgid()), "--env", "HOME=/tmp", "--mount", "type=bind,src=" + input + ",dst=/input/source.xlsx,readonly", "--mount", "type=bind,src=" + output + ",dst=/output", p.Image}
	if _, e = exec.CommandContext(c, "docker", args...).Output(); e != nil {
		if c.Err() != nil {
			return finish("CALC_TIMEOUT")
		}
		return finish("CALC_EXECUTION_FAILED")
	}
	result, e := excelize.OpenFile(filepath.Join(output, "recalculated.xlsx"))
	if e != nil {
		return finish("CALC_RESULT_MISSING")
	}
	defer result.Close()
	run.Status = "succeeded"
	run.Compatibility = "compatible"
	for i := range run.Formulas {
		f := &run.Formulas[i]
		value, err := result.GetCellValue(f.Cell.SheetID, f.Cell.Address, excelize.Options{RawCellValue: true})
		kind, typeErr := result.GetCellType(f.Cell.SheetID, f.Cell.Address)
		code := ""
		if err != nil || typeErr != nil {
			code = "CALC_CELL_READ_FAILED"
		} else if kind == excelize.CellTypeError || strings.HasPrefix(value, "Err:") {
			code = "SPREADSHEET_FORMULA_ERROR"
			switch value {
			case "#DIV/0!", "#VALUE!", "#REF!", "#NAME?", "#N/A", "#NUM!", "#NULL!", "#SPILL!", "#CALC!":
				code = value
			}
		} else if f.DependencyStatus != "complete" {
			code = "FORMULA_DEPENDENCY_" + strings.ToUpper(f.DependencyStatus)
		}
		if code != "" {
			f.ErrorCode = &code
			f.Compatibility = "partial"
			run.Status = "partial"
			run.Compatibility = "partial"
			run.Errors = append(run.Errors, CalculationError{Code: code, Message: code, Cell: &f.Cell})
		} else {
			f.Compatibility = "compatible"
		}
		if err == nil && typeErr == nil && kind != excelize.CellTypeError {
			f.RecalculatedValue = SpreadsheetValue(value)
		}
	}
	return finish("")
}
