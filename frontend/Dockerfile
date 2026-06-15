FROM node:20-alpine AS build

WORKDIR /src/frontend

ARG VITE_ADMIN_API_KEY=""
ENV VITE_ADMIN_API_KEY=${VITE_ADMIN_API_KEY}

COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

FROM nginx:1.27-alpine

COPY frontend/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/frontend/dist /usr/share/nginx/html

EXPOSE 80
