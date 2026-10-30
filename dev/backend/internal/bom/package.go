package bom

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"strings"
)

func safeCSV(v string) string {
	if strings.ContainsAny(strings.TrimLeft(v, " \t")[:min(1, len(strings.TrimLeft(v, " \t")))], "=+-@") {
		return "'" + v
	}
	return v
}
func Package(items []Item, importID, project string) ([]byte, error) {
	paths := []string{"server/servers.csv", "server/server_components.csv", "server/server_ports.csv", "fabric/switches.csv", "fabric/switch_ports.csv", "fabric/optic_modules.csv", "fabric/cables.csv"}
	groups := map[string][]Item{}
	ids := map[string]Item{}
	for _, i := range items {
		ids[i.UID] = i
	}
	for _, i := range items {
		p := "server/server_components.csv"
		switch i.Category {
		case "server":
			p = paths[0]
		case "switch":
			p = paths[3]
		case "port":
			p = paths[2]
			if ids[i.Parent].Category == "switch" {
				p = paths[4]
			}
		case "optic_module":
			p = paths[5]
		case "cable":
			p = paths[6]
		}
		groups[p] = append(groups[p], i)
	}
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	manifest := map[string]any{"schema_version": "keel-bom-openapi/v1", "project_id": project, "bom_import_batch_id": importID, "files": []any{}}
	files := []any{}
	header := []string{"item_uid", "category", "sub_type", "model_version_ref", "serial", "position", "parent_uid", "endpoint_a_uid", "endpoint_b_uid", "port_index", "port_name", "port_role", "rail", "pod", "role", "source_of_record", "status", "hostname", "attributes", "source"}
	for _, p := range paths {
		var buf bytes.Buffer
		cw := csv.NewWriter(&buf)
		cw.Write(header)
		for _, i := range groups[p] {
			pos := map[string]string{}
			for _, k := range []string{"site", "pod", "rack", "u_start", "u_height", "rack_height"} {
				if v := i.Fields[k]; v != "" {
					pos[k] = v
				}
			}
			position, _ := json.Marshal(pos)
			attributes, _ := json.Marshal(i.Fields)
			source, _ := json.Marshal(i.Sources)
			row := []string{i.UID, i.Category, i.Subtype, i.Fields["model_version_ref"], i.Serial, string(position), i.Parent, i.A, i.B, i.Port, i.Fields["port_name"], i.Fields["port_role"], i.Fields["rail"], i.Fields["pod"], i.Fields["role"], "bom_import", i.Status, i.Hostname, string(attributes), string(source)}
			for j := range row {
				row[j] = safeCSV(row[j])
			}
			if e := cw.Write(row); e != nil {
				return nil, e
			}
		}
		cw.Flush()
		if e := cw.Error(); e != nil {
			return nil, e
		}
		w, e := z.Create(p)
		if e != nil {
			return nil, e
		}
		w.Write(buf.Bytes())
		h := sha256.Sum256(buf.Bytes())
		files = append(files, map[string]any{"path": p, "sha256": hex.EncodeToString(h[:]), "rows": len(groups[p])})
	}
	manifest["files"] = files
	b, _ := json.MarshalIndent(manifest, "", "  ")
	w, e := z.Create("manifest.json")
	if e != nil {
		return nil, e
	}
	w.Write(b)
	if e = z.Close(); e != nil {
		return nil, e
	}
	return out.Bytes(), nil
}
