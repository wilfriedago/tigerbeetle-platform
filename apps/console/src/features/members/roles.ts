// The fixed roles of the platform (ADR-0028), as apps/api/internal/kernel/ledger/permission.go
// defines them. The console shows them; the API decides.

export const ORGANIZATION_ROLES = ['member', 'admin', 'owner'] as const
export const LEDGER_ROLES = ['viewer', 'operator', 'admin'] as const

export type OrganizationRole = (typeof ORGANIZATION_ROLES)[number]
export type LedgerRole = (typeof LEDGER_ROLES)[number]

export const organizationRoleText: Record<OrganizationRole, { summary: string; detail: string }> = {
  admin: { detail: 'Every ledger, plus members, API keys and machine clients.', summary: 'Every ledger, members and credentials' },
  member: { detail: 'Sees the organization. Ledger access only through ledger roles.', summary: 'Ledger access only through ledger roles' },
  owner: { detail: 'Admin, and the only role that can grant or remove owners.', summary: 'Admin, and grants owner' }
}

export const ledgerRoleText: Record<LedgerRole, { summary: string; detail: string; permissions: string }> = {
  admin: {
    detail: 'Operator, plus account rules, event types, ledger metadata and access.',
    permissions: 'read, move money, metadata, configure, access',
    summary: 'Rules, event types, access'
  },
  operator: {
    detail: 'Viewer, plus moves money (transactions, reverts, holds) and edits metadata.',
    permissions: 'read, move money, metadata',
    summary: 'Moves money, edits metadata'
  },
  viewer: { detail: 'Reads everything: accounts, balances, transactions, holds, requests.', permissions: 'read', summary: 'Reads everything' }
}

export interface PermissionRow {
  name: string
  description: string
  /** Which roles hold it: ledger roles, then organization roles. */
  ledger: Record<LedgerRole, boolean>
  organization: Record<OrganizationRole, boolean>
  note?: string
}

const none = { admin: false, operator: false, viewer: false }
const adminsOnly = { admin: true, member: false, owner: true }

/** The role table of plan section 6.4, the permissions use cases check. */
export const PERMISSIONS: readonly PermissionRow[] = [
  {
    description: 'Accounts, balances, history, transactions, holds, requests, overview, search.',
    ledger: { admin: true, operator: true, viewer: true },
    name: 'ledger.read',
    note: 'by ledger role',
    organization: adminsOnly
  },
  {
    description: 'Create and revert transactions; create, capture and void holds.',
    ledger: { admin: true, operator: true, viewer: false },
    name: 'money.write',
    note: 'by ledger role',
    organization: adminsOnly
  },
  {
    description: 'Replace the metadata of an account or a transaction.',
    ledger: { admin: true, operator: true, viewer: false },
    name: 'metadata.write',
    note: 'by ledger role',
    organization: adminsOnly
  },
  {
    description: 'Ledger metadata, account rules, event types.',
    ledger: { admin: true, operator: false, viewer: false },
    name: 'ledger.configure',
    note: 'by ledger role',
    organization: adminsOnly
  },
  {
    description: 'Grant and revoke roles on the ledger.',
    ledger: { admin: true, operator: false, viewer: false },
    name: 'ledger.members',
    note: 'by ledger role',
    organization: adminsOnly
  },
  { description: 'Create ledgers.', ledger: none, name: 'org.ledgers', organization: adminsOnly },
  { description: 'Invite and remove members, grant organization roles.', ledger: none, name: 'org.members', organization: adminsOnly },
  { description: 'Create and revoke API keys and machine clients.', ledger: none, name: 'org.credentials', organization: adminsOnly },
  { description: 'Reconciler status and service details.', ledger: none, name: 'org.operate', organization: adminsOnly }
]

export function isOrganizationRole(value: string | undefined): value is OrganizationRole {
  return (ORGANIZATION_ROLES as readonly string[]).includes(value ?? '')
}

export function isLedgerRole(value: string | undefined): value is LedgerRole {
  return (LEDGER_ROLES as readonly string[]).includes(value ?? '')
}

/** What a person can and cannot do, in plain sentences, for the member page. */
export function capabilities(role: string | undefined, ledgers: readonly { ledger: string; role: string }[]): { yes: string[]; no: string[] } {
  if (role === 'admin' || role === 'owner') {
    return {
      no: role === 'owner' ? [] : ['Grant or remove the owner role'],
      yes: ['Read and write every ledger', 'Change rules, event types and access on every ledger', 'Invite members and create API keys']
    }
  }
  const names = (roles: readonly string[]) => ledgers.filter((l) => roles.includes(l.role)).map((l) => l.ledger)
  const join = (list: string[]) => (list.length <= 1 ? (list[0] ?? '') : `${list.slice(0, -1).join(', ')} and ${list.at(-1)}`)
  const read = names(['viewer', 'operator', 'admin'])
  const write = names(['operator', 'admin'])
  const configure = names(['admin'])
  const yes: string[] = []
  const no: string[] = []
  if (read.length > 0) {
    yes.push(`Read ${join(read)}`)
  }
  if (write.length > 0) {
    yes.push(`Move money and edit metadata on ${join(write)}`)
  }
  if (configure.length > 0) {
    yes.push(`Change rules, event types and access on ${join(configure)}`)
  }
  if (configure.length === 0) {
    no.push('Change rules or event types on any ledger')
  }
  no.push(read.length === 0 ? 'See any ledger' : 'See other ledgers', 'Invite members or create API keys')
  return { no, yes }
}
