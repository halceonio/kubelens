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
RUN apk add --no-cache supervisor
COPY docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY docker/supervisord.conf /etc/supervisord.conf
COPY --from=backend-build /out/kubelens-backend /usr/local/bin/kubelens-backend
COPY --from=frontend-build /src/frontend/dist /usr/share/nginx/html
RUN mkdir -p /var/lib/kubelens
EXPOSE 80
ENTRYPOINT ["/usr/bin/supervisord","-c","/etc/supervisord.conf"]
