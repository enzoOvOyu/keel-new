package identity

import (
	"sort"
)

const (
	Viewer           = "viewer"
	DeliveryEngineer = "delivery_engineer"
	DeliveryLead     = "delivery_lead"
	DeveloperSRE     = "developer_sre"
	Admin            = "admin"
	NetworkEngineer  = "network_engineer"
)

type Role struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}
type Permission struct {
	ID          string `json:"id"`
	Module      string `json:"module"`
	Action      string `json:"action"`
	Description string `json:"description"`
}

var Modules = []string{"project", "dashboard", "delivery", "bom", "cmdb", "rack", "provision", "validation", "fabric", "fleet", "flows", "issues", "monitor", "bmc", "firmware", "ai", "admin", "config", "test", "cleanup", "media", "model", "approval"}

func Catalog() []Permission {
	out := []Permission{}
	for _, m := range Modules {
		for _, a := range []string{"read", "create", "edit", "import", "review", "approve", "execute", "delete"} {
			out = append(out, Permission{m + ":" + a, m, a, m + " / " + a})
		}
	}
	for _, p := range []Permission{{"fabric:approve-network", "fabric", "approve-network", "网络 NE 签署"}, {"fabric:approve-delivery", "fabric", "approve-delivery", "交付 DE 签署"}, {"project:members", "project", "members", "项目成员管理"}, {"identity:roles:read", "identity", "read", "角色权限目录"}} {
		out = append(out, p)
	}
	return out
}
func Roles() []Role {
	reads := []string{}
	for _, m := range Modules {
		if m != "admin" {
			reads = append(reads, m+":read")
		}
	}
	makeRole := func(id, name, desc string, extra ...string) Role {
		p := append([]string{}, reads...)
		p = append(p, extra...)
		sort.Strings(p)
		return Role{id, name, desc, p}
	}
	all := []string{}
	for _, p := range Catalog() {
		all = append(all, p.ID)
	}
	return []Role{makeRole(Viewer, "观察者", "仅查看授权项目"), makeRole(DeliveryEngineer, "交付工程师 DE", "交付执行，与网络工程师独立", "bom:create", "bom:edit", "bom:import", "bom:review", "bom:execute", "cmdb:edit", "rack:edit", "rack:import", "provision:create", "provision:edit", "provision:execute", "firmware:execute", "validation:execute", "test:execute", "cleanup:execute", "config:execute", "flows:execute", "issues:create", "issues:edit", "issues:execute", "fabric:approve-delivery", "bmc:execute"), makeRole(DeliveryLead, "交付组长 DL", "交付管理、审批和豁免", "project:edit", "project:members", "delivery:approve", "bom:review", "bom:approve", "provision:approve", "firmware:approve", "bmc:approve", "cleanup:approve", "config:approve", "issues:edit", "approval:read"), makeRole(DeveloperSRE, "研发 SRE", "Flow、自动化与研发工具", "flows:create", "flows:edit", "flows:execute", "flows:delete", "ai:create", "ai:edit", "ai:execute", "monitor:edit", "monitor:execute", "media:create", "media:edit", "issues:create", "issues:edit"), {Admin, "平台管理员", "平台治理；业务签署仍遵循职责分离", all}, makeRole(NetworkEngineer, "网络工程师 NE", "网络诊断、配置和故障处置，不授予装机或清理执行", "bom:create", "bom:edit", "bom:import", "bom:review", "bom:execute", "fabric:create", "fabric:edit", "fabric:import", "fabric:review", "fabric:execute", "fabric:approve-network", "config:edit", "issues:create", "issues:edit", "issues:execute", "monitor:execute")}
}

// The only compatibility mapping for the existing IA permission labels.
var Aliases = map[string]string{"bom:write": "bom:edit", "bom:commit": "bom:execute", "model:approve": "model:approve", "flows:run": "flows:execute", "device:operate": "bmc:execute", "issues:write": "issues:edit", "project:create": "project:create"}

func Canonical(p string) string {
	if v := Aliases[p]; v != "" {
		return v
	}
	return p
}

var SignatureRoles = map[string]string{"NE": NetworkEngineer, "DE": DeliveryEngineer, "DL": DeliveryLead}

func ProjectRole(id string) bool {
	for _, r := range Roles() {
		if r.ID == id {
			return r.ID != Admin
		}
	}
	return false
}
