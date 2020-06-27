package pbft

import (
	"fmt"
	"sync"
	"time"
)

const RunTickerInterval = time.Duration(1 * time.Millisecond)

type Pbft struct {
	mu       *sync.Mutex
	peers    []*peerWrapper
	n        int
	f        int
	me       int
	viewId   int
	seqId    int
	logs     map[int]*LogEntry
	prepares map[int]map[int]string
	commits  map[int]map[int]string
	clients  map[int]*peerWrapper
}

func (pf *Pbft) isPrimary() bool {
	n := len(pf.peers)
	return pf.me == pf.viewId%n
}

func (pf *Pbft) newClient(client *peerWrapper, clientId int) {
	pf.mu.Lock()
	defer pf.mu.Unlock()
	pf.clients[clientId] = client
}

func (pf *Pbft) boradcast(rpcname string, rpcargs interface{}) {
	reply := &DefaultReply{}
	for _, peer := range pf.peers {
		peer.Call("Raft."+rpcname, rpcargs, reply)
	}
}

func (pf *Pbft) getReplyFromLog(args *RequestArgs) int {
	// naive way
	for seqId, log := range pf.logs {
		if log.Request.ClientId == args.ClientId && log.Request.Timestamp == args.Timestamp {
			return seqId
		}
	}

	return 0
}

func (pf *Pbft) savePrepare(seqId int, replicaId int, digest string) {
	if pf.prepares[seqId] == nil {
		pf.prepares[seqId] = make(map[int]string)
	}
	pf.prepares[seqId][replicaId] = digest
}

func (pf *Pbft) saveCommits(seqId int, replicaId int, digest string) {
	if pf.commits[seqId] == nil {
		pf.commits[seqId] = make(map[int]string)
	}
	pf.commits[seqId][replicaId] = digest
}

func (pf *Pbft) processPrepares(seqId int) {
	if pf.prepares[seqId] == nil {
		return
	}

	if len(pf.prepares[seqId]) > 2*pf.f {
		prepares := pf.prepares[seqId]
		digestCnt := make(map[string]int)
		maxCnt := 0
		maxDigest := ""
		for _, digest := range prepares {
			digestCnt[digest]++
			if digestCnt[digest] > maxCnt {
				maxCnt = digestCnt[digest]
				maxDigest = digest
			}
		}

		if maxCnt <= pf.f {
			fmt.Println("ERROR: There is no prepare message whose count is more than f!")
			return
		}

		// go to commit phase
		commitArgs := &CommitArgs{}
		commitArgs.SeqId = seqId
		commitArgs.ViewId = pf.viewId
		commitArgs.Digest = maxDigest
		commitArgs.ReplicaId = pf.me
		pf.boradcast("Commit", commitArgs)
	}
}

func (pf *Pbft) processCommits(seqId int) {
	if pf.prepares[seqId] == nil {
		return
	}

	if len(pf.commits[seqId]) > 2*pf.f {
		// commit the request and reply to client
		logEntry := pf.logs[seqId]

		replyArgs := &ReplyArgs{}
		replyArgs.ViewId = pf.viewId
		replyArgs.ReplicaId = pf.me
		replyArgs.Timestamp = logEntry.Request.Timestamp
		// could apply the operation here
		replyArgs.Result = logEntry.Request.Operation
		logEntry.Reply = *replyArgs

		client := pf.clients[logEntry.Request.ClientId]
		if client != nil {
			defaultReply := &DefaultReply{}
			client.Call("Client.Reply", replyArgs, defaultReply)
		}
	}
}

func MakePbft(peers []*peerWrapper, me int) *Pbft {
	pf := &Pbft{}
	pf.mu = &sync.Mutex{}
	pf.peers = peers
	pf.me = me
	pf.clients = make(map[int]*peerWrapper)
	pf.viewId = 0
	pf.seqId = 0
	pf.logs = make(map[int]*LogEntry)
	pf.prepares = make(map[int]map[int]string)
	pf.commits = make(map[int]map[int]string)
	pf.n = len(pf.peers)
	pf.f = (pf.n - 1) / 3

	return pf
}
