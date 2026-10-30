package bom

import (
	"strconv"
	"strings"
)

var aliases = map[string]string{"serverlabelname": "hostname", "networklabelname": "hostname", "devicemodel": "model", "devicetype": "category", "customerrackname": "rack", "sn": "serial", "serial": "serial", "serialnumber": "serial", "序列号": "serial", "设备序列号": "serial", "hostname": "hostname", "devicename": "hostname", "设备名称": "hostname", "设备名": "hostname", "model": "model", "设备型号": "model", "型号": "model", "category": "category", "设备类型": "category", "物料类型": "category", "port": "port_index", "portindex": "port_index", "端口": "port_index", "rack": "rack", "机柜": "rack", "ip": "management_ip", "管理ip": "management_ip", "mac": "mac", "mtu": "mtu", "vlan": "vlan", "vrf": "vrf", "vni": "vni", "asn": "asn"}

type Mapping struct {
	ID         string  `json:"id"`
	Sheet      string  `json:"sheet_name"`
	Cell       string  `json:"source_cell"`
	Header     string  `json:"original_value"`
	Field      string  `json:"candidate_field"`
	Confidence float64 `json:"confidence"`
	Status     string  `json:"status"`
	Reason     string  `json:"reasoning"`
	Prompt     string  `json:"prompt_version"`
	Schema     string  `json:"schema_version"`
}

func Rules(w Workbook) []Mapping {
	out := []Mapping{}
	for _, s := range w.Sheets {
		for _, c := range s.Cells {
			if c.Row > 30 || c.Sensitive {
				continue
			}
			key := strings.ToLower(strings.NewReplacer(" ", "", "_", "", "-", "").Replace(c.Value))
			if target, ok := aliases[key]; ok {
				out = append(out, Mapping{s.Name + ":" + c.Ref, s.Name, c.Ref, c.Value, target, 1, "candidate", "Exact alias dictionary; requires human confirmation", "rules/v1", "mapping/v1"})
			}
		}
	}
	return out
}
func Draft(w Workbook) []Item {
	out := []Item{}
	mappings := Rules(w)
	for _, s := range w.Sheets {
		headers := map[int]Mapping{}
		headerRow := 0
		for _, m := range mappings {
			if m.Sheet == s.Name {
				for _, c := range s.Cells {
					if c.Ref == m.Cell {
						headers[c.Column] = m
						if c.Row > headerRow {
							headerRow = c.Row
						}
					}
				}
			}
		}
		if len(headers) < 2 {
			continue
		}
		rows := map[int]*Item{}
		order := []int{}
		for _, c := range s.Cells {
			if c.Row <= headerRow || c.Sensitive {
				continue
			}
			m, ok := headers[c.Column]
			if !ok {
				continue
			}
			item := rows[c.Row]
			if item == nil {
				item = &Item{Fields: map[string]string{}, Sources: map[string]Cell{}, Sheet: s.Name}
				rows[c.Row] = item
				order = append(order, c.Row)
			}
			item.Fields[m.Field] = c.Value
			item.Sources[m.Field] = c
			switch m.Field {
			case "serial":
				item.Serial = c.Value
			case "hostname":
				item.Hostname = c.Value
			case "model":
				item.Model = c.Value
			case "category":
				item.Category = strings.ToLower(c.Value)
			case "port_index":
				item.Port = c.Value
			}
		}
		for _, r := range order {
			i := rows[r]
			if i.Serial == "" && i.Hostname == "" {
				continue
			}
			if i.Category == "" {
				i.Category = "other"
			}
			i.UID = StableID("candidate:" + s.Name + ":" + strconv.Itoa(r))
			out = append(out, *i)
		}
	}
	return out
}
