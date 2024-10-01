IFACE_LIST=$(ip -brief -4 a | awk -F\  '{print $1" "$2" "$3}')
IP=""
MASK=""
while read IFACE; do
    INFOS=($IFACE)
    if [[ ${INFOS[0]} == *"$NET_IFACE"* && ${INFOS[1]} == "UP" ]]; then
        IFS='/' read IP MASK <<< "${INFOS[2]}"
        break
    fi
done <<< $IFACE_LIST
#echo $IFACE_LIST
#echo $IP
DONE=false
echo "Scanning for peers..."
while read line; do
    echo $line
    RES=$(timeout 1s gluster peer probe $line 2>&1)
    echo $RES
    if grep -vq "localhost" <<< "$RES"; then
        if $(grep -q "success" <<< "$RES") || $(grep -q "already part of" <<< "$RES") ; then
            DONE=true
            break
        fi
    fi
done <<< $(fping -aqg -i 10 -r 0 $IP/$MASK)

#echo $DONE
#sleep 5s
if [[ $DONE == false ]]; then
    echo "Shared volume not found. Creating one..."
    gluster volume create log $IP:/data force 
    gluster volume start log
fi