# 14. Console architecture: TanStack, feature slices and a shadcn design system

Status: Accepted, amended by [ADR-0023](0023-console-on-graphql.md)
Date: 2026-10-25

## Context and Problem Statement

ADR-0003 settles that the console is a React application that talks only to the API, through
`packages/api-client`. `docs/console.md` describes its pages, and the Pencil files in `.design/`
hold the "Ledger Instrument" design system (`ds.lib.pen`) and the screens (`home.pen`). Until
now the console was a single status page in plain CSS. Building every page needs choices that
ADR-0003 left open: routing, server state, forms, tables, the component library and how the
code is laid out so a page can be found and changed without reading the rest.

## Decision Drivers

- The URL is the state: filters, tabs, the cursor and the open drawer are shareable links.
- Money is exact: amounts stay integer strings and bigint, never JavaScript numbers.
- One ecosystem rather than a library per concern, as the owner asked.
- The components must look like the design system, not like a stock theme.
- A newcomer finds the code of a page from its URL, and a page can change without touching others.

## Considered Options

1. TanStack Router as a single-page app, with TanStack Query, Form, Table and Pacer; shadcn
   components on Base UI, restyled with the design tokens; feature slices (chosen).
2. TanStack Start, the same libraries plus server rendering and server functions.
3. React Router with Redux Toolkit Query and React Hook Form.

Option 2 adds a Node server between the browser and the API. The console has no data of its
own, no secrets and no SEO need; a server would duplicate the API's role and the static build
would stop being deployable behind any reverse proxy. Option 3 mixes three ecosystems and keeps
search parameters untyped.

## Decision Outcome

### Libraries

- **TanStack Router**, file-based (`src/routes`, generated `src/route-tree.gen.ts`, automatic code
  splitting). Search parameters are validated with zod schemas in the route files. Loaders call
  `ensureQueryData`, so a page renders with data and a 404 becomes the router's not-found view
  (`lib/loaders.ts`). Breadcrumbs come from `staticData.crumb`.
- **TanStack Query** owns server state. Each feature exports `queryOptions` factories; every key
  inside a ledger starts with `['ledger', name]`, so a write refreshes the ledger with one
  invalidation. Client errors are not retried; mutations never are, because a write is retried
  by a person with the same Idempotency-Key (ADR-0007).
- **TanStack Form** through `createFormHook` (`components/form/form.tsx`): fields render the
  design-system label, help and error, and validators mirror the API rules in
  `lib/validation.ts`. The API checks them again.
- **TanStack Table** v9 with core features only (`components/data-table`): sorting and filtering
  are the API's job, and the API cannot sort by balance (ADR-0002).
- **TanStack Pacer** debounces search inputs.

### Design system

shadcn's `base-nova` style on Base UI provides the component structure. The files live in
`src/components/ui` and are edited, not wrapped: the Ledger Instrument tokens (palette, roles,
light and dark modes, comfortable and compact density) are CSS variables in
`src/styles/globals.css`, and shadcn's variable names (`--primary`, `--muted`, ...) are mapped to
the design roles, so a component reads the design system even when it uses shadcn class names.
Ledger concepts (amounts, addresses, IDs, posting chains, balances, timelines, the problem panel)
are components in `src/components/ledger`, one per entry of the design library.

### Layout

```
src/routes/                 thin route files: search schema, loader, crumb, page component
src/features/<slice>/       api.ts (query options, mutations), pages, slice components
src/components/ui/          shadcn components restyled with the tokens
src/components/ledger/      domain display components shared by slices
src/components/{layout,form,data-table}/  shell, form hook, table
src/lib/                    money, time, ids, preflight, validation, the API client
```

A slice may import another slice's `api.ts` or a pure module from `lib`; components shared by
two slices move to `components`. Feature pages read their route through `getRouteApi`, so route
files do not import pages' internals and pages do not import route files.

### Writes

Every write form follows ADR-0007: an Idempotency-Key is made when the form opens, sent again on
a retry, and renewed after a final answer (`lib/idempotency.ts`). New transaction runs the chain
against the balances read beforehand (`lib/preflight.ts`) to show what each source can spend and
which posting would be declined. The preview is advisory; TigerBeetle decides.

### Consequences

- Good: every view is a typed link; a page is one route file and one feature folder.
- Good: the components and the Pencil library share names and tokens, so design changes map to
  one file.
- Good: the build is static files; any reverse proxy that maps `/api` serves it.
- Bad: shadcn components are copied code. Updates from upstream are merged by hand.
- Bad: the preflight duplicates TigerBeetle's limit checks in TypeScript. They are tested, and
  the page says the preview is advisory.
- Bad: TanStack Table v9 and Pacer are young; their APIs may move between minor versions.

## More Information

- ADR-0002, ADR-0003, ADR-0004, ADR-0005, ADR-0007, ADR-0011, ADR-0013.
- `docs/console.md` for the pages; `apps/console/README.md` for the commands.
