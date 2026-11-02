package engineering

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/xuri/excelize/v2"
)

type ModelingPackageBasis struct {
	SourceID string                 `json:"source_id"`
	Revision int                    `json:"revision"`
	Inputs   []ModelingInputVersion `json:"inputs"`
}

func WriteModelingPackage(step int, tables []StandardTableData, basis ModelingPackageBasis) ([]byte, error) {
	raw, err := WriteStepWorkbook(step, tables)
	if err != nil {
		return nil, err
	}
	f, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err = f.NewSheet("_keel"); err != nil {
		return nil, err
	}
	b, err := json.Marshal(basis)
	if err != nil {
		return nil, err
	}
	if err = f.SetCellStr("_keel", "A1", string(b)); err != nil {
		return nil, err
	}
	if err = f.SetCellStr("_keel", "A2", "版本依据，请保留。空白字段由用户补充；保存不代表审核或发布。"); err != nil {
		return nil, err
	}
	if err = f.SetSheetVisible("_keel", false); err != nil {
		return nil, err
	}
	buff, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buff.Bytes(), nil
}

func ReadModelingPackage(input io.Reader, step int) ([]InputPreviewRequest, *ModelingPackageBasis, error) {
	f, err := excelize.OpenReader(input)
	if err != nil {
		return nil, nil, fmt.Errorf("STEP_XLSX_INVALID")
	}
	defer f.Close()
	var basis *ModelingPackageBasis
	for _, sheet := range f.GetSheetList() {
		if sheet != "_keel" {
			continue
		}
		value, e := f.GetCellValue(sheet, "A1")
		if e != nil {
			return nil, nil, e
		}
		basis = &ModelingPackageBasis{}
		if json.Unmarshal([]byte(value), basis) != nil {
			return nil, nil, fmt.Errorf("STEP_PACKAGE_BASIS_INVALID")
		}
		if e = f.DeleteSheet(sheet); e != nil {
			return nil, nil, e
		}
	}
	buff, err := f.WriteToBuffer()
	if err != nil {
		return nil, nil, err
	}
	inputs, err := ReadStepWorkbookCandidates(bytes.NewReader(buff.Bytes()), step)
	return inputs, basis, err
}
