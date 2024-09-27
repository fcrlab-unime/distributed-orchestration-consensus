FROM ubuntu:20.04

RUN apt update && apt -y upgrade
RUN apt install -y vim software-properties-common iproute2 fping policycoreutils

RUN add-apt-repository ppa:gluster/glusterfs-7 && apt update
RUN apt install -y glusterfs-server
RUN mkdir /log

COPY ./init.sh /bin/init.sh
COPY ./connect.sh /bin/connect.sh
COPY ./check_new.sh /bin/check_new.sh

CMD ["/bin/init.sh"]