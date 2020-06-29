package pbft

type PbftPhase int

type LogEntry struct {
	SeqId   int
	ViewId  int
	Request RequestArgs
	Reply   ReplyArgs
}

type DefaultReply struct {
	Err string
}

type RequestArgs struct {
	Operation interface{}
	Timestamp int64
	ClientId  int
}

type ReplyArgs struct {
	ViewId    int
	Timestamp int64
	ReplicaId int
	Result    interface{}
}

type PrePrepareAgrs struct {
	ViewId  int
	SeqId   int
	Digest  string
	Request RequestArgs
}

type PrepareArgs struct {
	ViewId    int
	SeqId     int
	Digest    string
	ReplicaId int
}

type CommitArgs struct {
	ViewId    int
	SeqId     int
	Digest    string
	ReplicaId int
}
