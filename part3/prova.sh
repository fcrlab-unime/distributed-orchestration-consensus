go test -v -race . -run TestCommitOneCommand |& tee /tmp/raftlog
go run ../tools/raft-testlog-viz/main.go < /tmp/raftlog
mv /tmp/*.html ~/Desktop/raft/part3

