#!/bin/bash
mkdir -p /log

#RES=$(mount.glusterfs localhost:/log /log 2>&1)
RES=$(mount.glusterfs desktop-gluster-1:/log /log 2>&1)
#RES=$(mount.glusterfs tesi-gluster-1:/log /log 2>&1)
while [[ $RES != "" ]]; do
    sleep 1
    RES=$(mount.glusterfs desktop-gluster-1:/log /log 2>&1)
    #RES=$(mount.glusterfs tesi-gluster-1:/log /log 2>&1)
    echo $RES 2>&1
done

/home/raft/main
