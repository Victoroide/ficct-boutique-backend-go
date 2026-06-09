FROM node:20-alpine

RUN apk add --no-cache postgresql-client sqlite \
    && mkdir -p /app \
    && cd /app \
    && npm init -y >/dev/null 2>&1 \
    && npm install --omit=dev --no-audit --no-fund flatted >/dev/null 2>&1

WORKDIR /work
ENV NODE_PATH=/app/node_modules
ENTRYPOINT ["node", "/work/verify-n8n-invoice-e2e.mjs"]
