package kickoff

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
)

//go:embed catalog.json
var catalogue []byte

type Field struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
}
type Table struct {
	ID     string  `json:"id"`
	Fields []Field `json:"fields"`
}

func ValidateInput(table string, rows []map[string]string) error {
	var tables []Table
	if err := json.Unmarshal(catalogue, &tables); err != nil {
		return err
	}
	var definition *Table
	for i := range tables {
		if tables[i].ID == table {
			definition = &tables[i]
			break
		}
	}
	if definition == nil {
		return fmt.Errorf("KICKOFF_TABLE_UNKNOWN")
	}
	fields := map[string]Field{}
	for _, f := range definition.Fields {
		fields[f.Key] = f
	}
	for i, row := range rows {
		for key, value := range row {
			field, ok := fields[key]
			if !ok {
				return fmt.Errorf("第%d行存在未知字段%s", i+1, key)
			}
			if strings.TrimSpace(value) == "" {
				continue
			}
			if field.Kind == "number" || field.Kind == "positive" || field.Kind == "integer" {
				n, e := strconv.ParseFloat(value, 64)
				if e != nil || math.IsNaN(n) || math.IsInf(n, 0) || field.Kind == "positive" && n <= 0 || field.Kind == "integer" && (n < 0 || math.Trunc(n) != n) {
					return fmt.Errorf("第%d行%s格式无效", i+1, field.Label)
				}
			}
			if strings.Contains(key, "mac") {
				if _, e := net.ParseMAC(value); e != nil {
					return fmt.Errorf("第%d行MAC格式无效", i+1)
				}
			}
			if strings.Contains(key, "credential") && !strings.HasPrefix(value, "vault://") {
				return fmt.Errorf("凭据只能提供受控引用")
			}
		}
	}
	return nil
}
