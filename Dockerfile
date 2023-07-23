FROM golang:bullseye

RUN apt update && apt -y upgrade && apt -y install fping iproute2 telnet vim iputils-ping nmap

COPY . /home/raft

WORKDIR /home/raft

RUN chmod +x ./get_ip.sh

CMD [ "tail", "-f", "/dev/null"]
#CMD [ "go", "run", "main.go" ]