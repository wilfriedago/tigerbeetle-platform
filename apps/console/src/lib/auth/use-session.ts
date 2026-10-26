import { useQuery } from '@tanstack/react-query'

import { useOrganization } from '~/lib/organization'

import { can, type Permission, type Session, sessionQuery } from './session'

/** The session with the roles in the organization of the page; the console layout loaded it. */
export function useSession(): Session | undefined {
  const org = useOrganization()
  return useQuery(sessionQuery(org || undefined)).data
}

/**
 * Whether the signed-in person may do something on a ledger of the current organization, to hide
 * what the API would refuse (ADR-0028). The API still decides.
 */
export function useCan(permission: Permission, ledger: string | undefined): boolean {
  const session = useSession()
  return ledger ? can(session, permission, ledger) : false
}
