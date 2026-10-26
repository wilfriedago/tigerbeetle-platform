import { Badge } from '~/components/ui/badge'

import { cn } from '~/lib/utils'

// The person parts of the members pages (.design/auth.pen, Members): initials in a circle, the
// organization role as a badge, ledger roles as "ledger · role" tags.

/** "Ada Okafor" becomes AO; an email gives its first letter. */
export function initials(name: string): string {
  const words = name.split(/[\s@._-]+/).filter(Boolean)
  return (words.length > 1 ? (words[0]?.[0] ?? '') + (words[1]?.[0] ?? '') : (words[0]?.slice(0, 2) ?? '?')).toUpperCase()
}

export function Avatar({ name, className }: { name: string; className?: string }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        'inline-flex size-7 shrink-0 items-center justify-center rounded-full border border-border-subtle bg-quiet-bg text-caption font-medium text-quiet-fg',
        className
      )}
    >
      {initials(name)}
    </span>
  )
}

/** The name and email of a person, with "you" next to the signed-in one. */
export function PersonCell({ name, email, you = false, extra }: { name: string; email: string; you?: boolean; extra?: React.ReactNode }) {
  return (
    <span className="flex min-w-0 items-center gap-2.5">
      <Avatar name={name || email} />
      <span className="flex min-w-0 flex-col">
        <span className="flex items-center gap-1.5 truncate text-body text-foreground">
          {name || email}
          {you ? <span className="text-caption text-subtle-foreground">you</span> : null}
          {extra}
        </span>
        <span className="truncate text-caption text-muted-foreground">{email}</span>
      </span>
    </span>
  )
}

const roleVariant: Record<string, 'accent' | 'neutral' | 'muted'> = { admin: 'neutral', member: 'muted', owner: 'accent' }

export function RoleBadge({ role }: { role: string | undefined }) {
  if (!role) {
    return <span className="text-caption text-subtle-foreground">no role</span>
  }
  return (
    <Badge variant={roleVariant[role] ?? 'neutral'} dot={false}>
      {role}
    </Badge>
  )
}

/** A member's ledger roles, or what their organization role gives them. */
export function LedgerRoles({ member }: { member: { role?: string | undefined; ledgers: readonly { ledger: string; role: string }[] } }) {
  if (member.role === 'admin' || member.role === 'owner') {
    return (
      <span className="flex flex-col">
        <span className="text-body text-foreground">all ledgers</span>
        <span className="text-caption text-muted-foreground">through organization role</span>
      </span>
    )
  }
  if (member.ledgers.length === 0) {
    return <span className="text-caption text-subtle-foreground">No ledger access yet</span>
  }
  return (
    <span className="flex flex-wrap gap-1.5">
      {member.ledgers.map((l) => (
        <LedgerRoleTag key={l.ledger} role={l} />
      ))}
    </span>
  )
}

export function LedgerRoleTag({ role }: { role: { ledger: string; role: string } }) {
  return (
    <span className="inline-flex h-6 items-center gap-1.5 rounded-sm border border-border-subtle bg-surface-sunken px-1.5 text-caption">
      <span className="font-mono text-foreground">{role.ledger}</span>
      <span className="text-subtle-foreground">{role.role}</span>
    </span>
  )
}
