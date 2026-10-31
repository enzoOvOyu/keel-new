import { useRef, useState } from "react";
import type { InfraAsset, InfraCable } from "../../lib/api/infrastructure";
export function RackElevation({
  rack,
  devices,
  ports,
  cables,
  positions,
  face,
  selected,
  portSelected,
  edit,
  onSelect,
  onPort,
  onMove,
  svgRef,
}: {
  rack: InfraAsset;
  devices: InfraAsset[];
  ports: InfraAsset[];
  cables: InfraCable[];
  positions: Record<string, number>;
  face: string;
  selected: string;
  portSelected: string;
  edit: boolean;
  onSelect: (id: string) => void;
  onPort: (id: string) => void;
  onMove: (id: string, u: number) => void;
  svgRef: React.RefObject<SVGSVGElement | null>;
}) {
  const [ghost, setGhost] = useState<number | null>(null);
  const drag = useRef<{ id: string; offset: number } | null>(null);
  const height = Number(rack.fields.u_height),
    unit = 24,
    top = 42;
  const valid = devices.filter(
    (d) =>
      Number(d.fields.u_height) > 0 &&
      Number(positions[d.id] ?? d.fields.u_start ?? d.fields.rack_u) > 0,
  );
  const related = cables.filter((c) =>
    portSelected
      ? c.a === portSelected || c.b === portSelected
      : c.device_a === selected || c.device_b === selected,
  );
  const endpoints = new Set(related.flatMap((c) => [c.a, c.b]));
  const devicePorts = new Map(
    devices.map((d) => [
      d.id,
      ports.filter((p) => p.fields.owner_device_id === d.id),
    ]),
  );
  function point(id: string) {
    const p = ports.find((p) => p.id === id),
      d = devices.find((d) => d.id === p?.fields.owner_device_id);
    if (!d) return null;
    const ps = devicePorts.get(d.id) || [],
      idx = ps.findIndex((p) => p.id === id);
    return {
      x: 80 + (idx % 12) * 17,
      y:
        top +
        (height -
          Number(positions[d.id] ?? d.fields.u_start ?? d.fields.rack_u) -
          Number(d.fields.u_height) +
          1) *
          unit +
        Number(d.fields.u_height) * unit -
        12 -
        Math.floor(idx / 12) * 11,
    };
  }
  return (
    <svg
      ref={svgRef}
      viewBox={`0 0 350 ${top + height * unit + 30}`}
      role="img"
      aria-label={`${rack.name} ${face === "front" ? "正面" : "背面"} ${height}U`}
      onPointerMove={(e) => {
        if (drag.current && edit) {
          e.preventDefault();
          const rect = e.currentTarget.getBoundingClientRect();
          setGhost(
            Math.round(
              height -
                (((e.clientY - rect.top) * (top + height * unit + 30)) /
                  rect.height -
                  top) /
                  unit +
                drag.current.offset,
            ),
          );
        }
      }}
      onPointerUp={(e) => {
        if (!drag.current) return;
        const rect = e.currentTarget.getBoundingClientRect(),
          y =
            ((e.clientY - rect.top) * (top + height * unit + 30)) / rect.height;
        onMove(
          drag.current.id,
          Math.round(height - (y - top) / unit + drag.current.offset),
        );
        drag.current = null;
        setGhost(null);
      }}
    >
      <rect
        x="38"
        y="20"
        width="284"
        height={height * unit + 42}
        rx="7"
        fill="#172237"
      />
      <text x="180" y="36" fill="#dce6f7" fontSize="10" textAnchor="middle">
        {rack.name} · {height}U · {face === "front" ? "FRONT" : "REAR"}
      </text>
      {Array.from({ length: height }, (_, i) => {
        const u = height - i,
          occupied = valid.some((d) => {
            const start =
              positions[d.id] ?? Number(d.fields.u_start ?? d.fields.rack_u);
            return u >= start && u < start + Number(d.fields.u_height);
          });
        return (
          <g key={u}>
            <rect
              x="58"
              y={top + i * unit}
              width="244"
              height={unit - 1}
              fill={occupied ? "#243149" : "#101a2a"}
            />
            <text
              x="32"
              y={top + i * unit + 16}
              textAnchor="end"
              fontSize="9"
              fill="#71839e"
            >
              {u}
            </text>
            {!occupied && (
              <text
                x="175"
                y={top + i * unit + 15}
                textAnchor="middle"
                fontSize="8"
                fill="#53647d"
              >
                空闲 U{u}
              </text>
            )}
            <circle cx="48" cy={top + i * unit + 12} r="2" fill="#71839e" />
            <circle cx="312" cy={top + i * unit + 12} r="2" fill="#71839e" />
          </g>
        );
      })}
      {valid.map((d) => {
        const u =
            positions[d.id] ?? Number(d.fields.u_start ?? d.fields.rack_u),
          h = Number(d.fields.u_height) * unit - 2,
          y = top + (height - u - Number(d.fields.u_height) + 1) * unit;
        const ps = devicePorts.get(d.id) || [];
        return (
          <g
            key={d.id}
            role="button"
            tabIndex={0}
            aria-label={d.name}
            onKeyDown={(e) => {
              if (e.key === "Enter") onSelect(d.id);
              if (edit && ["ArrowUp", "ArrowDown"].includes(e.key)) {
                e.preventDefault();
                onMove(d.id, u + (e.key === "ArrowUp" ? 1 : -1));
              }
            }}
            onClick={() => onSelect(d.id)}
            onPointerDown={(e) => {
              onSelect(d.id);
              if (edit) {
                const bounds = svgRef.current!.getBoundingClientRect();
                const pointerU =
                  height -
                  (((e.clientY - bounds.top) * (top + height * unit + 30)) /
                    bounds.height -
                    top) /
                    unit;
                drag.current = { id: d.id, offset: u - pointerU };
                e.currentTarget.setPointerCapture(e.pointerId);
              }
            }}
            style={{ cursor: edit ? "grab" : "pointer" }}
          >
            <rect
              x="59"
              y={y + 1}
              width="242"
              height={h}
              rx="2"
              fill={
                d.type === "switch"
                  ? "#24514e"
                  : d.fields.gpu_count
                    ? "#334d74"
                    : "#35425a"
              }
              stroke={selected === d.id ? "#91bfff" : "#6b7c96"}
              strokeWidth={selected === d.id ? 3 : 1}
            />
            <rect
              x="63"
              y={y + 5}
              width="4"
              height={Math.max(6, h - 10)}
              fill={
                d.fields.maintenance_state === "maintained"
                  ? "#73a7ee"
                  : d.fields.health_status === "critical"
                    ? "#ec7a8c"
                    : d.fields.health_status === "healthy" &&
                        d.fields.freshness === "fresh"
                      ? "#8bbfa8"
                      : "#8695ab"
              }
            />
            <text x="74" y={y + 13} fill="#f0f5ff" fontSize="9">
              {d.name.slice(0, 33)}
            </text>
            {h > 42 && (
              <>
                <text x="74" y={y + 28} fill="#bacce7" fontSize="8">
                  {d.fields.model || "型号未确认"}
                </text>
                <text x="74" y={y + 41} fill="#bacce7" fontSize="7">
                  {d.id.slice(0, 30)} · U{u}
                </text>
              </>
            )}
            {face === "back" ? (
              ps.slice(0, h < 40 ? 12 : 48).map((p, i) => (
                <g
                  key={p.id}
                  onPointerDown={(e) => e.stopPropagation()}
                  onClick={(e) => {
                    e.stopPropagation();
                    onPort(p.id);
                  }}
                >
                  <rect
                    x={73 + (i % 12) * 17}
                    y={y + h - 18 - Math.floor(i / 12) * 11}
                    width="13"
                    height="8"
                    fill={endpoints.has(p.id) ? "#f4cb76" : "#111e31"}
                    stroke={endpoints.has(p.id) ? "#ffe3a3" : "#8ba4c3"}
                  />
                  <title>
                    {p.fields.physical_name || p.fields.port_index || p.id} ·{" "}
                    {p.fields.speed || "速率 unknown"}
                  </title>
                </g>
              ))
            ) : (
              <>
                {h > 55 && (
                  <path
                    d={`M80 ${y + h - 15} h170`}
                    stroke="#7188a8"
                    strokeDasharray="2 3"
                  />
                )}
                <text
                  x="288"
                  y={y + 13}
                  textAnchor="end"
                  fill="#c6d8ee"
                  fontSize="7"
                >
                  {d.type === "switch"
                    ? "SW"
                    : d.fields.gpu_count
                      ? "GPU"
                      : "SRV"}
                </text>
              </>
            )}
          </g>
        );
      })}
      {ghost !== null && (
        <g pointerEvents="none">
          <line
            x1="40"
            x2="320"
            y1={top + (height - ghost) * unit}
            y2={top + (height - ghost) * unit}
            stroke="#f0c46d"
            strokeWidth="3"
          />
          <text
            x="180"
            y={top + (height - ghost) * unit - 5}
            fill="#e8b95c"
            fontSize="12"
          >
            草稿 U{ghost}
          </text>
        </g>
      )}
      {face === "back" &&
        related.slice(0, 24).map((c) => {
          const a = point(c.a),
            b = point(c.b);
          return a && b ? (
            <path
              key={c.id}
              d={`M${a.x} ${a.y} C340 ${a.y},340 ${b.y},${b.x} ${b.y}`}
              fill="none"
              stroke="#efc164"
              strokeWidth="1.5"
            >
              <title>{c.id}</title>
            </path>
          ) : null;
        })}
    </svg>
  );
}
