#!/bin/bash
if [[ $1 = "ping" ]]; then
    while true; do
        fping -aqg -i 10 -r 0 $2/$3 | tee /tmp/ip.fifo > /tmp/newip.txt
#        sleep 5
    done
elif [[ $1 = "nc" ]]; then
    nc -zvn $2 $3 2>&1 > /dev/null | grep succeeded | cut -d\  -f 3
elif [[ $1 = "nmap" ]]; then
    nmap -np $RPC_PORT --open $2/$3 | grep "Nmap scan report for" | cut -d\  -f 5 > /tmp/newip.txt
fi