package pbft

func (pf *Pbft) Request(args *RequestArgs, reply *DefaultReply) error {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	if seqId := pf.getReplyFromLog(args); seqId != 0 {
		c := pf.clients[args.ClientId]
		if c != nil {
			replyArgs := pf.logs[seqId].Reply
			if replyArgs.Timestamp != 0 {
				defaultReply := DefaultReply{}
				c.Call("Client.Reply", &replyArgs, &defaultReply)
			}
		}
		return nil
	}

	if pf.isPrimary() {
		// insert requset to log
		pf.seqId++

		newLog := &LogEntry{}
		newLog.SeqId = pf.seqId
		newLog.Request = RequestArgs{args.Operation, args.Timestamp, args.ClientId}
		pf.logs[pf.seqId] = newLog

		prepreareArgs := &PrePrepareAgrs{}
		prepreareArgs.ViewId = pf.viewId
		prepreareArgs.SeqId = pf.seqId
		prepreareArgs.Request = newLog.Request
		// todo: digest
		prepreareArgs.Digest = "prepreare digest"
		pf.boradcast("Preprepare", prepreareArgs)
		return nil
	} else {
		// relay to primary
		// todo: request timer
		n := len(pf.peers)
		primaryId := pf.viewId % n
		err := pf.peers[primaryId].Call("Pbft.Request", args, reply)
		return err
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

	// todo: check sequence number h~H
	// accept PrePrepare Msg

	newLog := &LogEntry{}
	newLog.SeqId = args.SeqId
	newLog.Request = args.Request
	pf.logs[args.SeqId] = newLog

	// save to prepares
	pf.savePrepare(args.SeqId, pf.me, args.Digest)
	pf.processPrepares(args.SeqId)

	// boardcast Prepare
	prepareArgs := &PrepareArgs{}
	prepareArgs.SeqId = newLog.SeqId
	prepareArgs.ReplicaId = pf.me
	prepareArgs.ViewId = pf.viewId
	prepareArgs.Digest = args.Digest
	pf.boradcast("Prepare", prepareArgs)
	return nil
}

func (pf *Pbft) Prepare(args *PrepareArgs, reply *DefaultReply) error {
	pf.mu.Lock()
	defer pf.mu.Unlock()
	if args.ViewId != pf.viewId {
		reply.Err = "Wrong viewId"
		return nil
	}

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

	// todo: check sequence number h~H
	pf.saveCommits(args.SeqId, args.ReplicaId, args.Digest)
	pf.processCommits(args.SeqId)
	return nil
}

func (c *Client) Reply(args *ReplyArgs, reply *DefaultReply) error {
	c.saveReply(args)
	c.processReplies(args.Timestamp)
	return nil
}
