go test -v -race .  |& tee /tmp/raftlog
go run ../tools/raft-testlog-viz/main.go < /tmp/raftlog
mv /tmp/*.html ~/Documents/raft/part3

