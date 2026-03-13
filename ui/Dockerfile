FROM node:22-alpine AS builder
WORKDIR /app
COPY ui/package.json ui/package-lock.json ./
RUN npm ci
COPY ui/ .
ARG VERSION=dev
ENV PULSE_VERSION=${VERSION}
RUN npm run build

FROM nginx:alpine
RUN apk add --no-cache curl
COPY ui/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html
EXPOSE 80
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -sf http://localhost:80/ || exit 1
