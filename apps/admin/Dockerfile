# syntax=docker/dockerfile:1

FROM node:20-alpine AS builder

WORKDIR /app

RUN npm install --global pnpm@10.34.3

COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY . .

RUN pnpm run build

FROM nginx:1.27-alpine

ARG RUOMU_REVISION=development
LABEL org.opencontainers.image.source="https://github.com/nanashiwang/ruomu-card-platform" \
      org.opencontainers.image.revision="${RUOMU_REVISION}" \
      org.ruomu.component="admin"
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
