import { describe, expect, it } from 'vite-plus/test'

import { parseEmails } from './invite-dialog'
import { capabilities } from './roles'

describe('capabilities', () => {
  it('gives admins and owners every ledger', () => {
    expect(capabilities('owner', []).no).toEqual([])
    expect(capabilities('admin', []).no).toEqual(['Grant or remove the owner role'])
  })

  it('reads a member from their ledger roles', () => {
    const can = capabilities('member', [
      { ledger: 'eu-payments', role: 'operator' },
      { ledger: 'wa-wallets', role: 'viewer' }
    ])
    expect(can.yes).toEqual(['Read eu-payments and wa-wallets', 'Move money and edit metadata on eu-payments'])
    expect(can.no).toContain('Change rules or event types on any ledger')
  })

  it('says a member without ledger roles sees none', () => {
    expect(capabilities('member', []).no).toContain('See any ledger')
  })
})

describe('parseEmails', () => {
  it('splits on commas, spaces and new lines, lowercases and drops duplicates', () => {
    expect(parseEmails('Li.Na@acme.example, maria@northwind.example\nli.na@acme.example ;  ')).toEqual([
      'li.na@acme.example',
      'maria@northwind.example'
    ])
  })
})
