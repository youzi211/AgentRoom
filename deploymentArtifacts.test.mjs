import { strict as assert } from 'node:assert'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

const compose = readFileSync(new URL('./docker-compose.yml', import.meta.url), 'utf8')
const backendDockerfile = readFileSync(new URL('./backend/Dockerfile', import.meta.url), 'utf8')
const frontendDockerfile = readFileSync(new URL('./frontend/Dockerfile', import.meta.url), 'utf8')
const nginxConfig = readFileSync(new URL('./frontend/nginx.conf', import.meta.url), 'utf8')

test('docker compose wires v0.2 runtime services and security env', () => {
  assert.match(compose, /^\s*mysql:\s*$/m)
  assert.match(compose, /^\s*backend:\s*$/m)
  assert.match(compose, /^\s*frontend:\s*$/m)
  assert.match(compose, /ADMIN_API_KEY:/)
  assert.match(compose, /ALLOWED_ORIGINS:/)
  assert.match(compose, /VITE_ADMIN_API_KEY:/)
  assert.match(compose, /MYSQL_DSN:/)
})

test('backend and frontend images are buildable from local Dockerfiles', () => {
  assert.match(backendDockerfile, /FROM golang:1\.22-alpine AS build/)
  assert.match(backendDockerfile, /go build -o \/out\/agentroom/)
  assert.match(frontendDockerfile, /FROM node:20-alpine AS build/)
  assert.match(frontendDockerfile, /npm ci/)
  assert.match(frontendDockerfile, /npm run build/)
})

test('nginx proxies API traffic to the backend service', () => {
  assert.match(nginxConfig, /location \/api\/ \{/)
  assert.match(nginxConfig, /proxy_pass http:\/\/backend:8080\/api\//)
  assert.match(nginxConfig, /proxy_set_header Upgrade \$http_upgrade;/)
})
