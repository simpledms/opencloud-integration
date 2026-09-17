FROM golang:1.26.7-alpine AS build
WORKDIR /src
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /integration .

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && addgroup -g 1000 integration && adduser -D -u 1000 -G integration integration
COPY --from=build /integration /usr/local/bin/integration
USER 1000:1000
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/integration"]
