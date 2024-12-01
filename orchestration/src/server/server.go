// Server container for a Raft Consensus Module. Exposes Raft to the network
// and enables RPCs between Raft peers.
//
// Eli Bendersky [https://eli.thegreenplace.net]
// This code is in the public domain.
package server

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	st "storage"
	"sync"
	"test"
)

type Server struct {
	mu sync.Mutex

	serverId int
	peerIds  []int
	peers    map[int]net.Addr

	cm       *ConsensusModule
	storage  st.Storage
	rpcProxy *RPCProxy

	rpcServer *rpc.Server
	listener  net.Listener

	commitChan  chan<- CommitEntry
	peerClients map[int]*rpc.Client

	ready <-chan interface{}
	quit  chan interface{}
	wg    sync.WaitGroup

	Times      map[int]*test.Times
	SubmitChan chan struct{}
	commands   []Service
}

func NewServer(serverId int, storage st.Storage, ready <-chan interface{}, commitChan chan<- CommitEntry) *Server {
	s := new(Server)
	s.serverId = serverId
	s.peerIds = []int{}
	s.peers = make(map[int]net.Addr)
	s.peerClients = make(map[int]*rpc.Client)
	s.storage = storage
	s.ready = ready
	s.commitChan = commitChan
	s.quit = make(chan interface{})
	s.Times = make(map[int]*test.Times)
	s.SubmitChan = make(chan struct{})
	s.commands = []Service{}
	s.cm = NewConsensusModule(s.serverId, s, s.storage, s.ready)
	return s
}

func (s *Server) AppendCommand(command Service) {
	s.commands = append(s.commands, command)
	if len(s.commands) == 1 {
		s.SubmitChan <- struct{}{}
	}
}

func (s *Server) DeleteFirstCommand() {
	s.commands = s.commands[1:]
	if len(s.commands) > 0 {
		s.SubmitChan <- struct{}{}
	}
}

func (s *Server) Serve(ip net.Addr, wg *sync.WaitGroup, ready chan interface{}) {
	s.mu.Lock()

	// Create a new RPC server and register a RPCProxy that forwards all methods
	// to n.cm
	s.rpcServer = rpc.NewServer()
	s.rpcProxy = &RPCProxy{cm: s.cm}
	s.rpcServer.RegisterName("ConsensusModule", s.rpcProxy)

	var err error
	s.listener, err = net.Listen("tcp", ip.String()+":"+os.Getenv("RPC_PORT"))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("[%v] listening at %s", s.serverId, s.listener.Addr())
	s.mu.Unlock()
	ready <- struct{}{}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		for {
			conn, err := s.listener.Accept()
			if err != nil {
				select {
				case <-s.quit:
					return
				default:
					log.Fatal("accept error:", err)
				}
			}
			s.wg.Add(1)
			go func() {
				s.rpcServer.ServeConn(conn)
				s.wg.Done()
			}()
		}
	}()
	wg.Done()
}

// DisconnectAll closes all the client connections to peers for this server.
func (s *Server) DisconnectAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range s.peerClients {
		if s.peerClients[id] != nil {
			s.peerClients[id].Close()
			s.peerClients[id] = nil
		}
	}
}

// Shutdown closes the server and waits for it to shut down properly.
func (s *Server) Shutdown() {
	s.cm.Stop()
	close(s.quit)
	s.listener.Close()
	s.wg.Wait()
}

func (s *Server) GetListenAddr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listener.Addr()
}

func (s *Server) ConnectToPeer(peerId int, addr net.Addr) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Printf("Connecting to peer %d at %s\n", peerId, addr.String())
	if s.peerClients[peerId] == nil {
		client, err := rpc.Dial("tcp", addr.String()+":"+os.Getenv("RPC_PORT"))
		if err != nil {
			return err
		} else {
			s.peerClients[peerId] = client
			s.peerIds = append(s.peerIds, peerId)
			s.peers[peerId] = addr
			s.cm.ConnectPeer(peerId)
		}
	}
	return nil
}

// DisconnectPeer disconnects this server from the peer identified by peerId.
func (s *Server) DisconnectPeer(peerId int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.peerClients[peerId] != nil {
		err := s.peerClients[peerId].Close()
		s.cm.DisconnectPeer(peerId)
		delete(s.peerClients, peerId)
		for i, elem := range s.peerIds {
			if elem == peerId {
				s.peerIds = append(s.peerIds[:i], s.peerIds[i+1:]...)
				delete(s.peers, peerId)
				break
			}
		}
		return err
	}
	return nil
}

func (s *Server) Call(id int, serviceMethod string, args interface{}, reply interface{}) error {
	s.mu.Lock()
	peer := s.peerClients[id]
	s.mu.Unlock()

	// If this is called after shutdown (where client.Close is called), it will
	// return an error.
	if peer == nil {
		return fmt.Errorf("call client %d after it's closed", id)
	} else {
		return peer.Call(serviceMethod, args, reply)
	}
}

// RPCProxy is a trivial pass-thru proxy type for ConsensusModule's RPC methods.
// It's useful for:
//   - Simulating a small delay in RPC transmission.
//   - Avoiding running into https://github.com/golang/go/issues/19957
//   - Simulating possible unreliable connections by delaying some messages
//     significantly and dropping others when RAFT_UNRELIABLE_RPC is set.
type RPCProxy struct {
	cm *ConsensusModule
}

func (rpp *RPCProxy) RequestVote(args RequestVoteArgs, reply *RequestVoteReply) error {
	return rpp.cm.RequestVote(args, reply)
}

func (rpp *RPCProxy) AppendEntries(args AppendEntriesArgs, reply *AppendEntriesReply) error {
	return rpp.cm.AppendEntries(args, reply)
}

func (rpp *RPCProxy) Deploy(args DeployArgs, reply *DeployReply) error {
	return rpp.cm.Deploy(args, reply)
}

func (s *Server) GetQuit() chan interface{} {
	return s.quit
}

func (s *Server) GetId() int {
	return s.serverId
}

func (s *Server) GetConsensusModule() *ConsensusModule {
	return s.cm
}

// func (s *Server) Submit(command *Service, index ...int) {
func (s *Server) Submit(index ...int) {
	if len(s.commands) == 0 {
		return
	}
	if os.Getenv("TIME") == "1" {
		s.cm.Election(s.commands[0].index)
	} else {
		s.cm.Election()
	}
	<-s.cm.ElectionChan
	if os.Getenv("TIME") == "1" {
		s.Times[index[0]].SetDurationAndWrite(index[0], "ENVE", s.GetConsensusModule().StartTime)
		s.Times[index[0]].SetStartTime("CP")
		s.cm.Voting(&s.commands[0], s.commands[0].index)
	} else {
		s.cm.Voting(&s.commands[0])
	}
	<-s.cm.VotingChan
	//s.cm.Pause()
}

func (s *Server) SubmitCommands() {
	for {
		<-s.SubmitChan
		s.Submit()
	}
}
