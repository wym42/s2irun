FROM golang:1.14 as builder

ENV GOPROXY http://mirrors.aliyun.com/goproxy/
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
#FROM alpine:3.10

ARG GIT_COMMIT
ARG BUILD_TIME

RUN echo http://mirrors.aliyun.com/alpine/v3.10/main/ > /etc/apk/repositories && \
    echo http://mirrors.aliyun.com/alpine/v3.10/community/ >> /etc/apk/repositories

WORKDIR /root/

RUN apk update && apk upgrade && \
    apk add --no-cache bash git openssh

#COPY ./executor /bin/kaniko
#RUN chmod +x /bin/kaniko
ENV KANIKO_EXEC_PATH /bin/kaniko

ENV S2I_CONFIG_PATH=/root/data/config.json
COPY --from=builder /go/src/github.com/kubesphere/s2irun/kaniko /bin/kaniko
COPY --from=builder /go/src/github.com/kubesphere/s2irun/builder /bin/builder

CMD ["builder", "-v=4", "-logtostderr=true"]





