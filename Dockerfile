FROM golang:1.13 as build

WORKDIR /go/src/github.com/webdevops/azure-metrics-exporter

# Get deps (cached)
COPY ./go.mod /go/src/github.com/webdevops/azure-metrics-exporter
COPY ./go.sum /go/src/github.com/webdevops/azure-metrics-exporter
RUN go mod download

# Compile
COPY ./ /go/src/github.com/webdevops/azure-metrics-exporter
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o /azure-metrics-exporter \
    && chmod +x /azure-metrics-exporter
RUN /azure-metrics-exporter --help

#############################################
# FINAL IMAGE
#############################################
FROM gcr.io/distroless/static
COPY --from=build /azure-metrics-exporter /
USER 1000
ENTRYPOINT ["/azure-metrics-exporter"]
