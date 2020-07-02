package pbft

import "errors"

func (pf *Pbft) setAllMaliciousMode(maliciousMode MaliciousBehaviorMode) {
	pf.maliciousModes["Preprepare"] = maliciousMode
	pf.maliciousModes["Prepare"] = maliciousMode
	pf.maliciousModes["Commit"] = maliciousMode
	pf.maliciousModes["Checkpoint"] = maliciousMode
	pf.maliciousModes["ViewChange"] = maliciousMode
	pf.maliciousModes["NewView"] = maliciousMode
}

func (pf *Pbft) setMaliciousMode(rpcname string, maliciousMode int, partialVal int) error {
	pf.mu.Lock()
	defer pf.mu.Unlock()
	if maliciousMode < 0 || maliciousMode > MaliciousMode {
		return errors.New("Invalid malicious mode")
	}

	if rpcname == "all" {
		pf.setAllMaliciousMode(MaliciousBehaviorMode(maliciousMode))
	} else {
		_, ok := pf.maliciousModes[rpcname]
		if !ok {
			return errors.New("Invalid rpcname")
		}
		pf.maliciousModes[rpcname] = MaliciousBehaviorMode(maliciousMode)
	}

	if maliciousMode == PartiallyMaliciousMode {
		pf.maliciousPartialVal = partialVal
	}
	return nil
}

func (pf *Pbft) maliciousBroadcast(rpcname string, rpcargs interface{}, isPartial bool) {
	realArgs := rpcargs
	var fakeArgs interface{}
	switch rpcname {
	case "Preprepare":
		fakeArgs = pf.maliciousPreprepare(rpcargs.(*PrePrepareAgrs))
	case "Prepare":
		fakeArgs = pf.maliciousPrepare(rpcargs.(*PrepareArgs))
	case "Commit":
		fakeArgs = pf.maliciousCommit(rpcargs.(*CommitArgs))
	case "Checkpoint":
		fakeArgs = pf.maliciousCheckpoint(rpcargs.(*CheckpointArgs))
	case "ViewChange":
		fakeArgs = pf.maliciousViewChange(rpcargs.(*ViewChangeArgs))
	case "NewView":
		fakeArgs = pf.maliciousNewView(rpcargs.(*NewViewArgs))
	}

	maliciousCnt := pf.n
	if isPartial {
		maliciousCnt = pf.maliciousPartialVal
	}
	reply := &DefaultReply{}
	// todo: random range
	for _, peer := range pf.servers {
		p := peer
		if maliciousCnt > 0 {
			go p.Call("Pbft."+rpcname, fakeArgs, reply)
		} else {
			go p.Call("Pbft."+rpcname, realArgs, reply)
		}
		maliciousCnt--
	}
}

// just a sample here for each malicious behavior
func (pf *Pbft) maliciousPreprepare(args *PrePrepareAgrs) *PrePrepareAgrs {
	newArgs := &PrePrepareAgrs{}
	newArgs.ViewId = args.ViewId
	newArgs.SeqId = args.SeqId
	// todo: generate fake digest
	newArgs.Digest = "fake cmd"
	newArgs.Request = args.Request
	newArgs.Request.Operation = "fake cmd"
	return newArgs
}

func (pf *Pbft) maliciousPrepare(args *PrepareArgs) *PrepareArgs {
	newArgs := &PrepareArgs{}
	newArgs.ReplicaId = pf.me
	newArgs.ViewId = args.ViewId
	newArgs.SeqId = args.SeqId
	// todo: generate fake digest
	newArgs.Digest = "fake"
	return newArgs
}

func (pf *Pbft) maliciousCommit(args *CommitArgs) *CommitArgs {
	newArgs := &CommitArgs{}
	newArgs.ReplicaId = pf.me
	newArgs.ViewId = args.ViewId
	newArgs.SeqId = args.SeqId
	// todo: generate fake digest
	newArgs.Digest = "fake"
	return newArgs
}

func (pf *Pbft) maliciousCheckpoint(args *CheckpointArgs) *CheckpointArgs {
	newArgs := &CheckpointArgs{}
	newArgs.ReplicaId = pf.me
	newArgs.LastCommitted = args.LastCommitted + 1
	// todo: generate fake state digest
	newArgs.Digest = "fake state"
	return newArgs
}

func (pf *Pbft) maliciousViewChange(args *ViewChangeArgs) *ViewChangeArgs {
	newArgs := &ViewChangeArgs{}
	newArgs.ReplicaId = pf.me
	newArgs.ViewId = args.ViewId + 1
	newArgs.LastCheckpointSeqId = args.LastCheckpointSeqId
	newArgs.LastCheckpointDigest = args.LastCheckpointDigest
	newArgs.PreparedRequestSet = make(map[int]PreparedRequest)
	return newArgs
}

func (pf *Pbft) maliciousNewView(args *NewViewArgs) *NewViewArgs {
	newArgs := &NewViewArgs{}
	newArgs.ViewId = args.ViewId + 1
	newArgs.NewPreprepares = args.NewPreprepares
	newArgs.PreparedRequestSet = args.PreparedRequestSet
	return newArgs
}
