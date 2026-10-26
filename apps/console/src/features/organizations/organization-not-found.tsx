import { Link, useParams } from '@tanstack/react-router'

import { BuildingIcon } from 'lucide-react'

import { Page } from '~/components/layout/page'
import { Button } from '~/components/ui/button'
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '~/components/ui/empty'

/**
 * An organization URL the person does not belong to. The API answers it as if the organization did
 * not exist (ADR-0026), and so does the console.
 */
export function OrganizationNotFound() {
  const { org } = useParams({ strict: false })
  return (
    <Page>
      <Empty>
        <EmptyHeader>
          <EmptyMedia>
            <BuildingIcon />
          </EmptyMedia>
          <EmptyTitle>
            No organization <span className="font-mono">{org}</span>
          </EmptyTitle>
          <EmptyDescription>It does not exist, or you are not one of its members. Ask an owner to invite you.</EmptyDescription>
        </EmptyHeader>
        <EmptyContent>
          <Button render={<Link to="/" />}>Your organizations</Button>
        </EmptyContent>
      </Empty>
    </Page>
  )
}
