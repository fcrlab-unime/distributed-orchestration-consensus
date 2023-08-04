tail -f -n0 /var/log/glusterfs/glusterd.log | while read line; do

    if [[ "${line}" == *"MSGID: 106004"* && "${line}" == *"has disconnected from glusterd"* ]]; then
        IP_TO_REMOVE=$(cut -d\  -f 9 <<< "${line}" | sed 's/>//g; s/<//g')
        UUID_TO_REMOVE=$(cut -d\  -f 10 <<< "${line}" | sed 's/>),//g; s/(<//g')
        NUM=$(echo $(( $(gluster pool list | grep -v ${UUID_TO_REMOVE} | wc -l) - 1 )))
        DET=$(yes | gluster volume remove-brick log replica $NUM $IP_TO_REMOVE:/data force)
        echo $IP_TO_REMOVE > /aaa
        if [[ "${DET}" == *"success"* ]]; then
            RES=$(yes | gluster peer detach ${IP_TO_REMOVE})
            while [[ "${RES}" != *"success"* ]]; do
                sleep 1
                RES=$(yes | gluster peer detach ${IP_TO_REMOVE})
            done
        fi
    fi

done