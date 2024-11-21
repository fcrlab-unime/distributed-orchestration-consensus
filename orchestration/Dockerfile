FROM golang:1.22

#INSTALL UTILS PACKAGES
RUN apt update && apt -y upgrade
RUN apt -y install fping iproute2 telnet vim iputils-ping nmap netcat-openbsd fuse 
RUN wget -O - https://download.gluster.org/pub/gluster/glusterfs/9/rsa.pub | apt-key add - && \
    echo deb [arch=amd64] https://download.gluster.org/pub/gluster/glusterfs/9/LATEST/Debian/bullseye/amd64/apt bullseye main > /etc/apt/sources.list.d/gluster.list && \
    apt update

#INSTALL GLUSTERFS CLIENT
RUN apt install -y glusterfs-client

#INSTALL DOCKER COMPOSE TOOL
RUN curl -SL https://github.com/docker/compose/releases/download/v2.20.2/docker-compose-linux-x86_64 -o /usr/local/bin/docker-compose && \
    chmod +x /usr/local/bin/docker-compose

#COPY SOURCE CODE AND SCRIPTS
COPY ./src/ /home/raft/
COPY ./scripts/ /home/raft/scripts/

WORKDIR /home/raft
#RUN ls -la && sleep 15s #DEBUG: verify everithing is in the right path

#CREATE GO PROJECT AND INSTALL DEPENDENCIES
#RUN go mod init raft
#RUN go get golang.org/x/exp/slices
#RUN go get gopkg.in/yaml.v3
#RUN go mod tidy
#RUN cat go.mod && sleep 60s #DEBUG: verify go dependencies

#BUILD GO PROJECT
RUN go build main.go

#CREATE NEEDED FOLDERS
RUN mkdir services/
RUN mkdir /log
RUN mkdir /test

#SETTING DEFAULT ENV VARIABLES
ENV RPC_PORT=4000
ENV GATEWAY_PORT=9093
ENV DEBUG=0
ENV TIME=0

RUN chmod +x ./init.sh

#STARTING EVERYTHING
CMD [ "./init.sh" ]
