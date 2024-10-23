#!/bin/bash

sudo su
cp -r /var/lib/docker/volumes/distributed-orchestration-consensus_gluster/_data/ /home/pi/results
chmod 777 -R /home/pi/results