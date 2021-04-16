package rtsp

import (
	"fmt"
	"log"
	"net"
	"os"
	"sync"
)

// Server
type Server struct {
	SessionLogger
	TCPListener    *net.TCPListener
	TCPPort        int
	Stoped         bool
	pushers        map[string]*Pusher // Path <-> Pusher
	pushersLock    sync.RWMutex
	addPusherCh    chan *Pusher
	removePusherCh chan *Pusher
}

// Server Instance
var Instance *Server = &Server{
	SessionLogger:  SessionLogger{log.New(os.Stdout, "[RTSPServer]", log.LstdFlags|log.Lshortfile)},
	Stoped:         true,
	TCPPort:        554,
	pushers:        make(map[string]*Pusher),
	addPusherCh:    make(chan *Pusher),
	removePusherCh: make(chan *Pusher),
}

// GetServer
func GetServer() *Server {
	return Instance
}

// Start 启动
func (server *Server) Start() (err error) {
	logger := server.logger
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", server.TCPPort))
	if err != nil {
		return
	}
	// 监听 554 端口
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return
	}

	server.Stoped = false
	server.TCPListener = listener
	logger.Println("rtsp server start on", server.TCPPort)
	networkBuffer := 1048576

	for !server.Stoped {
		// 获取数据流
		conn, err := server.TCPListener.Accept()
		if err != nil {
			logger.Println(err)
			continue
		}
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			//设置读写缓存
			if err := tcpConn.SetReadBuffer(networkBuffer); err != nil {
				logger.Printf("rtsp server conn set read buffer error, %v", err)
			}
			if err := tcpConn.SetWriteBuffer(networkBuffer); err != nil {
				logger.Printf("rtsp server conn set write buffer error, %v", err)
			}
		}

		// NewSession
		session := NewSession(server, conn)
		go session.Start()
		go session.RTPHandler()

	}
	return
}

func (server *Server) Stop() {
	logger := server.logger
	logger.Println("rtsp server stop on", server.TCPPort)
	server.Stoped = true
	if server.TCPListener != nil {
		server.TCPListener.Close()
		server.TCPListener = nil
	}
	server.pushersLock.Lock()
	server.pushers = make(map[string]*Pusher)
	server.pushersLock.Unlock()

	close(server.addPusherCh)
	close(server.removePusherCh)
}

func (server *Server) AddPusher(pusher *Pusher) bool {
	logger := server.logger
	added := false
	server.pushersLock.Lock()
	_, ok := server.pushers[pusher.Path()]
	if !ok {
		server.pushers[pusher.Path()] = pusher
		logger.Printf("%v start, now pusher size[%d]", pusher, len(server.pushers))
		added = true
	} else {
		added = false
	}
	server.pushersLock.Unlock()
	if added {
		go pusher.Start()
		server.addPusherCh <- pusher
	}
	return added
}

func (server *Server) TryAttachToPusher(session *Session) (int, *Pusher) {
	server.pushersLock.Lock()
	attached := 0
	var pusher *Pusher = nil
	if _pusher, ok := server.pushers[session.Path]; ok {
		if _pusher.RebindSession(session) {
			session.logger.Printf("Attached to a pusher")
			attached = 1
			pusher = _pusher
		} else {
			attached = -1
		}
	}
	server.pushersLock.Unlock()
	return attached, pusher
}

func (server *Server) RemovePusher(pusher *Pusher) {
	logger := server.logger
	removed := false
	server.pushersLock.Lock()
	if _pusher, ok := server.pushers[pusher.Path()]; ok && pusher.ID() == _pusher.ID() {
		delete(server.pushers, pusher.Path())
		logger.Printf("%v end, now pusher size[%d]\n", pusher, len(server.pushers))
		removed = true
	}
	server.pushersLock.Unlock()
	if removed {
		server.removePusherCh <- pusher
	}
}

func (server *Server) GetPusher(path string) (pusher *Pusher) {
	server.pushersLock.RLock()
	pusher = server.pushers[path]
	server.pushersLock.RUnlock()
	return
}

func (server *Server) GetPushers() (pushers map[string]*Pusher) {
	pushers = make(map[string]*Pusher)
	server.pushersLock.RLock()
	for k, v := range server.pushers {
		pushers[k] = v
	}
	server.pushersLock.RUnlock()
	return
}

func (server *Server) GetPusherSize() (size int) {
	server.pushersLock.RLock()
	size = len(server.pushers)
	server.pushersLock.RUnlock()
	return
}
