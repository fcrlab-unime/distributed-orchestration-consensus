go test -v -race . -run TestNewCommitOneCommand |& tee /tmp/raftlog
go run ../tools/raft-testlog-viz/main.go < /tmp/raftlog
mv /tmp/*.html ~/Documents/Tesi

