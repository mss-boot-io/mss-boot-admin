import { theme } from 'antd';
import { useId } from 'react';

export interface MonitorTrendPoint {
  timestamp: number;
  value: number;
}

export interface MonitorTrendProps {
  ariaLabel: string;
  color?: string;
  points: readonly MonitorTrendPoint[];
}

const WIDTH = 640;
const HEIGHT = 176;
const LEFT = 12;
const RIGHT = 12;
const TOP = 12;
const BOTTOM = 12;

function coordinates(points: readonly MonitorTrendPoint[]): Array<[number, number]> {
  const drawableWidth = WIDTH - LEFT - RIGHT;
  const drawableHeight = HEIGHT - TOP - BOTTOM;
  const start = points.at(0)?.timestamp ?? 0;
  const end = points.at(-1)?.timestamp ?? start;
  const duration = Math.max(1, end - start);
  return points.map((point) => [
    LEFT + ((point.timestamp - start) / duration) * drawableWidth,
    TOP + ((100 - point.value) / 100) * drawableHeight,
  ]);
}

export default function MonitorTrend({ ariaLabel, color, points }: MonitorTrendProps) {
  const { token } = theme.useToken();
  const gradientId = `monitor-trend-${useId().replaceAll(':', '')}`;
  const plotted = coordinates(points);
  const line = plotted.map(([x, y]) => `${x},${y}`).join(' ');
  const area = plotted.length
    ? `${LEFT},${HEIGHT - BOTTOM} ${line} ${WIDTH - RIGHT},${HEIGHT - BOTTOM}`
    : '';
  const stroke = color ?? token.colorPrimary;

  return (
    <svg
      aria-label={ariaLabel}
      className="block h-44 w-full"
      preserveAspectRatio="none"
      role="img"
      viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
    >
      <defs>
        <linearGradient id={gradientId} x1="0" x2="0" y1="0" y2="1">
          <stop offset="0%" stopColor={stroke} stopOpacity="0.28" />
          <stop offset="100%" stopColor={stroke} stopOpacity="0.02" />
        </linearGradient>
      </defs>
      {[0, 50, 100].map((value) => {
        const y = TOP + ((100 - value) / 100) * (HEIGHT - TOP - BOTTOM);
        return (
          <line
            key={value}
            stroke={token.colorBorderSecondary}
            strokeDasharray="4 5"
            vectorEffect="non-scaling-stroke"
            x1={LEFT}
            x2={WIDTH - RIGHT}
            y1={y}
            y2={y}
          />
        );
      })}
      {plotted.length > 0 ? (
        <>
          <polygon fill={`url(#${gradientId})`} points={area} />
          <polyline
            fill="none"
            points={line}
            stroke={stroke}
            strokeLinejoin="round"
            strokeWidth="2"
            vectorEffect="non-scaling-stroke"
          />
        </>
      ) : null}
    </svg>
  );
}
