interface Props {
  pricesA: number[];
  pricesB: number[];
  volumesB: number[];
  lagDays: number;
  labelA: string;
  labelB: string;
}

export function DualSeriesChart({ pricesA, pricesB, volumesB, lagDays, labelA, labelB }: Props) {
  const W = 480, PH = 160, VH = 28, PAD = 8;

  // Shift B forward by lagDays so peaks align visually with A
  const bShifted: (number | null)[] = [
    ...Array.from({ length: lagDays }, () => null),
    ...pricesB,
  ];

  const allPrices = [...pricesA, ...pricesB].filter((v) => Number.isFinite(v));
  if (allPrices.length < 2) {
    return <p className="text-eve-dim text-xs py-4 text-center">Insufficient data for chart</p>;
  }

  const minP = Math.min(...allPrices);
  const maxP = Math.max(...allPrices);
  const rangeP = maxP - minP || 1;
  const n = Math.max(pricesA.length, bShifted.length);

  function toY(v: number) {
    return PH - PAD - ((v - minP) / rangeP) * (PH - 2 * PAD);
  }

  function polyline(series: (number | null)[], color: string) {
    const segments: string[] = [];
    let current: string[] = [];
    series.forEach((v, i) => {
      if (v === null || !Number.isFinite(v)) {
        if (current.length > 1) segments.push(current.join(" "));
        current = [];
        return;
      }
      const x = PAD + (i / Math.max(n - 1, 1)) * (W - 2 * PAD);
      current.push(`${x.toFixed(1)},${toY(v).toFixed(1)}`);
    });
    if (current.length > 1) segments.push(current.join(" "));
    return segments.map((pts, i) => (
      <polyline key={i} points={pts} fill="none" stroke={color} strokeWidth="1.5" strokeLinejoin="round" />
    ));
  }

  const maxVol = Math.max(...volumesB.filter((v) => v > 0), 1);

  return (
    <div className="flex flex-col gap-2">
      <svg width="100%" viewBox={`0 0 ${W} ${PH + VH + 4}`} className="overflow-visible">
        {/* Volume bars for B */}
        {volumesB.map((v, i) => {
          const bh = (v / maxVol) * VH;
          const x = PAD + ((i + lagDays) / Math.max(n - 1, 1)) * (W - 2 * PAD);
          return (
            <rect
              key={i}
              x={x - 1}
              y={PH + 4 + VH - bh}
              width={2}
              height={bh}
              className="fill-eve-dim/30"
            />
          );
        })}
        {/* Series A */}
        {polyline(pricesA, "#7ec8e3")}
        {/* Series B (lag-shifted) */}
        {polyline(bShifted, "#f59e0b")}
      </svg>
      <div className="flex gap-4 text-[10px] text-eve-dim">
        <span>
          <span className="inline-block w-3 h-0.5 align-middle mr-1" style={{ backgroundColor: "#7ec8e3" }} />
          {labelA}
        </span>
        <span>
          <span className="inline-block w-3 h-0.5 bg-amber-400 align-middle mr-1" />
          {labelB}{lagDays > 0 ? ` (lag ${lagDays}d)` : ""}
        </span>
        <span className="text-eve-dim/50">▬ vol</span>
      </div>
    </div>
  );
}
