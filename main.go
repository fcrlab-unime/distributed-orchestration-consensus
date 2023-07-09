package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"reflect"
	. "server"
	. "storage"
	"strconv"
	"strings"
	"time"
)

func main(){
	server := startServer()
	
	go waitSubmit(server)
	for{}
}

func startServer() *Server {
	ready := make(chan interface{})
	storage := NewMapStorage()
	commitChannel := make(chan CommitEntry)
	serverIp, subnetMask := getNetworkInfo()
	serverId, _ := strconv.Atoi(strings.Split(serverIp.String(), ".")[3])

	peersAddrs := getPeersIp(serverIp, subnetMask)
	peersIds := []int{}
	peers := make(map[int]net.Addr)

	// Create all Servers in this cluster, assign ids and peer ids.
	for p := 0; p < len(peersAddrs); p++ {
		if peersAddrs[p] != serverIp {
			id, _ := strconv.Atoi(strings.Split(peersAddrs[p].String(), ".")[3])
			peers[id] = peersAddrs[p]
			peersIds = append(peersIds, id)
		}
	}

	
	server := NewServer(serverId, peersIds, storage, ready, commitChannel)
	server.Serve(serverIp)
	// Connect all peers to each other.
	time.Sleep(1 * time.Second)
	
	for id, addr := range peers {
		error := server.ConnectToPeer(id, addr)
		fmt.Printf("Result: %v\n", error)
		if error != nil {
			server.DisconnectPeer(id)
			delete(peers, id)
		}
	}
	close(ready)

	go checkNewPeers(server, &peers)


	return server
}

func getNetworkInfo() (ip net.Addr, subnetMask string) {
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

func getPeersIp(serverIp net.Addr, subnetMask string) (peersIp []net.Addr) {
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

func checkNewPeers(server *Server, peersPtr *map[int]net.Addr) {
	peers := *peersPtr
	for {
		select {
		case <-server.GetQuit():
			return
		case <-time.After(100 * time.Millisecond):
			ip, mask := getNetworkInfo()
			newPeersIp := getPeersIp(ip, mask)
			newPeersIds := []int{}
			newPeers := make(map[int]net.Addr)
			defaultGateway := getDefaultGateway()
			// Calculate new peers ids
			for i := 0; i < len(newPeersIp); i++ {
				if newPeersIp[i].String() != ip.String() && newPeersIp[i].String() != defaultGateway.String() {
					id, _ := strconv.Atoi(strings.Split(newPeersIp[i].String(), ".")[3])
					newPeersIds = append(newPeersIds, id)
					newPeers[id] = newPeersIp[i]
				}
			}

			if reflect.DeepEqual(peers, newPeers) {
				break
			}
			fmt.Printf("Peers: %v\nNew Peers: %v\n", peers, newPeers)
			
			for id, addr := range newPeers {
				if _, ok := peers[id]; !ok {
					peers[id] = addr
				}
			}

			for id := range peers {
				if _, ok := newPeers[id]; !ok {
					server.DisconnectPeer(id)
					delete(peers, id)
				}
			}
			
			
			for id, addr := range peers {
				error := server.ConnectToPeer(id, addr)
				fmt.Printf("Result: %v\n", error)
				if error != nil {
					server.DisconnectPeer(id)
					delete(peers, id)
				}
			}
		}
	}
}

func waitSubmit(server *Server) {
	listener, err := net.Listen("tcp", ":9093")
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	for {
		conn, err := listener.Accept()
		if err != nil {
			panic(err)
		}
		go handleConnection(conn, server)
	}
}

func handleConnection(conn net.Conn, server *Server) {
	for {
		buf := make([]byte, 4096) 
		var mess string
		n, err := conn.Read(buf[0:])
	
		if err != nil {
			fmt.Printf("\n\nError: %v\n\n", err)
			return
		}
		if	n > 0 {
			//TODO: da modificare
			mess = strings.ReplaceAll(strings.ReplaceAll(string(buf[0:n]), "\n", ""), "\r", "")
			buf = buf[:0]
			command, err := strconv.Atoi(mess)
			fmt.Printf("Received: %v, and error is %v\n", command, err)
			if err == nil {
				server.Submit(command) 
			}
		}
	}
}

func getDefaultGateway() (*net.IPAddr) {
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