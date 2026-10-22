# The public gateway with the built web app (rule 65): Node runs only in the build stage, never in production.
# Build context is the repository root. The build stage runs on the build machine's platform, since the bundle is
# the same for every architecture.
FROM --platform=$BUILDPLATFORM node:26-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web ./
RUN npm run build

FROM caddy:2.10-alpine
COPY --from=web /src/web/dist /srv/web
