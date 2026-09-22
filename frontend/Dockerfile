FROM node:22-alpine AS build
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:1.27-alpine
# Rendered at container start: ${API_KEY} is substituted from the environment.
COPY nginx.conf /etc/nginx/templates/default.conf.template
COPY --from=build /src/dist /usr/share/nginx/html
EXPOSE 80
# Poll every 2s while starting (so `depends_on: service_healthy` does not wait),
# then only every 5 minutes: a health probe every few seconds is log noise.
HEALTHCHECK --interval=5m --timeout=3s --start-period=30s --start-interval=2s --retries=3 \
  CMD wget -qO- --spider http://127.0.0.1/ || exit 1
