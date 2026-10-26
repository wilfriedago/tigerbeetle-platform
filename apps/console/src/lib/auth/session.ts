import { type QueryClient, queryOptions } from '@tanstack/react-query'
import { redirect } from '@tanstack/react-router'

import { HttpStatusError, ORGANIZATION_HEADER, randomRequestId, REQUEST_ID_HEADER } from '@tigerbeetle-platform/api-client'

/**
 * The console signs in through the API's backend-for-frontend (ADR-0030): the browser holds an
 * HttpOnly session cookie and never a token. These are its routes, under the API's base URL.
 */
let base = '/api'

/** Points sign-in at the API; lib/api.ts calls it with the clients' base URL. */
export function configureSession(baseUrl: string): void {
  base = baseUrl.replace(/\/+$/, '')
}

export interface SessionOrganization {
  alias: string
  name: string
  /** A suspended organization refuses every call in its name (ADR-0027). */
  status: 'active' | 'suspended'
  /** Why staff suspended it. */
  suspensionReason?: string
  /** The person's organization role there: member, admin or owner. */
  role?: string
  /** The person's ledger roles there, by ledger name. */
  ledgers: Record<string, string>
}

/** The signed-in person, as GET /auth/session answers. */
export interface Session {
  subject: string
  name: string
  email?: string
  /** Keycloak's page where the person manages their password, two-factor and sessions. */
  accountUrl?: string
  /** Staff run the platform; they manage organizations and see no ledger without a grant. */
  staff: boolean
  organizations: SessionOrganization[]
  /** The organization role in the organization asked about: member, admin or owner. */
  role?: string
  /** Ledger roles in that organization: viewer, operator or admin, by ledger name. */
  ledgers: Record<string, string>
}

/** The answer when nobody is signed in, or the session has ended. */
export class SignedOutError extends Error {
  constructor() {
    super('signed out')
    this.name = 'SignedOutError'
  }
}

async function fetchSession(organization: string | undefined, signal?: AbortSignal): Promise<Session> {
  const headers: Record<string, string> = { Accept: 'application/json', [REQUEST_ID_HEADER]: randomRequestId() }
  if (organization) {
    headers[ORGANIZATION_HEADER] = organization
  }
  const response = await globalThis.fetch(`${base}/auth/session`, { credentials: 'same-origin', headers, signal: signal ?? null })
  if (response.status === 401) {
    throw new SignedOutError()
  }
  if (!response.ok) {
    throw new HttpStatusError(response.status, response.statusText, response.headers.get(REQUEST_ID_HEADER) ?? undefined)
  }
  return (await response.json()) as Session
}

/** The session, with the roles in one organization when it is given. */
export function sessionQuery(organization?: string) {
  return queryOptions({
    queryFn: async ({ signal }) => fetchSession(organization, signal),
    queryKey: ['session', organization ?? ''] as const,
    retry: false,
    staleTime: 60_000
  })
}

/**
 * The session, or a redirect to sign in that comes back to returnTo: what every route that needs a
 * signed-in person calls first in its beforeLoad (ADR-0030).
 */
export async function requireSession(queryClient: QueryClient, returnTo: string): Promise<Session> {
  try {
    return await queryClient.ensureQueryData(sessionQuery())
  } catch (error) {
    if (error instanceof SignedOutError) {
      throw redirect({ href: signInUrl(returnTo), reloadDocument: true })
    }
    throw error
  }
}

/** Where the browser signs in, then comes back to returnTo (a path of the console). */
export function signInUrl(returnTo: string): string {
  return `${base}/auth/login?return_to=${encodeURIComponent(returnTo)}`
}

/** Sends the browser to sign in, then back to the current page. */
export function signIn(): void {
  location.assign(signInUrl(`${location.pathname}${location.search}`))
}

/** Ends the session, then the provider's session, and lands back on the console. */
export async function signOut(): Promise<void> {
  const response = await globalThis.fetch(`${base}/auth/logout`, { credentials: 'same-origin', method: 'POST' })
  const body = response.ok ? ((await response.json()) as { logoutUrl?: string }) : {}
  location.assign(body.logoutUrl || '/')
}

const roleRank: Record<string, number> = { admin: 3, operator: 2, viewer: 1 }

/** What a ledger role allows, as the API decides it (ADR-0028); the API still checks. */
const needs: Record<Permission, number> = { 'ledger.configure': 3, 'ledger.read': 1, 'metadata.write': 2, 'money.write': 2 }

export type Permission = 'ledger.read' | 'money.write' | 'metadata.write' | 'ledger.configure'

/**
 * Whether the session may do something on a ledger of its organization: organization admins and
 * owners may do everything, members what their ledger role allows. It only hides what would be
 * refused; the API decides.
 */
export function can(session: Session | undefined, permission: Permission, ledger: string): boolean {
  if (!session) {
    return false
  }
  if (session.role === 'admin' || session.role === 'owner') {
    return true
  }
  return (roleRank[session.ledgers[ledger] ?? ''] ?? 0) >= needs[permission]
}

/** Whether the session may create ledgers in its organization. */
export function canCreateLedgers(session: Session | undefined): boolean {
  return session?.role === 'admin' || session?.role === 'owner'
}

/** Whether the session may invite and remove members and change organization roles (org.members). */
export function canManageMembers(session: Session | undefined): boolean {
  return session?.role === 'admin' || session?.role === 'owner'
}

/** Whether the session may grant roles on a ledger (ledger.members). */
export function canManageLedgerAccess(session: Session | undefined, ledger: string): boolean {
  return canManageMembers(session) || session?.ledgers[ledger] === 'admin'
}
