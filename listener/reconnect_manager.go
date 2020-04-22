package listener

import (
	go_logger "github.com/pefish/go-logger"
	"net"
	"time"
)

type ReconnectManager struct {
	reconnectionInterval time.Duration
}

func NewReconnectManager() *ReconnectManager {
	return &ReconnectManager{
		reconnectionInterval: 3 *time.Second,
	}
}

func (rm *ReconnectManager) Reconnect(addr string) (<- chan net.Conn, chan <- bool) {
	isReconnectChan := make(chan bool, 1)
	connChan := make(chan net.Conn, 1)
	go func() {
		timer := time.NewTimer(0)
		for {
			select {
			case <- isReconnectChan:
				timer.Reset(rm.reconnectionInterval)
			case <- timer.C:
				go_logger.Logger.InfoF("connecting server %s...", addr)
				conn, err := net.Dial("tcp", addr)
				if err != nil {
					go_logger.Logger.ErrorF("connect server err - %s", err)
					timer.Reset(rm.reconnectionInterval)
					break
				}
				timer.Stop()
				connChan <- conn
			}
		}
	}()
	return connChan, isReconnectChan
}
