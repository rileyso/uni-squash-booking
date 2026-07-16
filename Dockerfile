FROM golang:1.26.4-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go test ./... && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/squash ./cmd/web && mkdir /out/data

FROM scratch
COPY --from=build /out/squash /squash
COPY --from=build --chown=65532:65532 /out/data /data
USER 65532:65532
EXPOSE 18080
ENTRYPOINT ["/squash"]
