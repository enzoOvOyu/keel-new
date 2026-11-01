package engineering

import (
	"encoding/csv"
	"strings"
	"testing"
)

func TestStepInputCatalog(t *testing.T) {
	c := StepInputCatalog()
	seen := map[string]bool{}
	steps := map[int]bool{}
	for _, table := range c.Tables {
		if seen[table.ID] {
			t.Fatal("duplicate table", table.ID)
		}
		seen[table.ID] = true
		steps[table.Step] = true
		fields := map[string]bool{}
		for _, f := range table.Fields {
			if f.Code == "" || f.Label == "" || fields[f.Code] {
				t.Fatal("invalid field", table.ID, f)
			}
			fields[f.Code] = true
		}
	}
	if len(steps) != 5 {
		t.Fatal("missing step")
	}
	var b strings.Builder
	w := csv.NewWriter(&b)
	headers := []string{}
	for _, f := range c.Tables[0].Fields {
		headers = append(headers, f.Label)
	}
	w.Write(headers)
	w.Write([]string{"00001234567890123456", "00:11:22:33:44:55", "Ethernet1", "00002", "R1-U10", "00:11:22:33:44:66", "Ethernet2", "客户定义端接"})
	w.Flush()
	got := PreviewStepInput(InputPreviewRequest{"terminations", b.String()})
	if !got.Valid || got.Rows[0][0] != "00001234567890123456" {
		t.Fatalf("identity changed: %+v", got)
	}
	bad := PreviewStepInput(InputPreviewRequest{"terminations", strings.Replace(b.String(), "00:11:22:33:44:55", "invalid", 1)})
	if bad.Valid || len(bad.Issues) != 1 || bad.Issues[0].Field != "a_mac" {
		t.Fatalf("bad MAC accepted: %+v", bad)
	}
	missing := PreviewStepInput(InputPreviewRequest{"terminations", strings.Replace(b.String(), "00002,", ",", 1)})
	if missing.Valid {
		t.Fatal("missing SN accepted")
	}
	changed := PreviewStepInput(InputPreviewRequest{"terminations", strings.Replace(b.String(), "a端SN", "本端序列号", 1)})
	if changed.Valid {
		t.Fatal("nonstandard header accepted")
	}
}
func TestStepInputInvalidNumbers(t *testing.T) {
	for _, value := range []string{"0", "-1", "NaN", "+Inf", "1.2", "1e3"} {
		got := PreviewStepInput(InputPreviewRequest{"network_asns", "设备或域引用,ASN,来源依据\nS1," + value + ",客户规划\n"})
		if got.Valid {
			t.Fatal("invalid ASN accepted", value)
		}
	}
	if PreviewStepInput(InputPreviewRequest{"missing", "a\nb"}).Valid {
		t.Fatal("unknown table")
	}
}
