package test

import (
	"math"
	"os"
	"strconv"
	"sync"
	"time"

	"gonum.org/v1/gonum/stat"
)

type Times struct {
	// Time for the elaboration of the request.
	RequestElabStartTime time.Time
	RequestElabDuration  time.Duration
	// Network times for the election of the leader 1.
	ElectionNetworkStartTimes map[int]time.Time
	ElectionNetworkDurations  map[int]time.Duration
	// Vote election times for the election of the leader by the others.
	VoteElectionDurations map[int]time.Duration
	// Time taken to choose the node to assign to the request.
	ChoosingPhaseStartTime time.Time
	ChoosingPhaseDuration  time.Duration
	// Network times for the consensus voting phase.
	VoteConsNetStartTimes map[int]time.Time
	VoteConsNetDurations  map[int]time.Duration
	// Elaboration times for the consensus voting phase.
	VoteConsElabDurations map[int]time.Duration
	// Time taken to write the log.
	WriteLogStartTime time.Time
	WriteLogDuration  time.Duration
	// Time taken to transfer the request.
	TransfertReqStartTime time.Time
	TransfertReqDuration  time.Duration

	// Mutex for the times.
	Mu sync.Mutex

	// File for the times.
	File *os.File
}

func NewTimesStruct(serverId int) *Times {
	times := new(Times)
	times.RequestElabStartTime, _ = time.Parse("2006-01-02 15:04:05", "1970-01-01 00:00:00")
	times.RequestElabDuration = 0
	times.ElectionNetworkStartTimes = make(map[int]time.Time)
	times.ElectionNetworkDurations = make(map[int]time.Duration)
	times.VoteElectionDurations = make(map[int]time.Duration)
	times.VoteConsNetStartTimes = make(map[int]time.Time)
	times.VoteConsNetDurations = make(map[int]time.Duration)
	times.VoteConsElabDurations = make(map[int]time.Duration)
	times.ChoosingPhaseStartTime, _ = time.Parse("2006-01-02 15:04:05", "1970-01-01 00:00:00")
	times.ChoosingPhaseDuration = 0
	times.WriteLogStartTime, _ = time.Parse("2006-01-02 15:04:05", "1970-01-01 00:00:00")
	times.WriteLogDuration = 0
	times.Mu = sync.Mutex{}
	times.File, _ = os.OpenFile("/log/times"+strconv.Itoa(serverId)+".csv", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	return times
}

func (times *Times) SetDurationAndWrite(index int, which string, start time.Time) {
	mess := strconv.Itoa(index) + "," + time.Now().String()
	times.Mu.Lock()
	defer times.Mu.Unlock()
	switch {
	case which == "RE":
		times.RequestElabDuration = time.Since(times.RequestElabStartTime)
		mess += ",RequestElab," + strconv.FormatInt(times.RequestElabDuration.Microseconds(), 10) + "\n"
	case which == "ENVE":
		for k, elem := range times.ElectionNetworkDurations {
			times.ElectionNetworkDurations[k] = elem - times.VoteElectionDurations[k]
		}
		meanNet := electionValuesCalc(times.ElectionNetworkDurations)
		meanVote := electionValuesCalc(times.VoteElectionDurations)

		mess += ",ElectionNetworkMean," + strconv.FormatInt(int64(meanNet), 10) + "\n" +
			mess + ",VoteElectionMean," + strconv.FormatInt(int64(meanVote), 10) + "\n"
	case which == "CP":
		times.ChoosingPhaseDuration = time.Since(times.ChoosingPhaseStartTime)
		mess += ",ChoosingPhase," + strconv.FormatInt(times.ChoosingPhaseDuration.Microseconds(), 10) + "\n"
	case which == "VCNVE":
		meanNet := electionValuesCalc(times.VoteConsNetDurations)
		meanVote := electionValuesCalc(times.VoteConsElabDurations)

		mess += ",VoteConsNetMean," + strconv.FormatInt(int64(meanNet-meanVote), 10) + "\n" +
			mess + ",VoteConsElabMean," + strconv.FormatInt(int64(meanVote), 10) + "\n"
	case which == "WL":
		times.WriteLogDuration = time.Since(times.WriteLogStartTime)
		mess += ",WriteLog," + strconv.FormatInt(times.WriteLogDuration.Microseconds(), 10) + "\n"
	case which == "TR":
		times.TransfertReqDuration = time.Since(times.TransfertReqStartTime)
		mess += ",TransfertReq," + strconv.FormatInt(times.TransfertReqDuration.Microseconds(), 10) + "\n"
	case which == "TRE":
		mess += ",TransfertReq,0" + "\n"
	}
	times.File.WriteString(mess)
}

func (times *Times) SetStartTime(which string, netowrkIndex ...int) {
	times.Mu.Lock()
	switch {
	case which == "RE":
		times.RequestElabStartTime = time.Now()
	case which == "EN1":
		times.ElectionNetworkStartTimes[netowrkIndex[0]] = time.Now()
	case which == "CP":
		times.ChoosingPhaseStartTime = time.Now()
	case which == "VCN1":
		times.VoteConsNetStartTimes[netowrkIndex[0]] = time.Now()
	case which == "WL":
		times.WriteLogStartTime = time.Now()
	case which == "TR":
		times.TransfertReqStartTime = time.Now()
	}
	times.Mu.Unlock()
}

func electionValuesCalc(values map[int]time.Duration) (mean float64) {
	tmp := []float64{}
	for _, value := range values {
		tmp = append(tmp, float64(value.Microseconds()))
	}
	mean = stat.Mean(tmp, nil)
	if math.IsNaN(mean) {
		mean = 0
	}
	return mean
}
