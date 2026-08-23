FROM node:20-alpine AS builder
WORKDIR /app
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web ./
RUN npm run build

FROM node:20-alpine
ENV NODE_ENV=production
ENV HOSTNAME=0.0.0.0
RUN addgroup -S ledgersync && adduser -S ledgersync -G ledgersync
WORKDIR /app
COPY --from=builder --chown=ledgersync:ledgersync /app/.next/standalone ./
COPY --from=builder --chown=ledgersync:ledgersync /app/.next/static ./.next/static
USER ledgersync
EXPOSE 3000
CMD ["node", "server.js"]
