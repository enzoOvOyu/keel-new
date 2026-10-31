import ELK from "elkjs/lib/elk.bundled.js";
const elk = new ELK();
self.onmessage = async ({ data }) => {
  try {
    const result = await elk.layout({
      id: "root",
      layoutOptions: {
        "elk.algorithm": "layered",
        "elk.direction": "DOWN",
        "elk.edgeRouting": "ORTHOGONAL",
        "elk.spacing.nodeNode": "35",
        "elk.layered.spacing.nodeNodeBetweenLayers": "75",
      },
      children: data.nodes.map((n: { id: string }) => ({
        id: n.id,
        width: 168,
        height: 56,
      })),
      edges: data.edges.map(
        (e: { id: string; source: string; target: string }) => ({
          id: e.id,
          sources: [e.source],
          targets: [e.target],
        }),
      ),
    });
    self.postMessage({ nodes: result.children, edges: result.edges });
  } catch (e) {
    self.postMessage({ error: String(e) });
  }
};
