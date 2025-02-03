# Install go into wolfi for build
FROM cgr.dev/chainguard/wolfi-base AS build
RUN apk update && apk add ca-certificates-bundle build-base openssh git go-1.23~=1.23.5

WORKDIR /app
COPY src/ /app/
RUN make build

FROM cgr.dev/chainguard/glibc-dynamic AS metronomikon
COPY --from=0 /app/metronomikon /bin/
COPY example/config.yaml /etc/metronomikon/config.yaml
ENTRYPOINT ["metronomikon"]
