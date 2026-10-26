# syntax=docker/dockerfile:1
# Vite dev server for the console, built from the repository root so the workspace packages
# (api-client, graphql, openapi) are available. Source directories are bind-mounted by compose.yaml.
FROM node:24.22.1-trixie-slim

# Same pnpm as the packageManager field in package.json.
RUN npm install --global pnpm@12.15.0

WORKDIR /workspace
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/console/package.json apps/console/
COPY packages/api-client/package.json packages/api-client/
COPY packages/design-tokens/package.json packages/design-tokens/
COPY packages/graphql/package.json packages/graphql/
COPY packages/openapi/package.json packages/openapi/
# --ignore-scripts skips the root prepare script, which installs the git hooks and needs git.
# Dependency build scripts are not approved in pnpm-workspace.yaml, so nothing else is skipped.
RUN pnpm install --frozen-lockfile --ignore-scripts --filter console...

COPY packages ./packages
COPY apps/console ./apps/console

EXPOSE 5173
# --host: inside the container the dev server must listen on all interfaces to be reachable.
CMD ["pnpm", "--filter", "console", "dev", "--host"]
