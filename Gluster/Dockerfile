FROM ubuntu:20.04

RUN apt update && apt -y upgrade
RUN apt install -y vim software-properties-common 

RUN add-apt-repository ppa:gluster/glusterfs-7 && apt update
RUN apt install -y glusterfs-server

COPY ./init.sh /bin/init.sh

CMD ["/bin/init.sh"]