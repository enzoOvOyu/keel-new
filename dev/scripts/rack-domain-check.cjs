// Execute pure rack/path functions without a browser or database writes.
const fs = require("fs"),
  ts = require("typescript"),
  assert = require("node:assert/strict");
const source = fs.readFileSync(
  new URL("../src/lib/api/infrastructure.ts", "file://" + __filename),
  "utf8",
);
const output = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.CommonJS,
    target: ts.ScriptTarget.ES2022,
  },
}).outputText;
const mod = { exports: {} };
new Function("require", "exports", "module", output)(
  () => ({
    cmdbFetch() {
      throw Error("No HTTP calls in domain checks");
    },
  }),
  mod.exports,
  mod,
);
const { locationIssues, physicalPath } = mod.exports;
for (const height of [42, 45, 48]) {
  const device = {
    id: "synthetic-server",
    name: "synthetic",
    fields: { u_start: height - 5, u_height: 6 },
  };
  assert.equal(locationIssues([device], height).length, 0);
  assert(
    locationIssues([device], height, { [device.id]: height - 4 }).some(
      (x) => x.code === "U_RANGE",
    ),
  );
  assert(
    locationIssues([device, { ...device, id: "synthetic-peer" }], height).some(
      (x) => x.code === "U_OVERLAP",
    ),
  );
}
const links = [
  {
    id: "synthetic-a",
    a: "nic-port",
    b: "leaf-down",
    device_a: "server",
    device_b: "leaf",
  },
  {
    id: "synthetic-b",
    a: "leaf-up",
    b: "spine-down",
    device_a: "leaf",
    device_b: "spine",
  },
];
assert.deepEqual(
  physicalPath(links, "nic-port", "spine-down").map((c) => c.id),
  ["synthetic-a", "synthetic-b"],
);
assert.equal(physicalPath(links, "nic-port", "nonexistent"), null);
console.log(
  "42U/45U/48U bounds, overlap, physical endpoint path and missing-path checks passed (synthetic only).",
);
