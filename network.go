package pbft

import (
	"errors"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"sync"
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

func createPeers(addresses []string) []peerWrapper {
	peers := make([]peerWrapper, len(addresses))
	for i := 0; i < len(addresses); i++ {
		peers[i].client = nil
		peers[i].address = addresses[i]
	}

	return peers
}

func RunPbftServer(id int, serverAddrs, clientAddrs []string, debug bool, debugAddr string, wg *sync.WaitGroup) *Pbft {
	debugCh := make(chan interface{}, 1024)
	servers := createPeers(serverAddrs)
	clients := createPeers(clientAddrs)
	pbft := MakePbft(id, servers, clients, debugCh)

	if debug {
		MakePbftDebugServer(debugAddr, debugCh, pbft, wg)
	}

	rpc.Register(pbft)
	rpc.HandleHTTP()
	l, err := net.Listen("tcp", serverAddrs[id])
	if err != nil {
		log.Fatal("listen error:", err)
		return nil
	}

	go http.Serve(l, nil)
	return pbft
}

func RunClient(id int, clientAddr string, pbftAddrs []string, debug bool, debugAddr string, wg *sync.WaitGroup) *Client {
	debugCh := make(chan interface{}, 1024)
	peers := createPeers(pbftAddrs)
	client := MakeClient(id, peers, debugCh)

	if debug {
		MakeClientDebugServer(debugAddr, debugCh, client, wg)
	}

	rpc.Register(client)
	rpc.HandleHTTP()
	l, err := net.Listen("tcp", clientAddr)
	if err != nil {
		log.Fatal("listen error:", err)
		return nil
	}

	go http.Serve(l, nil)
	return client
}
