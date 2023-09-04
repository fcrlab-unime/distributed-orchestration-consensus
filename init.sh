#!/bin/bash
mkdir -p /log

RES=$(mount.glusterfs localhost:/log /log 2>&1)
while [[ $RES != "" ]]; do
    sleep 1
    RES=$(mount.glusterfs localhost:/log /log 2>&1)
done

/home/raft/main
