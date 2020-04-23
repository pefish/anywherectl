package server

import (
	"context"
	"flag"
	"fmt"
	"github.com/google/uuid"
	"github.com/pefish/anywherectl/internal/protocol"
	"github.com/pefish/anywherectl/internal/version"
	"github.com/pefish/anywherectl/listener"
	go_logger "github.com/pefish/go-logger"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

type ListenerConn struct {
	listener        *listener.Listener
	conn            net.Conn
	sendCommandLock sync.Mutex // 发送命令的锁(针对每个连接的锁)，保证业务包完整性
	pingErrCount    int        // ping错误次数
}

type Server struct {
	wg                    sync.WaitGroup // 退出时等待所有连接handle完毕
	listeners             sync.Map       // map[string]*ListenerConn // 连接到server的所有listener,key是listener的name
	connIdListenerNameMap sync.Map       // map[string]string        // 连接的标识与listener的name的对应关系
	heartbeatInterval     time.Duration  // listener的心跳间隔
	listenerToken         string         // listener连接是需要的token
	clientToken           string         // client连接时需要的token
	tcpListener           net.Listener
	cancelFunc            context.CancelFunc
	finishChan            chan<- bool
	clientConnCache       sync.Map // map[string]net.Conn // 缓存的client连接
}

func NewServer() *Server {
	return &Server{
		heartbeatInterval:     5 * time.Second,
	}
}

func (s *Server) DecorateFlagSet(flagSet *flag.FlagSet) {
	flagSet.String("tcp-address", "0.0.0.0:8181", "<addr>:<port> to listen on for TCP clients")
	flagSet.String("listener-token", "", "token for listeners")
	flagSet.String("client-token", "", "token for clients")
}

func (s *Server) ParseFlagSet(flagSet *flag.FlagSet) {
	err := flagSet.Parse(os.Args[2:])
	if err != nil {
		log.Fatal(err)
	}
}

func (s *Server) Start(finishChan chan<- bool, flagSet *flag.FlagSet) {
	s.finishChan = finishChan

	listenerToken := flagSet.Lookup("listener-token").Value.(flag.Getter).Get().(string)
	if listenerToken == "" {
		go_logger.Logger.Error("listener token must be set")
		return
	}
	s.listenerToken = listenerToken

	clientToken := flagSet.Lookup("client-token").Value.(flag.Getter).Get().(string)
	if clientToken == "" {
		go_logger.Logger.Error("client token must be set")
		return
	}
	s.clientToken = clientToken

	tcpAddress := flagSet.Lookup("tcp-address").Value.(flag.Getter).Get().(string)
	tcpListener, err := net.Listen("tcp", tcpAddress)
	if err != nil {
		go_logger.Logger.ErrorF("listen (%s) failed - %s", tcpAddress, err)
		return
	}
	s.tcpListener = tcpListener
	go_logger.Logger.InfoF("TCP: listening on %s", tcpListener.Addr())

	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFunc = cancel

	s.wg.Add(1)
	go s.heartbeatLoop(ctx)

	s.wg.Add(1)
	go s.acceptConnLoop(ctx)
}

func (s *Server) acceptConnLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			goto exit
		default:
			clientConn, err := s.tcpListener.Accept()
			if err != nil {
				//go_logger.Logger.Error(err)
				if err, ok := err.(net.Error); ok && err.Temporary() {
					continue
				}
				break
			}

			s.wg.Add(1)
			go_logger.Logger.InfoF("TCP: new client(%s)", clientConn.RemoteAddr())
			go s.receiveMessageLoop(ctx, clientConn)
		}

	}
exit:
	go_logger.Logger.Info("SHUTDOWN: acceptConnLoop.")
	s.wg.Done()
}

func (s *Server) heartbeatLoop(ctx context.Context) {
	timer := time.NewTimer(s.heartbeatInterval)
	for {
		select {
		case <-timer.C:
			s.listeners.Range(func(key, value interface{}) bool {
				listenerConn := value.(*ListenerConn)
				go_logger.Logger.DebugF("Heartbeat: %s", listenerConn.listener.Name)
				err := s.sendToListener(listenerConn, "PING", nil)
				if err != nil {
					listenerConn.pingErrCount++
					go_logger.Logger.WarnF("LISTENER(%s): ping error, count: %d. - %s", listenerConn.listener.Name, listenerConn.pingErrCount, err)
					if listenerConn.pingErrCount > 10 {
						go_logger.Logger.WarnF("LISTENER(%s): ping error too many, close this connection.", listenerConn.listener.Name)
						s.destroyListenerConn(listenerConn.conn)
						return false
					}
				} else {
					listenerConn.pingErrCount = 0
				}
				return true
			})
			timer.Reset(s.heartbeatInterval)
		case <-ctx.Done():
			goto exit
		}
	}
exit:
	go_logger.Logger.Info("SHUTDOWN: heartbeat.")
	s.wg.Done()
}

func (s *Server) Exit() {
	close(s.finishChan)
}

func (s *Server) Clear() {
	s.tcpListener.Close()
	// 断开所有listener连接
	s.listeners.Range(func(key, value interface{}) bool {
		s.destroyListenerConn(value.(*ListenerConn).conn)
		return true
	})
	s.cancelFunc()
	s.wg.Wait()
}

func (s *Server) receiveMessageLoop(ctx context.Context, conn net.Conn) {
	var zeroTime time.Time
	err := conn.SetDeadline(zeroTime) // 设置tcp连接的读写截止时间。到了截止时间连接会被关闭
	if err != nil {
		go_logger.Logger.WarnF("failed to set conn timeout - %s", err)
	}
	for {
		select {
		case <-ctx.Done():
			goto exitMessageLoop
		default:
			packageData, err := protocol.ReadPackage(conn)
			listenerNameInterface, ok := s.connIdListenerNameMap.Load(conn.RemoteAddr().String())
			listenerName := "new conn"
			if ok {
				listenerName = listenerNameInterface.(string)
			}
			if err != nil {
				if strings.HasSuffix(err.Error(), "use of closed network connection") {
					goto exitMessageLoop
				}
				if strings.HasSuffix(err.Error(), "EOF") {
					goto exitMessageLoop
				}
				go_logger.Logger.ErrorF("CONN(%s): read command and params error - '%s'", listenerName, err)
				goto exitMessageLoop
			}
			go_logger.Logger.DebugF("CONN(%s): received package '%#v'", listenerName, packageData)

			if packageData.Version != version.ProtocolVersion {
				go_logger.Logger.ErrorF("CONN(%s): bad protocol version", conn.RemoteAddr())
				sendErr := s.sendToListener(&ListenerConn{
					conn:            conn,
				}, "VERSION_ERROR", nil)
				if sendErr != nil {
					go_logger.Logger.WarnF("failed to exec ERROR command - %s", err)
				}
				goto exitMessageLoop
			}

			// 校验token，区分出是listener还是client
			if packageData.ServerToken != s.listenerToken && packageData.ServerToken != s.clientToken {
				go_logger.Logger.ErrorF("CONN(%s): bad token", conn.RemoteAddr())
				sendErr := s.sendToListener(&ListenerConn{
					conn:            conn,
				}, "TOKEN_ERROR", nil)
				if sendErr != nil {
					go_logger.Logger.WarnF("failed to exec ERROR command - %s", err)
				}
				goto exitMessageLoop
			}
			if packageData.ServerToken == s.clientToken { // client连接
				go_logger.Logger.InfoF("client(%s) connected.", conn.RemoteAddr())
				// 权限校验 TODO

				// 加上client id转发
				listenerConnI, ok := s.listeners.Load(packageData.ListenerName)
				if !ok {
					go_logger.Logger.ErrorF("client(%s): listener not found", conn.RemoteAddr())
					sendErr := s.sendToListener(&ListenerConn{
						conn:            conn,
					}, "LISTENER_NOT_FOUND", nil)
					if sendErr != nil {
						go_logger.Logger.WarnF("failed to exec LISTENER_NOT_FOUND command - %s", err)
					}
					goto exitMessageLoop
				}
				listenerConn := listenerConnI.(*ListenerConn)

				uuidStr := uuid.New().String()
				s.clientConnCache.Store(uuidStr, conn)
				err = s.sendToListener(listenerConn, packageData.Command, append([]string{uuidStr}, packageData.Params...))
				if err != nil {
					go_logger.Logger.ErrorF("client(%s): send command to listener - %s", listenerName, packageData.Command, err)
				}
				break
			}

			// 执行命令
			go s.execCommand(conn, packageData) // execCommand出错就关闭连接
		}
	}
exitMessageLoop:
	time.Sleep(2 * time.Second) // 延迟关闭，等待客户端处理消息完成，避免立马断开导致客户端不必要的重连
	err = conn.Close()
	if err != nil {
		go_logger.Logger.DebugF("failed to close conn - %s", err)
	}
	go_logger.Logger.InfoF("CONN(%s): closed.", conn.RemoteAddr())
	s.wg.Done()
}

func (s *Server) execCommand(conn net.Conn, packageData *protocol.ProtocolPackage) {
	if packageData.Command == "REGISTER" {
		listenerConn, err := s.cmdRegister(conn, packageData)
		if err != nil {
			tempListenerConn := &ListenerConn{
				conn:            conn,
			}
			sendErr := s.sendToListener(tempListenerConn, "REGISTER_FAIL", []string{err.Error()})
			if sendErr != nil {
				go_logger.Logger.WarnF("failed to exec REGISTER_FAIL command - %s", err)
			}
		}
		err = s.sendToListener(listenerConn, "REGISTER_OK", nil)
		if err != nil {
			go_logger.Logger.WarnF("failed to exec REGISTER_OK command - %s", err)
		}
	} else if packageData.Command == "PONG" {
		listenerNameInterface, _ := s.connIdListenerNameMap.Load(conn.RemoteAddr().String())
		listenerName := listenerNameInterface.(string)
		go_logger.Logger.DebugF("LISTENER(%s): received PONG.", listenerName)
	} else if packageData.Command == "SHELL_RESULT" {
		connI, ok := s.clientConnCache.Load(packageData.Params[0])
		if !ok {
			go_logger.Logger.WarnF("client not found when send shell result to client, clientId: %s", packageData.Params[0])
			return
		}
		clientConn := connI.(net.Conn)
		_, err := protocol.WritePackage(clientConn, &protocol.ProtocolPackage{
			Version:       version.ProtocolVersion,
			ServerToken:   "",
			ListenerName:  "",
			ListenerToken: "",
			Command:       "RESULT",
			Params:        packageData.Params[1:],
		})
		if err != nil {
			go_logger.Logger.WarnF("write to conn when send shell result to client, clientId: %s", packageData.Params[0])
		}
		// 延迟关闭client连接
		time.Sleep(2 * time.Second)
		clientConn.Close()
	}
}

func (s *Server) destroyListenerConn(conn net.Conn) {
	err := conn.Close()
	if err != nil {
		go_logger.Logger.WarnF("failed to close old listener conn - %s", err)
	}
	connId := conn.RemoteAddr().String()
	listenerNameInterface, _ := s.connIdListenerNameMap.Load(connId)
	listenerName := listenerNameInterface.(string)
	s.listeners.Delete(listenerName)
	s.connIdListenerNameMap.Delete(connId)
}

func (s *Server) cmdRegister(conn net.Conn, packageData *protocol.ProtocolPackage) (*ListenerConn, error) {
	if len(packageData.Params) != 1 {
		return nil, fmt.Errorf("cmdRegister param length error. length: %d", len(packageData.Params))
	}
	//clientTokensStr := packageData.Params[0]
	// 保存连接
	if listenerConn, ok := s.listeners.Load(packageData.ListenerName); ok { // 已经存在的话，就断开老的连接
		s.destroyListenerConn(listenerConn.(*ListenerConn).conn)
	}
	listenerConn := &ListenerConn{
		listener:        listener.NewListener(packageData.ListenerName),
		conn:            conn,
	}
	s.listeners.Store(packageData.ListenerName, listenerConn)
	s.connIdListenerNameMap.Store(conn.RemoteAddr().String(), packageData.ListenerName)
	return listenerConn, nil
}

func (s *Server) sendToListener(listenerConn *ListenerConn, command string, params []string) error {
	listenerConn.sendCommandLock.Lock()
	defer listenerConn.sendCommandLock.Unlock()

	_, err := protocol.WritePackage(listenerConn.conn, &protocol.ProtocolPackage{
		Version:       version.ProtocolVersion,
		ServerToken:   "",
		ListenerToken: "",
		Command:       command,
		Params:        params,
	})
	return err
}
