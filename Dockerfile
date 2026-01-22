FROM golang:alpine AS build
RUN apk add --no-cache protobuf protobuf-dev
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN protoc --proto_path=. \
    --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    proto/service.proto
RUN CGO_ENABLED=0 go build

FROM gcr.io/distroless/static-debian12
COPY --from=build /app/kusaibot .
CMD ["/kusaibot"]
