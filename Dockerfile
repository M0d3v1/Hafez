FROM golang:1.22 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/hafez ./cmd/hafez

FROM scratch

COPY --from=build /out/hafez /hafez
EXPOSE 6379
ENTRYPOINT ["/hafez"]
CMD ["--port", "6379"]
