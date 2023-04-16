fping -aqg $1/$2 > /tmp/ip.txt &
sleep 3
pkill -9 fping