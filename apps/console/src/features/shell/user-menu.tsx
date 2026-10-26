import { LogOutIcon, UserIcon } from 'lucide-react'

import { Button } from '~/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '~/components/ui/dropdown-menu'

import { signOut } from '~/lib/auth/session'
import { useSession } from '~/lib/auth/use-session'

// The signed-in person and "Sign out" (ADR-0029). Signing out ends the session in the API, then
// at the identity provider, and comes back to the console's sign-in.
export function UserMenu() {
  const session = useSession()
  return (
    <DropdownMenu>
      <DropdownMenuTrigger render={<Button aria-label="Account" size="icon" variant="ghost" />}>
        <UserIcon />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-56">
        <DropdownMenuGroup>
          <DropdownMenuLabel>
            <span className="block text-foreground">{session?.name || session?.subject}</span>
            {session?.role ? <span className="block text-small text-muted-foreground">{session.role} of this organization</span> : null}
          </DropdownMenuLabel>
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={() => void signOut()}>
          <LogOutIcon />
          Sign out
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
