tail -f -n0 /var/log/glusterfs/glusterd.log | while read line; do   
    if [[ "${line}" == *"MSGID: 106170"* && "${line}" == *"Rejecting management handshake request from unknown peer"* ]]; then
    
        TO_ADD=$(cut -d\  -f 15 <<< "${line}" | cut -d: -f 1)
        RES=$(gluster peer probe ${TO_ADD})

        if [[ "${RES}" == *"success"* ]]; then
            NUM=$(echo $(( $(gluster pool list | wc -l) - 1 )))
            RES=$(gluster volume add-brick log replica $NUM $TO_ADD:/data force)
            while [[ $RES != *"success"* ]]; do
                sleep 1
                RES=$(gluster volume add-brick log replica $NUM $TO_ADD:/data force)
            done
        fi
    fi
done