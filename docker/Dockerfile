# syntax=docker/dockerfile:1

FROM golang:1.25.4-alpine AS backend-build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download
COPY backend ./backend
RUN cd backend && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/kubelens-backend ./cmd/server

FROM node:20-alpine AS frontend-build
WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN if [ -f package-lock.json ]; then npm ci; else npm install; fi
COPY frontend ./
RUN npm run build

FROM nginx:1.25-alpine
RUN apk add --no-cache supervisor sqlite \
  && apk add --no-cache \
    --repository=http://dl-cdn.alpinelinux.org/alpine/edge/main \
    --repository=http://dl-cdn.alpinelinux.org/alpine/edge/community \
    valkey
COPY docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY docker/supervisord.conf /etc/supervisord.conf
COPY docker/valkey.conf /etc/valkey/valkey.conf
COPY docker/entrypoint.sh /usr/local/bin/kubelens-entrypoint.sh
COPY --from=backend-build /out/kubelens-backend /usr/local/bin/kubelens-backend
COPY --from=frontend-build /src/frontend/dist /usr/share/nginx/html
RUN mkdir -p /var/lib/kubelens /etc/supervisord.d /etc/valkey \
  && chmod +x /usr/local/bin/kubelens-entrypoint.sh
EXPOSE 80
ENTRYPOINT ["/usr/local/bin/kubelens-entrypoint.sh"]
