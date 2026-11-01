package engineering

import (
	"encoding/csv"
	"io"
	"net"
	"strconv"
	"strings"
)

// InputTable is the shared contract for template, form and Agent candidates.
// These are staging fields; approved facts remain in typed engineering models.
type InputField struct {
	Code     string `json:"code"`
	Label    string `json:"label"`
	Kind     string `json:"kind"`
	Required bool   `json:"required"`
}
type InputTable struct {
	ID        string       `json:"id"`
	Step      int          `json:"step"`
	Name      string       `json:"name"`
	Condition string       `json:"condition"`
	Fields    []InputField `json:"fields"`
}
type InputCatalog struct {
	Version string       `json:"version"`
	Tables  []InputTable `json:"tables"`
}
type InputPreviewRequest struct {
	TableID string `json:"table_id"`
	CSV     string `json:"csv"`
}
type InputProblem struct {
	Row     int    `json:"row"`
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
type InputPreview struct {
	TableID string         `json:"table_id"`
	Rows    [][]string     `json:"rows"`
	Issues  []InputProblem `json:"issues"`
	Valid   bool           `json:"valid"`
}

// A trailing ? declares a conditionally required field. The condition is shown
// with the table; preview validates syntax, never declares a domain complete.
func StepInputCatalog() InputCatalog {
	c := InputCatalog{Version: "1", Tables: []InputTable{}}
	add := func(id string, step int, name, condition, spec string) {
		t := InputTable{ID: id, Step: step, Name: name, Condition: condition, Fields: []InputField{}}
		for _, s := range strings.Split(spec, "|") {
			p := strings.Split(s, ":")
			code := p[0]
			required := !strings.HasSuffix(code, "?")
			code = strings.TrimSuffix(code, "?")
			kind := "text"
			if len(p) > 2 {
				kind = p[2]
			}
			t.Fields = append(t.Fields, InputField{code, p[1], kind, required})
		}
		c.Tables = append(c.Tables, t)
	}
	add("terminations", 1, "设备关联", "八列必须保留；MAC不适用须有已确认端接类型依据；预览通过不代表身份或端接已确认。", "a_serial:a端SN|a_mac:a端MAC:mac|a_port:a端端口|b_serial:b端SN|b_rack_position:b端机架位|b_mac:b端MAC:mac|b_port:b端端口|termination_type:端接类型")
	add("rooms", 2, "机房", "场地和机房编码在项目内唯一。", "site_code:场地编码|site_name:场地名称|room_code:机房编码|room_name:机房名称")
	add("racks", 2, "机柜", "U位检查需要容量U；空间计算需要毫米尺寸。", "room_code:机房编码|rack_code:机柜编码|name:机柜名称|capacity_u?:容量U:positive_integer|width_mm?:宽mm:positive|depth_mm?:深mm:positive|height_mm?:高mm:positive")
	add("equipment_specs", 2, "设备规格", "U位安装必须提供高度U；规格引用必须经确认。", "serial:设备SN|equipment_type:设备类型|model:型号或规格引用|height_u?:高度U:positive_integer")
	add("placements", 2, "设备落位", "U位安装要求起始U和占用U；非U安装要求位置编码。", "serial:设备SN|room_code:机房编码|rack_code:机柜编码|mount_type:安装方式|start_u?:起始U:positive_integer|height_u?:占用U:positive_integer|face:安装面|position_code?:非U位置编码")
	add("port_specs", 3, "端口规格", "Breakout要求父端口和子口序号；其他字段依端接类型。", "serial:设备SN|port:端口|medium:介质|connector:连接器|rate:速率:positive|rate_unit:速率单位|parent_port?:父端口|breakout_index?:Breakout子口序号:positive_integer")
	add("patch_paths", 3, "配线路径", "有中间配线节点时必填；直连无需此表。", "connection:连接引用|sequence:段序号:positive_integer|device:配线设备引用|in_port:入口端口|out_port:出口端口")
	add("route_nodes", 3, "路径节点", "坐标计算时必须提供坐标系和X/Y/Z；实测路段可以不填坐标。", "code:节点编码|room_code:机房编码|rack_code?:关联机柜编码|frame?:坐标系|x?:X:number|y?:Y:number|z?:Z:number|unit?:长度单位")
	add("route_edges", 3, "可走线路段", "只能在明确允许的通道上计算路径。", "code:路段编码|from:起点节点|to:终点节点|distance:距离:positive|unit:长度单位|category:允许线路类别")
	add("rack_access", 3, "机柜接入", "接入节点必须存在；引线规则必须有批准版本。", "rack_code:机柜编码|node:接入节点|rule:引线规则引用")
	add("cable_specs", 3, "线缆规格", "可采购长度按同一规格逐行填写。", "code:规格编码|medium:介质|connector_a:本端连接器|connector_b:对端连接器|rate:支持速率:positive|rate_unit:速率单位|length:可采购长度:positive|unit:长度单位|optic_mode:模块需求方式")
	add("optic_specs", 3, "光模块规格", "根据端口和产品规则匹配，不固定每端一个模块。", "model:型号|port_type:端口类型|medium:介质|rate:速率:positive|rate_unit:速率单位|distance:适用距离:positive|unit:长度单位")
	add("cabling_rules", 3, "布线规则", "参数和公式需独立校验审批；不能直接执行任意代码。", "code:规则编码|version:版本:positive_integer|strategy:路径策略|lead_rule:引线参数规则|slack_rule:余量参数规则|length_rule:标准长度选择规则|source:来源")
	add("network_planes", 4, "网络域与平面", "仅声明客户实际选用的网络配置集。", "fabric:Fabric编码|type:类型|plane:Plane编码|profile:配置集")
	add("network_roles", 4, "设备角色", "引用已经确认的设备。", "serial:设备SN|fabric:Fabric编码|plane:Plane编码|role:角色")
	add("network_interfaces", 4, "接口映射", "VLAN/VRF仅在使用时填写，不从物理线缆推断。", "serial:设备SN|port:物理端口|interface:逻辑接口|domain:网络域|vlan?:VLAN|vrf?:VRF")
	add("network_addresses", 4, "地址分配", "必须来自明确规划或已批准地址池。", "serial:设备SN|interface:接口|address:地址及前缀|purpose:用途|source:分配依据")
	add("network_asns", 4, "ASN分配", "使用BGP等需要ASN的配置集才必填。", "reference:设备或域引用|asn:ASN:positive_integer|source:来源依据")
	add("network_bgp", 4, "BGP邻居", "使用BGP时必填；双方身份和配置需一致。", "serial:本端SN|interface:本端接口|peer:对端引用|local_asn:本端ASN:positive_integer|peer_asn:对端ASN:positive_integer|family:地址族|policy:策略引用")
	add("network_bfd", 4, "BFD", "使用BFD时必填，单位为毫秒。", "serial:设备SN|interface:接口|peer:对端引用|tx_ms:发送间隔ms:positive_integer|rx_ms:接收间隔ms:positive_integer|multiplier:检测倍数:positive_integer|source:来源")
	add("network_qos", 4, "RoCE与QoS", "使用相应配置集时必填，值由网络规划决定。", "serial:设备SN|interface:接口|profile:配置集|traffic_class:流量类别|pfc:无损优先级|ecn:ECN策略|source:来源")
	add("network_ar", 4, "Adaptive Routing", "使用Adaptive Routing时必填。", "serial:设备SN|scope:作用范围|profile:策略引用|source:来源")
	add("network_evpn", 4, "EVPN与VXLAN", "使用该配置集时必填。", "serial:设备SN|vrf:VRF|vni:VNI:positive_integer|rd:RD|import_rt:导入RT|export_rt:导出RT|source:来源")
	add("power_profiles", 5, "设备功率", "规划、铭牌、实测分别记录，不能混用。", "reference:设备SN或型号引用|power:功率:positive|unit:功率单位|basis:功率依据|source:来源")
	add("rack_capacity", 5, "机柜容量", "容量未知不得按0或通过处理。", "rack_code:机柜编码|capacity:可用容量:positive|unit:功率单位|basis:容量依据|cooling_type?:冷却方式|cooling_capacity?:冷却容量:positive")
	add("power_circuits", 5, "供电对象与回路", "声称回路或冗余校验时必填；上游根节点可空。", "code:对象编码|location:机房或机柜编码|upstream?:上游引用|circuit:回路编码|capacity:可用容量:positive|unit:功率单位")
	add("power_connections", 5, "设备供电关系", "声称供电关系或冗余校验时必填。", "serial:设备SN|psu:电源口或PSU|circuit:供电对象或回路|group:A或B分组|rule:负载分配规则引用")
	add("power_rules", 5, "电力规则", "规则参数需明确来源和审批，不能猜测A/B分摊。", "code:规则编码|version:版本:positive_integer|scenario:运行或冗余场景|basis:负载容量口径|factor:需用系数:positive|source:来源")
	return c
}

func PreviewStepInput(q InputPreviewRequest) InputPreview {
	out := InputPreview{TableID: q.TableID, Rows: [][]string{}, Issues: []InputProblem{}}
	problem := func(row int, field, code, message string) {
		out.Issues = append(out.Issues, InputProblem{row, field, code, message})
	}
	var table *InputTable
	for _, t := range StepInputCatalog().Tables {
		if t.ID == q.TableID {
			copy := t
			table = &copy
			break
		}
	}
	if table == nil {
		problem(0, "", "UNKNOWN_TABLE", "未知模板")
		return out
	}
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(q.CSV, "\ufeff")))
	reader.FieldsPerRecord = -1
	headers, e := reader.Read()
	if e != nil {
		problem(1, "", "INVALID_HEADER", "无法读取表头")
		return out
	}
	if len(headers) != len(table.Fields) {
		problem(1, "", "HEADER_MISMATCH", "列数与模板不一致")
	}
	for i, f := range table.Fields {
		if i >= len(headers) || headers[i] != f.Label {
			problem(1, f.Code, "HEADER_MISMATCH", "请保留模板列名和顺序："+f.Label)
		}
	}
	if len(out.Issues) > 0 {
		return out
	}
	for row := 2; ; row++ {
		values, e := reader.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			problem(row, "", "INVALID_CSV", e.Error())
			break
		}
		if len(values) != len(table.Fields) {
			problem(row, "", "COLUMN_COUNT", "数据列数与模板不一致")
			continue
		}
		out.Rows = append(out.Rows, values)
		for i, f := range table.Fields {
			value := strings.TrimSpace(values[i])
			if value == "" {
				if f.Required {
					problem(row, f.Code, "REQUIRED", "请补充"+f.Label)
				}
				continue
			}
			switch f.Kind {
			case "mac":
				m, e := net.ParseMAC(value)
				if e != nil || len(m) != 6 {
					problem(row, f.Code, "INVALID_MAC", "MAC应为48位地址")
				}
			case "number", "positive", "positive_integer":
				n, e := strconv.ParseFloat(value, 64)
				if e != nil || strings.EqualFold(value, "nan") || strings.Contains(strings.ToLower(value), "inf") || f.Kind != "number" && n <= 0 {
					problem(row, f.Code, "INVALID_NUMBER", "数值或范围无效")
				} else if f.Kind == "positive_integer" {
					if _, e := strconv.ParseUint(value, 10, 64); e != nil {
						problem(row, f.Code, "INVALID_INTEGER", "必须为正整数")
					}
				}
			}
		}
	}
	if len(out.Rows) == 0 {
		problem(0, "", "EMPTY_INPUT", "请填写至少一行资料")
	}
	out.Valid = len(out.Issues) == 0
	return out
}
