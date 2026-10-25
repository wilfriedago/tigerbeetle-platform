import { useState } from 'react'

import { MenuIcon } from 'lucide-react'

import { Button } from '~/components/ui/button'
import { Sheet, SheetContent, SheetTitle } from '~/components/ui/sheet'

import { Breadcrumbs } from './breadcrumbs'

// The console frame: the sidebar on the left (a drawer below the lg breakpoint), a 52 px top bar
// with the breadcrumb on the left and search, health and theme on the right, and the scrolling page.
interface AppShellProps {
  sidebar: (props: { onNavigate?: () => void; className?: string }) => React.ReactNode
  topBarActions: React.ReactNode
  children: React.ReactNode
}

export function AppShell({ sidebar, topBarActions, children }: AppShellProps) {
  const [menuOpen, setMenuOpen] = useState(false)
  return (
    <div className="flex h-dvh overflow-hidden bg-background">
      <div className="hidden lg:flex">{sidebar({})}</div>
      <Sheet open={menuOpen} onOpenChange={setMenuOpen}>
        <SheetContent side="left" showCloseButton={false} className="w-60 border-0 sm:max-w-60">
          <SheetTitle className="sr-only">Navigation</SheetTitle>
          {sidebar({ className: 'border-r-0', onNavigate: () => setMenuOpen(false) })}
        </SheetContent>
      </Sheet>
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-13 shrink-0 items-center justify-between gap-4 border-b border-border bg-background px-4 sm:px-5">
          <div className="flex min-w-0 items-center gap-2">
            <Button variant="ghost" size="icon-sm" className="lg:hidden" aria-label="Open navigation" onClick={() => setMenuOpen(true)}>
              <MenuIcon />
            </Button>
            <Breadcrumbs />
          </div>
          <div className="flex shrink-0 items-center gap-3">{topBarActions}</div>
        </header>
        <main id="main" className="min-h-0 flex-1 overflow-y-auto">
          {children}
        </main>
      </div>
    </div>
  )
}
