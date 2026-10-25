import { Tabs as TabsPrimitive } from '@base-ui/react/tabs'

import { cn } from '~/lib/utils'

// 13 Navigation, Tabs: tabs switch views of one entity. 36 px, label 14 px with an optional mono
// count; the active tab is medium weight with a 2 px accent underline over the list hairline.

function Tabs({ className, ...props }: TabsPrimitive.Root.Props) {
  return <TabsPrimitive.Root data-slot="tabs" className={cn('group/tabs flex flex-col gap-4', className)} {...props} />
}

const tabsListClassName = 'group/tabs-list relative flex w-full items-center gap-6 border-b border-border'

function TabsList({ className, ...props }: TabsPrimitive.List.Props) {
  return <TabsPrimitive.List data-slot="tabs-list" className={cn(tabsListClassName, className)} {...props} />
}

// Shared with route-driven tabs (TabLink) so URL tabs and in-page tabs look the same.
const tabTriggerClassName =
  "relative -mb-px inline-flex h-9 items-center gap-1.5 border-b-2 border-transparent px-0.5 text-body whitespace-nowrap text-muted-foreground transition-colors outline-none hover:text-foreground focus-visible:rounded-sm focus-visible:ring-3 focus-visible:ring-focus-ring disabled:pointer-events-none disabled:opacity-45 data-active:border-primary data-active:font-medium data-active:text-foreground [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4"

function TabsTrigger({ className, ...props }: TabsPrimitive.Tab.Props) {
  return <TabsPrimitive.Tab data-slot="tabs-trigger" className={cn(tabTriggerClassName, className)} {...props} />
}

/** The mono count next to a tab label. */
function TabsCount({ className, ...props }: React.ComponentProps<'span'>) {
  return <span data-slot="tabs-count" className={cn('font-mono text-[11px] font-normal text-subtle-foreground', className)} {...props} />
}

function TabsContent({ className, ...props }: TabsPrimitive.Panel.Props) {
  return <TabsPrimitive.Panel data-slot="tabs-content" className={cn('flex-1 outline-none', className)} {...props} />
}

export { Tabs, TabsContent, TabsCount, TabsList, TabsTrigger, tabsListClassName, tabTriggerClassName }
