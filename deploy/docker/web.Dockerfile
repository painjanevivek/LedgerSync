FROM node:24-alpine@sha256:d32cdf619f63fe0471182d08996dd516c6275bb5fd31ae06e55a570bd9e1ad43 AS builder
WORKDIR /app
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY contracts /contracts
COPY web ./
RUN npm run build

FROM node:24-alpine@sha256:d32cdf619f63fe0471182d08996dd516c6275bb5fd31ae06e55a570bd9e1ad43
ENV NODE_ENV=production
ENV HOSTNAME=0.0.0.0
RUN apk upgrade --no-cache \
    && addgroup -S ledgersync \
    && adduser -S ledgersync -G ledgersync \
    && rm -rf /usr/local/lib/node_modules/npm /usr/local/lib/node_modules/corepack /opt/yarn-v* \
    && rm -f /usr/local/bin/npm /usr/local/bin/npx /usr/local/bin/corepack /usr/local/bin/yarn /usr/local/bin/yarnpkg
WORKDIR /app
COPY --from=builder --chown=ledgersync:ledgersync /app/.next/standalone ./
COPY --from=builder --chown=ledgersync:ledgersync /app/.next/static ./.next/static
USER ledgersync
EXPOSE 3000
CMD ["node", "server.js"]
