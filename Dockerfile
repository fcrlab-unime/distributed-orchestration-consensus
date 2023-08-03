FROM golang:bullseye

RUN apt update && apt -y upgrade && apt -y install fping iproute2 telnet vim iputils-ping nmap netcat

COPY . /home/raft

WORKDIR /home/raft

RUN chmod +x ./scripts/get_ip.sh

ENV RPC_PORT=4000
ENV SERVICE_PORT=4001
ENV GATEWAY_PORT=9093
ENV DEBUG=0

RUN go build main.go

CMD [ "./main" ]