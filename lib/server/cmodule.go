// Core Raft implementation - Consensus Module.
//
// Eli Bendersky [https://eli.thegreenplace.net]
// This code is in the public domain.
package server

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"strconv"
	"log"
	l "server/load"
	st "storage"
	"crypto/sha512"
	"sync"
	"time"
)

const DebugCM = 1

// CommitEntry is the data reported by Raft to the commit channel. Each commit
// entry notifies the client that consensus was reached on a command and it can
// be applied to the client's state machine.
type CommitEntry struct {
	// Command is the client command being committed.
	Command interface{}

	// Index is the log index at which the client command is committed.
	Index int

	// Term is the Raft term at which the client command is committed.
	Term int
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
	Command interface{}
	Term    int
	LeaderId int
	Index int
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
	startSendingAEsChan chan interface{}
	
	// ResumeChan is used to resume the election
	// SubmitChan is used to submit the command
	ResumeChan chan interface{}
	SubmitChan chan interface{}

	// storage is used to persist state.
	storage st.Storage

	// commitChan is the channel where this CM is going to report committed log
	// entries. It's passed in by the client during construction.
	commitChan chan<- CommitEntry

	// newCommitReadyChan is an internal notification channel used by goroutines
	// that commit new entries to the log to notify that these entries may be sent
	// on commitChan.
	newCommitReadyChan chan struct{}

	// triggerAEChan is an internal notification channel used to trigger
	// sending new AEs to followers when interesting changes occurred.
	triggerAEChan chan struct{}

	// Persistent Raft state on all servers
	currentTerm int
	votedFor    int
	log         []LogEntry

	// Volatile Raft state on all servers
	commitIndex        int
	lastApplied        int
	state              CMState
	electionResetEvent time.Time

	// Volatile Raft state on leaders
	nextIndex  map[int]int
	matchIndex map[int]int
}

// NewConsensusModule creates a new CM with the given ID, list of peer IDs and
// server. The ready channel signals the CM that all peers are connected and
// it's safe to start its state machine. commitChan is going to be used by the
// CM to send log entries that have been committed by the Raft cluster.
func NewConsensusModule(id int, peerIds []int, server *Server, storage st.Storage, ready <-chan interface{}, commitChan chan<- CommitEntry) *ConsensusModule {
	cm := new(ConsensusModule)
	cm.id = id
	cm.peerIds = peerIds
	cm.server = server
	cm.storage = storage
	cm.commitChan = commitChan
	cm.ResumeChan = make(chan interface{}, 2)
	cm.SubmitChan = make(chan interface{}, 1)
	cm.newCommitReadyChan = make(chan struct{})
	cm.triggerAEChan = make(chan struct{}, 1)
	cm.state = Follower
	cm.votedFor = -1
	cm.stopSendingAEsChan = make(chan interface{}, 1)
	cm.startSendingAEsChan = make(chan interface{}, 1)
	cm.loadLevel = 10
	cm.commitIndex = -1
	cm.lastApplied = -1
	cm.nextIndex = make(map[int]int)
	cm.matchIndex = make(map[int]int)

	if cm.storage.HasData() {
		cm.restoreFromStorage()
	}

	//go func() {
	//	// The CM is dormant until ready is signaled; then, it starts a countdown
	//	// for leader election.
	//	<-ready
	//	cm.Mu.Lock()
	//	cm.electionResetEvent = time.Now()
	//	cm.Mu.Unlock()	
	//	//if cm.id > 5 {
	//	//	cm.startElection()
	//	//} else {
	//	//	cm.runElectionTimer()
	//	//}
	//}()
	
	go cm.monitorLoad()
	go cm.commitChanSender()
	return cm
}

// Report reports the state of this CM.
func (cm *ConsensusModule) Report() (id int, term int, isLeader bool) {
	cm.Mu.Lock()
	defer cm.Mu.Unlock()
	return cm.id, cm.currentTerm, cm.state == Leader
}

// Submit submits a new command to the CM. This function doesn't block; clients
// read the commit channel passed in the constructor to be notified of new
// committed entries. It returns true iff this CM is the leader - in which case
// the command is accepted. If false is returned, the client will have to find
// a different CM to submit this command to.
func (cm *ConsensusModule) Submit(command interface{}) {
	cm.Mu.Lock()
	cm.Dlog("Submit received: %v", command)
	if cm.state == Leader {
		cm.log = append(cm.log, LogEntry{Command: command, Term: cm.currentTerm, LeaderId: cm.id, Index: len(cm.log)})
		cm.persistToStorage()
		cm.Dlog("... log=%v", cm.log)
		cm.Mu.Unlock()
		cm.triggerAEChan <- struct{}{}
		} else {
			cm.Mu.Unlock()
		}
	cm.SubmitChan <- struct{}{}
}

// Stop stops this CM, cleaning up its state. This method returns quickly, but
// it may take a bit of time (up to ~election timeout) for all goroutines to
// exit.
func (cm *ConsensusModule) Stop() {
	cm.Mu.Lock()
	defer cm.Mu.Unlock()
	cm.state = Dead
	cm.Dlog("becomes Dead")
	close(cm.newCommitReadyChan)
}

// restoreFromStorage restores the persistent state of this CM from storage.
// It should be called during constructor, before any concurrency concerns.
func (cm *ConsensusModule) restoreFromStorage() {
	if termData, found := cm.storage.Get("currentTerm"); found {
		d := gob.NewDecoder(bytes.NewBuffer([]byte(termData)))
		if err := d.Decode(&cm.currentTerm); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Fatal("currentTerm not found in storage")
	}
	if votedData, found := cm.storage.Get("votedFor"); found {
		d := gob.NewDecoder(bytes.NewBuffer([]byte(votedData)))
		if err := d.Decode(&cm.votedFor); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Fatal("votedFor not found in storage")
	}
	if logData, found := cm.storage.Get("log"); found {
		d := gob.NewDecoder(bytes.NewBuffer([]byte(logData)))
		if err := d.Decode(&cm.log); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Fatal("log not found in storage")
	}
}

// persistToStorage saves all of CM's persistent state in cm.storage.
// Expects cm.Mu to be locked.
//func (cm *ConsensusModule) persistToStorage() {
	//var termData bytes.Buffer
	//if err := gob.NewEncoder(&termData).Encode(cm.currentTerm); err != nil {
		//log.Fatal(err)
	//}
	//cm.storage.Set("currentTerm", termData.Bytes())

	//var votedData bytes.Buffer
	//if err := gob.NewEncoder(&votedData).Encode(cm.votedFor); err != nil {
		//log.Fatal(err)
	//}
	//cm.storage.Set("votedFor", votedData.Bytes())

	//var logData bytes.Buffer
	//if err := gob.NewEncoder(&logData).Encode(cm.log); err != nil {
		//log.Fatal(err)
	//}
	//cm.storage.Set("log", logData.Bytes())
//}

func (cm *ConsensusModule) persistToStorage() {
	termData := make(map[string]string)
	last := 0
	sum := []byte{}

	if (len(cm.log)-1 < 0) {
		last = 0
	} else {
		last = len(cm.log)-1
	}

	termData["Id"] = fmt.Sprintf("%x", last)
	termData["Term"] = strconv.Itoa(cm.currentTerm)
	termData["Leader"] = strconv.Itoa(cm.votedFor)
	termData["Command"] = fmt.Sprintf("%s", cm.log[last].Command)
	termData["Leader"] = strconv.Itoa(cm.log[last].LeaderId)
	for _, v := range termData {
		sum = append(sum, []byte(v)...)
	}

	termData["checksum"] = fmt.Sprintf("%x", sha512.Sum512(sum))

	cm.storage.Set(termData)

}

// Dlog logs a debugging message is DebugCM > 0.
func (cm *ConsensusModule) Dlog(format string, args ...interface{}) {
	if DebugCM > 0 {
		format = fmt.Sprintf("[%d] ", cm.id) + format
		log.Printf(format, args...)
	}
}

// See figure 2 in the paper.
type RequestVoteArgs struct {
	Term         int
	CandidateId  int
	LastLogIndex int
	LastLogTerm  int
	LoadLevel    int
}

type RequestVoteReply struct {
	Term        int
	VoteGranted bool
	LoadLevel   int
}

// RequestVote RPC.
func (cm *ConsensusModule) RequestVote(args RequestVoteArgs, reply *RequestVoteReply) error {
	cm.Mu.Lock()
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
	if cm.state != Candidate {
		runVoteDelay(args.LoadLevel)
	}
	cm.Mu.Lock()
	if cm.currentTerm == args.Term &&
		(cm.votedFor == -1 || cm.votedFor == args.CandidateId) &&
		(args.LastLogTerm > lastLogTerm ||
			(args.LastLogTerm == lastLogTerm && args.LastLogIndex >= lastLogIndex)) {
		cm.Dlog("waited for vote delay of %v", time.Duration(1000/args.LoadLevel)*time.Millisecond)
		reply.VoteGranted = true
		reply.LoadLevel = cm.loadLevel
		cm.votedFor = args.CandidateId
		cm.electionResetEvent = time.Now()
	} else {
		reply.VoteGranted = false
	}
	reply.Term = cm.currentTerm
	//cm.persistToStorage()
	cm.Dlog("... RequestVote reply: %+v", reply)
	return nil
}

// See figure 2 in the paper.
type AppendEntriesArgs struct {
	Term     int
	LeaderId int

	PrevLogIndex int
	PrevLogTerm  int
	Entries      []LogEntry
	LeaderCommit int
}

type AppendEntriesReply struct {
	Term    int
	Success bool

	// Faster conflict resolution optimization (described near the end of section
	// 5.3 in the paper.)
	ConflictIndex int
	ConflictTerm  int
}

func (cm *ConsensusModule) AppendEntries(args AppendEntriesArgs, reply *AppendEntriesReply) error {
	cm.Mu.Lock()
	defer cm.Mu.Unlock()
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
		cm.electionResetEvent = time.Now()

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
				cm.Dlog("... log is now: %v", cm.log)
			}

			// Set commit index.
			if args.LeaderCommit > cm.commitIndex {
				cm.commitIndex = intMin(args.LeaderCommit, len(cm.log)-1)
				cm.Dlog("... setting commitIndex=%d", cm.commitIndex)
				cm.Mu.Unlock()
				cm.newCommitReadyChan <- struct{}{}
				cm.Mu.Lock()
				//cm.stopElectionChan = true
			}
			cm.persistToStorage()
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
	cm.Dlog("AppendEntries reply: %+v", *reply)
	return nil
}

// electionTimeout generates a pseudo-random election timeout duration.
//func (cm *ConsensusModule) electionTimeout() time.Duration {
//	// If RAFT_FORCE_MORE_REELECTION is set, stress-test by deliberately
//	// generating a hard-coded number very often. This will create collisions
//	// between different servers and force more re-elections.
//	if len(os.Getenv("RAFT_FORCE_MORE_REELECTION")) > 0 && rand.Intn(3) == 0 {
//		return time.Duration(150 * len(cm.peerIds)) * time.Millisecond
//	} else {
//		return time.Duration(150 * len(cm.peerIds) +rand.Intn(150)) * time.Millisecond
//	}
//}

// runElectionTimer implements an election timer. It should be launched whenever
// we want to start a timer towards becoming a candidate in a new election.
//
// This function is blocking and should be launched in a separate goroutine;
// it's designed to work for a single (one-shot) election timer, as it exits
// whenever the CM state changes from follower/candidate or the term changes.
//func (cm *ConsensusModule) runElectionTimer() {
//	timeoutDuration := cm.electionTimeout()
//	cm.Mu.Lock()
//	termStarted := cm.currentTerm
//    if cm.stopElectionChan {
//		cm.Mu.Unlock()
//        return
//    }
//	cm.Mu.Unlock()
//	//cm.Dlog("election timer started (%v), term=%d", timeoutDuration, termStarted)
//	// This loops until either:
//	// - we discover the election timer is no longer needed, or
//	// - the election timer expires and this CM becomes a candidate
//	// In a follower, this typically keeps running in the background for the
//	// duration of the CM's lifetime.
//	ticker := time.NewTicker(10 * time.Millisecond)
//	defer ticker.Stop()
//	for {
//		<-ticker.C
//
//		cm.Mu.Lock()
//		if cm.state != Candidate && cm.state != Follower {
//			cm.Dlog("in election timer state=%s, bailing out", cm.state)
//			cm.Mu.Unlock()
//			return
//		}
//		
//		if termStarted != cm.currentTerm {
//			cm.Dlog("in election timer term changed from %d to %d, bailing out", termStarted, cm.currentTerm)
//			cm.Mu.Unlock()
//			return
//		}
//		
//		// Start an election if we haven't heard from a leader or haven't voted for
//		// someone for the duration of the timeout.
//		if elapsed := time.Since(cm.electionResetEvent); elapsed >= timeoutDuration {
//			cm.StartElection()
//			cm.Mu.Unlock()
//			return
//		}
//		cm.Mu.Unlock()
//	}
//}

func runVoteDelay(loadLevel int) {
	delay := time.Duration(100/loadLevel) * time.Millisecond
	//fmt.Printf("delay: %d\n", time.Duration((1 / float64(loadLevel)) * float64(time.Millisecond)))
	time.Sleep(delay)
}

// startElection starts a new election with this CM as a candidate.
// Expects cm.Mu to be locked.
func (cm *ConsensusModule) StartElection() {
	cm.state = Candidate
	cm.currentTerm += 1
	savedCurrentTerm := cm.currentTerm
	cm.electionResetEvent = time.Now()
	cm.votedFor = cm.id
	cm.Dlog("becomes Candidate (currentTerm=%d); log=%v; loadLevel=%v", savedCurrentTerm, cm.log, cm.loadLevel)

	votesReceived := 1

	// Send RequestVote RPCs to all other servers concurrently.
	fmt.Printf("peers: %v", cm.peerIds)
	for _, peerId := range cm.peerIds {
		go func(peerId int) {
			cm.Mu.Lock()
			savedLastLogIndex, savedLastLogTerm := cm.lastLogIndexAndTerm()
			cm.Mu.Unlock()

			args := RequestVoteArgs{
				Term:         savedCurrentTerm,
				CandidateId:  cm.id,
				LastLogIndex: savedLastLogIndex,
				LastLogTerm:  savedLastLogTerm,
				LoadLevel:    cm.loadLevel,
			}

			cm.Dlog("sending RequestVote to %d: %+v", peerId, args)
			var reply RequestVoteReply
			if err := cm.server.Call(peerId, "ConsensusModule.RequestVote", args, &reply); err == nil {
				cm.Mu.Lock()
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
						if votesReceived*2 > len(cm.server.peerIds)/*+1*/ {
							// +1 is canceled because it should be the server itself, but
							// I must subtract 1 because the default gateway is included
							// and it is not a server

							// Won the election!
							cm.Dlog("wins election with %d votes", votesReceived)
							cm.startLeader()
							return
						}
					}
				}
			}
		}(peerId)
	}

	// Run another election timer, in case this election is not successful.
	//go cm.runElectionTimer()
	//go cm.StartElection()
}

// becomeFollower makes cm a follower and resets its state.
// Expects cm.Mu to be locked.
func (cm *ConsensusModule) becomeFollower(term int) {
	cm.Dlog("becomes Follower with term=%d; log=%v", term, cm.log)
	cm.state = Follower
	cm.currentTerm = term
	cm.votedFor = -1
	//cm.electionResetEvent = time.Now()
	//go cm.runElectionTimer()
}

// startLeader switches cm into a leader state and begins process of heartbeats.
// Expects cm.Mu to be locked.
func (cm *ConsensusModule) startLeader() {
	cm.state = Leader
	cm.ResumeChan <- struct{}{}
	for _, peerId := range cm.peerIds {
		cm.nextIndex[peerId] = len(cm.log)
		cm.matchIndex[peerId] = -1
	}
	cm.Dlog("becomes Leader; term=%d, nextIndex=%v, matchIndex=%v; log=%v", cm.currentTerm, cm.nextIndex, cm.matchIndex, cm.log)

	// This goroutine runs in the background and sends AEs to peers:
	// * Whenever something is sent on triggerAEChan
	// * ... Or every 50 ms, if no events occur on triggerAEChan
	go func(heartbeatTimeout time.Duration) {
		// Immediately send AEs to peers.
		cm.leaderSendAEs()

		t := time.NewTimer(heartbeatTimeout)
		defer t.Stop()
		doSend := true
		for {
			select {	
				case <-cm.stopSendingAEsChan:
					// Stop the heartbeat loop.
					doSend = false
					if !t.Stop() {
						<-t.C
					}
					t.Reset(heartbeatTimeout)
				
				case <-cm.startSendingAEsChan:
					// Reset timer to fire again after heartbeatTimeout.
					doSend = true
					cm.ResumeChan <- struct{}{}
					if !t.Stop() {
						<-t.C
					}
					t.Reset(heartbeatTimeout)
				case <-t.C:
					// Reset timer to fire again after heartbeatTimeout.
					t.Stop()
					t.Reset(heartbeatTimeout)
				case <-cm.triggerAEChan:
					// Reset timer for heartbeatTimeout.
					if !t.Stop() {
						<-t.C
					}
					t.Reset(heartbeatTimeout)
			}
			
			if doSend {
				// If this isn't a leader any more, stop the heartbeat loop.
				cm.Mu.Lock()
				if cm.state != Leader {
					cm.Mu.Unlock()
					return
				}
				cm.Mu.Unlock()
				cm.leaderSendAEs()
			}
		}
	}(2000 * time.Millisecond)
}

// leaderSendAEs sends a round of AEs to all peers, collects their
// replies and adjusts cm's state.
func (cm *ConsensusModule) leaderSendAEs() {
	cm.Mu.Lock()
	if cm.state != Leader {
		cm.Mu.Unlock()
		return
	}
	savedCurrentTerm := cm.currentTerm
	cm.Mu.Unlock()

	for _, peerId := range cm.peerIds {
		go func(peerId int) {
			cm.Mu.Lock()
			ni := cm.nextIndex[peerId]
			prevLogIndex := ni - 1
			prevLogTerm := -1
			if prevLogIndex >= 0 {
				prevLogTerm = cm.log[prevLogIndex].Term
			}
			entries := cm.log[ni:]

			args := AppendEntriesArgs{
				Term:         savedCurrentTerm,
				LeaderId:     cm.id,
				PrevLogIndex: prevLogIndex,
				PrevLogTerm:  prevLogTerm,
				Entries:      entries,
				LeaderCommit: cm.commitIndex,
			}
			cm.Mu.Unlock()
			cm.Dlog("sending AppendEntries to %v: ni=%d, args=%+v", peerId, ni, args)
			var reply AppendEntriesReply
			if err := cm.server.Call(peerId, "ConsensusModule.AppendEntries", args, &reply); err == nil {
				cm.Mu.Lock()
				if reply.Term > cm.currentTerm {
					cm.Dlog("term out of date in heartbeat reply")
					cm.becomeFollower(reply.Term)
					cm.Mu.Unlock()
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
							cm.newCommitReadyChan <- struct{}{}
							cm.triggerAEChan <- struct{}{}
						} else {
							cm.Mu.Unlock()
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
					}
				}
			}
		}(peerId)
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

// commitChanSender is responsible for sending committed entries on
// cm.commitChan. It watches newCommitReadyChan for notifications and calculates
// which new entries are ready to be sent. This method should run in a separate
// background goroutine; cm.commitChan may be buffered and will limit how fast
// the client consumes new committed entries. Returns when newCommitReadyChan is
// closed.
func (cm *ConsensusModule) commitChanSender() {
	for {
		<-cm.newCommitReadyChan
		// Find which entries we have to apply.
		cm.Mu.Lock()
		savedTerm := cm.currentTerm
		savedLastApplied := cm.lastApplied
		var entries []LogEntry
		if cm.commitIndex > cm.lastApplied {
			entries = cm.log[cm.lastApplied+1 : cm.commitIndex+1]
			cm.lastApplied = cm.commitIndex
		}
		cm.Mu.Unlock()
		cm.Dlog("commitChanSender entries=%v, savedLastApplied=%d", entries, savedLastApplied)

		for i, entry := range entries {
			cm.commitChan <- CommitEntry{
				Command: entry.Command,
				Index:   savedLastApplied + i + 1,
				Term:    savedTerm,
			}
		}
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
	cm.stopSendingAEsChan <- struct{}{}
	cm.Mu.Unlock()
}

func (cm *ConsensusModule) Resume() {
	cm.Mu.Lock()
	if cm.state == Follower {
		cm.StartElection()
	} else {
		cm.startSendingAEsChan <- struct{}{}
	}
	cm.Mu.Unlock()
}

func (cm *ConsensusModule) monitorLoad() {
	load := 0
	for {
		load = l.GetLoadLevel()
		//if time.Now().Unix() % 10 == 6 {
		//	load = 10
		//}

		cm.Mu.Lock()
		cm.loadLevel = load
		if cm.loadLevel > 8 {
			cm.Mu.Unlock()
			cm.server.Submit(cm.id)
		} else {
			cm.Mu.Unlock()
		}
		time.Sleep(300 * time.Millisecond)	
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