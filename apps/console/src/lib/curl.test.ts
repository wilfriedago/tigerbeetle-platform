import { describe, expect, it } from 'vite-plus/test'

import { apiPath, curlCommand } from './curl'

describe('curlCommand', () => {
  it('writes method, headers and a single-quoted body', () => {
    const command = curlCommand({
      body: { reference: "it's", postings: [] },
      idempotencyKey: 'idem_1',
      method: 'POST',
      path: '/v1/ledgers/eu/transactions'
    })
    expect(command).toBe(
      [
        "curl -X POST 'http://localhost:8080/v1/ledgers/eu/transactions'",
        "  -H 'Content-Type: application/json'",
        "  -H 'Idempotency-Key: idem_1'",
        String.raw`  -d '{"reference":"it'\''s","postings":[]}'`
      ].join(' \\\n')
    )
  })

  it('omits the body headers on a GET', () => {
    expect(curlCommand({ method: 'GET', path: '/v1/ledgers' })).toBe("curl -X GET 'http://localhost:8080/v1/ledgers'")
  })
})

describe('apiPath', () => {
  it('escapes each segment', () => {
    expect(apiPath('ledgers', 'eu', 'rules', 'users:')).toBe('/v1/ledgers/eu/rules/users%3A')
  })
})
