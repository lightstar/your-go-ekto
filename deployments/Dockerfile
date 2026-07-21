FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /ekto-server ./cmd/ekto-server

FROM alpine:3 AS final
WORKDIR /app
COPY --from=build /ekto-server .
EXPOSE 8080
ENTRYPOINT [ "./ekto-server" ]
