FROM golang:bullseye

RUN apt update && apt -y upgrade
RUN apt -y install fping iproute2 telnet vim iputils-ping nmap netcat fuse 
RUN wget -O - https://download.gluster.org/pub/gluster/glusterfs/9/rsa.pub | apt-key add - && \
    echo deb [arch=amd64] https://download.gluster.org/pub/gluster/glusterfs/9/LATEST/Debian/bullseye/amd64/apt bullseye main > /etc/apt/sources.list.d/gluster.list && \
    apt update

RUN apt install -y glusterfs-client

RUN curl -SL https://github.com/docker/compose/releases/download/v2.20.2/docker-compose-linux-x86_64 -o /usr/local/bin/docker-compose && \
    chmod +x /usr/local/bin/docker-compose

COPY . /home/raft

WORKDIR /home/raft
RUN mkdir services/
ENV RPC_PORT=4000
ENV SERVICE_PORT=4001
ENV GATEWAY_PORT=9093
ENV DEBUG=0

RUN go build main.go

CMD [ "./init.sh" ]
