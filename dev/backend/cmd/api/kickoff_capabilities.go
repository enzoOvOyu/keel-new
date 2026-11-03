package main

import (
	"context"
	"fmt"
)

// Read published object closure and frozen input references, never re-Project
// historical workbooks just to draw the overview's readiness indicators.
func (s *Server) kickoffCapabilities(ctx context.Context, q engineeringQuery, total, servers, ready int) ([]map[string]any, error) {
	var current, hosts, located, network, installation int
	err := q.QueryRow(ctx, `WITH latest AS (
 SELECT DISTINCT ON(o.entity_id) o.entity_id,p.id,p.model_revision FROM kickoff_publication p JOIN kickoff_publication_object o ON o.publication_id=p.id AND o.kind='device'
 WHERE p.project_id=$1 AND p.state='published' ORDER BY o.entity_id,p.revision DESC
 ), inputs AS MATERIALIZED (
 SELECT p.publication_id,p.table_id,r.rows FROM kickoff_publication_input p JOIN kickoff_input_revision r USING(project_id,table_id,version) WHERE p.project_id=$1
 ), configured AS (
 SELECT l.*,e.equipment_type,
 EXISTS(SELECT 1 FROM kickoff_publication_object o JOIN eng_placement placement ON placement.revision_id=l.model_revision AND placement.equipment_id=l.entity_id AND o.entity_id IN(placement.entity_id,placement.equipment_id) WHERE o.publication_id=l.id AND o.kind='placement') located,
 EXISTS(SELECT 1 FROM inputs i,jsonb_array_elements(i.rows) r WHERE i.publication_id=l.id AND i.table_id='network_roles' AND r->>'device_id'=l.entity_id::text)
 AND EXISTS(SELECT 1 FROM inputs i,jsonb_array_elements(i.rows) r WHERE i.publication_id=l.id AND i.table_id='network_addresses' AND r->>'device_id'=l.entity_id::text AND COALESCE(r->>'address','')<>'' AND COALESCE(r->>'asn','')<>'')
 AND EXISTS(SELECT 1 FROM inputs i,jsonb_array_elements(i.rows) r WHERE i.publication_id=l.id AND i.table_id='network_policies' AND r->>'scope' IN(l.entity_id::text,e.equipment_type||'s') AND COALESCE(r->>'profile','')<>'') network,
 EXISTS(SELECT 1 FROM inputs i,jsonb_array_elements(i.rows) r WHERE i.publication_id=l.id AND i.table_id='boot_interfaces' AND r->>'device_id'=l.entity_id::text AND COALESCE(r->>'port','')<>'' AND COALESCE(r->>'mac','')<>'' AND COALESCE(r->>'boot_mode','')<>'' AND COALESCE(r->>'network','')<>'')
 AND EXISTS(SELECT 1 FROM inputs i,jsonb_array_elements(i.rows) r WHERE i.publication_id=l.id AND i.table_id='install_profiles' AND r->>'scope' IN(l.entity_id::text,'servers') AND COALESCE(r->>'image_ref','')<>'' AND COALESCE(r->>'disk_policy','')<>'' AND COALESCE(r->>'architecture','')<>'') installation
 FROM latest l JOIN eng_equipment e ON e.revision_id=l.model_revision AND e.entity_id=l.entity_id
 ) SELECT count(*),count(*) FILTER(WHERE equipment_type='server'),count(*) FILTER(WHERE located),count(*) FILTER(WHERE network),count(*) FILTER(WHERE equipment_type='server' AND installation) FROM configured`, s.project).Scan(&current, &hosts, &located, &network, &installation)
	if err != nil {
		return nil, err
	}
	makeCap := func(id, title, detail string, n, total int, action, href string) map[string]any {
		state := "needs_input"
		if total > 0 && n == total {
			state = "ready"
		} else if n > 0 {
			state = "partial"
		}
		return map[string]any{"id": id, "title": title, "detail": detail, "available": n, "total": total, "state": state, "action": action, "href": href}
	}
	cmdbDetail, cmdbAction := "确认后即可建立统一资产视图", fmt.Sprintf("确认 %d 台可用设备", ready)
	if current > 0 {
		cmdbDetail = "统一身份已可供下游读取"
		cmdbAction = "查看入库版本"
	}
	installDetail := "设备入库后即可建立待装机清单"
	if hosts > 0 {
		installDetail = fmt.Sprintf("%d 台服务器可进入待装机清单", hosts)
	}
	return []map[string]any{makeCap("cmdb", "CMDB 资产与关系", cmdbDetail, current, total, cmdbAction, "publish"), makeCap("rack", "机柜图与空间", "立面按已知U位绘制，空间参数可后补", located, total, "补充空间与落位", "models/facility"), makeCap("fabric", "Fabric 网络交付", "物理清单可用，配置生成按目标检查意图", network, total, "完善网络意图", "models/network"), makeCap("install", "装机管理", installDetail, installation, servers, "关联启动与安装策略", "models/provisioning")}, nil
}
