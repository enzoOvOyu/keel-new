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
 ), facts AS MATERIALIZED (
 SELECT DISTINCT p.publication_id,p.table_id,
 CASE WHEN p.table_id IN ('network_policies','install_profiles') THEN item->>'scope' ELSE item->>'device_id' END AS subject
 FROM kickoff_publication_input p JOIN kickoff_input_revision r USING(project_id,table_id,version)
 CROSS JOIN LATERAL jsonb_array_elements(r.rows) item
 WHERE p.project_id=$1 AND CASE p.table_id
 WHEN 'network_roles' THEN true
 WHEN 'network_addresses' THEN COALESCE(item->>'address','')<>'' AND COALESCE(item->>'asn','')<>''
 WHEN 'network_policies' THEN COALESCE(item->>'profile','')<>''
 WHEN 'boot_interfaces' THEN COALESCE(item->>'port','')<>'' AND COALESCE(item->>'mac','')<>'' AND COALESCE(item->>'boot_mode','')<>'' AND COALESCE(item->>'network','')<>''
 WHEN 'install_profiles' THEN COALESCE(item->>'image_ref','')<>'' AND COALESCE(item->>'disk_policy','')<>'' AND COALESCE(item->>'architecture','')<>''
 ELSE false END
 ), placed AS MATERIALIZED (
 SELECT DISTINCT l.id,l.entity_id FROM latest l JOIN eng_placement p ON p.revision_id=l.model_revision AND p.equipment_id=l.entity_id
 JOIN kickoff_publication_object o ON o.publication_id=l.id AND o.kind='placement' AND o.entity_id IN(p.entity_id,p.equipment_id)
 ), configured AS (
 SELECT l.entity_id,e.equipment_type,pl.entity_id IS NOT NULL AS located,
 nr.subject IS NOT NULL AND na.subject IS NOT NULL AND (np.subject IS NOT NULL OR nt.subject IS NOT NULL) AS network,
 bi.subject IS NOT NULL AND (ip.subject IS NOT NULL OR it.subject IS NOT NULL) AS installation
 FROM latest l JOIN eng_equipment e ON e.revision_id=l.model_revision AND e.entity_id=l.entity_id
 LEFT JOIN placed pl ON pl.id=l.id AND pl.entity_id=l.entity_id
 LEFT JOIN facts nr ON nr.publication_id=l.id AND nr.table_id='network_roles' AND nr.subject=l.entity_id::text
 LEFT JOIN facts na ON na.publication_id=l.id AND na.table_id='network_addresses' AND na.subject=l.entity_id::text
 LEFT JOIN facts np ON np.publication_id=l.id AND np.table_id='network_policies' AND np.subject=l.entity_id::text
 LEFT JOIN facts nt ON nt.publication_id=l.id AND nt.table_id='network_policies' AND nt.subject=e.equipment_type||'s'
 LEFT JOIN facts bi ON bi.publication_id=l.id AND bi.table_id='boot_interfaces' AND bi.subject=l.entity_id::text
 LEFT JOIN facts ip ON ip.publication_id=l.id AND ip.table_id='install_profiles' AND ip.subject=l.entity_id::text
 LEFT JOIN facts it ON it.publication_id=l.id AND it.table_id='install_profiles' AND it.subject='servers'
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
