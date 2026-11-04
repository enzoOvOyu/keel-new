package main

import (
	"keel.local/dev/internal/ai"
	"keel.local/dev/internal/engineering"
	"keel.local/dev/internal/kickoff"
	"regexp"
	"strconv"
	"strings"
)

var kickoffStartU = regexp.MustCompile(`(?i)^\s*(\d+)\s*u\s*-?\s*$`)

// All readers and adoption consume the same prepared candidates. Original rows
// and cell evidence remain immutable in the analysis artifact.
func kickoffPrepareCandidates(result *ai.KickoffResult) {
	identities := map[string]bool{}
	for _, c := range result.Candidates {
		if c.TableID == "devices" {
			for _, r := range kickoffAgentValidRows(c) {
				for _, f := range []string{"device_ref", "label", "serial"} {
					if r[f] != "" {
						identities[kickoff.NormalizeDeviceRef(r[f])] = true
					}
				}
			}
		}
	}
	for ci := range result.Candidates {
		c := &result.Candidates[ci]
		if c.TableID != "placements" {
			continue
		}
		issues := []engineering.InputProblem{}
		for _, issue := range c.Issues {
			if issue.Code == "KICKOFF_PLACEMENT_UNMATCHED" {
				continue
			}
			if issue.Code == "KICKOFF_INPUT_INVALID" && strings.Contains(issue.Message, "起始U格式无效") {
				continue
			}
			issues = append(issues, issue)
		}
		for i, original := range c.Rows {
			row := map[string]string{}
			for k, v := range original {
				row[k] = v
			}
			c.Rows[i] = row
			if !identities[kickoff.NormalizeDeviceRef(row["device_id"])] {
				issues = append(issues, engineering.InputProblem{Row: i + 1, Field: "device_id", Code: "KICKOFF_PLACEMENT_UNMATCHED", Message: "此位置未匹配端接设备，保留来源待确认，不纳入本次采用"})
				continue
			}
			raw := strings.TrimSpace(row["start_u"])
			if m := kickoffStartU.FindStringSubmatch(raw); m != nil {
				row["start_u"] = m[1]
				raw = m[1]
			}
			if raw != "" {
				n, e := strconv.Atoi(raw)
				if e != nil || n <= 0 {
					row["start_u"] = ""
					issues = append(issues, engineering.InputProblem{Row: i + 1, Field: "start_u", Code: "KICKOFF_POSITION_NEEDS_INPUT", Message: "保留机柜关联，起始U待确认；原值见来源证据"})
				}
			}
		}
		c.Issues = issues
	}
}
