import { useEffect, useState } from 'react'

import { CheckIcon, CopyIcon } from 'lucide-react'

import { Button } from '~/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '~/components/ui/tooltip'

import { cn } from '~/lib/utils'

const COPIED_MS = 2000

/** Copies a value and shows a check for two seconds (20 Money and identity, "after copy, 2 s"). */
export function useCopy(): [copied: boolean, copy: (value: string) => void] {
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (!copied) {
      return
    }
    const timer = globalThis.setTimeout(() => setCopied(false), COPIED_MS)
    return () => globalThis.clearTimeout(timer)
  }, [copied])

  const copy = (value: string) => {
    void navigator.clipboard?.writeText(value).then(() => setCopied(true))
  }
  return [copied, copy]
}

interface CopyButtonProps {
  value: string
  /** What is copied, for the tooltip and the accessible name: "Copy ID", "Copy request ID". */
  label?: string
  size?: 'icon-xs' | 'icon-sm' | 'icon'
  variant?: 'ghost' | 'secondary'
  className?: string
}

export function CopyButton({ value, label = 'Copy', size = 'icon-xs', variant = 'ghost', className }: CopyButtonProps) {
  const [copied, copy] = useCopy()
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            type="button"
            variant={variant}
            size={size}
            aria-label={label}
            className={cn(className)}
            onClick={(event) => {
              event.stopPropagation()
              copy(value)
            }}
          />
        }
      >
        {copied ? <CheckIcon className="text-success-fg" /> : <CopyIcon className="text-subtle-foreground" />}
      </TooltipTrigger>
      <TooltipContent>{copied ? 'Copied' : label}</TooltipContent>
    </Tooltip>
  )
}
