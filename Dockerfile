FROM golang:1.12 as build

# golang deps
WORKDIR /tmp/app/
COPY ./src/glide.yaml /tmp/app/
COPY ./src/glide.lock /tmp/app/
RUN curl https://glide.sh/get | sh \
    && glide install

WORKDIR /go/src/azure-metrics-exporter/src
COPY ./src /go/src/azure-metrics-exporter/src
RUN mkdir /app/ \
    && cp -a /tmp/app/vendor ./vendor/ \
    && CGO_ENABLED=0 GOOS=linux go build -o /app/azure-metrics-exporter

#############################################
# FINAL IMAGE
#############################################
FROM gcr.io/distroless/static

COPY --from=build /app/ /app/
USER 65534
EXPOSE 8080
ENTRYPOINT ["/app/azure-metrics-exporter"]
