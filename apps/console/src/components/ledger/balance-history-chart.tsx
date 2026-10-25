import { useEffect, useRef, useState } from 'react'

import { assetCode, assetScale, formatAmount } from '~/lib/money'
import { formatTimestamp } from '~/lib/time'

// 21 Ledger patterns, balance history: one series, so no legend; the title names it. A step line,
// because a balance holds its value until the next transfer. 2 px accent line, recessive grid, one
// axis, and a crosshair that snaps to the nearest change. Geometry uses bigint ratios; every number
// a reader sees is formatted from the integer string.

export interface HistoryPoint {
  /** ISO time of the change. */
  at: string
  /** Balance after the change, smallest unit. */
  balance: string
}

const HEIGHT = 220
const PAD = { bottom: 28, left: 12, right: 12, top: 16 }

function useWidth<T extends HTMLElement>(): [React.RefObject<T | null>, number] {
  const ref = useRef<T | null>(null)
  const [width, setWidth] = useState(0)
  useEffect(() => {
    const element = ref.current
    if (!element) {
      return
    }
    const observer = new ResizeObserver(([entry]) => setWidth(entry?.contentRect.width ?? 0))
    observer.observe(element)
    return () => observer.disconnect()
  }, [])
  return [ref, width]
}

/** Position of `value` between `min` and `max` as a fraction, computed with bigint. */
function fraction(value: bigint, min: bigint, max: bigint): number {
  if (max === min) {
    return 0.5
  }
  return Number(((value - min) * 10_000n) / (max - min)) / 10_000
}

export function BalanceHistoryChart({ points, asset }: { points: readonly HistoryPoint[]; asset: string }) {
  const [ref, width] = useWidth<HTMLDivElement>()
  const [hover, setHover] = useState<number | undefined>()
  const scale = assetScale(asset)
  const sorted = points.toSorted((a, b) => a.at.localeCompare(b.at))
  const values = sorted.map((point) => BigInt(point.balance))
  const times = sorted.map((point) => Date.parse(point.at))
  if (sorted.length === 0 || width === 0) {
    return <div ref={ref} style={{ height: HEIGHT }} />
  }
  let min = values.reduce((a, b) => (b < a ? b : a))
  let max = values.reduce((a, b) => (b > a ? b : a))
  // Zero is part of the frame: a balance reads against nothing, not against its own minimum.
  if (min > 0n) {
    min = 0n
  }
  if (max < 0n) {
    max = 0n
  }
  const first = times[0] ?? 0
  const last = Math.max(times.at(-1) ?? 0, first + 1)
  const plotW = width - PAD.left - PAD.right
  const plotH = HEIGHT - PAD.top - PAD.bottom
  const x = (t: number) => PAD.left + ((t - first) / (last - first)) * plotW
  const y = (v: bigint) => PAD.top + (1 - fraction(v, min, max)) * plotH

  let path = ''
  sorted.forEach((_, index) => {
    const px = x(times[index] ?? first)
    const py = y(values[index] ?? 0n)
    if (index === 0) {
      path = `M${px.toFixed(1)},${py.toFixed(1)}`
    } else {
      path += `H${px.toFixed(1)}V${py.toFixed(1)}`
    }
  })
  path += `H${(PAD.left + plotW).toFixed(1)}`

  const zeroY = y(0n)
  const gridValues = [max, min].filter((value, index, all) => all.indexOf(value) === index)
  const active = hover === undefined ? undefined : sorted[hover]

  const onPointer = (event: React.PointerEvent<SVGRectElement>) => {
    const box = event.currentTarget.getBoundingClientRect()
    const t = first + ((event.clientX - box.left) / box.width) * (last - first)
    // The balance at time t is the last change at or before t.
    let index = 0
    for (const [i, time] of times.entries()) {
      if (time <= t) {
        index = i
      }
    }
    setHover(index)
  }

  return (
    <div ref={ref} className="relative">
      <svg width={width} height={HEIGHT} role="img" aria-label={`Balance over time in ${asset}, ${sorted.length} changes`} className="block">
        {gridValues.map((value) => (
          <g key={value.toString()}>
            <line x1={PAD.left} x2={PAD.left + plotW} y1={y(value)} y2={y(value)} className="stroke-border-subtle" strokeWidth={1} />
            <text x={PAD.left + plotW} y={y(value) - 4} textAnchor="end" className="fill-subtle-foreground font-mono text-[10px]">
              {formatAmount(value.toString(), scale)}
            </text>
          </g>
        ))}
        <line x1={PAD.left} x2={PAD.left + plotW} y1={zeroY} y2={zeroY} className="stroke-border-strong" strokeWidth={1} />
        <path d={path} fill="none" className="stroke-bar-available" strokeWidth={2} strokeLinejoin="round" />
        <text x={PAD.left} y={HEIGHT - 8} className="fill-subtle-foreground text-[10px]">
          {formatTimestamp(sorted[0]?.at ?? '')}
        </text>
        <text x={PAD.left + plotW} y={HEIGHT - 8} textAnchor="end" className="fill-subtle-foreground text-[10px]">
          {formatTimestamp(sorted.at(-1)?.at ?? '')}
        </text>
        {active && hover !== undefined ? (
          <g>
            <line
              x1={x(times[hover] ?? first)}
              x2={x(times[hover] ?? first)}
              y1={PAD.top}
              y2={PAD.top + plotH}
              className="stroke-border-strong"
              strokeWidth={1}
            />
            <circle cx={x(times[hover] ?? first)} cy={y(values[hover] ?? 0n)} r={4} className="fill-bar-available stroke-surface" strokeWidth={2} />
          </g>
        ) : null}
        <rect
          x={PAD.left}
          y={PAD.top}
          width={plotW}
          height={plotH}
          fill="transparent"
          onPointerMove={onPointer}
          onPointerLeave={() => setHover(undefined)}
        />
      </svg>
      {active && hover !== undefined ? (
        <div
          role="status"
          className="pointer-events-none absolute top-2 flex flex-col gap-0.5 rounded-md bg-tooltip px-2.5 py-1.5 text-caption text-tooltip-foreground shadow-float"
          style={{ left: Math.min(Math.max(x(times[hover] ?? first) + 10, 0), width - 180) }}
        >
          <span className="font-mono font-medium tabular-nums">
            {formatAmount(active.balance, scale)} {assetCode(asset)}
          </span>
          <span className="opacity-75">{formatTimestamp(active.at)}</span>
        </div>
      ) : null}
    </div>
  )
}
