import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { download } from "../../lib/api";
import { useProject } from "../../app/ProjectContext";
import {
  infrastructureApi,
  locationIssues,
  physicalPath,
  type InfraAsset,
} from "../../lib/api/infrastructure";
import { realCmdbApi } from "../../lib/api/cmdb-http";
import { Button, Badge, DataTable, Modal } from "../../components/ui";
import { DrawerDetail, openDevice } from "../../components/DeviceDrawers";
import { RackElevation } from "./RackElevation";
import { TopologyCanvas, type DrawNode, type DrawEdge } from "./TopologyCanvas";
import "./infrastructure.css";
const rackTabs = [
  ["elevation", "机柜视图"],
  ["devices", "设备清单"],
  ["servers", "服务器详情"],
  ["switches", "交换机与端口"],
  ["cabling", "线序与物理布线"],
  ["power", "电源与空间"],
  ["mapping", "Mapping 关联信息"],
] as const;
export function InfrastructureWorkspace({
  rackId = "",
  rackOnly = false,
}: {
  rackId?: string;
  rackOnly?: boolean;
}) {
  const cache=useQueryClient();
  useEffect(()=>realCmdbApi.subscribe(()=>{void cache.invalidateQueries({queryKey:["infrastructure"]})}),[cache]);
  const ctx = useProject(),
    nav = useNavigate();
  const [url, setUrl] = useSearchParams();
  const view = url.get("view") || (rackOnly ? "groups" : "global"),
    focus = url.get("focus") || "",
    selected = url.get("selected") || "",
    cableId = url.get("cable") || "",
    portId = url.get("port") || "",
    tab = url.get("rack_tab") || "elevation",
    face = url.get("face") || "front";
  const [search, setSearch] = useState(url.get("q") || ""),
    [edit, setEdit] = useState(false),
    [positions, setPositions] = useState<Record<string, number>>({}),
    [history, setHistory] = useState<Record<string, number>[]>([]),
    [future, setFuture] = useState<Record<string, number>[]>([]),
    [error, setError] = useState(""),
    [receipt, setReceipt] = useState(""),
    [confirm, setConfirm] = useState(false),
    [reason, setReason] = useState(""),
    [zoom, setZoom] = useState(1),
    [drawerDevices, setDrawerDevices] = useState<any[]>([]);
  const svg = useRef<SVGSVGElement>(null);
  const scopeReady=useRef(false);
  useEffect(()=>{if(url.has("project_id")){const p=url.get("project_id")||null;if(!p||ctx.projects.some(a=>a.id===p))ctx.setFilter(p)}},[url.get("project_id")]);
  useEffect(()=>{if(!scopeReady.current){scopeReady.current=true;return}if(url.has("project_id")&&url.get("project_id")!==(ctx.filter||"")){const next=new URLSearchParams(url);next.set("project_id",ctx.filter||"");setUrl(next,{replace:true})}},[ctx.filter]);
  const query = useQuery({
    queryKey: [
      "infrastructure",
      ctx.filter,
      ctx.permissionRevision,
      rackId,
      focus,
    ],
    queryFn: ({ signal }) =>
      infrastructureApi.snapshot(ctx.filter, rackId, focus, signal),
    staleTime: 60000,
  });
  const directory = useQuery({
    queryKey: ["infrastructure", ctx.filter, ctx.permissionRevision, "", ""],
    enabled: !!rackId,
    queryFn: ({ signal }) =>
      infrastructureApi.snapshot(ctx.filter, "", "", signal),
    staleTime: 60000,
  });
  const data = query.data;
  const assets = data?.assets || [],
    cables = data?.cables || [];
  const byId = useMemo(() => new Map(assets.map((a) => [a.id, a])), [data]);
  const rack = byId.get(rackId);
  const devices = assets.filter((a) =>
    ["server", "switch", "patch_panel", "pdu"].includes(a.type),
  );
  const local = devices.filter((a) => a.fields.rack_id === rackId);
  const racks = (rackId ? directory.data?.assets || assets : assets).filter(
    (a) => a.type === "rack",
  );
  const ports = assets.filter((a) => a.type === "port");
  const chosen = byId.get(selected) || byId.get(portId);
  const height = Number(rack?.fields.u_height);
  const occupied = new Set(
    local.flatMap((d) => {
      const start = Number(d.fields.u_start ?? d.fields.rack_u),
        h = Number(d.fields.u_height);
      return start > 0 && h > 0
        ? Array.from({ length: Math.min(h, 200) }, (_, i) => start + i)
        : [];
    }),
  );
  const faults = locationIssues(local, height, positions, rack);
  const related = cables.filter(
    (c) =>
      c.id === cableId ||
      c.a === portId ||
      c.b === portId ||
      c.device_a === selected ||
      c.device_b === selected,
  );
  const activeCable = cables.find((c) => c.id === cableId);
  const draftKey = "keel-layout:" + ctx.filter + ":" + rackId;
  const fingerprint = local
    .map((d) => d.id + ":" + d.revision)
    .sort()
    .join("|");
  function change(patch: Record<string, string>) {
    const q = new URLSearchParams(url);
    for (const [k, v] of Object.entries(patch)) v ? q.set(k, v) : q.delete(k);
    setUrl(q);
  }
  useEffect(() => {
    setPositions({});
    setHistory([]);
    setFuture([]);
    setEdit(false);
    setReceipt("");
    setError("");
    if (!rackId) return;
    const saved = localStorage.getItem(draftKey);
    if (saved) {
      try {
        const d = JSON.parse(saved);
        if (d.fingerprint === fingerprint) setPositions(d.positions);
        else setReceipt("CMDB 版本已变化，旧展示草稿不再应用。");
      } catch {
        localStorage.removeItem(draftKey);
      }
    }
  }, [draftKey, fingerprint]);
  function move(id: string, u: number) {
    const next = { ...positions, [id]: u };
    const nextFaults = locationIssues(local, height, next, rack);
    if (nextFaults.some((f) => f.asset_id === id || f.code === "U_OVERLAP")) {
      setError(nextFaults.map((f) => f.message).join("；"));
      return;
    }
    setError("");
    setHistory([...history, positions]);
    setFuture([]);
    setPositions(next);
  }
  async function openDeviceDrawer(a: InfraAsset) {
    try {
      await realCmdbApi.asset(a.id, a.project_id);
      const d = realCmdbApi.device(a.id);
      if (!d) throw Error("设备详情未就绪");
      setDrawerDevices(devices.map(a=>({...a.fields,id:a.id,category:a.type,project_id:a.project_id,hostname:a.name,serial:a.fields.serial,model:a.fields.model})));
      setUrl(openDevice(url, d));
    } catch (e) {
      setError(String(e));
    }
  }
  async function requestPosition() {
    if (!rack || !reason.trim()) return;
    try {
      const schema = await realCmdbApi.schema();
      const changes = local
        .filter(
          (d) =>
            positions[d.id] !== undefined &&
            positions[d.id] !== Number(d.fields.u_start ?? d.fields.rack_u),
        )
        .map((d) => ({
          op: "asset.relocate",
          asset_id: d.id,
          expected_revision: d.revision,
          attributes: { u_start: positions[d.id] },
        }));
      if (!changes.length) throw Error("没有实际位置差异");
      const r = await realCmdbApi.createChangeSet({
        project_id: rack.project_id,
        title: "机柜安装位置调整 " + rack.name,
        reason,
        schema_version: schema.data.version,
        changes,
      });
      setConfirm(false);
      setReceipt(
        "已创建变更草稿 " +
          r.data.id +
          "，须在 CMDB 审核发布后才改变实际位置。",
      );
    } catch (e) {
      setError(String(e));
    }
  }
  async function exportRack(format: string) {
    if (!svg.current) return;
    const copy = svg.current.cloneNode(true) as SVGSVGElement;
    copy.setAttribute("xmlns", "http://www.w3.org/2000/svg");
    const xml = new XMLSerializer().serializeToString(copy);
    if (format === "svg")
      download((rack?.name || "rack") + ".svg", xml, "image/svg+xml");
    else if (format === "png") {
      const image = new Image();
      image.src = "data:image/svg+xml;charset=utf-8," + encodeURIComponent(xml);
      await image.decode();
      const canvas = document.createElement("canvas");
      canvas.width = 700;
      canvas.height = (height * 24 + 72) * 2;
      canvas
        .getContext("2d")!
        .drawImage(image, 0, 0, canvas.width, canvas.height);
      const a = document.createElement("a");
      a.download = rack?.name + ".png";
      a.href = canvas.toDataURL();
      a.click();
    } else window.print();
  }
  const path = useMemo(
    () => physicalPath(cables, url.get("from") || "", url.get("to") || ""),
    [data, url.get("from"), url.get("to")],
  );
  const nodeLimit=Math.min(1000,Math.max(100,Number(url.get("nodes"))||200));
  const graph = useMemo(() => {
    const nodes: DrawNode[] = [],
      edges: DrawEdge[] = [];
    const rackMode = view === "global" || view === "groups";
    const group = url.get("group") || "";
    const within = (a: InfraAsset) => {
      if (!group) return true;
      let id = a.id;
      for (let i = 0; i < 6; i++) {
        if (id === group) return true;
        id = String(byId.get(id)?.fields.parent_uid || "");
        if (!id) break;
      }
      return false;
    };
    const pathDevices = new Set(
      path?.flatMap((c) => [c.device_a, c.device_b]) || [],
    );
    const candidates = (
      view === "global"
        ? assets.filter((a) =>
            ["data_center", "region", "pod"].includes(a.type),
          )
        : rackMode
          ? racks
          : devices
    ).filter(
      (a) =>
        within(a) &&
        (!url.get("site") ||
          (a.fields.site || byId.get(a.fields.rack_id)?.fields.site) ===
            url.get("site")) &&
        (!url.get("role") || a.fields.role === url.get("role")) &&
        (view !== "trace" || !path || pathDevices.has(a.id)),
    );
    const filtered = candidates
      .filter(
        (a) =>
          !search ||
          [a.id, a.name, a.fields.serial, a.fields.management_ip]
            .join(" ")
            .toLowerCase()
            .includes(search.toLowerCase()),
      )
      .slice(0, nodeLimit);
    for (const a of filtered)
      nodes.push({
        id: a.id,
        label: a.name,
        type: a.type,
        health: a.fields.health_status,
      });
    const ids = new Set(nodes.map((n) => n.id));
    const aggregated = new Map<string, DrawEdge & { count: number }>();
    for (const c of cables) {
      if(view==="global")continue;
      const da = byId.get(c.device_a),
        db = byId.get(c.device_b);
      const a = rackMode ? da?.fields.rack_id : c.device_a,
        b = rackMode ? db?.fields.rack_id : c.device_b;
      if (!a || !b || a === b || !ids.has(a) || !ids.has(b)) continue;
      const key = [a, b].sort().join("|");
      if (rackMode) {
        const old = aggregated.get(key);
        if (old) old.count++;
        else
          aggregated.set(key, {
            id: "group:" + key,
            source: a,
            target: b,
            count: 1,
            planned: true,
          });
      } else
        edges.push({
          id: c.id,
          source: a,
          target: b,
          label: c.fields.speed || "",
          planned: c.fact !== "observed" && c.fact !== "actual",
        });
    }
    for (const e of aggregated.values())
      edges.push({ ...e, label: e.count + " 条物理记录" });
    if (view === "global") {
      for (const a of filtered) {
        const parent = a.fields.parent_uid;
        if (parent && ids.has(parent))
          edges.push({
            id: "hierarchy:" + a.id,
            source: parent,
            target: a.id,
            label: "包含",
          });
      }
    }
    return { nodes, edges:edges.slice(0,3000),totalEdges:edges.length };
  }, [
    data,
    view,
    search,
    url.get("group"),
    url.get("site"),
    url.get("role"),
    path,nodeLimit,
  ]);
  const findingRows = [...(data?.findings || []), ...faults];
  function selectCable(id: string) {
    const c = cables.find((c) => c.id === id);
    if (!c) return;
    change({
      cable: id,
      selected: c.device_a,
      port: c.a,
      rack_tab: "cabling",
      face: "back",
    });
  }
  return (
    <div className="workspace infrastructure">
      <div className="page-heading">
        <h1>{rack?rack.name:rackOnly?'机柜管理':({global:'全局拓扑',network:'网络架构',groups:'机柜组',cabling:'物理布线',trace:'链路追踪',quality:'数据质量'} as Record<string,string>)[view]||'数据中心拓扑'}</h1>
        <div className="infra-heading-actions">{assets.some(a=>a.fields.is_demo)&&<Badge value="演示数据"/>}<Button onClick={()=>query.refetch()}>刷新</Button></div>
      </div>
      {query.isPending && <div role="status">正在加载获授权 CMDB 资产…</div>}
      {(query.error || error) && (
        <div className="error-panel" role="alert">
          {error || query.error?.message}
          <Button onClick={() => setError("")}>关闭提示</Button>
        </div>
      )}
      {receipt && <p role="status">{receipt}</p>}
      {data?.truncated && (
        <p role="alert">
          查询达到 20,000
          个资产上限，请选择项目或机柜缩小范围，当前视图不是完整数据。
        </p>
      )}
      {!rackId && (
        <>

          <div className="toolbar">
            <input
              aria-label="拓扑设备搜索"
              placeholder="资产 ID / Hostname / SN / 管理 IP"
              value={search}
              onChange={(e) => {
                setSearch(e.target.value);
                change({ q: e.target.value });
              }}
            />
            <select
              aria-label="拓扑机房筛选"
              value={url.get("site") || ""}
              onChange={(e) => change({ site: e.target.value })}
            >
              <option value="">全部机房</option>
              {[
                ...new Set(racks.map((r) => r.fields.site).filter(Boolean)),
              ].map((s) => (
                <option key={s}>{s}</option>
              ))}
            </select>
            <select
              aria-label="拓扑角色筛选"
              value={url.get("role") || ""}
              onChange={(e) =>
                change({ role: e.target.value, view: "network" })
              }
            >
              <option value="">全部设备角色</option>
              {[
                ...new Set(devices.map((d) => d.fields.role).filter(Boolean)),
              ].map((r) => (
                <option key={r}>{r}</option>
              ))}
            </select>
            <Button
              onClick={() =>
                change({
                  focus: "",
                  selected: "",
                  port: "",
                  cable: "",
                  group: "",
                  role: "",
                  site: "",
                })
              }
            >
              返回全局聚合
            </Button>
            {focus && <Badge value={"局部范围 " + focus} />}
            <span>
              {graph.nodes.length} 节点 · {data?.total_assets || 0} 资产
            </span>{graph.nodes.length>=nodeLimit&&<Button disabled={nodeLimit>=1000} onClick={()=>change({nodes:String(nodeLimit+200)})}>展开更多</Button>}{graph.totalEdges>3000&&<span>当前边超过3000条，请缩小机柜/局部范围；画布仅展示前3000条。</span>}
          </div>
        </>
      )}
      {rack && (
        <>
          <div className="infra-summary">
            <span>
              {rack.fields.site || "机房未采集"} /{" "}
              {rack.fields.pod_id || "Pod 未采集"}
            </span>
            <b>{Number.isFinite(height) ? height + "U" : "高度未知"}</b>
            <span>
              占用 {occupied.size}U · 空闲{" "}
              {Number.isFinite(height) ? height - occupied.size : "unknown"}U
            </span>
            <span>
              {local.length} 设备 /{" "}
              {local.filter((d) => d.type === "server").length} 服务器 /{" "}
              {local.filter((d) => d.type === "switch").length} 交换机
            </span>
            <span>
              {cables.length} 连接记录 · {findingRows.length} 条质量提示
            </span>
            <Link
              to={
                "/topology?focus=" +
                encodeURIComponent(selected || rackId) +
                "&view=network&selected=" +
                encodeURIComponent(selected || rackId)
              }
            >
              查看关联拓扑 ↗
            </Link>
          </div>
          <nav className="module-tabs">
            {rackTabs.map(([v, n]) => (
              <button
                key={v}
                aria-current={tab === v ? "page" : undefined}
                onClick={() => change({ rack_tab: v })}
              >
                {n}
              </button>
            ))}
          </nav>
          <div className="toolbar">
            <Button
              onClick={() =>
                change({ face: face === "front" ? "back" : "front" })
              }
            >
              {face === "front" ? "切换背面" : "切换正面"}
            </Button>
            <Button onClick={() => setEdit(!edit)}>
              {edit ? "结束布局编辑" : "编辑展示布局"}
            </Button>
            <Button
              disabled={!history.length}
              onClick={() => {
                setFuture([positions, ...future]);
                setPositions(history.at(-1)!);
                setHistory(history.slice(0, -1));
              }}
            >
              撤销
            </Button>
            <Button
              disabled={!future.length}
              onClick={() => {
                setHistory([...history, positions]);
                setPositions(future[0]);
                setFuture(future.slice(1));
              }}
            >
              重做
            </Button>
            <Button
              onClick={() => {
                localStorage.setItem(
                  draftKey,
                  JSON.stringify({ fingerprint, positions }),
                );
                setReceipt("展示布局已保存到此浏览器，未修改 CMDB。");
              }}
            >
              保存展示布局
            </Button>
            <Button
              disabled={
                !Object.keys(positions).length ||
                !!faults.length ||
                !ctx.permissions.includes("cmdb:edit")
              }
              onClick={() => setConfirm(true)}
            >
              申请修改实际位置
            </Button>
            <Button onClick={() => setZoom(Math.min(1.5, zoom + 0.1))}>
              放大
            </Button>
            <Button onClick={() => setZoom(Math.max(0.5, zoom - 0.1))}>
              缩小
            </Button>
            {["svg", "png", "pdf"].map((f) => (
              <Button key={f} onClick={() => exportRack(f)}>
                导出 {f.toUpperCase()}
              </Button>
            ))}
          </div>
        </>
      )}
      {data && (
        <div className="infra-workbench">
          <aside className="panel infra-tree">
            <h3>机房 / 机柜</h3>
            {racks.map((r) => (
              <button
                key={r.id}
                className={r.id === rackId ? "active" : ""}
                onClick={() => nav("/racks/" + encodeURIComponent(r.id)+"?project_id="+encodeURIComponent(r.project_id))}
              >
                <b>{r.name}</b>
                <small>
                  {r.fields.site || "机房未采集"} · {r.fields.u_height || "?"}U
                </small>
              </button>
            ))}
            {rackId && <Link to="/topology">返回全局拓扑</Link>}
          </aside>
          <section className="panel infra-primary">{!query.isPending&&data&&!assets.length&&<div className="infra-empty"><h3>当前项目暂无数据</h3><Button onClick={()=>ctx.setFilter(null)}>查看全部项目</Button></div>}
            {rackId ? (
              rack &&
              Number.isInteger(height) &&
              height > 0 &&
              height <= 100 ? (
                <div className="infra-rack-body">
                  <div
                    className="infra-elevation"
                    style={{ width: 350 * zoom }}
                  >
                    <RackElevation
                      rack={rack}
                      devices={local}
                      ports={ports}
                      cables={cables}
                      positions={positions}
                      face={face}
                      selected={selected}
                      portSelected={portId}
                      edit={edit}
                      onSelect={(id) =>
                        change({ selected: id, port: "", cable: "" })
                      }
                      onPort={(id) => {
                        const c = cables.find((c) => c.a === id || c.b === id);
                        change({
                          port: id,
                          cable: c?.id || "",
                          selected: String(
                            byId.get(id)?.fields.owner_device_id || "",
                          ),
                        });
                      }}
                      onMove={move}
                      svgRef={svg}
                    />
                  </div>
                  <div className="infra-rack-content">
                    {tab === "elevation" && (
                      <>
                        <h2>机柜立面与安装事实</h2>
                        <p>
                          {edit
                            ? "当前拖动仅调整展示草稿，可使用键盘上下键移动。"
                            : "选择设备查看资产和连接；背面端口只显示已登记端口。"}
                        </p>
                        <p>
                          已确认布局{" "}
                          {
                            local.filter(
                              (d) =>
                                Number(d.fields.u_start ?? d.fields.rack_u) >
                                  0 && Number(d.fields.u_height) > 0,
                            ).length
                          }{" "}
                          / {local.length}，未确认的位置不会自动排布。
                        </p>
                        {chosen && (
                          <>
                            <h3>{chosen.name}</h3>
                            <p>
                              {chosen.fields.model || "型号未采集"} · U
                              {chosen.fields.u_start ??
                                chosen.fields.rack_u ??
                                "?"}{" "}
                              · {chosen.fields.u_height || "?"}U ·{" "}
                              {chosen.fields.health_status || "unknown"}
                            </p>
                            <p>
                              {
                                ports.filter(
                                  (p) => p.fields.owner_device_id === chosen.id,
                                ).length
                              }{" "}
                              个已登记端口 · {related.length} 条关联物理连接
                            </p>
                            <Button
                              onClick={() => change({ rack_tab: "mapping" })}
                            >
                              查看所选设备关联信息
                            </Button>
                          </>
                        )}
                        {faults.map((f, i) => (
                          <p key={i} className="infra-warning">
                            {byId.get(f.asset_id)?.name} · {f.message}
                          </p>
                        ))}
                      </>
                    )}
                    {["devices", "servers", "switches", "power"].includes(
                      tab,
                    ) && (
                      <>
                        <h2>{rackTabs.find((t) => t[0] === tab)?.[1]}</h2>
                        <DataTable
                          rows={assets
                            .filter((a) =>
                              tab === "devices"
                                ? local.includes(a)
                                : tab === "servers"
                                  ? a.type === "server" && local.includes(a)
                                  : tab === "switches"
                                    ? a.type === "switch" && local.includes(a)
                                    : ["pdu", "psu"].includes(a.type),
                            )
                            .map((a) => ({
                              id: a.id,
                              name: a.name,
                              type: a.type,
                              ...a.fields,
                              cpu: a.fields.cpu || a.fields.cpu_model,
                              memory: a.fields.memory || a.fields.memory_gb,
                              source: a.source,
                            }))}
                          columns={
                            tab === "servers"
                              ? [
                                  "name",
                                  "model",
                                  "cpu",
                                  "gpu_model",
                                  "gpu_count",
                                  "memory",
                                  "bmc_ip",
                                ]
                              : tab === "switches"
                                ? [
                                    "name",
                                    "role",
                                    "model",
                                    "management_ip",
                                    "port_count",
                                  ]
                                : [
                                    "name",
                                    "type",
                                    "model",
                                    "serial",
                                    "rack_u",
                                    "u_height",
                                    "face",
                                    "health_status",
                                  ]
                          }
                          onSelect={(a) => change({ selected: a.id })}
                        />
                        {tab === "switches" && chosen?.type === "switch" && (
                          <>
                            <h3>{chosen.name} · 已登记端口</h3>
                            <DataTable
                              rows={ports
                                .filter(
                                  (p) => p.fields.owner_device_id === chosen.id,
                                )
                                .map((p) => {
                                  const c = cables.find(
                                    (c) => c.a === p.id || c.b === p.id,
                                  );
                                  return {
                                    id: p.id,
                                    ...p.fields,
                                    cable_id: c?.id || "未登记线缆",
                                    peer_port: c
                                      ? c.a === p.id
                                        ? c.b
                                        : c.a
                                      : "unknown",
                                    source: p.source,
                                  };
                                })}
                              columns={[
                                "id",
                                "physical_name",
                                "speed",
                                "peer_port",
                                "cable_id",
                                "source",
                              ]}
                              onSelect={(p) => {
                                const c = cables.find(
                                  (c) => c.a === p.id || c.b === p.id,
                                );
                                change({
                                  port: p.id,
                                  cable: c?.id || "",
                                  face: "back",
                                });
                              }}
                            />
                          </>
                        )}
                        {tab === "power" && (
                          <p>
                            电源到 PDU
                            的连接证据未提供时显示未采集；不以双电源规格推断真实供电冗余。
                          </p>
                        )}
                      </>
                    )}
                    {(tab === "cabling" || tab === "mapping") && (
                      <>
                        <h2>
                          {tab === "mapping"
                            ? "所选设备关联信息"
                            : "线序与物理布线"}
                        </h2>
                        <p>
                          端口、线缆、所属设备按 CMDB ID
                          追溯，规划连接不等于实际链路已通过核验。
                        </p>
                        <DataTable
                          rows={(selected ? related : cables).map((c) => ({
                            id: c.id,
                            a_device: byId.get(c.device_a)?.name || c.device_a,
                            a_port: byId.get(c.a)?.fields.physical_name || c.a,
                            b_device: byId.get(c.device_b)?.name || c.device_b,
                            b_port: byId.get(c.b)?.fields.physical_name || c.b,
                            ...c.fields,
                            fact: c.fact,
                          }))}
                          columns={[
                            "id",
                            "a_device",
                            "a_port",
                            "b_device",
                            "b_port",
                            "media_type",
                            "length_m",
                            "fact",
                          ]}
                          onSelect={(c) => selectCable(c.id)}
                        />
                      </>
                    )}
                  </div>
                </div>
              ) : (
                <p>
                  机柜不存在、无权访问或机柜总高度未确认；请到 CMDB
                  核查，禁止猜测高度。
                </p>
              )
            ) : view === "quality" ? (
              <>
                <h2>CMDB 物理数据质量</h2>
                <DataTable
                  rows={findingRows.map((f, i) => ({ id: String(i), ...f }))}
                  columns={["asset_id", "code", "message"]}
                  onSelect={(f) =>
                    nav("/cmdb/quality?asset=" + encodeURIComponent(f.asset_id))
                  }
                />
              </>
            ) : view === "cabling" ? (
              <>
                <h2>物理端点与链路追踪</h2>
                <p>
                  选择端口、设备或线缆后按证据展开；默认两跳局部范围，不推断设备内部流量路径。
                </p>
                <DataTable
                  rows={cables
                    .filter(
                      (c) =>
                        !search ||
                        JSON.stringify(c)
                          .toLowerCase()
                          .includes(search.toLowerCase()),
                    )
                    .map((c) => ({
                      id: c.id,
                      a: c.a,
                      b: c.b,
                      device_a: c.device_a,
                      device_b: c.device_b,
                      fact: c.fact,
                      source: c.source,
                    }))}
                  columns={["id", "a", "b", "device_a", "device_b", "fact"]}
                  onSelect={(c) =>
                    change({
                      focus: c.id,
                      cable: c.id,
                      selected: c.device_a,
                      view: "network",
                      port: c.a,
                    })
                  }
                />
              </>
            ) : (
              <>
                {view === "trace" && (
                  <div className="padded">
                    <h2>端口级物理关联路径</h2>
                    <div className="toolbar">
                      <input
                        aria-label="链路起点 ID"
                        list="infra-identities"
                        value={url.get("from") || ""}
                        onChange={(e) =>
                          change({ from: e.target.value, focus: "" })
                        }
                        placeholder="起点设备或端口 ID"
                      />
                      <input
                        aria-label="链路终点 ID"
                        list="infra-identities"
                        value={url.get("to") || ""}
                        onChange={(e) =>
                          change({ to: e.target.value, focus: "" })
                        }
                        placeholder="终点设备或端口 ID"
                      />
                      <datalist id="infra-identities">
                        {devices.map((a) => (
                          <option key={a.id} value={a.id}>
                            {a.name}
                          </option>
                        ))}
                      </datalist>
                    </div>
                    <p>
                      按已登记 Cable
                      双端搜索设备物理关联路径，不证明交换机转发或实际流量。中间配线架没有明确跳接证据时不补造。
                    </p>
                    {url.get("from") &&
                      url.get("to") &&
                      (path ? (
                        <DataTable
                          rows={path.map((c, i) => ({
                            id: c.id,
                            segment: i + 1,
                            a: c.a,
                            b: c.b,
                            device_a: c.device_a,
                            device_b: c.device_b,
                            fact: c.fact,
                          }))}
                          columns={[
                            "segment",
                            "id",
                            "device_a",
                            "a",
                            "device_b",
                            "b",
                            "fact",
                          ]}
                          onSelect={(c) => selectCable(c.id)}
                        />
                      ) : (
                        <p role="status">
                          没有找到完整的已登记物理关联路径；可能缺少线缆、端点或中间跳接证据。
                        </p>
                      ))}
                  </div>
                )}
                <TopologyCanvas
                  {...graph}
                  activeIds={path?.map((c) => c.id)}
                  selected={cableId || selected}
                  onSelect={(id, edge) => {
                    if (id.startsWith("group:")) {
                      change({ view: "network" });
                      return;
                    }
                    const a = byId.get(id);
                    if (
                      a &&
                      ["data_center", "region", "pod"].includes(a.type)
                    ) {
                      change({ group: a.id, view: "groups" });
                      return;
                    }
                    if (a?.type === "rack")
                      nav("/racks/" + encodeURIComponent(id)+"?project_id="+encodeURIComponent(a?.project_id||""));
                    else if (edge) selectCable(id);
                    else change({ selected: id, focus: id });
                  }}
                />
              </>
            )}
          </section>
          <aside className="panel infra-inspector">
            <h3>{chosen?.name || activeCable?.id || "资产与路径详情"}</h3>
            {chosen && (
              <>
                <Badge value={chosen.type} />
                <p className="mono">{chosen.id}</p>
                <p>
                  {chosen.source} · revision {chosen.revision}
                </p>
                {Object.entries(chosen.fields)
                  .filter(([, v]) => typeof v !== "object")
                  .map(([k, v]) => (
                    <div className="property-row" key={k}>
                      <dt>{k}</dt>
                      <dd>{String(v ?? "unknown")}</dd>
                    </div>
                  ))}
                {["server", "switch"].includes(chosen.type) && (
                  <Button onClick={() => openDeviceDrawer(chosen)}>
                    打开设备详情抽屉
                  </Button>
                )}
                {chosen.fields.rack_id && (
                  <Link
                    to={
                      "/racks/" +
                      encodeURIComponent(chosen.fields.rack_id) +
                      "?selected=" +
                      encodeURIComponent(chosen.id) +
                      "&rack_tab=mapping"
                    }
                  >
                    定位机柜 U 位 ↗
                  </Link>
                )}
                <Link
                  to={
                    "/topology?view=network&focus=" +
                    encodeURIComponent(chosen.id) +
                    "&selected=" +
                    encodeURIComponent(chosen.id)
                  }
                >
                  定位数据中心拓扑 ↗
                </Link>
                <Link
                  to={
                    "/cmdb/configuration?asset=" + encodeURIComponent(chosen.id)
                  }
                >
                  查看来源、基线与实际配置 ↗
                </Link>
                <h4>已登记端口</h4>
                {ports
                  .filter((p) => p.fields.owner_device_id === chosen.id)
                  .slice(0, 64)
                  .map((p) => (
                    <button
                      className="infra-port"
                      key={p.id}
                      onClick={() => {
                        change({
                          port: p.id,
                          focus: rackId ? "" : p.id,
                          selected: chosen.id,
                          face: "back",
                        });
                      }}
                    >
                      {p.fields.physical_name || p.fields.port_index || p.id} ·{" "}
                      {p.fields.speed || "unknown"}
                    </button>
                  ))}
              </>
            )}
            {related.map((c) => (
              <section key={c.id} className="infra-link">
                <b>{c.id}</b>
                <p>
                  {c.a} → {c.b}
                </p>
                <p>
                  {c.fact} · {c.fields.media_type || "介质未知"} ·{" "}
                  {c.fields.length_m ?? "长度未知"}{" "}
                  {c.fields.length_m != null ? "m" : ""}
                </p>
                <small>
                  source: {c.source || "未记录"} · 运行状态
                  unknown，缺少双端观测不判 PASS
                </small>
                <Button
                  onClick={() =>
                    nav(
                      "/topology?view=network&focus=" +
                        encodeURIComponent(c.id) +
                        "&cable=" +
                        encodeURIComponent(c.id),
                    )
                  }
                >
                  查看两跳关联路径
                </Button>
              </section>
            ))}
            {!chosen && !activeCable && (
              <p>选择对象查看详情。</p>
            )}
          </aside>
        </div>
      )}
      {confirm && (
        <Modal title="申请修改实际安装位置" onClose={() => setConfirm(false)}>
          <p>
            当前仅创建 CMDB ChangeSet 草稿，后续需要审核和发布。
            {Object.keys(positions).length} 条设备位置差异。
          </p>
          <input
            aria-label="位置变更原因"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder="填写现场依据和变更原因"
          />
          <Button disabled={!reason.trim()} onClick={requestPosition}>
            确认创建变更草稿
          </Button>
        </Modal>
      )}
      <DrawerDetail devices={drawerDevices} />
    </div>
  );
}
