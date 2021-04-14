FROM 10.225.108.70:9443/proxy/golang:1.16.2-buster as builder
WORKDIR /go/src/github.com/influxdata/telegraf

COPY . /go/src/github.com/influxdata/telegraf
RUN CGO_ENABLED=0 make deps telegraf go-install

FROM 10.225.108.70:9443/proxy/alpine:3.13.3
RUN echo 'hosts: files dns' >> /etc/nsswitch.conf
RUN apk add --no-cache iputils ca-certificates net-snmp-tools procps lm_sensors && \
    update-ca-certificates
COPY --from=builder /go/bin/* /usr/bin/
COPY etc/telegraf.conf /etc/telegraf/telegraf.conf

EXPOSE 8125/udp 8092/udp 8094

COPY scripts/docker-entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
CMD ["telegraf"]