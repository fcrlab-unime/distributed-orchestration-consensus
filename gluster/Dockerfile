FROM ubuntu:20.04

#INSTALL UTILS PACKAGES
RUN apt update && apt -y upgrade
RUN DEBIAN_FRONTEND=noninteractive
RUN apt install -y tzdata
RUN ln -fs /usr/share/zoneinfo/UTC /etc/localtime && \
    dpkg-reconfigure --frontend noninteractive tzdata
RUN apt install -y vim software-properties-common iproute2 fping policycoreutils policycoreutils-python-utils selinux-utils selinux-basics

#CONFIGURE UTILS PACKAGES
RUN selinux-activate

#INSTALL GLUSTERFS SERVER
RUN add-apt-repository ppa:gluster/glusterfs-7 && apt update
RUN apt install -y glusterfs-server

#RUN mkdir /log

#COPY SOURCE CODE
COPY ./init.sh /bin/init.sh
COPY ./connect.sh /bin/connect.sh
COPY ./check_new.sh /bin/check_new.sh

RUN chmod +x /bin/init.sh /bin/connect.sh /bin/check_new.sh

#START EVERITHING
CMD ["/bin/init.sh"]