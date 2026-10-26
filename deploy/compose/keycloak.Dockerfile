# syntax=docker/dockerfile:1
# Keycloak with the platform's login theme (apps/keycloak-theme), packaged by Keycloakify. Packaging
# the theme needs Node and Maven, so the build stage carries both and the runtime image is
# Keycloak's own with the theme's jar among its providers.
FROM maven:3.9.11-eclipse-temurin-21@sha256:6fdc855a6ed81d288ca7ca37ac6ff5e9308b612485c0801d70b25a858c83d237 AS theme

# Node from the image the console uses; the Maven image's /usr/local holds nothing of its own.
COPY --from=node:24.22.1-trixie-slim@sha256:43b9ba1142b8dcf33fa8f922ca1be85714adc444e0586c7cf7726daaeabc1bea /usr/local /usr/local
# Same pnpm as the packageManager field in package.json.
RUN npm install --global pnpm@12.15.0

WORKDIR /workspace
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/keycloak-theme/package.json apps/keycloak-theme/
COPY packages/design-tokens/package.json packages/design-tokens/
# --ignore-scripts skips the root prepare script, which installs the git hooks and needs git.
RUN pnpm install --frozen-lockfile --ignore-scripts --filter keycloak-theme...

COPY packages/design-tokens ./packages/design-tokens
COPY apps/keycloak-theme ./apps/keycloak-theme
RUN pnpm --filter keycloak-theme build-keycloak-theme

FROM quay.io/keycloak/keycloak:26.8.1@sha256:a78d6b79572168cb25886b80a8b5b8d4ed4390743e3eecb810538683e209dd6b
COPY --from=theme /workspace/apps/keycloak-theme/dist_keycloak/tbp-keycloak-theme.jar /opt/keycloak/providers/
