# syntax=docker/dockerfile:1
# Vite dev server for the console, built from the repository root so the workspace packages
# (api-client, openapi) are available. Source directories are bind-mounted by compose.yaml.
FROM node:24.22.1-trixie-slim

# Same pnpm as the packageManager field in package.json.
RUN npm install --global pnpm@12.15.0

WORKDIR /workspace
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/api/package.json apps/api/
COPY apps/console/package.json apps/console/
COPY packages/api-client/package.json packages/api-client/
COPY packages/openapi/package.json packages/openapi/
RUN pnpm install --frozen-lockfile --filter console...

COPY packages ./packages
COPY apps/console ./apps/console

EXPOSE 5173
CMD ["pnpm", "--filter", "console", "dev"]
