package main

import (
	"github.com/xuri/excelize/v2"
	"os"
)

func main() {
	f := excelize.NewFile()
	name := "Sheet1"
	headers := []string{"category", "SN", "Hostname", "Model", "parent_uid", "port_index", "endpoint_a_uid", "endpoint_b_uid", "speed", "media_type"}
	rows := [][]string{headers, {"server", "SYNTHETIC-REAL-API-SERVER", "synthetic-server", "synthetic-server-model"}, {"switch", "SYNTHETIC-REAL-API-SWITCH", "synthetic-switch", "synthetic-switch-model"}, {"port", "", "", "", "server-ref", "1", "", "", "100G"}, {"port", "", "", "", "switch-ref", "1", "", "", "100Gbps"}, {"cable", "", "", "", "", "", "port-a-ref", "port-b-ref", "", "DAC"}}
	for r, row := range rows {
		for c, v := range row {
			axis, _ := excelize.CoordinatesToCellName(c+1, r+1)
			f.SetCellValue(name, axis, v)
		}
	}
	if e := f.SaveAs(os.Args[1]); e != nil {
		panic(e)
	}
}
