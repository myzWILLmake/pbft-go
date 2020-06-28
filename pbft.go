package pbft

import (
	"fmt"
	"sync"
)

type Pbft struct {
	mu       *sync.Mutex
	servers  []peerWrapper
	clients  []peerWrapper
	n        int
	f        int
	me       int
	viewId   int
	seqId    int
	logs     map[int]*LogEntry
	prepares map[int]map[int]string
	commits  map[int]map[int]string

	debugCh chan interface{}
}

func (pf *Pbft) isPrimary() bool {
	return pf.me == pf.viewId%pf.n
}

func (pf *Pbft) boradcast(rpcname string, rpcargs interface{}) {
	pf.debugPrint("Boradcast: " + rpcname + "\n")
	reply := &DefaultReply{}
	for _, peer := range pf.servers {
		p := peer
		go p.Call("Pbft."+rpcname, rpcargs, reply)
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

		delete(pf.prepares, seqId)
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
		defaultReply := &DefaultReply{}
		go client.Call("Client.Reply", replyArgs, defaultReply)

		delete(pf.commits, seqId)
	}
}

func (pf *Pbft) getServerInfo() map[string]interface{} {
	info := make(map[string]interface{})
	pf.mu.Lock()
	defer pf.mu.Unlock()
	info["id"] = pf.me
	info["viewId"] = pf.viewId
	info["seqId"] = pf.seqId
	info["n"] = pf.n
	return info
}

func (pf *Pbft) debugPrint(msg string) {
	pf.debugCh <- msg
}

func MakePbft(id int, serverPeers, clientPeers []peerWrapper, debugCh chan interface{}) *Pbft {
	pf := &Pbft{}
	pf.mu = &sync.Mutex{}
	pf.servers = serverPeers
	pf.me = id
	pf.clients = clientPeers
	pf.viewId = 0
	pf.seqId = 0
	pf.logs = make(map[int]*LogEntry)
	pf.prepares = make(map[int]map[int]string)
	pf.commits = make(map[int]map[int]string)
	pf.n = len(pf.servers)
	pf.f = (pf.n - 1) / 3
	pf.debugCh = debugCh

	return pf
}
