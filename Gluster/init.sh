#!/bin/bash
glusterd -N -p /var/run/glusterd.pid &

/bin/connect.sh
/bin/check_new.sh & 
tail -f /dev/null