// Isolated synthetic layout benchmark. Never imports or writes CMDB assets.
import ELK from "elkjs/lib/elk.bundled.js";
import { writeFile } from "node:fs/promises";
const rows = [];
for (const [nodes, relations] of [
  [100, 300],
  [500, 2000],
  [1000, 5000],
  [5000, 20000],
  [10000, 50000],
]) {
  const begin = performance.now(),
    groups = Math.min(100, nodes),
    links = new Map();
  for (let i = 0; i < relations; i++) {
    const a = i % nodes,
      b = (a + 1 + (i % 3) * Math.ceil(nodes / groups)) % nodes;
    const source = "group-" + Math.floor((a * groups) / nodes),
      target = "group-" + Math.floor((b * groups) / nodes);
    if (source !== target)
      links.set(source + ">" + target, {
        id: source + ">" + target,
        sources: [source],
        targets: [target],
      });
  }
  const grouped = performance.now();
  const result = await new ELK().layout({
    id: "synthetic",
    layoutOptions: {
      "elk.algorithm": "layered",
      "elk.direction": "DOWN",
      "elk.edgeRouting": "ORTHOGONAL",
    },
    children: Array.from({ length: groups }, (_, i) => ({
      id: "group-" + i,
      width: 168,
      height: 56,
    })),
    edges: [...links.values()],
  });
  rows.push({
    synthetic: true,
    input_nodes: nodes,
    input_relations: relations,
    render_nodes: result.children.length,
    render_edges: result.edges.length,
    aggregation_ms: +(grouped - begin).toFixed(2),
    elk_ms: +(performance.now() - grouped).toFixed(2),
  });
}
const report = {
  scope:
    "Synthetic aggregation + ELK layout only, NOT raw full-graph render FPS or production capacity",
  rows,
};
console.log(JSON.stringify(report, null, 2));
await writeFile(
  new URL("../../docs/INFRASTRUCTURE-PERF.json", import.meta.url),
  JSON.stringify(report, null, 2) + "\n",
);
