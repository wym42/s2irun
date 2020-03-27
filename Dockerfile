FROM golang:1.11-alpine as builder

WORKDIR /go/src/github.com/kubesphere/s2irun
COPY cmd/ cmd/
COPY pkg/ pkg/
COPY vendor/ vendor/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o builder github.com/kubesphere/s2irun/cmd
COPY ./executor /go/src/github.com/kubesphere/s2irun/kaniko

RUN chmod +x /go/src/github.com/kubesphere/s2irun/kaniko
RUN chmod +x /go/src/github.com/kubesphere/s2irun/builder


FROM alpine:latest
WORKDIR /root/

RUN apk update && apk upgrade && \
    apk add --no-cache bash git openssh
COPY ./executor /bin/kaniko
RUN chmod +x /bin/kaniko
ENV KANIKO_EXEC_PATH /bin/kaniko

ENV S2I_CONFIG_PATH=/root/data/config.json
COPY --from=builder /go/src/github.com/kubesphere/s2irun/kaniko .
COPY --from=builder /go/src/github.com/kubesphere/s2irun/builder .

CMD ["./builder", "-v=4", "-logtostderr=true"]





