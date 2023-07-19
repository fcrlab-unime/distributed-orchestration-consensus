package server

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"time"
)


func GetNetworkInfo() (ip net.Addr, subnetMask string) {
	infosCmd, _ := exec.Command("ip", "-4", "-brief" , "address").Output()
	tmpInfos := strings.Split(string(infosCmd), "\n")
	infos := []string{}
	for i := 0; i < len(tmpInfos); i++ {
		infos = strings.Fields(tmpInfos[i])
		if strings.Contains(infos[1], "UP"){
			infos = strings.Split(infos[2], "/")
			infos = []string {infos[0], infos[1]}
			break
		}
	}
	ip = &net.IPAddr{IP: net.ParseIP(infos[0])}
	subnetMask = infos[1]
	return ip, subnetMask
}

func GetPeersIp(serverIp net.Addr, subnetMask string) (peersIp []net.Addr) {
	if _, err := os.Stat("/tmp/ip.txt"); err == nil {
		os.Truncate("/tmp/ip.txt", 0)
	}
	exec.Command("bash", "/home/raft/get_ip.sh", serverIp.String(), subnetMask).Run()
	peersIpFile, _ := os.ReadFile("/tmp/ip.txt")
	peersIpStr := strings.Split(string(peersIpFile), "\n")
	for i := 0; i < len(peersIpStr); i++ {
		if peersIpStr[i] != serverIp.String() && peersIpStr[i] != "" {
			ip := net.ParseIP(peersIpStr[i])
			peersIp = append(peersIp, &net.IPAddr{IP: ip})
		}
	}
	return peersIp
}

func CheckNewPeers(server *Server, peersPtr *map[int]net.Addr) {
	peers := *peersPtr
	for {
		select {
		case <-server.GetQuit():
			return
		case <-time.After(100 * time.Millisecond):
			ip, mask := GetNetworkInfo()
			newPeersIp := GetPeersIp(ip, mask)
			newPeers := make(map[int]net.Addr)
			defaultGateway := GetDefaultGateway()
			// Calculate new peers ids
			for i := 0; i < len(newPeersIp); i++ {
				if newPeersIp[i].String() != ip.String() && newPeersIp[i].String() != defaultGateway.String() {
					id, _ := strconv.Atoi(strings.Split(newPeersIp[i].String(), ".")[3])
					newPeers[id] = newPeersIp[i]
				}
			}

			if reflect.DeepEqual(peers, newPeers) {
				continue
			}
			fmt.Printf("Peers: %v\nNew Peers: %v\n", peers, newPeers)
			
			for id, addr := range newPeers {
				if _, ok := peers[id]; !ok {
					error := server.ConnectToPeer(id, addr)
					fmt.Printf("Result: %v\n", error)
					if error != nil {
						server.DisconnectPeer(id)
						delete(peers, id)
					} else {
						peers[id] = addr
					}
				}
			}

			for id := range peers {
				if _, ok := newPeers[id]; !ok {
					server.DisconnectPeer(id)
					delete(peers, id)
				}
			}
		}
	}
}

func GetDefaultGateway() (*net.IPAddr) {
	infosCmd, _ := exec.Command("ip", "route").Output()
	tmpInfos := strings.Split(string(infosCmd), "\n")

	for i := 0; i < len(tmpInfos); i++ {
		if strings.Contains(tmpInfos[i], "default") {
			infos := strings.Split(tmpInfos[i], " ")
			return &net.IPAddr{IP: net.ParseIP(infos[2])}
		}
	}

	return nil
}

func GetServerIdFromIp() (id int) {
	ip, netmask := GetNetworkInfo()

	ipList := strings.Split(ip.String(), ".")
	ipIntList := []int{}
	for i := 0; i < len(ipList); i++ {
		ipInt, _ := strconv.Atoi(ipList[i])
		ipIntList = append(ipIntList, ipInt)
	}

	netmaskInt, _ := strconv.Atoi(netmask)

	if netmaskInt >= 24 {
		id = ipIntList[3]
	} else if netmaskInt >= 16 {
		id = ipIntList[2]*256 + ipIntList[3]
	} else if netmaskInt >= 8 {
		id = ipIntList[1]*256*256 + ipIntList[2]*256 + ipIntList[3]
	} else {
		id = ipIntList[0]*256*256*256 + ipIntList[1]*256*256 + ipIntList[2]*256 + ipIntList[3]
	}

	return id
}

func GetServerIpFromId(id int) (LeaderIp net.Addr) {
	ip, netmask := GetNetworkInfo()

	ipList := strings.Split(ip.String(), ".")
	ipIntList := []int{}
	for i := 0; i < len(ipList); i++ {
		ipInt, _ := strconv.Atoi(ipList[i])
		ipIntList = append(ipIntList, ipInt)
	}

	netmaskInt, _ := strconv.Atoi(netmask)

	if netmaskInt >= 24 {
		ipIntList[3] = id
	} else if netmaskInt >= 16 {
		ipIntList[2] = id/256
		ipIntList[3] = id%256
	} else if netmaskInt >= 8 {
		ipIntList[1] = id/256/256
		ipIntList[2] = id/256%256
		ipIntList[3] = id%256
	} else {
		ipIntList[0] = id/256/256/256
		ipIntList[1] = id/256/256%256
		ipIntList[2] = id/256%256
		ipIntList[3] = id%256
	}

	ipStr := ""
	for i := 0; i < len(ipIntList); i++ {
		ipStr += strconv.Itoa(ipIntList[i])
		if i != len(ipIntList)-1 {
			ipStr += "."
		}
	}

	return &net.IPAddr{IP: net.ParseIP(ipStr)}

}