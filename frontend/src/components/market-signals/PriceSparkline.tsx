interface Props {
  data: number[];
  width?: number;
  height?: number;
  color?: string;
}

export function PriceSparkline({ data, width = 80, height = 24, color = "#7ec8e3" }: Props) {
  const safe = data.filter(v => Number.isFinite(v));
  if (safe.length < 2) return <span className="text-eve-dim text-xs">—</span>;
  let min = safe[0], max = safe[0];
  for (let i = 1; i < safe.length; i++) {
    if (safe[i] < min) min = safe[i];
    else if (safe[i] > max) max = safe[i];
  }
  const range = max - min || 1;
  const pts = safe
    .map((v, i) => {
      const x = (i / (safe.length - 1)) * width;
      const y = height - ((v - min) / range) * height;
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");
  return (
    <svg width={width} height={height} className="shrink-0 overflow-visible">
      <polyline points={pts} fill="none" stroke={color} strokeWidth="1.5" strokeLinejoin="round" />
    </svg>
  );
}
