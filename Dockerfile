FROM golang

RUN apt update && apt -y upgrade && apt -y install fping iproute2 telnet vim

COPY . /home/raft

WORKDIR /home/raft

RUN chmod +x ./get_ip.sh

CMD [ "tail", "-f", "/dev/null"]
#CMD [ "go", "run", "main.go" ]