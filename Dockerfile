FROM golang:1.17 as build

WORKDIR /go/src/github.com/webdevops/azure-metrics-exporter

# Get deps (cached)
COPY ./go.mod /go/src/github.com/webdevops/azure-metrics-exporter
COPY ./go.sum /go/src/github.com/webdevops/azure-metrics-exporter
COPY ./Makefile /go/src/github.com/webdevops/azure-metrics-exporter
RUN make dependencies

# Compile
COPY ./ /go/src/github.com/webdevops/azure-metrics-exporter
RUN make test
RUN make lint
RUN make build
RUN ./azure-metrics-exporter --help

#############################################
# FINAL IMAGE
#############################################
FROM gcr.io/distroless/static
ENV LOG_JSON=1
COPY --from=build /go/src/github.com/webdevops/azure-metrics-exporter/azure-metrics-exporter /
USER 1000:1000
ENTRYPOINT ["/azure-metrics-exporter"]
