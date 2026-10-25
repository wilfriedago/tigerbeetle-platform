import { ChevronLeftIcon, ChevronRightIcon } from 'lucide-react'

import { Button } from '~/components/ui/button'

import { cn } from '~/lib/utils'

// 13 Navigation, Cursor Pager: Previous and Next only. The pager never shows totals or page numbers
// because the API pages with cursors (ADR-0004).
interface CursorPagerProps {
  info?: React.ReactNode
  hasPrevious: boolean
  hasNext: boolean
  onPrevious: () => void
  onNext: () => void
  busy?: boolean
  className?: string
}

export function CursorPager({
  info = 'Newest first. 25 per page.',
  hasPrevious,
  hasNext,
  onPrevious,
  onNext,
  busy = false,
  className
}: CursorPagerProps) {
  return (
    <nav aria-label="Pagination" className={cn('flex items-center justify-between gap-2', className)}>
      <span className="text-caption text-muted-foreground">{info}</span>
      <div className="flex gap-2">
        <Button variant="secondary" size="sm" disabled={!hasPrevious || busy} onClick={onPrevious}>
          <ChevronLeftIcon />
          Previous
        </Button>
        <Button variant="secondary" size="sm" disabled={!hasNext || busy} onClick={onNext}>
          Next
          <ChevronRightIcon />
        </Button>
      </div>
    </nav>
  )
}
