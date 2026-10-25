import { SunMoonIcon } from 'lucide-react'

import { Button } from '~/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '~/components/ui/tooltip'

import { setPreferences, usePreferences } from '~/lib/preferences'

const NEXT = { dark: 'system', light: 'dark', system: 'light' } as const

/** Cycles light, dark and the system setting. */
export function ThemeToggle() {
  const { theme } = usePreferences()
  const next = NEXT[theme]
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button variant="ghost" size="icon-sm" aria-label={`Theme: ${theme}. Switch to ${next}`} onClick={() => setPreferences({ theme: next })} />
        }
      >
        <SunMoonIcon />
      </TooltipTrigger>
      <TooltipContent>Theme: {theme}</TooltipContent>
    </Tooltip>
  )
}
