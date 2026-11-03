package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"keel.local/dev/internal/engineering"
	fab "keel.local/dev/internal/fabric"
	"net/http"
	"strings"
	"time"
)

func (s *Server) fabricCompile(state *fab.State, input *fabricPublishedInput, c fab.Command) (string, error) {
	bp := fabricActiveBlueprint(state)
	if bp == nil || bp.Status != "COMMITTED" || bp.Input != input.ID {
		return "", studioError("FABRIC_COMMITTED_BLUEPRINT_REQUIRED")
	}
	if c.Domain != "SWITCH" && c.Domain != "HOST" && c.Domain != "MULTIPLANE" {
		return "", studioError("FABRIC_DOMAIN_INVALID")
	}
	if len(c.IDs) == 0 {
		return "", studioError("FABRIC_TARGETS_REQUIRED")
	}
	build := fab.Build{ID: uuid.NewString(), Blueprint: bp.ID, Input: input.ID, Profiles: []string{}, Compiler: "keel.network-intent/1", Status: "BUILT", Artifacts: []string{}, CreatedAt: time.Now().UTC(), Targets: []string{}}
	seen := map[string]bool{}
	for _, id := range c.IDs {
		if seen[id] {
			return "", studioError("FABRIC_DUPLICATE_TARGET")
		}
		seen[id] = true
		var kind string
		for _, a := range input.Assets {
			if a.ID == id {
				kind = a.Type
				break
			}
		}
		if kind == "" || kind != "server" && kind != "switch" && kind != "device" {
			return "", studioError("FABRIC_TARGET_OUTSIDE_PUBLISHED_SCOPE")
		}
		if (c.Domain == "SWITCH") != (kind == "switch") {
			return "", studioError("FABRIC_TARGET_DOMAIN_MISMATCH")
		}
		var profile *fab.Profile
		for i := len(state.Profiles) - 1; i >= 0; i-- {
			p := &state.Profiles[i]
			if p.Domain == c.Domain && p.Status == "PUBLISHED" && (c.Profile == "" || p.ID+"@"+p.Version == c.Profile) {
				profile = p
				break
			}
		}
		if profile == nil {
			return "", studioError("FABRIC_PUBLISHED_PROFILE_REQUIRED")
		}
		a := fab.Artifact{ID: uuid.NewString(), Device: id, Domain: c.Domain, Format: "application/vnd.keel.fabric-intent+json", Sources: []string{input.ID, input.ModelRevision}, Capability: "unknown", Validation: "UNSUPPORTED", Changes: []fab.SemanticChange{}, Unsupported: []string{"NATIVE_NOS_RENDERER_NOT_BOUND", "DEVICE_CAPABILITY_EVIDENCE_REQUIRED"}, Missing: []string{}, Profile: profile.ID, ProfileVersion: profile.Version, Input: input.ID, Parameters: fab.Parameters{Peers: []fab.Peer{}, Sources: []string{input.ModelRevision}}}
		iface := map[string]bool{}
		portIDs := map[string]bool{}
		for _, asset := range input.Assets {
			if asset.Type == "port" && (fmt.Sprint(asset.Fields["device_id"]) == id || fmt.Sprint(asset.Fields["parent_uid"]) == id) {
				portIDs[asset.ID] = true
			}
		}
		plans := []engineering.NetworkPlan{}
		for _, p := range input.NetworkPlans {
			out := p
			out.DeviceRoles = nil
			out.InterfaceMappings = nil
			out.Allocations = nil
			out.BGP = nil
			out.BFD = nil
			out.QoS = nil
			out.AdaptiveRouting = nil
			out.EVPNVXLAN = nil
			for _, v := range p.DeviceRoles {
				if v.EquipmentID == id {
					out.DeviceRoles = append(out.DeviceRoles, v)
				}
			}
			for _, v := range p.InterfaceMappings {
				if portIDs[v.PhysicalPortID] {
					out.InterfaceMappings = append(out.InterfaceMappings, v)
					iface[v.ID] = true
				}
			}
			for _, v := range p.Allocations {
				if v.EntityID == id {
					out.Allocations = append(out.Allocations, v)
				}
			}
			for _, v := range p.BGP {
				if iface[v.AInterfaceID] || iface[v.ZInterfaceID] {
					out.BGP = append(out.BGP, v)
				}
			}
			for _, v := range p.QoS {
				for _, intf := range v.InterfaceIDs {
					if iface[intf] {
						out.QoS = append(out.QoS, v)
						break
					}
				}
			}
			if len(out.DeviceRoles)+len(out.InterfaceMappings)+len(out.Allocations) > 0 {
				plans = append(plans, out)
			}
		}
		if len(plans) == 0 {
			a.Missing = append(a.Missing, "PUBLISHED_NETWORK_INTENT_FOR_TARGET_REQUIRED")
			a.Validation = "BLOCKED"
		}
		if kind == "device" {
			a.Missing = append(a.Missing, "DEVICE_TYPE_REQUIRED")
			a.Validation = "BLOCKED"
		}
		document := map[string]any{"schema": "keel.fabric.intent/v1", "device_id": id, "source_publication": input.ID, "input_digest": input.Digest, "profile": profile.ID + "@" + profile.Version, "network_plans": plans, "execution_eligible": false, "unsupported": a.Unsupported, "missing": a.Missing}
		raw, e := json.MarshalIndent(document, "", "  ")
		if e != nil {
			return "", e
		}
		a.Content = string(raw)
		a.Hash = fabricDigest(document)
		a.ContentRef = "fabric:artifact:" + a.ID
		var previous *fab.Artifact
		for i := len(state.Artifacts) - 1; i >= 0; i-- {
			if state.Artifacts[i].Device == id && state.Artifacts[i].Domain == c.Domain {
				previous = &state.Artifacts[i]
				break
			}
		}
		if previous == nil {
			a.Changes = append(a.Changes, fab.SemanticChange{Field: "network_intent", After: a.Hash, Operation: "ADD", Source: input.ModelRevision})
		} else if previous.Hash != a.Hash {
			a.Changes = append(a.Changes, fab.SemanticChange{Field: "network_intent", Before: previous.Hash, After: a.Hash, Operation: "MODIFY", Source: input.ModelRevision})
		}
		state.Artifacts = append(state.Artifacts, a)
		build.Artifacts = append(build.Artifacts, a.ID)
		build.Targets = append(build.Targets, id)
		ref := profile.ID + "@" + profile.Version
		found := false
		for _, v := range build.Profiles {
			found = found || v == ref
		}
		if !found {
			build.Profiles = append(build.Profiles, ref)
		}
		build.Status = "INVALID"
	}
	state.Builds = append(state.Builds, build)
	for i := range state.Changes {
		if state.Changes[i].Status == "APPROVED" || state.Changes[i].Status == "IN_REVIEW" {
			state.Changes[i].Status = "STALE"
		}
	}
	return build.ID, nil
}
func (s *Server) fabricValidate(state *fab.State, input *fabricPublishedInput) (string, error) {
	build := fabricActiveBuild(state)
	if build == nil || build.Status == "STALE" || build.Input != input.ID {
		return "", studioError("FABRIC_CURRENT_BUILD_REQUIRED")
	}
	v := fab.Validation{ID: uuid.NewString(), Build: build.ID, Status: "PASS", Findings: []fab.Finding{}, Evidence: []string{}, CreatedAt: time.Now().UTC()}
	add := func(device, domain, status, message, source string) {
		f := fab.Finding{ID: uuid.NewString(), Device: device, Check: strings.ToLower(strings.ReplaceAll(domain, " ", "-")), Domain: domain, Status: status, Message: message, Required: true, Source: source, Evidence: source}
		v.Findings = append(v.Findings, f)
		if status == "PASS" {
			v.Passed++
		} else if status == "UNSUPPORTED" {
			v.Unsupported++
			v.Blocked++
			v.Status = "BLOCKED"
		} else if status == "WARNING" {
			v.Warnings++
		} else {
			v.Blocked++
			v.Status = "BLOCKED"
		}
	}
	for _, id := range build.Artifacts {
		var a *fab.Artifact
		for i := range state.Artifacts {
			if state.Artifacts[i].ID == id {
				a = &state.Artifacts[i]
				break
			}
		}
		if a == nil {
			return "", studioError("FABRIC_ARTIFACT_REFERENCE_INVALID")
		}
		if len(a.Missing) > 0 {
			add(a.Device, "Input", "BLOCKED", strings.Join(a.Missing, ", "), input.ModelRevision)
		} else {
			add(a.Device, "Input", "PASS", "产物引用当前已发布范围与资产身份", input.ID)
		}
		add(a.Device, "Capability", "UNSUPPORTED", "缺少精确型号/NOS/ASIC能力证据及经验证的原生Renderer；意图JSON不可执行", a.Capability)
		add(a.Device, "Physical", "NOT_RUN", "尚无本阶段当前目标的交换机互联/端口就绪证据；不等待装机后主机证据", input.ModelRevision)
		if len(a.Missing) > 0 {
			add(a.Device, "Logical", "BLOCKED", "缺少该目标已发布的网络意图，不能判定网络配置一致", input.ModelRevision)
		} else {
			add(a.Device, "Logical", "NOT_RUN", "已读取规划；独立逻辑一致性校验器尚未接入", a.ContentRef)
		}
		add(a.Device, "QoS / RoCE", "NOT_RUN", "需要已批准模式、基线及版本适配证据；不填默认PFC/ECN阈值", input.ModelRevision)
		add(a.Device, "Operational Health", "NOT_RUN", "尚无设备实际运行和独立采集证据", build.ID)
		add(a.Device, "Change Safety", "NOT_RUN", "需冻结冗余域、任务版本、变更窗口和独立签署", build.ID)
	}
	v.Evidence = append(v.Evidence, input.ID, build.ID)
	state.Validations = append(state.Validations, v)
	for i := range state.Changes {
		if state.Changes[i].Build == build.ID {
			state.Changes[i].Status = "STALE"
		}
	}
	return v.ID, nil
}
func (s *Server) fabricCreateChange(ctx context.Context, r *http.Request, state *fab.State, input *fabricPublishedInput, c fab.Command) (string, error) {
	build := fabricActiveBuild(state)
	if build == nil || build.Status != "BUILT" || len(state.Validations) == 0 {
		return "", studioError("FABRIC_VALIDATED_EXECUTABLE_BUILD_REQUIRED")
	}
	v := state.Validations[len(state.Validations)-1]
	if v.Build != build.ID || v.Blocked > 0 || (v.Status != "PASS" && v.Status != "WARNING") {
		return "", studioError("FABRIC_REQUIRED_CHECKS_NOT_PASSED")
	}
	if strings.TrimSpace(c.Purpose) == "" || strings.TrimSpace(c.Window) == "" {
		return "", studioError("FABRIC_PURPOSE_AND_WINDOW_REQUIRED")
	}
	if v.Warnings > 0 && strings.TrimSpace(c.Ack) == "" {
		return "", studioError("FABRIC_WARNING_ACK_REQUIRED")
	}
	targets := c.IDs
	if len(targets) == 0 {
		targets = build.Targets
	}
	allowed := map[string]bool{}
	for _, id := range build.Targets {
		allowed[id] = true
	}
	seen := map[string]bool{}
	for _, id := range targets {
		if !allowed[id] || seen[id] {
			return "", studioError("FABRIC_TARGET_SCOPE_INVALID")
		}
		seen[id] = true
	}
	taskList, e := s.fabricTaskCatalog(ctx, r, state.Project)
	if e != nil {
		return "", e
	}
	var task *fabricTask
	for i := range taskList {
		if taskList[i].ID == c.Task {
			task = &taskList[i]
		}
	}
	if task == nil || !task.Available {
		return "", studioError("FABRIC_TASK_VERSION_UNAVAILABLE")
	}
	apply := false
	for _, tag := range task.Tags {
		apply = apply || strings.HasSuffix(tag, ".apply")
	}
	if !apply {
		return "", studioError("FABRIC_APPLY_TASK_REQUIRED")
	}
	if c.Strategy != "SERIAL" && c.Strategy != "CANARY" && c.Strategy != "PROGRESSIVE" && c.Strategy != "PARALLEL" {
		return "", studioError("FABRIC_STRATEGY_INVALID")
	}
	if c.Strategy != "SERIAL" && len(targets) > 1 {
		return "", studioError("FABRIC_APPROVED_REDUNDANCY_PLAN_REQUIRED")
	}
	if task.Timeout <= 0 {
		return "", studioError("FABRIC_TASK_TIMEOUT_REQUIRED")
	}
	rollout := fab.Rollout{ID: uuid.NewString(), Strategy: c.Strategy, Waves: []fab.Wave{}, Concurrency: 1, Timeout: task.Timeout, Retry: "new-approved-change-for-failed-scope", Safe: true, Warnings: []string{}}
	for i, id := range targets {
		deps := []string{}
		if i > 0 {
			deps = append(deps, rollout.Waves[i-1].ID)
		}
		rollout.Waves = append(rollout.Waves, fab.Wave{ID: uuid.NewString(), Sequence: i + 1, Targets: []string{id}, Tasks: []string{task.ID}, Concurrency: 1, Executions: []string{}, Depends: deps})
	}
	change := fab.Change{ID: uuid.NewString(), Build: build.ID, Validation: v.ID, Targets: targets, Risk: "normal", Status: "DRAFT", Rollout: rollout.ID, Input: input.ID, Purpose: c.Purpose, CreatedBy: subjectOf(r.Context()).ID, CreatedAt: time.Now().UTC(), Window: c.Window, Task: task.ID, Acknowledgement: c.Ack}
	state.Changes = append(state.Changes, change)
	state.Rollouts = append(state.Rollouts, rollout)
	return change.ID, nil
}
