package ai

import (
	"context"
	"time"
)

type Evaluation struct {
	Mode                 string   `json:"mode"`
	Dataset              string   `json:"dataset"`
	FieldAccuracy        *float64 `json:"field_accuracy"`
	KeyRecall            *float64 `json:"key_field_recall"`
	SheetAccuracy        *float64 `json:"sheet_accuracy"`
	RelationAccuracy     *float64 `json:"relation_accuracy"`
	ManualCorrectionRate *float64 `json:"manual_correction_rate"`
	DurationMS           int64    `json:"duration_ms"`
	TokenCost            *float64 `json:"token_cost"`
	Errors               []string `json:"errors"`
	Notes                string   `json:"notes"`
}

func Evaluate(ctx context.Context, a Adapter, mode string) Evaluation {
	start := time.Now()
	e := Evaluation{Mode: mode, Dataset: "synthetic-labelled-golden/v2", Errors: []string{}, Notes: "Independent synthetic golden cases. Rules/Mock scores are not real AI accuracy. Correction rate is golden-required correction fraction, not observed operator effort. Token cost remains unknown without provider usage evidence."}
	gold := map[string]string{"SN": "serial", "Hostname": "hostname", "Device Model": "model", "MTU": "mtu", "VLAN": "vlan", "MAC": "mac"}
	headers := []string{"SN", "Hostname", "Device Model", "MTU", "VLAN", "MAC"}
	req := Request{Task: "field", Headers: headers, PromptVersion: "field/v1", SchemaVersion: "mapping/v1", SourceFile: "synthetic-golden", SheetName: "synthetic-assets"}
	candidates, err := a.Suggest(ctx, req)
	if err != nil {
		e.Errors = append(e.Errors, err.Error())
	} else {
		correct := 0
		seen := map[string]bool{}
		for _, c := range candidates {
			if gold[c.Original] == c.Field && !seen[c.Original] {
				correct++
				seen[c.Original] = true
			}
		}
		score := float64(correct) / float64(len(gold))
		e.FieldAccuracy = &score
		e.KeyRecall = &score
		correction := 1 - score
		e.ManualCorrectionRate = &correction
	}
	sheetCorrect := 0
	for _, test := range []struct {
		Headers []string
		Purpose string
	}{{[]string{"SN", "Hostname", "Model"}, "assets"}, {[]string{"本端设备名称", "对端设备名称", "本端端口名称", "对端端口名称"}, "cabling"}, {[]string{"synthetic unclassified header"}, "unknown"}} {
		req.Task = "sheet"
		req.Headers = test.Headers
		req.PromptVersion = "sheet/v1"
		out, err := a.Suggest(ctx, req)
		if err != nil {
			e.Errors = append(e.Errors, err.Error())
		} else if len(out) == 1 && out[0].Normalized == test.Purpose {
			sheetCorrect++
		}
	}
	ss := float64(sheetCorrect) / 3
	e.SheetAccuracy = &ss
	relationCorrect := 0
	for _, test := range []struct {
		Ref       ExplicitRef
		Expected  string
		Ambiguous bool
	}{{ExplicitRef{"synthetic-explicit-A", []string{"synthetic-id-A"}}, "synthetic-id-A", false}, {ExplicitRef{"synthetic-missing", []string{}}, "", true}, {ExplicitRef{"synthetic-multiple", []string{"synthetic-id-B", "synthetic-id-C"}}, "", true}} {
		req.Task = "relation"
		req.PromptVersion = "relation/v1"
		req.Headers = nil
		req.ExplicitRefs = []ExplicitRef{test.Ref}
		out, err := a.Suggest(ctx, req)
		if err != nil {
			e.Errors = append(e.Errors, err.Error())
		} else if len(out) == 1 && out[0].Normalized == test.Expected && out[0].Ambiguity == test.Ambiguous {
			relationCorrect++
		}
	}
	rs := float64(relationCorrect) / 3
	e.RelationAccuracy = &rs
	e.DurationMS = time.Since(start).Milliseconds()
	return e
}
