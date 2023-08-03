#!/bin/bash  
touch /var/log/glusterfs/glusterd.log
glusterd -N -p /var/run/glusterd.pid &
tail -f -n1 /var/log/glusterfs/glusterd.log | while read line; do   
    if [[ "${line}" == *"MSGID: 106170"* && "${line}" == *"Rejecting management handshake request from unknown peer"* ]]; then
    	TO_ADD=$(cut -d\  -f 15 <<< "${line}" | cut -d: -f 1)
		gluster peer probe ${TO_ADD}
    fi
done