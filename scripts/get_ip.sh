#!/bin/bash
if [[ $1 = "ping" ]]; then
    if [[ ! -f /tmp/oldip.txt ]]; then
        touch /tmp/oldip.txt
    fi

    while true; do
        fping -aqg -i 1 $2/$3 | tee /tmp/ip.fifo > /tmp/newip.txt
        while read line; do
            CHECK=`grep -c $line /tmp/newip.txt`
            if [[ $CHECK == "0" ]]; then
                echo $line > /tmp/exip.fifo
            fi
        done < /tmp/oldip.txt
        mv /tmp/newip.txt /tmp/oldip.txt
        sleep 5
    done
elif [[ $1 = "nc" ]]; then
    nc -zvn $2 $3 2>&1 > /dev/null | grep succeeded | cut -d\  -f 3
elif [[ $1 = "nmap" ]]; then
    nmap -np $RPC_PORT --open $2/$3 | grep "Nmap scan report for" | cut -d\  -f 5 > /tmp/newip.txt
fi