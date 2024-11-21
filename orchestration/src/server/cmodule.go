// Core Raft implementation - Consensus Module.
//
// Eli Bendersky [https://eli.thegreenplace.net]
// This code is in the public domain.
package server

import (
	"crypto/sha256"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"reflect"
	l "server/resource"

	//"sort"
	st "storage"
	"strconv"

	//"strings"
	"sync"
	"time"
)

var DebugCM string = os.Getenv("DEBUG")

// CommitEntry is the data reported by Raft to the commit channel. Each commit
// entry notifies the client that consensus was reached on a command and it can
// be applied to the client's state machine.
type CommitEntry struct {
	// Command is the client command being committed.
	Command Service

	// Index is the log index at which the client command is committed.
	Index int

	// Term is the Raft term at which the client command is committed.
	Term int

	// ChosenId is the ID of the chosen client.
	ChosenId int
}

type CMState int

const (
	Follower CMState = iota
	Candidate
	Leader
	Dead
)

func (s CMState) String() string {
	switch s {
	case Follower:
		return "Follower"
	case Candidate:
		return "Candidate"
	case Leader:
		return "Leader"
	case Dead:
		return "Dead"
	default:
		panic("unreachable")
	}
}

type LogEntry struct {
	Command   Service
	Term      int
	LeaderId  int
	Index     string
	ChosenId  int
	Timestamp string
}

// ConsensusModule (CM) implements a single node of Raft consensus.
type ConsensusModule struct {
	// Mu protects concurrent access to a CM.
	Mu sync.Mutex

	// id is the server ID of this CM.
	id int

	// peerIds lists the IDs of our peers in the cluster.
	peerIds []int

	// server is the server containing this CM. It's used to issue RPC calls
	// to peers.
	server *Server

	// loadLevel is the load level of this CM
	loadLevel int

	// stopSendingAEsChan is used to stop sending AEs
	// startSendingAEsChan is used to start sending AEs
	stopSendingAEsChan chan interface{}

	// ElectionChan is used at the end of the election
	// VotingChan is used at the end of the voting phase
	ElectionChan chan interface{}
	VotingChan   chan interface{}
	CPUChan      chan interface{}

	StartTime time.Time
	// storage is used to persist state.
	storage st.Storage

	// loadLevelMap is used to store the load level of each CM
	// usually used by the leader
	loadLevelMap map[int]int

	// chosenChan signals the CM that must execute some command
	chosenChan chan interface{}

	// triggerAEChan is an internal notification channel used to trigger
	// sending new AEs to followers when interesting changes occurred.
	triggerAEChan chan struct{}

	// Persistent Raft state on all servers
	currentTerm int
	votedFor    int
	log         []LogEntry

	// Volatile Raft state on all servers
	commitIndex int
	lastApplied int
	state       CMState

	// Volatile Raft state on leaders
	nextIndex  map[int]int
	matchIndex map[int]int
}

// NewConsensusModule creates a new CM with the given ID, list of peer IDs and
// server. The ready channel signals the CM that all peers are connected and
// it's safe to start its state machine. commitChan is going to be used by the
// CM to send log entries that have been committed by the Raft cluster.
func NewConsensusModule(id int, server *Server, storage st.Storage, ready <-chan interface{}) *ConsensusModule {
	cm := new(ConsensusModule)
	cm.id = id
	cm.peerIds = []int{}
	cm.server = server
	cm.storage = storage
	cm.loadLevelMap = make(map[int]int)
	cm.ElectionChan = make(chan interface{}, 1)
	cm.VotingChan = make(chan interface{}, 1)
	cm.CPUChan = make(chan interface{}, 1)
	cm.StartTime = time.Now()
	cm.chosenChan = make(chan interface{}, 1)
	cm.triggerAEChan = make(chan struct{}, 1)
	cm.state = Follower
	cm.votedFor = -1
	cm.stopSendingAEsChan = make(chan interface{}, 1)
	cm.loadLevel = -1
	cm.commitIndex = -1
	cm.lastApplied = -1
	cm.nextIndex = make(map[int]int)
	cm.matchIndex = make(map[int]int)

	//go cm.commitChanSender()
	return cm
}

// Report reports the state of this CM.
func (cm *ConsensusModule) Report() (id int, term int, isLeader bool) {
	cm.Mu.Lock()
	cm.Dlog("Function Report acquired lock on CM")
	defer cm.Mu.Unlock()
	cm.Dlog("Function Report released lock on CM")
	return cm.id, cm.currentTerm, cm.state == Leader
}

// Voting submits a new command to the CM. This function doesn't block; clients
// read the commit channel passed in the constructor to be notified of new
// committed entries. It returns true iff this CM is the leader - in which case
// the command is accepted. If false is returned, the client will have to find
// a different CM to submit this command to.
func (cm *ConsensusModule) Voting(command *Service, index ...int) {
	cm.Mu.Lock()
	cm.Dlog("Function voting acquired lock on CM")
	cm.Dlog("Voting received: %v", command)
	if cm.state == Leader {
		chosenId := cm.minLoadLevelMap()
		if os.Getenv("TIME") == "1" {
			cm.server.Times[index[0]].SetDurationAndWrite(cm.currentTerm, "CP", cm.StartTime)
		}
		newLog := cm.NewLog(command, chosenId)
		cm.log = append(cm.log, newLog)

		cm.Mu.Unlock()
		cm.Dlog("Function voting released lock on CM")
		cm.Dlog("... log=%v", cm.log)
		cm.triggerAEChan <- struct{}{}
	} else {
		cm.Mu.Unlock()
		cm.Dlog("Function voting released lock on CM")
	}
	cm.VotingChan <- struct{}{}
}

// Stop stops this CM, cleaning up its state. This method returns quickly, but
// it may take a bit of time (up to ~election timeout) for all goroutines to
// exit.
func (cm *ConsensusModule) Stop() {
	cm.Mu.Lock()
	cm.Dlog("Function Stop acquired lock on CM")
	defer cm.Mu.Unlock()
	cm.state = Dead
	cm.Dlog("becomes Dead")
	cm.Dlog("Funtion Stop released lock on CM")
}

type DeployArgs struct {
	Id      string
	Service []byte
}

type DeployReply struct{}

func (cm *ConsensusModule) Deploy(args DeployArgs, reply *DeployReply) error {
	if err := os.WriteFile("services/"+args.Id, args.Service, 0644); err != nil {
		return err
	}
	go Exec(args.Id)
	return nil
}

// persistToStorage saves all of CM's persistent state in cm.storage.
// Expects cm.Mu to be locked.

func (cm *ConsensusModule) persistToStorage(logs []LogEntry, index ...int) {

	if os.Getenv("TIME") == "1" && index != nil {
		cm.server.Times[index[0]].SetStartTime("WL")
	}

	for _, log := range logs {
		termData := make(map[string]interface{})
		termData["Term"] = strconv.Itoa(log.Term)
		termData["Command"] = log.Command
		termData["Leader"] = strconv.Itoa(log.LeaderId)
		termData["Chosen"] = strconv.Itoa(log.ChosenId)
		termData["Id"] = log.Index
		termData["Timestamp"] = log.Timestamp

		cm.storage.Set(termData, cm.CheckCMId(log.LeaderId))
		if os.Getenv("TIME") == "1" && cm.CheckCMId(log.LeaderId) && index != nil {
			cm.server.Times[index[0]].SetDurationAndWrite(cm.currentTerm, "WL", cm.StartTime)
		}

		if log.Term >= cm.currentTerm {
			leaderId := log.LeaderId
			chosenId := log.ChosenId
			isLeader, isChosen := cm.CheckCMId(leaderId), cm.CheckCMId(chosenId)
			if isLeader {
				if isChosen {
					fmt.Println("Leader execution")
					go Exec(termData["Command"].(Service).ServiceID)
					if os.Getenv("TIME") == "1" && index != nil {
						cm.server.Times[index[0]].SetDurationAndWrite(cm.currentTerm, "TRE", cm.StartTime)
					}
				} else {
					file, _ := os.ReadFile("services/" + termData["Command"].(Service).ServiceID)
					args := DeployArgs{
						Id:      termData["Command"].(Service).ServiceID,
						Service: file,
					}
					var reply DeployReply
					if os.Getenv("TIME") == "1" && index != nil {
						cm.server.Times[index[0]].SetStartTime("TR")
					}
					if err := cm.server.Call(chosenId, "ConsensusModule.Deploy", args, &reply); err == nil && os.Getenv("TIME") == "1" && index != nil {
						cm.server.Times[index[0]].SetDurationAndWrite(cm.currentTerm, "TR", cm.StartTime)
					}
				}
				//cm.server.deleteDeployedService()
				cm.server.SubmitChan <- struct{}{}
			}
		}
	}
}

// Dlog logs a debugging message is DebugCM > 0.
func (cm *ConsensusModule) Dlog(format string, args ...interface{}) {
	if DebugCM != "0" {
		format = fmt.Sprintf("[%d] ", cm.id) + format
		log.Printf(format, args...)
	}
}

type RequestVoteArgs struct {
	Term         int
	CandidateId  int
	LastLogIndex int
	LastLogTerm  int
	LoadLevel    int
}

type RequestVoteReply struct {
	Term         int
	VoteGranted  bool
	LoadLevel    int
	VoteElabTime time.Duration
}

// RequestVote RPC.
func (cm *ConsensusModule) RequestVote(args RequestVoteArgs, reply *RequestVoteReply) error {
	cm.Mu.Lock()
	cm.Dlog("Function RequestVote acquired lock on CM")
	voteTime := time.Now()
	defer cm.Mu.Unlock()
	if cm.state == Dead {
		return nil
	}
	lastLogIndex, lastLogTerm := cm.lastLogIndexAndTerm()
	cm.Dlog("RequestVote: %+v [currentTerm=%d, votedFor=%d, log index/term=(%d, %d)]", args, cm.currentTerm, cm.votedFor, lastLogIndex, lastLogTerm)

	if args.Term > cm.currentTerm {
		cm.Dlog("... term out of date in RequestVote")
		cm.becomeFollower(args.Term)
	}

	cm.Mu.Unlock()
	cm.Dlog("Function RequestVote released lock on CM")
	if cm.state != Candidate {
		runVoteDelay(args.LoadLevel)
	}
	cm.Mu.Lock()
	cm.Dlog("Function RequestVote acquired lock on CM")
	if cm.currentTerm == args.Term &&
		(cm.votedFor == -1 || cm.votedFor == args.CandidateId) &&
		(args.LastLogTerm > lastLogTerm ||
			(args.LastLogTerm == lastLogTerm && args.LastLogIndex >= lastLogIndex)) {
		cm.Dlog("waited for vote delay of %v", time.Duration(1000/args.LoadLevel)*time.Millisecond)
		reply.VoteGranted = true
		reply.LoadLevel = cm.loadLevel
		cm.votedFor = args.CandidateId
	} else {
		reply.VoteGranted = false
	}
	reply.Term = cm.currentTerm
	reply.VoteElabTime = time.Since(voteTime)
	cm.Dlog("... RequestVote reply: %+v", reply)
	cm.Dlog("Function RequestVote released lock on CM")
	return nil
}

type AppendEntriesArgs struct {
	Term     int
	LeaderId int

	PrevLogIndex int
	PrevLogTerm  int
	Entries      []LogEntry
	LeaderCommit int
	ChosenId     int
}

type AppendEntriesReply struct {
	Term    int
	Success bool

	ConflictIndex int
	ConflictTerm  int

	VoteElabTime time.Duration
}

func (cm *ConsensusModule) AppendEntries(args AppendEntriesArgs, reply *AppendEntriesReply) error {
	cm.Mu.Lock()
	cm.Dlog("Function AppendEntries acquired lock on CM - beginnig")
	defer cm.Mu.Unlock()
	voteElabTime := time.Now()
	if cm.state == Dead {
		return nil
	}
	cm.Dlog("AppendEntries: %+v", args)

	if args.Term > cm.currentTerm {
		cm.Dlog("... term out of date in AppendEntries")
		cm.becomeFollower(args.Term)
	}

	reply.Success = false
	if args.Term == cm.currentTerm {
		if cm.state != Follower {
			cm.becomeFollower(args.Term)
		}

		// Does our log contain an entry at PrevLogIndex whose term matches
		// PrevLogTerm? Note that in the extreme case of PrevLogIndex=-1 this is
		// vacuously true.
		if args.PrevLogIndex == -1 ||
			(args.PrevLogIndex < len(cm.log) && args.PrevLogTerm == cm.log[args.PrevLogIndex].Term) {
			reply.Success = true

			// Find an insertion point - where there's a term mismatch between
			// the existing log starting at PrevLogIndex+1 and the new entries sent
			// in the RPC.
			logInsertIndex := args.PrevLogIndex + 1
			newEntriesIndex := 0

			for {
				if logInsertIndex >= len(cm.log) || newEntriesIndex >= len(args.Entries) {
					break
				}
				if cm.log[logInsertIndex].Term != args.Entries[newEntriesIndex].Term {
					break
				}
				logInsertIndex++
				newEntriesIndex++
			}
			// At the end of this loop:
			// - logInsertIndex points at the end of the log, or an index where the
			//   term mismatches with an entry from the leader
			// - newEntriesIndex points at the end of Entries, or an index where the
			//   term mismatches with the corresponding log entry
			if newEntriesIndex < len(args.Entries) {
				cm.Dlog("... inserting entries %v from index %d", args.Entries[newEntriesIndex:], logInsertIndex)
				cm.log = append(cm.log[:logInsertIndex], args.Entries[newEntriesIndex:]...)
				cm.persistToStorage(cm.log[logInsertIndex:])
				cm.Dlog("... log is now: %v", cm.log)
			}

			// Set commit index.
			if args.LeaderCommit > cm.commitIndex {
				cm.commitIndex = intMin(args.LeaderCommit, len(cm.log)-1)
				cm.Dlog("... setting commitIndex=%d", cm.commitIndex)
				//cm.Mu.Unlock()
				//cm.Dlog("Function AppendEntries released lock on CM - before commit ready chan")
				//cm.newCommitReadyChan <- struct{}{}
				//cm.Dlog("Function AppendEntries has put a structure inside commit ready chan")
				//<-cm.commitSendDoneChan
				//cm.Dlog("Function AppendEntries is going to acquire lock on CM - after commit ready chan")
				//cm.Mu.Lock()
				//cm.Dlog("Function AppendEntries acquired lock on CM - after commit ready chan")
			}
		} else {
			// No match for PrevLogIndex/PrevLogTerm. Populate
			// ConflictIndex/ConflictTerm to help the leader bring us up to date
			// quickly.
			if args.PrevLogIndex >= len(cm.log) {
				reply.ConflictIndex = len(cm.log)
				reply.ConflictTerm = -1
			} else {
				// PrevLogIndex points within our log, but PrevLogTerm doesn't match
				// cm.log[PrevLogIndex].
				reply.ConflictTerm = cm.log[args.PrevLogIndex].Term

				var i int
				for i = args.PrevLogIndex - 1; i >= 0; i-- {
					if cm.log[i].Term != reply.ConflictTerm {
						break
					}
				}
				reply.ConflictIndex = i + 1
			}
		}
	}

	reply.Term = cm.currentTerm
	reply.VoteElabTime = time.Since(voteElabTime)
	cm.Dlog("AppendEntries reply: %+v", *reply)
	cm.Dlog("Function AppendEntries released lock on CM - end")
	return nil
}

func runVoteDelay(loadLevel int) {
	delay := time.Duration(100/loadLevel) * time.Millisecond
	time.Sleep(delay)
}

// startElection starts a new election with this CM as a candidate.
// Expects cm.Mu to be locked.
func (cm *ConsensusModule) Election(index ...int) {
	cm.Mu.Lock()
	cm.Dlog("Function Election acquired lock on CM")
	defer cm.Mu.Unlock()
	cm.state = Candidate
	cm.currentTerm += 1
	savedCurrentTerm := cm.currentTerm
	cm.votedFor = cm.id
	cm.Dlog("becomes Candidate (currentTerm=%d); log=%v; loadLevel=%v", savedCurrentTerm, cm.log, cm.loadLevel)
	votesReceived := 1

	// Send RequestVote RPCs to all other servers concurrently.
	cm.loadLevelMap[cm.id] = cm.loadLevel
	for t, peerId := range cm.peerIds {
		go func(peerId int, t int) {
			cm.Mu.Lock()
			cm.Dlog("Function inside Election acquired lock on CM")
			savedLastLogIndex, savedLastLogTerm := cm.lastLogIndexAndTerm()
			cm.Mu.Unlock()
			cm.Dlog("Function inside Election released lock on CM")

			args := RequestVoteArgs{
				Term:         savedCurrentTerm,
				CandidateId:  cm.id,
				LastLogIndex: savedLastLogIndex,
				LastLogTerm:  savedLastLogTerm,
				LoadLevel:    cm.loadLevel,
			}

			cm.Dlog("sending RequestVote to %d: %+v", peerId, args)
			var reply RequestVoteReply
			if os.Getenv("TIME") == "1" {
				cm.server.Times[index[0]].SetStartTime("EN1", t)
			}
			if err := cm.server.Call(peerId, "ConsensusModule.RequestVote", args, &reply); err == nil {
				cm.Mu.Lock()
				cm.Dlog("Function inside Election acquired lock on CM")
				if os.Getenv("TIME") == "1" {
					cm.server.Times[index[0]].Mu.Lock()
					cm.server.Times[index[0]].ElectionNetworkDurations[t] = time.Since(cm.server.Times[index[0]].ElectionNetworkStartTimes[t])
					cm.server.Times[index[0]].VoteElectionDurations[t] = reply.VoteElabTime
					cm.server.Times[index[0]].Mu.Unlock()
				}
				cm.loadLevelMap[peerId] = reply.LoadLevel
				defer cm.Mu.Unlock()
				cm.Dlog("received RequestVoteReply %+v", reply)

				if cm.state != Candidate {
					cm.Dlog("while waiting for reply, state = %v", cm.state)
					return
				}

				if reply.Term > savedCurrentTerm {
					cm.Dlog("term out of date in RequestVoteReply")
					cm.becomeFollower(reply.Term)
					return
				} else if reply.Term == savedCurrentTerm {
					if reply.VoteGranted {
						votesReceived += 1
						if votesReceived*2 > len(cm.peerIds) /*+1*/ {
							// +1 is canceled because it should be the server itself, but
							// I must subtract 1 because the default gateway is included
							// and it is not a server

							// Won the election!
							cm.Dlog("wins election with %d votes", votesReceived)
							if os.Getenv("TIME") == "1" {
								cm.startLeader(index[0])
							} else {
								cm.startLeader()
							}
							return
						}
					}
				}
			}
			cm.Dlog("Function inside Election released lock on CM")
		}(peerId, t)
	}

}

// becomeFollower makes cm a follower and resets its state.
// Expects cm.Mu to be locked.
func (cm *ConsensusModule) becomeFollower(term int) {
	cm.Dlog("becomes Follower with term=%d; log=%v", term, cm.log)
	cm.state = Follower
	cm.currentTerm = term
	cm.votedFor = -1
}

// startLeader switches cm into a leader state and begins process of heartbeats.
// Expects cm.Mu to be locked.
func (cm *ConsensusModule) startLeader(index ...int) {
	cm.state = Leader
	cm.ElectionChan <- struct{}{}
	for _, peerId := range cm.peerIds {
		cm.nextIndex[peerId] = len(cm.log)
		cm.matchIndex[peerId] = -1
	}
	cm.Dlog("becomes Leader; term=%d, nextIndex=%v, matchIndex=%v; log=%v", cm.currentTerm, cm.nextIndex, cm.matchIndex, cm.log)

	// This goroutine runs in the background and sends AEs to peers
	// Whenever something is sent on triggerAEChan
	var tmp []int = nil
	if os.Getenv("TIME") == "1" {
		tmp = index
	}
	go func(index []int) {
		for {
			select {
			case <-cm.stopSendingAEsChan:
				return
			case <-cm.triggerAEChan:
				cm.Mu.Lock()
				cm.Dlog("Function startLeader acquired lock on CM")
				if cm.state != Leader {
					cm.Mu.Unlock()
					cm.Dlog("Function startLeader released lock on CM")
					return
				}
				cm.Mu.Unlock()
				cm.Dlog("Function startLeader released lock on CM")
				if os.Getenv("TIME") == "1" {
					cm.leaderSendAEs(index[0])
				} else {
					cm.leaderSendAEs()
				}
				/* cm.leaderSendAEs() */
			}
		}
	}(tmp)
}

// leaderSendAEs sends a round of AEs to all peers, collects their
// replies and adjusts cm's state.
func (cm *ConsensusModule) leaderSendAEs(index ...int) {
	cm.Mu.Lock()
	cm.Dlog("Function leaderSendAEs acquired lock on CM")
	if cm.state != Leader {
		cm.Mu.Unlock()
		cm.Dlog("Function leaderSendAEs released lock on CM")
		return
	}
	savedCurrentTerm := cm.currentTerm
	cm.Mu.Unlock()
	cm.Dlog("Function leaderSendAEs released lock on CM")
	for t, peerId := range cm.peerIds {
		go func(peerId int, t int) {
			cm.Mu.Lock()
			cm.Dlog("Function inside leaderSendAEs acquired lock on CM - 1")
			ni := cm.nextIndex[peerId]
			prevLogIndex := ni - 1
			prevLogTerm := -1
			if prevLogIndex >= 0 {
				prevLogTerm = cm.log[prevLogIndex].Term
			}
			entries := cm.log[ni:]
			chosenId := -1
			if len(entries) > 0 {
				chosenId = entries[0].ChosenId
			}

			args := AppendEntriesArgs{
				Term:         savedCurrentTerm,
				LeaderId:     cm.id,
				PrevLogIndex: prevLogIndex,
				PrevLogTerm:  prevLogTerm,
				Entries:      entries,
				LeaderCommit: cm.commitIndex,
				ChosenId:     chosenId,
			}
			cm.Mu.Unlock()
			cm.Dlog("Function inside leaderSendAEs released lock on CM - 1")
			cm.Dlog("sending AppendEntries to %v: ni=%d, args=%+v", peerId, ni, args)
			var reply AppendEntriesReply
			if os.Getenv("TIME") == "1" && index != nil {
				cm.server.Times[index[0]].SetStartTime("VCN1", t)
			}
			if err := cm.server.Call(peerId, "ConsensusModule.AppendEntries", args, &reply); err == nil {
				cm.Mu.Lock()
				cm.Dlog("Function inside leaderSendAEs acquired lock on CM -2")
				if os.Getenv("TIME") == "1" && index != nil {
					cm.server.Times[index[0]].Mu.Lock()
					cm.server.Times[index[0]].VoteConsNetDurations[t] = time.Since(cm.server.Times[index[0]].VoteConsNetStartTimes[t])
					cm.server.Times[index[0]].VoteConsElabDurations[t] = reply.VoteElabTime
					cm.server.Times[index[0]].Mu.Unlock()
				}
				if reply.Term > cm.currentTerm {
					cm.Dlog("term out of date in heartbeat reply")
					cm.becomeFollower(reply.Term)
					cm.Mu.Unlock()
					cm.Dlog("Function inside leaderSendAEs released lock on CM - 2")
					return
				}

				if cm.state == Leader && savedCurrentTerm == reply.Term {
					if reply.Success {
						cm.nextIndex[peerId] = ni + len(entries)
						cm.matchIndex[peerId] = cm.nextIndex[peerId] - 1

						savedCommitIndex := cm.commitIndex
						for i := cm.commitIndex + 1; i < len(cm.log); i++ {
							if cm.log[i].Term == cm.currentTerm {
								matchCount := 1
								for _, peerId := range cm.peerIds {
									if cm.matchIndex[peerId] >= i {
										matchCount++
									}
								}
								if matchCount*2 > len(cm.peerIds)+1 {
									cm.commitIndex = i
								}
							}
						}
						cm.Dlog("AppendEntries reply from %d success: nextIndex := %v, matchIndex := %v; commitIndex := %d", peerId, cm.nextIndex, cm.matchIndex, cm.commitIndex)
						if cm.commitIndex != savedCommitIndex {
							cm.Dlog("leader sets commitIndex := %d", cm.commitIndex)
							// Commit index changed: the leader considers new entries to be
							// committed. Send new entries on the commit channel to this
							// leader's clients, and notify followers by sending them AEs.
							cm.Mu.Unlock()
							cm.Dlog("Function inside leaderSendAEs released lock on CM - 3")
							if os.Getenv("TIME") == "1" && index != nil {
								cm.server.Times[index[0]].SetDurationAndWrite(cm.currentTerm, "VCNVE", cm.StartTime)
								cm.persistToStorage(cm.log[savedCommitIndex+1:cm.commitIndex+1], index[0])
							} else {
								cm.persistToStorage(cm.log[savedCommitIndex+1 : cm.commitIndex+1])
							}
							//cm.persistToStorage(cm.log[savedCommitIndex+1 : cm.commitIndex+1])
							//cm.newCommitReadyChan <- struct{}{}
							cm.triggerAEChan <- struct{}{}
						} else {
							cm.Mu.Unlock()
							cm.Dlog("Function inside leaderSendAEs released lock on CM -3")
						}
					} else {
						if reply.ConflictTerm >= 0 {
							lastIndexOfTerm := -1
							for i := len(cm.log) - 1; i >= 0; i-- {
								if cm.log[i].Term == reply.ConflictTerm {
									lastIndexOfTerm = i
									break
								}
							}
							if lastIndexOfTerm >= 0 {
								cm.nextIndex[peerId] = lastIndexOfTerm + 1
							} else {
								cm.nextIndex[peerId] = reply.ConflictIndex
							}
						} else {
							cm.nextIndex[peerId] = reply.ConflictIndex
						}
						cm.Dlog("AppendEntries reply from %d !success: nextIndex := %d", peerId, ni-1)
						cm.Mu.Unlock()
						cm.Dlog("Function inside leaderSendAEs released lock on CM - 4")
					}
				} else {
					cm.Mu.Unlock()
					cm.Dlog("Function inside leaderSendAEs released lock on CM - 4")
				}
			}
		}(peerId, t)
	}
}

// lastLogIndexAndTerm returns the last log index and the last log entry's term
// (or -1 if there's no log) for this server.
// Expects cm.Mu to be locked.
func (cm *ConsensusModule) lastLogIndexAndTerm() (int, int) {
	if len(cm.log) > 0 {
		lastIndex := len(cm.log) - 1
		return lastIndex, cm.log[lastIndex].Term
	} else {
		return -1, -1
	}
}

func intMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (cm *ConsensusModule) Pause() {
	cm.Mu.Lock()
	cm.Dlog("Function Pause acquired lock on CM")
	cm.stopSendingAEsChan <- struct{}{}
	cm.Mu.Unlock()
	cm.Dlog("Function Pause released lock on CM")
}

func (cm *ConsensusModule) MonitorLoad() {
	var cpu float64
	var mem float64
	var load int
	for {
		cm.Mu.Lock()
		//cm.Dlog("Function MonitorLoad acquired lock on CM")
		load, cpu, mem = l.GetLoadLevel()
		cm.Mu.Unlock()
		//cm.Dlog("Function MonitorLoad released lock on CM")
		select {
		case <-cm.CPUChan:
			go cm.MonitorForTest(&cpu, &mem)
		default:
			cm.Mu.Lock()
			//cm.Dlog("Function MonitorLoad acquired lock on CM")
			cm.loadLevel = load
			cm.Mu.Unlock()
			//cm.Dlog("Function MonitorLoad released lock on CM")
			time.Sleep(20 * time.Millisecond)
		}
	}
}

func (cm *ConsensusModule) MonitorForTest(cpu *float64, mem *float64) {
	timer := time.NewTimer(8 * time.Millisecond)
	folderPath := "/test/results/resources/"
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		os.MkdirAll(folderPath, os.ModePerm)
	}
	f, err := os.OpenFile(folderPath+strconv.Itoa(cm.id)+".csv", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	f.WriteString("Times,Cpu,Mem\n")
	for {
		<-timer.C
		timer.Reset(8 * time.Millisecond)
		//when := time.Since(cm.StartTime)
		when := time.Now().String()
		cm.Mu.Lock()
		//cm.Dlog("Function MonitorForTest acquired lock on CM")
		f.WriteString(fmt.Sprintf("%v,%.2f,%.2f\n", when, *cpu, *mem))
		cm.Mu.Unlock()
		//cm.Dlog("Function MonitorForTest released lock on CM")
	}
}

func (cm *ConsensusModule) DisconnectPeer(peerId int) {
	cm.Mu.Lock()
	for i, peer := range cm.peerIds {
		if peer == peerId {
			cm.peerIds = append(cm.peerIds[:i], cm.peerIds[i+1:]...)
			break
		}
	}
	cm.Mu.Unlock()
}

func (cm *ConsensusModule) ConnectPeer(peerId int) {
	cm.Mu.Lock()
	cm.Dlog("Function ConnectPeer acquired lock on CM")
	cm.peerIds = append(cm.peerIds, peerId)
	cm.Mu.Unlock()
	cm.Dlog("Function ConnectPeer released lock on CM")
}

func (cm *ConsensusModule) CheckCMId(peerId int) bool {
	return cm.id == peerId
}

func (cm *ConsensusModule) minLoadLevelMap() int {
	lowestPeers := make([]int, 0)
	lastPeer := 0
	lowestLoad := 11

	for peerId, loadLevel := range cm.loadLevelMap {
		_, ok := cm.loadLevelMap[lastPeer]
		if !ok || loadLevel < lowestLoad {
			lowestLoad = loadLevel
			lowestPeers = []int{peerId}
		} else if loadLevel == lowestLoad {
			lowestPeers = append(lowestPeers, peerId)
		}
		lastPeer = peerId
	}
	return lowestPeers[rand.Intn(len(lowestPeers))]
}

func (cm *ConsensusModule) NewLog(command *Service, chosenId int) (log LogEntry) {
	newLog := LogEntry{
		Command:   *command,
		Term:      cm.currentTerm,
		LeaderId:  cm.id,
		ChosenId:  chosenId,
		Index:     "",
		Timestamp: time.Now().Local().Format("2006-01-02 15:04:05.0000"),
	}
	values := reflect.ValueOf(newLog)
	sum := []byte{}
	for i := 0; i < values.NumField(); i++ {
		sum = append(sum, []byte(fmt.Sprintf("%v", values.Field(i).Interface()))...)
	}
	newLog.Index = fmt.Sprintf("%x", sha256.Sum256(sum))
	return newLog

}

func Exec(service string) {
	exec.Command("docker-compose", "-f", "/home/raft/services/"+service, "up", "-d").Start()
	fmt.Printf("Eseguito %s\n", service)
}
