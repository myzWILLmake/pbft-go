package pbft

import "errors"

func (pf *Pbft) setMaliciousMode(rpcname string, maliciousMode int, partialVal int) error {
	pf.mu.Lock()
	defer pf.mu.Unlock()
	_, ok := pf.maliciousModes[rpcname]
	if !ok {
		return errors.New("Invalid rpcname")
	}
	if maliciousMode < 0 || maliciousMode > MaliciousMode {
		return errors.New("Invalid malicious mode")
	}
	pf.maliciousModes[rpcname] = MaliciousBehaviorMode(maliciousMode)
	if maliciousMode == PartiallyMaliciousMode {
		pf.maliciousPartialVal = partialVal
	}
	return nil
}

func (pf *Pbft) maliciousBoardcast(rpcname string, rpcargs interface{}, isPartial bool) {
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
	return nil
}

func (pf *Pbft) maliciousPrepare(args *PrepareArgs) *PrepareArgs {
	return nil
}

func (pf *Pbft) maliciousCommit(args *CommitArgs) *CommitArgs {
	return nil
}

func (pf *Pbft) maliciousCheckpoint(args *CheckpointArgs) *CheckpointArgs {
	return nil
}

func (pf *Pbft) maliciousViewChange(args *ViewChangeArgs) *ViewChangeArgs {
	return nil
}

func (pf *Pbft) maliciousNewView(args *NewViewArgs) *NewViewArgs {
	return nil
}
