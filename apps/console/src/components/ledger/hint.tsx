import { InfoIcon } from 'lucide-react'

import { cn } from '~/lib/utils'

// 12 Feedback, Hint: explains the ledger in place, where a TigerBeetle rule bites ("Balances live in
// TigerBeetle, so lists cannot sort by them").
export function Hint({ children, className, icon: Icon = InfoIcon }: { children: React.ReactNode; className?: string; icon?: React.ElementType }) {
  return (
    <p className={cn('flex items-center gap-1.5 text-caption text-muted-foreground', className)}>
      <Icon aria-hidden="true" className="size-3.5 shrink-0 text-subtle-foreground" />
      <span>{children}</span>
    </p>
  )
}
