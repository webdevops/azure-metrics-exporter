FROM golang:1.15 as build

WORKDIR /go/src/github.com/webdevops/azure-metrics-exporter

# Get deps (cached)
COPY ./go.mod /go/src/github.com/webdevops/azure-metrics-exporter
COPY ./go.sum /go/src/github.com/webdevops/azure-metrics-exporter
RUN go mod download

# Compile
COPY ./ /go/src/github.com/webdevops/azure-metrics-exporter
RUN make lint
RUN make build
RUN ./azure-metrics-exporter --help

#############################################
# FINAL IMAGE
#############################################
FROM gcr.io/distroless/static
COPY --from=build /go/src/github.com/webdevops/azure-metrics-exporter/azure-metrics-exporter /
USER 1000
ENTRYPOINT ["/azure-metrics-exporter"]
