package pbft

import (
	"fmt"
)

func (pf *Pbft) Request(args *RequestArgs, reply *DefaultReply) error {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	pf.debugPrint(fmt.Sprintf("Recieved Request[Time %d Cmd %s] from Client[%d]\n", args.Timestamp, args.Operation, args.ClientId))
	if seqId := pf.getReplyFromLog(args); seqId != 0 {
		c := pf.clients[args.ClientId]
		replyArgs := pf.logs[seqId].Reply
		if replyArgs.Timestamp != 0 {
			defaultReply := DefaultReply{}
			go c.Call("Client.Reply", &replyArgs, &defaultReply)
		}
		return nil
	}

	pf.newRequestTimer(args.Timestamp)

	if pf.isPrimary() {
		// insert requset to log
		pf.seqId++

		prepreareArgs := &PrePrepareAgrs{}
		prepreareArgs.ViewId = pf.viewId
		prepreareArgs.SeqId = pf.seqId
		prepreareArgs.Request = *args
		// todo: digest
		prepreareArgs.Digest = "prepreare digest"
		pf.broadcast("Preprepare", prepreareArgs)

		newLog := &LogEntry{}
		newLog.SeqId = prepreareArgs.SeqId
		newLog.Request = prepreareArgs.Request
		newLog.ViewId = pf.viewId
		newLog.Phase = PbftPhasePrepare
		pf.logs[prepreareArgs.SeqId] = newLog

		return nil
	} else {
		// relay to primary
		// todo: request timer
		n := len(pf.servers)
		primaryId := pf.viewId % n
		go pf.servers[primaryId].Call("Pbft.Request", args, reply)
		return nil
	}
	return nil
}

func (pf *Pbft) Preprepare(args *PrePrepareAgrs, reply *DefaultReply) error {
	pf.mu.Lock()
	defer pf.mu.Unlock()
	if pf.viewId != args.ViewId {
		reply.Err = "Wrong viewId"
		return nil
	}

	pf.debugPrint(fmt.Sprintf("Received Preprepare[Seq %d, View %d]\n", args.SeqId, args.ViewId))

	// todo: check sequence number h~H
	// accept PrePrepare Msg
	var newLog *LogEntry
	if pf.isPrimary() {
		newLog = pf.logs[args.SeqId]
	} else {
		newLog = &LogEntry{}
		newLog.SeqId = args.SeqId
		newLog.Request = args.Request
		newLog.ViewId = pf.viewId
		newLog.Phase = PbftPhasePrepare
		pf.logs[args.SeqId] = newLog
	}

	// save to prepares
	pf.savePrepare(args.SeqId, pf.me, args.Digest)
	pf.processPrepares(args.SeqId)

	// broadcast Prepare
	prepareArgs := &PrepareArgs{}
	prepareArgs.SeqId = newLog.SeqId
	prepareArgs.ReplicaId = pf.me
	prepareArgs.ViewId = pf.viewId
	prepareArgs.Digest = args.Digest
	pf.broadcast("Prepare", prepareArgs)
	return nil
}

func (pf *Pbft) Prepare(args *PrepareArgs, reply *DefaultReply) error {
	pf.mu.Lock()
	defer pf.mu.Unlock()
	if args.ViewId != pf.viewId {
		reply.Err = "Wrong viewId"
		return nil
	}

	pf.debugPrint(fmt.Sprintf("Received Prepare[Seq %d, View %d, Rep %d]\n", args.SeqId, args.ViewId, args.ReplicaId))
	// todo: check sequence number h~H

	pf.savePrepare(args.SeqId, args.ReplicaId, args.Digest)
	pf.processPrepares(args.SeqId)
	return nil
}

func (pf *Pbft) Commit(args *CommitArgs, reply *DefaultReply) error {
	pf.mu.Lock()
	defer pf.mu.Unlock()
	if args.ViewId != pf.viewId {
		reply.Err = "Wrong viewId"
		return nil
	}

	pf.debugPrint(fmt.Sprintf("Received Commit[Seq %d, View %d, Rep %d]\n", args.SeqId, args.ViewId, args.ReplicaId))
	// todo: check sequence number h~H
	pf.saveCommits(args.SeqId, args.ReplicaId, args.Digest)
	pf.processCommits(args.SeqId)
	return nil
}

func (pf *Pbft) CheckPoint(args *CheckpointArgs, reply *DefaultReply) error {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	pf.debugPrint(fmt.Sprintf("Received Checkpoint[LastCommitted %d, Digest %s, Rep %d]\n", args.LastCommitted, args.Digest, args.ReplicaId))
	pf.saveCheckpoints(args.LastCommitted, args.ReplicaId, args.Digest)
	return nil
}

func (pf *Pbft) ViewChange(args *ViewChangeArgs, reply *DefaultReply) error {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	pf.debugPrint(fmt.Sprintf("Received ViewChange[ViewId %d, Rep %d, LastCheckpoint %d]\n", args.ViewId, args.ReplicaId, args.LastCheckpointSeqId))
	// check view change message valid
	if args.ViewId != pf.viewId+1 {
		reply.Err = "Invalid viewId"
		return nil
	}

	if args.LastCheckpointSeqId != pf.lastCheckpointSeqId {
		reply.Err = "Invalid checkpoint sequenceId"
		return nil
	}

	if args.LastCheckpointDigest != pf.lastCheckpointDigest {
		reply.Err = "Invalid checkpoint digest"
		return nil
	}

	pf.saveViewChange(args.LastCheckpointSeqId, args.ReplicaId, args.PreparedRequestSet)
	pf.provessViewChange(args.ViewId)
	return nil
}

func (pf *Pbft) NewView(args *NewViewArgs, reply *DefaultReply) error {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	pf.debugPrint(fmt.Sprintf("Received NewView[ViewId %d]\n", args.ViewId))
	if args.ViewId != pf.viewId+1 {
		reply.Err = "Invalid viewId"
		return nil
	}

	// todo: check correctness of newPreprepares
	// same as primary generating the newprepreares

	// enter new view
	pf.viewId = args.ViewId
	pf.viewChanges = make(map[int]map[int]PreparedRequest)
	for seqId, preprepareArgs := range args.NewPreprepares {
		pf.logs[seqId] = nil
		delete(pf.prepares, seqId)
		delete(pf.commits, seqId)

		reply := &DefaultReply{}
		args := &preprepareArgs
		go pf.Preprepare(args, reply)
	}

	return nil
}

func (c *Client) Reply(args *ReplyArgs, reply *DefaultReply) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.debugPrint(fmt.Sprintf("Received Reply[%d, %d, %d] from ReplicaId[%d]\n", args.Timestamp, args.ReplicaId, args.ViewId, args.ReplicaId))
	c.saveReply(args)
	c.processReplies(args.Timestamp)
	return nil
}
