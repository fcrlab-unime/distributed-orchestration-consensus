FROM golang:1.22

RUN apt update && apt -y upgrade
RUN apt -y install fping iproute2 telnet vim iputils-ping nmap netcat-openbsd fuse 
RUN wget -O - https://download.gluster.org/pub/gluster/glusterfs/9/rsa.pub | apt-key add - && \
    echo deb [arch=amd64] https://download.gluster.org/pub/gluster/glusterfs/9/LATEST/Debian/bullseye/amd64/apt bullseye main > /etc/apt/sources.list.d/gluster.list && \
    apt update

RUN apt install -y glusterfs-client

RUN curl -SL https://github.com/docker/compose/releases/download/v2.20.2/docker-compose-linux-x86_64 -o /usr/local/bin/docker-compose && \
    chmod +x /usr/local/bin/docker-compose

COPY ./src/. /home/raft/.

WORKDIR /home/raft
#RUN ls -la && sleep 15s

RUN go mod init raft
#RUN cat go.mod && sleep 30s
#RUN go get github.com/GoogleCloudPlatform/golang-samples/iam/namesgenerator
#RUN go get github.com/brianvoe/gofakeit/v7
#RUN go get github.com/Pallinder/go-randomdata/namesgenerator
RUN go get golang.org/x/exp/slices
RUN go get gopkg.in/yaml.v3
RUN go mod tidy
#RUN cat go.mod && sleep 60s
RUN go build main.go

RUN mkdir services/
RUN mkdir /log
ENV RPC_PORT=4000
ENV GATEWAY_PORT=9093
ENV DEBUG=0
ENV TIME=0

CMD [ "./init.sh" ]
