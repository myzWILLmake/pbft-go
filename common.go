package pbft

import (
	"time"
)

type TimerWithCancel struct {
	d time.Duration
	t *time.Timer
	c chan interface{}
	f func()
}

func NewTimerWithCancel(d time.Duration) *TimerWithCancel {
	t := &TimerWithCancel{}
	t.d = d
	t.c = make(chan interface{})
	return t
}

func (t *TimerWithCancel) Start() {
	t.t = time.NewTimer(t.d)
	go func() {
		select {
		case <-t.t.C:
			t.f()
		case <-t.c:
		}
	}()
}

func (t *TimerWithCancel) SetTimeout(f func()) {
	t.f = f
}

func (t *TimerWithCancel) Cancel() {
	t.c <- nil
}

type PbftPhase int

const (
	PbftPhasePrepare = iota
	PbftPhasecommit
	PbftPhasecommitted
)

type LogEntry struct {
	SeqId   int
	ViewId  int
	Phase   PbftPhase
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

type CheckpointArgs struct {
	LastCommitted int
	Digest        string
	ReplicaId     int
}

type PreparedRequest struct {
	Request  LogEntry
	Prepares map[int]string
}

type ViewChangeArgs struct {
	ViewId               int
	ReplicaId            int
	LastCheckpointSeqId  int
	LastCheckpointDigest string
	PreparedRequestSet   map[int]PreparedRequest
}

type NewViewArgs struct {
	ViewId             int
	PreparedRequestSet map[int]PreparedRequest
	NewPreprepares     map[int]PrePrepareAgrs
}

type MaliciousBehaviorMode int

const (
	NormalMode = iota
	CrashedLikeMode
	PartiallyMaliciousMode
	MaliciousMode
)
