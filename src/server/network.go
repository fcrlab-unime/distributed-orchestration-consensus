package server

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
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

func GetPeersIp(serverIp net.Addr, subnetMask string, peerChan *chan net.Addr, exPeerChan *chan net.Addr, check bool) (newPeers []net.Addr) {
	
	if check {
		if _, err := os.Stat("/tmp/ip.fifo"); os.IsNotExist(err) {
			syscall.Mkfifo("/tmp/ip.fifo", 0666)
		}

		if _, err := os.Stat("/tmp/exip.fifo"); os.IsNotExist(err) {
			syscall.Mkfifo("/tmp/exip.fifo", 0666)
		}

		
		go func(exPeerChan *chan net.Addr) {
			exPipe, _ := os.OpenFile("/tmp/exip.fifo", os.O_RDONLY|syscall.O_NONBLOCK, os.ModeNamedPipe)
			exReader := bufio.NewReader(exPipe)
			for {
				line, _, err := exReader.ReadLine()
				if err != nil {
					continue
				}
				nline := strings.TrimSuffix(string(line), "\n")
				*exPeerChan <- &net.IPAddr{IP: net.ParseIP(string(nline))}
			}
		}(exPeerChan)
		
		newPipe, _ := os.OpenFile("/tmp/ip.fifo", os.O_RDONLY|syscall.O_NONBLOCK, os.ModeNamedPipe)
		newReader := bufio.NewReader(newPipe)
		exec.Command("bash", "/home/raft/get_ip.sh", "ping", serverIp.String(), subnetMask).Start()		
		for {
			line, _, err := newReader.ReadLine()
			if err != nil {
				continue
			}
			nline := strings.TrimSuffix(string(line), "\n")
			fmt.Println(nline)
			*peerChan <- &net.IPAddr{IP: net.ParseIP(string(nline))}
		}
	} else {
		if _, err := os.Stat("/tmp/ip.txt"); err == nil {
			os.Truncate("/tmp/ip.txt", 0)
		}
		exec.Command("bash", "/home/raft/get_ip.sh", "nmap", serverIp.String(), subnetMask).Run()
		peersIpFile, _ := os.ReadFile("/tmp/ip.txt")
		peersIpStr := strings.Split(string(peersIpFile), "\n")
		for i := 0; i < len(peersIpStr); i++ {
			if peersIpStr[i] != serverIp.String() && peersIpStr[i] != "" {
				newPeers = append(newPeers, &net.IPAddr{IP: net.ParseIP(peersIpStr[i])})
			}
		}
		return newPeers
	}

}

func CheckNewPeers(server *Server, peersPtr *map[int]net.Addr) {
	peers := *peersPtr
	peerChan := make(chan net.Addr, 100)
	exPeerChan := make(chan net.Addr, 100)
	ip, mask:= GetNetworkInfo()
	var connect int
	go GetPeersIp(ip, mask, &peerChan, &exPeerChan, true)

	go func(exPeerChan *chan net.Addr) {
		for {
			addr := <- *exPeerChan
			defaultGateway := GetDefaultGateway()
			tmpId := GetServerIdFromIp(addr, mask)

			if addr.String() != ip.String() && addr.String() != defaultGateway.String() {
				if peers[tmpId] != nil {
					server.DisconnectPeer(tmpId)
					delete(peers, tmpId)
				}
			}

		}
	}(&exPeerChan)

	for {
		addr := <-peerChan
		defaultGateway := GetDefaultGateway()
		tmpId := 0
		connect = 1
		ok, err := exec.Command("bash", "/home/raft/get_ip.sh", "nc", addr.String(), os.Getenv("RPC_PORT")).Output()
		if err != nil {
			fmt.Printf("Error net: %v\n", err)
			continue
		}
		if string(ok) == "" {
			connect = 0
		}

		if addr.String() != ip.String() && addr.String() != defaultGateway.String() {
			tmpId = GetServerIdFromIp(addr, mask)
		} else {
			continue
		}
	
		if peers[tmpId] != nil {
			if connect == 0 {
				server.DisconnectPeer(tmpId)
				delete(peers, tmpId)
			}
		} else {
			if connect == 1 {
				error := server.ConnectToPeer(tmpId, addr)
				if error != nil {
					server.DisconnectPeer(tmpId)
					delete(peers, tmpId)
				} else {
					peers[tmpId] = addr
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

func GetServerIdFromIp(ip net.Addr, netmask string) (id int) {

	if ip == nil && netmask == "" {
		ip, netmask = GetNetworkInfo()
	} else if netmask == "" {
		_, netmask = GetNetworkInfo()
	} else if ip == nil {
		ip, _ = GetNetworkInfo()
	}

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