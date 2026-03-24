FROM golang:alpine AS build-env
WORKDIR /src/app

COPY ./main.go .
COPY ./go.mod .
COPY ./go.sum .
COPY ./pkg ./pkg

RUN go build .

FROM alpine
RUN addgroup --gid 10001 --system nonroot \
    && adduser --uid 10000 --system --ingroup nonroot --home /home/nonroot nonroot \
    && apk update \
    && apk add --no-cache tini bind-tools

COPY --from=build-env /src/app/brainstorm /sbin/brainstorm
ENTRYPOINT ["/sbin/tini", "--", "/sbin/brainstorm"]
WORKDIR /data
USER nonroot
