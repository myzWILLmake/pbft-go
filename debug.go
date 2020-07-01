package pbft

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
)

type IDebugServer interface {
	handleConnArgs(net.Conn, []string)
}

type DebugServerBase struct {
	addr     string
	tcpl     net.Listener
	clients  map[string]net.Conn
	notifyCh chan interface{}
}

func (ds *DebugServerBase) getNotifyMsg() {
	for true {
		msg := <-ds.notifyCh
		for _, conn := range ds.clients {
			conn.Write([]byte(msg.(string)))
		}
	}
}

func (ds *DebugServerBase) readConnData(ids IDebugServer, conn net.Conn) {
	buf := make([]byte, 4096)
	for {
		cnt, err := conn.Read(buf)
		if err != nil {
			return
		}
		msg := string(buf[:cnt])
		args := strings.Fields(msg)
		if len(args) > 0 {
			ids.handleConnArgs(conn, args)
		}
	}
}

func (ds *DebugServerBase) run(ids IDebugServer, wg *sync.WaitGroup) {
	go ds.getNotifyMsg()
	for true {
		tcpConn, err := ds.tcpl.Accept()
		if err != nil {
			break
		}
		ds.clients[tcpConn.RemoteAddr().String()] = tcpConn
		go ds.readConnData(ids, tcpConn)
	}
	wg.Done()
}

type PbftDebugServer struct {
	DebugServerBase
	pbftServer *Pbft
}

func (pds *PbftDebugServer) handlePrint(conn net.Conn) {
	info := pds.pbftServer.getServerInfo()
	msg := fmt.Sprintf(`
Pbft Server State:
id:			%d
n:			%d
viewId:		%d
seqId:		%d
`, info["id"].(int), info["n"].(int), info["seqId"].(int), info["seqId"].(int))
	conn.Write([]byte(msg))
}

func (pds *PbftDebugServer) handleConnArgs(conn net.Conn, args []string) {
	switch args[0] {
	case "kill":
		conn.Write([]byte("Kill Server...\n"))
		conn.Close()
		pds.tcpl.Close()
	case "print":
		pds.handlePrint(conn)
	case "quit":
		conn.Write([]byte("Bye!\n"))
		conn.Close()
	case "echo":
		msg := strings.Join(args[1:], " ")
		conn.Write([]byte(msg))
	}
}

func MakePbftDebugServer(addr string, ch chan interface{}, pbft *Pbft, wg *sync.WaitGroup) *PbftDebugServer {
	tcpListener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal("clientSrv listen error:", addr, err)
	}
	pds := &PbftDebugServer{}
	pds.tcpl = tcpListener
	pds.clients = make(map[string]net.Conn)
	pds.addr = addr
	pds.notifyCh = ch
	pds.pbftServer = pbft
	wg.Add(1)
	go pds.run(pds, wg)
	return pds
}

type ClientDebugServer struct {
	DebugServerBase
	clientServer *Client
}

func (cds *ClientDebugServer) handleRequest(conn net.Conn, args []string) {
	request := strings.Join(args[1:], " ")
	cds.clientServer.newRequest(request)
	reply := fmt.Sprintf("Request[%s] sent.\n", request)
	conn.Write([]byte(reply))
}

func (cds *ClientDebugServer) handleConnArgs(conn net.Conn, args []string) {
	switch args[0] {
	case "req":
		cds.handleRequest(conn, args)
	case "kill":
		conn.Write([]byte("Kill Server...\n"))
		conn.Close()
		cds.tcpl.Close()
	case "print":
		// handlePrint()
	case "quit":
		conn.Write([]byte("Bye!\n"))
		conn.Close()
	case "echo":
		msg := strings.Join(args[1:], " ")
		conn.Write([]byte(msg))
	}
}

func MakeClientDebugServer(addr string, ch chan interface{}, client *Client, wg *sync.WaitGroup) *ClientDebugServer {
	tcpListener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal("clientSrv listen error:", addr, err)
	}
	cds := &ClientDebugServer{}
	cds.tcpl = tcpListener
	cds.clients = make(map[string]net.Conn)
	cds.addr = addr
	cds.notifyCh = ch
	cds.clientServer = client
	wg.Add(1)
	go cds.run(cds, wg)
	return cds
}
