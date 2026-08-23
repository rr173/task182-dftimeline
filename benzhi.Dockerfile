FROM --platform=$BUILDPLATFORM docker.m.daocloud.io/library/golang:1.26.3-bookworm AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
ENV CGO_ENABLED=0
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.google.cn
ENV GOTOOLCHAIN=local
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build ./... \
    && GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /app/dftimeline ./cmd/dftimeline

FROM docker.m.daocloud.io/library/alpine:3.20
COPY --from=build /app/dftimeline /app/dftimeline
WORKDIR /data
ENTRYPOINT ["/app/dftimeline"]
CMD ["--smoke-test"]
