#!/bin/bash
echo "Mounting the log filesystem...."
RES=$(mount.glusterfs localhost:/log /log 2>&1)
echo $RES
while [[ $RES != "" ]]; do
    sleep 1
    RES=$(mount.glusterfs localhost:/log /log 2>&1)
    echo $RES
done
echo "Log filesystem mounted."
echo "Starting orchestration module..."
/home/raft/main
tail -f /dev/null #TO DEBUG IN CASE OF CRASH