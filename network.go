package pbft

import (
	"errors"
	"net/rpc"
)

type peerWrapper struct {
	client  *rpc.Client
	address string
}

func (c *peerWrapper) Call(serviceMethod string, args interface{}, reply interface{}) error {
	var err error
	if c.client == nil {
		err = errors.New("")
	} else {
		err = c.client.Call(serviceMethod, args, reply)
	}
	if err != nil {
		var errdial error
		c.client, errdial = rpc.DialHTTP("tcp", c.address)
		if errdial != nil {
			return errdial
		}
		err = c.client.Call(serviceMethod, args, reply)
	}
	return err
}

func createPeers(id int, addresses []string) []peerWrapper {
	peers := make([]peerWrapper, len(addresses))
	for i := 0; i < len(addresses); i++ {
		peers[i].client = nil
		peers[i].address = addresses[i]
	}

	return peers
}
