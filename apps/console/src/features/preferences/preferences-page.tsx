import { Link } from '@tanstack/react-router'

import { Page, PageHeader } from '~/components/layout/page'
import { Address } from '~/components/ledger/address'
import { Amount } from '~/components/ledger/amount'
import { Timestamp } from '~/components/ledger/timestamp'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table'
import { ToggleGroup, ToggleGroupItem } from '~/components/ui/toggle-group'

import { type Preferences, setPreferences, usePreferences } from '~/lib/preferences'

// docs/console.md, Preferences: stored in this browser only. Each change applies at once; the
// sample row below shows the effect on a real-looking table.

interface ChoiceProps<K extends keyof Preferences> {
  name: K
  label: string
  description: string
  options: readonly { value: Preferences[K]; label: string }[]
}

function Choice<K extends keyof Preferences>({ name, label, description, options }: ChoiceProps<K>) {
  const preferences = usePreferences()
  return (
    <div className="flex flex-wrap items-center justify-between gap-4 border-b border-border-subtle py-4 last:border-b-0">
      <div className="flex flex-col gap-0.5">
        <span className="text-body font-medium">{label}</span>
        <span className="text-small text-muted-foreground">{description}</span>
      </div>
      <ToggleGroup
        aria-label={label}
        value={[preferences[name]]}
        onValueChange={(next) => {
          const [value] = next
          const option = options.find((candidate) => candidate.value === value)
          if (option) {
            setPreferences({ [name]: option.value })
          }
        }}
      >
        {options.map((option) => (
          <ToggleGroupItem key={option.value} value={option.value}>
            {option.label}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
    </div>
  )
}

export function PreferencesPage() {
  return (
    <Page className="max-w-[880px]">
      <PageHeader
        title="Preferences"
        description={
          <>
            How the console looks in this browser. Nothing here is sent to the API. Every component is on the{' '}
            <Link to="/design-system" className="text-primary-text hover:underline">
              design system
            </Link>{' '}
            page.
          </>
        }
      />
      <Card>
        <CardContent className="py-0">
          <Choice
            name="theme"
            label="Theme"
            description="System follows the operating system setting."
            options={[
              { label: 'System', value: 'system' },
              { label: 'Light', value: 'light' },
              { label: 'Dark', value: 'dark' }
            ]}
          />
          <Choice
            name="density"
            label="Density"
            description="Compact rows fit more of a list on screen."
            options={[
              { label: 'Comfortable', value: 'comfortable' },
              { label: 'Compact', value: 'compact' }
            ]}
          />
          <Choice
            name="amounts"
            label="Amounts"
            description="Major units use the asset scale; raw shows the integer the API stores."
            options={[
              { label: '1,250.00', value: 'major' },
              { label: '125000', value: 'raw' }
            ]}
          />
          <Choice
            name="timeZone"
            label="Times"
            description="Committed times come from TigerBeetle; this only changes how they are shown."
            options={[
              { label: 'Local', value: 'local' },
              { label: 'UTC', value: 'utc' }
            ]}
          />
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>Preview</CardTitle>
          <CardDescription>A sample row with the settings above.</CardDescription>
        </CardHeader>
        <Table framed={false} aria-label="Preview">
          <TableHeader>
            <TableRow>
              <TableHead>Committed</TableHead>
              <TableHead className="w-full">Account</TableHead>
              <TableHead align="right">Amount</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow>
              <TableCell>
                <Timestamp value="2026-10-24T14:32:08Z" />
              </TableCell>
              <TableCell>
                <Address value="users:1234:wallet" />
              </TableCell>
              <TableCell align="right">
                <Amount value="125000" asset="EUR/2" />
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </Card>
    </Page>
  )
}
