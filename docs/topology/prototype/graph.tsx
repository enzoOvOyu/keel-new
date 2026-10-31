import { useEffect, useRef, useState } from "react";
import { Graph } from "@antv/g6";
export type DrawNode = {
  id: string;
  label: string;
  type: string;
  health?: string;
};
export type DrawEdge = {
  id: string;
  source: string;
  target: string;
  label?: string;
  planned?: boolean;
};
export function TopologyCanvas({
  nodes,
  edges,
  selected,
  activeIds = [],
  onSelect,
}: {
  nodes: DrawNode[];
  edges: DrawEdge[];
  selected: string;
  activeIds?: string[];
  onSelect: (id: string, edge: boolean) => void;
}) {
  const host = useRef<HTMLDivElement>(null),
    graph = useRef<Graph | null>(null),
    callback = useRef(onSelect);
  callback.current = onSelect;
  const selectionRef = useRef({ selected, activeIds });
  selectionRef.current = { selected, activeIds };
  const [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  useEffect(() => {
    if (!host.current) return;
    let disposed = false;
    setBusy(true);
    setError("");
    const worker = new Worker(new URL("../../../dev/src/features/infrastructure/layout.worker.ts", import.meta.url), {
      type: "module",
    });
    worker.onmessage = async ({ data }) => {
      if (disposed) return;
      if (data.error) {
        setError(data.error);
        setBusy(false);
        return;
      }
      try {
        const selection = selectionRef.current;
        const high = new Set([selection.selected, ...selection.activeIds]);
        for (const e of edges)
          if (
            high.has(e.id) ||
            e.source === selection.selected ||
            e.target === selection.selected
          ) {
            high.add(e.id);
            high.add(e.source);
            high.add(e.target);
          }
        const state = (id: string) =>
          selection.selected || selection.activeIds.length
            ? [high.has(id) ? "active" : "inactive"]
            : [];
        const positions = new Map(data.nodes.map((n: any) => [n.id, n]));
        const sections = new Map(
          data.edges.map((e: any) => [e.id, e.sections?.[0]]),
        );
        const g = new Graph({
          container: host.current!,
          autoResize: true,
          height: host.current!.clientHeight || 620,
          animation: false,
          autoFit: "view",
          behaviors: ["drag-canvas", "zoom-canvas"],
          node: {
            type: "rect",
            state: {
              active: { lineWidth: 3, stroke: "#6d82cf" },
              inactive: { opacity: 0.2 },
            },
            style: {
              size: [168, 56],
              radius: 5,
              labelFill: "#26344a",
              labelFontSize: 11,
              labelPlacement: "center",
            },
          },
          edge: {
            type: "polyline",
            state: {
              active: { lineWidth: 3, stroke: "#c29536" },
              inactive: { opacity: 0.12 },
            },
            style: { stroke: "#a6b4ca", endArrow: false },
          },
          data: {
            nodes: nodes.map((n) => {
              const p = positions.get(n.id) as any;
              return {
                id: n.id,
                states: state(n.id),
                data: { label: n.label },
                style: {
                  x: p.x + 84,
                  y: p.y + 28,
                  labelText: n.label,
                  fill:
                    n.type === "rack"
                      ? "#e5edf9"
                      : n.type === "switch"
                        ? "#dceee7"
                        : "#f1f4fa",
                  stroke: n.health === "critical" ? "#dc5b6b" : "#8e9eb8",
                },
              };
            }),
            edges: edges.map((e) => {
              const section = sections.get(e.id) as any;
              return {
                ...e,
                states: state(e.id),
                style: {
                  labelText: e.label,
                  lineDash: e.planned ? [5, 4] : undefined,
                  controlPoints: section?.bendPoints?.map((p: any) => [p.x,p.y]) || [],
                },
              };
            }),
          },
        });
        graph.current = g;
        g.on("node:click", (e: any) => callback.current(e.target.id, false));
        g.on("node:dblclick", (e: any) => callback.current(e.target.id, false));
        g.on("edge:click", (e: any) => callback.current(e.target.id, true));
        await g.render();
        await g.zoomTo(Math.min(1,g.getZoom()*.9));
        if (disposed) g.destroy();
      } catch (e) {
        if (!disposed) setError(String(e));
      } finally {
        if (!disposed) setBusy(false);
      }
    };
    worker.onerror = (e) => {
      setError(e.message);
      setBusy(false);
    };
    worker.postMessage({ nodes, edges });
    return () => {
      disposed = true;
      worker.terminate();
      graph.current?.destroy();
      graph.current = null;
    };
  }, [nodes, edges]);
  useEffect(() => {
    const g = graph.current;
    if (!g) return;
    const ids = new Set([selected, ...activeIds]);
    for (const e of edges)
      if (ids.has(e.id) || e.source === selected || e.target === selected) {
        ids.add(e.id);
        ids.add(e.source);
        ids.add(e.target);
      }
    void g
      .setElementState(
        Object.fromEntries(
          [...nodes, ...edges].map((n) => [
            n.id,
            selected || activeIds.length
              ? ids.has(n.id)
                ? "active"
                : "inactive"
              : [],
          ]),
        ),
      )
      .catch(() => {});
  }, [selected, nodes, edges, JSON.stringify(activeIds)]);
  return (
    <div className="infra-graph-wrap">
      <div className="graph-tools"><button aria-label="放大画布" onClick={()=>graph.current?.zoomTo(Math.min(2,(graph.current?.getZoom()||1)*1.2))}>＋</button><button aria-label="缩小画布" onClick={()=>graph.current?.zoomTo(Math.max(.2,(graph.current?.getZoom()||1)/1.2))}>−</button><button onClick={async()=>{await graph.current?.fitView();if((graph.current?.getZoom()||1)>1)await graph.current?.zoomTo(1)}}>适应画布</button></div>
      <div
        ref={host}
        className="infra-graph"
        aria-label="数据中心物理拓扑画布"
      />
      {busy && (
        <div className="infra-canvas-status">Worker 正在计算分层布局…</div>
      )}
      {error && (
        <div className="error-panel" role="alert">
          {error}
        </div>
      )}
      <small>
        滚轮缩放 · 拖动画布 · 点击节点或链路查看 ·
        实线仅表示记录；不代表运行验证通过
      </small>
    </div>
  );
}
