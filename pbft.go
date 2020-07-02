package pbft

import (
	"fmt"
	"sync"
	"time"
)

const CheckPointSequenceInterval = 10
const RequestTimeout = 5000

type Pbft struct {
	mu                   *sync.Mutex
	servers              []peerWrapper
	clients              []peerWrapper
	n                    int
	f                    int
	me                   int
	viewId               int
	seqId                int
	requestTimer         map[int64]*TimerWithCancel
	logs                 map[int]*LogEntry
	prepares             map[int]map[int]string
	commits              map[int]map[int]string
	checkpoints          map[int]map[int]string
	viewChanges          map[int]map[int]PreparedRequest
	maxCommitted         int
	lastCheckpointSeqId  int
	lastCheckpointDigest string

	// debug
	debugCh chan interface{}

	// malicious behaviors
	// string: rpc method name
	maliciousModes map[string]MaliciousBehaviorMode
	// define how many malicious msgs in PartiallyMaliciousMode
	maliciousPartialVal int
}

func (pf *Pbft) isPrimary() bool {
	return pf.me == pf.viewId%pf.n
}

func (pf *Pbft) broadcast(rpcname string, rpcargs interface{}) {
	pf.debugPrint("Broadcast: " + rpcname + "\n")
	reply := &DefaultReply{}
	maliciousMode := pf.maliciousModes[rpcname]
	switch maliciousMode {
	case NormalMode:
		for _, peer := range pf.servers {
			p := peer
			go p.Call("Pbft."+rpcname, rpcargs, reply)
		}
	case CrashedLikeMode:
		return
	case PartiallyMaliciousMode:
		pf.maliciousBroadcast(rpcname, rpcargs, true)
	case MaliciousMode:
		pf.maliciousBroadcast(rpcname, rpcargs, false)
	}
}

func (pf *Pbft) newRequestTimer(timestamp int64) {
	if pf.requestTimer[timestamp] != nil {
		pf.requestTimer[timestamp].Cancel()
		delete(pf.requestTimer, timestamp)
	}
	newTimer := NewTimerWithCancel(time.Duration(RequestTimeout * time.Millisecond))
	newTimer.SetTimeout(func() {
		pf.debugPrint(fmt.Sprintf("Request timeout: Timestamp[%d]\n", timestamp))
		delete(pf.requestTimer, timestamp)
		// todo: timer for viewchange
		pf.sendViewChange()
	})
	newTimer.Start()
	pf.requestTimer[timestamp] = newTimer
}

func (pf *Pbft) sendViewChange() {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	// find all prepared but not committed request
	preparedRequestSet := make(map[int]PreparedRequest)
	for seqId, prepares := range pf.prepares {
		log, ok := pf.logs[seqId]
		if !ok {
			continue
		}
		if log.Phase != PbftPhasecommitted && len(prepares) > 2*pf.f {
			digestCnt := make(map[string]int)
			isDigestValid := false
			for _, digest := range prepares {
				digestCnt[digest]++
				if digestCnt[digest] > 2*pf.f {
					isDigestValid = true
					break
				}
			}
			if isDigestValid {
				preparedRequest := PreparedRequest{}
				preparedRequest.Request = *log
				preparedRequest.Prepares = prepares
				preparedRequestSet[seqId] = preparedRequest
			}
		}
	}

	viewChangeArgs := &ViewChangeArgs{}
	viewChangeArgs.ViewId = pf.viewId + 1
	viewChangeArgs.ReplicaId = pf.me
	viewChangeArgs.LastCheckpointDigest = pf.lastCheckpointDigest
	viewChangeArgs.LastCheckpointSeqId = pf.lastCheckpointSeqId
	viewChangeArgs.PreparedRequestSet = preparedRequestSet

	pf.broadcast("ViewChange", viewChangeArgs)
}

func (pf *Pbft) getReplyFromLog(args *RequestArgs) int {
	// naive way
	for seqId, log := range pf.logs {
		if log.Request.ClientId == args.ClientId &&
			log.Request.Timestamp == args.Timestamp {
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

	logEntry, ok := pf.logs[seqId]
	if !ok || logEntry.Phase != PbftPhasePrepare || logEntry.ViewId != pf.viewId {
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
		logEntry.Phase = PbftPhasecommit

		commitArgs := &CommitArgs{}
		commitArgs.SeqId = seqId
		commitArgs.ViewId = pf.viewId
		commitArgs.Digest = maxDigest
		commitArgs.ReplicaId = pf.me
		pf.broadcast("Commit", commitArgs)

		delete(pf.prepares, seqId)
	}
}

func (pf *Pbft) processCommits(seqId int) {
	if pf.commits[seqId] == nil {
		return
	}

	logEntry, ok := pf.logs[seqId]
	if !ok || logEntry.Phase != PbftPhasecommit || logEntry.ViewId != pf.viewId {
		return
	}

	if len(pf.commits[seqId]) > 2*pf.f {
		// commit the request and reply to client
		logEntry.Phase = PbftPhasecommitted

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

		if seqId > pf.maxCommitted {
			pf.maxCommitted = seqId
			// multicast checkpoint
			if pf.maxCommitted-pf.lastCheckpointSeqId > CheckPointSequenceInterval {
				checkpointArgs := &CheckpointArgs{}
				checkpointArgs.LastCommitted = pf.maxCommitted
				// todo: current state to digest
				checkpointArgs.Digest = "checkpoint digest"
				checkpointArgs.ReplicaId = pf.me
				pf.broadcast("Checkpoint", checkpointArgs)
			}
		}

		timestamp := logEntry.Request.Timestamp
		if timer, ok := pf.requestTimer[timestamp]; ok {
			timer.Cancel()
			delete(pf.requestTimer, timestamp)
		}

		delete(pf.commits, seqId)
		delete(pf.prepares, seqId)
	}
}

func (pf *Pbft) saveCheckpoints(seqId int, replicaId int, digest string) {
	if pf.checkpoints[seqId] == nil {
		pf.checkpoints[seqId] = make(map[int]string)
	}
	pf.checkpoints[seqId][replicaId] = digest
}

func (pf *Pbft) processCheckpoints(seqId int) {
	if pf.checkpoints[seqId] == nil {
		return
	}

	if seqId <= pf.lastCheckpointSeqId {
		return
	}

	if len(pf.checkpoints[seqId]) > 2*pf.f {
		checkpoints := pf.checkpoints[seqId]
		digestCnt := make(map[string]int)
		validDigest := ""
		for _, digest := range checkpoints {
			digestCnt[digest]++
			if digestCnt[digest] > 2*pf.f {
				validDigest = digest
				break
			}
		}

		if validDigest != "" {
			pf.lastCheckpointSeqId = seqId
			pf.lastCheckpointDigest = validDigest
			pf.garbageCollect(seqId)
			pf.viewChanges = make(map[int]map[int]PreparedRequest)
		}
	}
}

func (pf *Pbft) saveViewChange(seqId int, replicaId int, preparedRequestSet map[int]PreparedRequest) {
	if seqId != pf.lastCheckpointSeqId {
		return
	}

	// check valid prepared request
	for seqId, preparedRequest := range preparedRequestSet {
		if preparedRequest.Request.ViewId != pf.viewId {
			delete(preparedRequestSet, seqId)
			continue
		}

		if preparedRequest.Request.SeqId < pf.lastCheckpointSeqId {
			delete(preparedRequestSet, seqId)
			continue
		}

		prepares := preparedRequest.Prepares
		if len(prepares) <= 2*pf.f {
			delete(preparedRequestSet, seqId)
			continue
		}

		isValid := false
		digestCnt := make(map[string]int)
		for _, digest := range prepares {
			digestCnt[digest]++
			if digestCnt[digest] > 2*pf.f {
				isValid = true
				break
			}
		}

		if !isValid {
			delete(preparedRequestSet, seqId)
		}
	}

	pf.viewChanges[replicaId] = preparedRequestSet
}

func (pf *Pbft) provessViewChange(viewId int) {
	// only primary
	if (viewId)%pf.n != pf.me {
		return
	}

	if len(pf.viewChanges) > 2*pf.f {
		// combine all prepared requests
		minSeq := pf.seqId
		maxSeq := pf.lastCheckpointSeqId
		allPreparedRequests := make(map[int]PreparedRequest)
		for _, PreparedRequestSet := range pf.viewChanges {
			for seqId, preparedRequest := range PreparedRequestSet {
				allPreparedRequests[seqId] = preparedRequest
				if seqId > maxSeq {
					maxSeq = seqId
				}

				if seqId < minSeq {
					minSeq = seqId
				}
			}
		}

		// find all not committed message between minSeq ~ maxSeq
		// and generate new preprepare messages
		newPreprepares := make(map[int]PrePrepareAgrs)
		for seqId, logEntry := range pf.logs {
			if seqId >= minSeq && seqId <= maxSeq {
				if logEntry.ViewId == pf.viewId && logEntry.Phase != PbftPhasecommitted {
					preprepareArgs := PrePrepareAgrs{}
					preprepareArgs.ViewId = pf.viewId + 1
					preprepareArgs.SeqId = seqId
					preprepareArgs.Request = logEntry.Request
					// todo: digest
					preprepareArgs.Digest = "prepreare digest"
					newPreprepares[seqId] = preprepareArgs
				}
			}
		}

		newViewAgrs := &NewViewArgs{}
		newViewAgrs.ViewId = pf.viewId + 1
		newViewAgrs.PreparedRequestSet = allPreparedRequests
		newViewAgrs.NewPreprepares = newPreprepares
		pf.broadcast("NewView", newViewAgrs)
	}
}

func (pf *Pbft) garbageCollect(seqId int) {
	for id := range pf.prepares {
		if id <= seqId {
			delete(pf.prepares, id)
		}
	}

	for id := range pf.commits {
		if id <= seqId {
			delete(pf.prepares, id)
		}
	}

	for id := range pf.checkpoints {
		if id <= seqId {
			delete(pf.prepares, id)
		}
	}

	for id, log := range pf.logs {
		if id <= seqId && log.Phase != PbftPhasecommitted {
			delete(pf.logs, id)
		}
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
	pf.requestTimer = make(map[int64]*TimerWithCancel)
	pf.prepares = make(map[int]map[int]string)
	pf.commits = make(map[int]map[int]string)
	pf.viewChanges = make(map[int]map[int]PreparedRequest)
	pf.maxCommitted = 0
	pf.lastCheckpointSeqId = 0
	pf.n = len(pf.servers)
	pf.f = (pf.n - 1) / 3
	pf.debugCh = debugCh
	pf.maliciousModes = make(map[string]MaliciousBehaviorMode)
	pf.maliciousModes["Preprepare"] = NormalMode
	pf.maliciousModes["Prepare"] = NormalMode
	pf.maliciousModes["Commit"] = NormalMode
	pf.maliciousModes["Checkpoint"] = NormalMode
	pf.maliciousModes["ViewChange"] = NormalMode
	pf.maliciousModes["NewView"] = NormalMode

	return pf
}
