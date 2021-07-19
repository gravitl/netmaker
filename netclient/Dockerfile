FROM debian:latest

RUN apt-get update && apt-get -y install systemd procps

WORKDIR /root/

COPY netclient .

CMD ["./netclient checkin"]
