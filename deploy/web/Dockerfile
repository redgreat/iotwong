# iotwong web image — multi-stage: SvelteKit static SPA built with Node 24,
# served by Nginx with SPA fallback + /api reverse proxy (incl. SSE tuning).
# Build from repository root: docker build -f deploy/web/Dockerfile .
FROM node:24-alpine AS builder
ARG NPM_REGISTRY=https://registry.npmjs.org/
# 国内底图构建期配置（静态 SPA：构建时注入 import.meta.env）
ARG VITE_MAP_PROVIDER=offline
ARG VITE_MAP_KEY=
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm config set registry ${NPM_REGISTRY} && npm ci
COPY web/ .
ENV VITE_MAP_PROVIDER=${VITE_MAP_PROVIDER} VITE_MAP_KEY=${VITE_MAP_KEY}
RUN npm run build

FROM nginx:1.29-alpine
COPY deploy/web/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /src/web/build/ /usr/share/nginx/html/
# nginx worker runs as an unprivileged user: make every artifact world-readable
# (source-checkout ACLs can otherwise leak 0600/0660 modes into the image)
RUN chmod -R a+rX /usr/share/nginx/html
EXPOSE 80
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1/ || exit 1
