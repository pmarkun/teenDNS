FROM alpine:3.22

RUN apk add --no-cache unbound
COPY config/lab/unbound.conf /etc/unbound/unbound.conf
EXPOSE 5353/udp 5353/tcp
ENTRYPOINT ["unbound", "-d", "-c", "/etc/unbound/unbound.conf"]
