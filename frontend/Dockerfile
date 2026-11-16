# syntax=docker/dockerfile:1
# Frontend Next.js (output: 'standalone') - build multi-stage.
# NEXT_PUBLIC_* sao embutidas no bundle em tempo de BUILD: passe via build-args.

# SEC-47: base fixada por digest (indice multi-arch). Com tag@digest o Docker
# usa so o digest: NODE_VERSION e NODE_DIGEST mudam JUNTOS, iguais ao NODE_IMAGE
# do Jenkinsfile (teste confere). "make deps-audit-imagens" avisa digest novo.
ARG NODE_VERSION=26.13.1-alpine
ARG NODE_DIGEST=sha256:6bd9ab43c0ec0190ca7b2d193906cb8b7edf6e7f4afa43ddeccd6452710bffb5

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
# ECOM-02: Client ID PUBLICO do "Entrar com Google" (vazio = botao oculto).
ARG NEXT_PUBLIC_GOOGLE_CLIENT_ID
ENV NEXT_PUBLIC_API_URL=${NEXT_PUBLIC_API_URL} \
    NEXT_PUBLIC_TURNSTILE_SITE_KEY=${NEXT_PUBLIC_TURNSTILE_SITE_KEY} \
    NEXT_PUBLIC_GOOGLE_CLIENT_ID=${NEXT_PUBLIC_GOOGLE_CLIENT_ID} \
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
