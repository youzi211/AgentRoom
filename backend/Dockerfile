FROM golang:1.23-alpine AS build

WORKDIR /src/backend

ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.google.cn

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/agentroom ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates && adduser -D -H -u 10001 agentroom

WORKDIR /app
COPY --from=build /out/agentroom /app/agentroom

USER agentroom
EXPOSE 8080

ENTRYPOINT ["/app/agentroom"]
