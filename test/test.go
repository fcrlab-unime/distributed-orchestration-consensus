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
	RequestElabStartTime		time.Time
	RequestElabDuration 		time.Duration
	// Network times for the election of the leader 1.
	ElectionNetworkStartTimes	map[int]time.Time
	ElectionNetworkDurations 	map[int]time.Duration
	// Vote election times for the election of the leader by the others.
	VoteElectionDurations		map[int]time.Duration
	// Time taken to choose the node to assign to the request.
	ChoosingPhaseStartTime		time.Time
	ChoosingPhaseDuration		time.Duration
	// Time taken to prepare the voting.
	VotingPrepStartTime			time.Time
	VotingPrepDuration			time.Duration
	// Network times for the consensus voting phase.
	VoteConsNetStartTimes		map[int]time.Time
	VoteConsNetDurations		map[int]time.Duration
	// Elaboration times for the consensus voting phase.
	VoteConsElabDurations		map[int]time.Duration

	WriteLogStartTime			time.Time
	WriteLogDuration			time.Duration
	// Mutex for the times.
	Mu sync.Mutex

	// File for the times.
	File *os.File
}

func NewTimesStruct(serverId int) (*Times) {
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
	times.VotingPrepStartTime, _ = time.Parse("2006-01-02 15:04:05", "1970-01-01 00:00:00")
	times.VotingPrepDuration = 0
	times.WriteLogStartTime, _ = time.Parse("2006-01-02 15:04:05", "1970-01-01 00:00:00")
	times.WriteLogDuration = 0
	times.Mu = sync.Mutex{}
	times.File, _ = os.OpenFile("/log/times" + strconv.Itoa(serverId) + ".csv", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	return times
}

func (times *Times) SetDurationAndWrite(index int, which string, netowrkIndex ...int) {
	mess := strconv.Itoa(index)
	switch {
		case which == "RE":
			times.RequestElabDuration = time.Since(times.RequestElabStartTime)
			mess += ",RequestElab," + strconv.FormatInt(times.RequestElabDuration.Microseconds(), 10) + "\n"
		case which == "ENVE":
			times.Mu.Lock()
			for k, elem := range times.ElectionNetworkDurations {
				times.ElectionNetworkDurations[k] = elem - times.VoteElectionDurations[k]
			}
			meanNet, stdDevNet := electionValuesCalc(times.ElectionNetworkDurations)
			meanVote, stdDevVote := electionValuesCalc(times.VoteElectionDurations)

			mess += ",ElectionNetworkMean," + strconv.FormatInt(int64(meanNet), 10) + "\n" +
				strconv.Itoa(index) + ",ElectionNetworkStdDev," + strconv.FormatInt(int64(stdDevNet), 10) + "\n" +
				strconv.Itoa(index) + ",VoteElectionMean," + strconv.FormatInt(int64(meanVote), 10) + "\n" +
				strconv.Itoa(index) + ",VoteElectionStdDev," + strconv.FormatInt(int64(stdDevVote), 10) + "\n"
			times.Mu.Unlock()
		case which == "CP":
			times.ChoosingPhaseDuration = time.Since(times.ChoosingPhaseStartTime)
			mess += ",ChoosingPhase," + strconv.FormatInt(times.ChoosingPhaseDuration.Microseconds(), 10) + "\n"
		case which == "VP":
			times.VotingPrepDuration = time.Since(times.VotingPrepStartTime)
			mess += ",VotingPrep," + strconv.FormatInt(times.VotingPrepDuration.Microseconds(), 10) + "\n"
		case which == "VCNVE":
			times.Mu.Lock()
			meanNet, stdDevNet := electionValuesCalc(times.VoteConsNetDurations)
			meanVote, stdDevVote := electionValuesCalc(times.VoteConsElabDurations)

			mess += ",VoteConsNetMean," + strconv.FormatInt(int64(meanNet-meanVote), 10) + "\n" +
				strconv.Itoa(index) + ",VoteConsNetStdDev," + strconv.FormatInt(int64(stdDevNet), 10) + "\n" +
				strconv.Itoa(index) + ",VoteConsElabMean," + strconv.FormatInt(int64(meanVote), 10) + "\n" +
				strconv.Itoa(index) + ",VoteConsElabStdDev," + strconv.FormatInt(int64(stdDevVote), 10) + "\n"
			times.Mu.Unlock()
		case which == "WL":
			times.WriteLogDuration = time.Since(times.WriteLogStartTime)
			mess += ",WriteLog," + strconv.FormatInt(times.WriteLogDuration.Microseconds(), 10) + "\n"
	}
	times.Mu.Lock()
	times.File.WriteString(mess)
	times.Mu.Unlock()
}

func (times *Times) SetStartTime(which string, netowrkIndex ...int) {
	switch {
		case which == "RE":
			times.RequestElabStartTime = time.Now()
		case which == "EN1":
			times.ElectionNetworkStartTimes[netowrkIndex[0]] = time.Now()
		case which == "CP":
			times.ChoosingPhaseStartTime = time.Now()
		case which == "VP":
			times.VotingPrepStartTime = time.Now()
		case which == "VCN1":
			times.VoteConsNetStartTimes[netowrkIndex[0]] = time.Now()
		case which == "WL":
			times.WriteLogStartTime = time.Now()
	}
}

func electionValuesCalc(values map[int]time.Duration) (mean float64, stdDev float64) {
	tmp := []float64{}
	for _, value := range values {
		tmp = append(tmp, float64(value.Microseconds()))
	}
	mean, stdDev = stat.MeanStdDev(tmp, nil)
	if math.IsNaN(stdDev) {
		stdDev = 0
	}
	return mean, stdDev
}
