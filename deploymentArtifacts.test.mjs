import { strict as assert } from 'node:assert'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

const compose = readFileSync(new URL('./docker-compose.yml', import.meta.url), 'utf8')
const backendDockerfile = readFileSync(new URL('./backend/Dockerfile', import.meta.url), 'utf8')
const frontendDockerfile = readFileSync(new URL('./frontend/Dockerfile', import.meta.url), 'utf8')
const nginxConfig = readFileSync(new URL('./frontend/nginx.conf', import.meta.url), 'utf8')
const envExample = readFileSync(new URL('./.env.example', import.meta.url), 'utf8')
const readme = readFileSync(new URL('./README.md', import.meta.url), 'utf8')
const dockerUpPowerShell = readFileSync(new URL('./scripts/docker-up.ps1', import.meta.url), 'utf8')
const dockerUpShell = readFileSync(new URL('./scripts/docker-up.sh', import.meta.url), 'utf8')

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
  assert.match(backendDockerfile, /FROM golang:1\.23-alpine AS build/)
  assert.match(backendDockerfile, /GOPROXY=https:\/\/goproxy\.cn,direct/)
  assert.match(backendDockerfile, /GOSUMDB=sum\.golang\.google\.cn/)
  assert.match(backendDockerfile, /go build -o \/out\/agentroom/)
  assert.match(frontendDockerfile, /FROM node:20-alpine AS build/)
  assert.match(frontendDockerfile, /NPM_CONFIG_REGISTRY=https:\/\/registry\.npmmirror\.com/)
  assert.match(frontendDockerfile, /NPM_CONFIG_REPLACE_REGISTRY_HOST=always/)
  assert.match(frontendDockerfile, /npm ci/)
  assert.match(frontendDockerfile, /npm run build/)
})

test('nginx proxies API traffic to the backend service', () => {
  assert.match(nginxConfig, /location \/api\/ \{/)
  assert.match(nginxConfig, /proxy_pass http:\/\/backend:8080\/api\//)
  assert.match(nginxConfig, /proxy_set_header Upgrade \$http_upgrade;/)
})

test('one-click PowerShell bootstrap handles env setup and compose startup', () => {
  assert.match(dockerUpPowerShell, /Copy-Item /)
  assert.match(dockerUpPowerShell, /\.env\.example/)
  assert.match(dockerUpPowerShell, /\.env'/)
  assert.match(dockerUpPowerShell, /Invoke-CheckedCommand/)
  assert.match(dockerUpPowerShell, /'info'/)
  assert.match(dockerUpPowerShell, /compose', 'up', '-d'/)
  assert.match(dockerUpPowerShell, /--build/)
  assert.match(dockerUpPowerShell, /VITE_ADMIN_API_KEY/)
  assert.match(dockerUpPowerShell, /ADMIN_API_KEY/)
  assert.match(dockerUpPowerShell, /PUBLIC_ORIGIN/)
  assert.match(dockerUpPowerShell, /'port', 'backend', '8080'/)
  assert.match(dockerUpPowerShell, /'port', 'frontend', '80'/)
  assert.match(dockerUpPowerShell, /api\/health/)
})

test('one-click shell bootstrap mirrors the PowerShell deployment flow', () => {
  assert.match(dockerUpShell, /cp /)
  assert.match(dockerUpShell, /\.env\.example/)
  assert.match(dockerUpShell, /\.env/)
  assert.match(dockerUpShell, /docker info/)
  assert.match(dockerUpShell, /docker compose up -d --build/)
  assert.match(dockerUpShell, /VITE_ADMIN_API_KEY/)
  assert.match(dockerUpShell, /ADMIN_API_KEY/)
  assert.match(dockerUpShell, /PUBLIC_ORIGIN/)
  assert.match(dockerUpShell, /api\/health/)
  assert.match(dockerUpShell, /ip -4 route get 1\.1\.1\.1/)
  assert.match(dockerUpShell, /docker compose port backend 8080/)
  assert.match(dockerUpShell, /docker compose port frontend 80/)
  assert.match(dockerUpShell, /Frontend \(direct IP\):/)
  assert.match(dockerUpShell, /Backend health \(direct IP\):/)
  assert.match(dockerUpShell, /KEEP_WSL_ALIVE/)
  assert.match(dockerUpShell, /nohup/)
})

test('server deploy docs and env example document the public origin flow', () => {
  assert.match(envExample, /^PUBLIC_ORIGIN=/m)
  assert.match(readme, /git clone /)
  assert.match(readme, /PUBLIC_ORIGIN=/)
  assert.match(readme, /bash \.\/scripts\/docker-up\.sh/)
})
