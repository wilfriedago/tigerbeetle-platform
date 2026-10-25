import { cn } from '~/lib/utils'

// 22 Pickers and steps, Stepper/Labeled: equal pill segments filled up to the current step. A
// finished step can be reopened by clicking it; upcoming steps cannot be skipped to.

interface StepperProps {
  steps: readonly string[]
  /** Zero-based index of the current step. */
  current: number
  onSelect?: (index: number) => void
  className?: string
}

export function Stepper({ steps, current, onSelect, className }: StepperProps) {
  return (
    <ol className={cn('grid gap-3', className)} style={{ gridTemplateColumns: `repeat(${steps.length}, minmax(0, 1fr))` }}>
      {steps.map((name, index) => {
        const done = index < current
        const active = index === current
        const reachable = done && onSelect !== undefined
        const content = (
          <>
            <span className={cn('h-1.5 w-full rounded-full', done || active ? 'bg-primary' : 'bg-primary-subtle-hover')} />
            <span className="flex items-center gap-1.5">
              <span className={cn('font-mono text-caption', active ? 'text-primary-text' : 'text-subtle-foreground')}>{index + 1}</span>
              <span className={cn('text-small', active ? 'font-medium text-foreground' : done ? 'text-muted-foreground' : 'text-subtle-foreground')}>
                {name}
              </span>
            </span>
          </>
        )
        return (
          <li key={name} aria-current={active ? 'step' : undefined}>
            {reachable ? (
              <button
                type="button"
                onClick={() => onSelect(index)}
                className="flex w-full flex-col gap-2 rounded-sm text-left outline-none hover:[&>span:last-child>span:last-child]:text-foreground focus-visible:ring-3 focus-visible:ring-focus-ring"
              >
                {content}
              </button>
            ) : (
              <div className="flex flex-col gap-2">{content}</div>
            )}
          </li>
        )
      })}
    </ol>
  )
}
