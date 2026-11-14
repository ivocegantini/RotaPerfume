# syntax=docker/dockerfile:1
# Frontend Next.js (output: 'standalone') - build multi-stage.
# NEXT_PUBLIC_* sao embutidas no bundle em tempo de BUILD: passe via build-args.

# SEC-47: base fixada por digest (indice multi-arch). Com tag@digest o Docker
# usa so o digest: NODE_VERSION e NODE_DIGEST mudam JUNTOS, iguais ao NODE_IMAGE
# do Jenkinsfile (teste confere). "make deps-audit-imagens" avisa digest novo.
ARG NODE_VERSION=22.23.4-alpine
ARG NODE_DIGEST=sha256:0e1754758c2df4cfa3e68d59e2e1b7374bb7fd9ea6328eaaacf15cc934af8202

# ---------- deps ----------
FROM node:${NODE_VERSION}@${NODE_DIGEST} AS deps
WORKDIR /app
RUN apk add --no-cache libc6-compat
COPY package.json package-lock.json ./
RUN npm ci

# ---------- build ----------
FROM node:${NODE_VERSION}@${NODE_DIGEST} AS builder
WORKDIR /app
ARG NEXT_PUBLIC_API_URL
ARG NEXT_PUBLIC_TURNSTILE_SITE_KEY
ENV NEXT_PUBLIC_API_URL=${NEXT_PUBLIC_API_URL} \
    NEXT_PUBLIC_TURNSTILE_SITE_KEY=${NEXT_PUBLIC_TURNSTILE_SITE_KEY} \
    NEXT_TELEMETRY_DISABLED=1
COPY --from=deps /app/node_modules ./node_modules
COPY . .
# Garante que o diretorio public exista (o projeto pode nao ter um).
RUN mkdir -p public && npm run build

# ---------- runner ----------
FROM node:${NODE_VERSION}@${NODE_DIGEST} AS runner
WORKDIR /app
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    HOSTNAME=0.0.0.0 \
    PORT=3000
COPY --from=builder --chown=node:node /app/public ./public
COPY --from=builder --chown=node:node /app/.next/standalone ./
COPY --from=builder --chown=node:node /app/.next/static ./.next/static
USER node
EXPOSE 3000
CMD ["node", "server.js"]
