FROM node:20-alpine

RUN apk add --no-cache postgresql-client

WORKDIR /work
ENTRYPOINT ["node", "/work/verify-n8n-invoice-e2e.mjs"]
