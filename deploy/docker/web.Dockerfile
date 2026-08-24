FROM node:24-alpine AS builder
WORKDIR /app
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web ./
RUN npm run build

FROM node:24-alpine
ENV NODE_ENV=production
ENV HOSTNAME=0.0.0.0
RUN addgroup -S ledgersync && adduser -S ledgersync -G ledgersync \
    && rm -rf /usr/local/lib/node_modules/npm /usr/local/lib/node_modules/corepack /opt/yarn-v* \
    && rm -f /usr/local/bin/npm /usr/local/bin/npx /usr/local/bin/corepack /usr/local/bin/yarn /usr/local/bin/yarnpkg
WORKDIR /app
COPY --from=builder --chown=ledgersync:ledgersync /app/.next/standalone ./
COPY --from=builder --chown=ledgersync:ledgersync /app/.next/static ./.next/static
USER ledgersync
EXPOSE 3000
CMD ["node", "server.js"]
