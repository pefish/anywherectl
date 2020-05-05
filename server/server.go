package server

import (
	"context"
	"flag"
	"fmt"
	"github.com/google/uuid"
	"github.com/pefish/anywherectl/internal/protocol"
	"github.com/pefish/anywherectl/internal/version"
	"github.com/pefish/anywherectl/listener"
	go_config "github.com/pefish/go-config"
	go_json "github.com/pefish/go-json"
	go_logger "github.com/pefish/go-logger"
	go_reflect "github.com/pefish/go-reflect"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"regexp"
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
	pprofHttpServer       *http.Server
	cancelFunc            context.CancelFunc
	finishChan            chan<- bool
	clientConnCache       sync.Map // map[string]net.Conn // 缓存的client连接
}

func NewServer() *Server {
	return &Server{
		heartbeatInterval: 10 * time.Second,
	}
}

func (s *Server) DecorateFlagSet(flagSet *flag.FlagSet) {
	flagSet.String("tcp-address", "0.0.0.0:8181", "<addr>:<port> to listen on for TCP clients")
	flagSet.String("listener-token", "", "token for listeners")
	flagSet.String("client-token", "", "token for clients")
	flagSet.Bool("enable-pprof", false, "enable pprof")
	flagSet.String("pprof-address", "0.0.0.0:9191", "<addr>:<port> to listen on for pprof")
}

func (s *Server) ParseFlagSet(flagSet *flag.FlagSet) {
	err := flagSet.Parse(os.Args[2:])
	if err != nil {
		log.Fatal(err)
	}
}

func (s *Server) Start(finishChan chan<- bool, flagSet *flag.FlagSet) {
	s.finishChan = finishChan

	listenerToken, err := go_config.Config.GetString("listener-token")
	if err != nil {
		go_logger.Logger.ErrorF("get config error - %s", err)
		return
	}
	if listenerToken == "" {
		go_logger.Logger.Error("listener token must be set")
		return
	}
	s.listenerToken = listenerToken

	clientToken, err := go_config.Config.GetString("client-token")
	if err != nil {
		go_logger.Logger.ErrorF("get config error - %s", err)
		return
	}
	if clientToken == "" {
		go_logger.Logger.Error("client token must be set")
		return
	}
	s.clientToken = clientToken

	tcpAddress, err := go_config.Config.GetString("tcp-address")
	if err != nil {
		go_logger.Logger.ErrorF("get config error - %s", err)
		return
	}
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

	pprofEnable, err := go_config.Config.GetBool("enable-pprof")
	if err != nil {
		go_logger.Logger.ErrorF("get config error - %s", err)
		return
	}
	pprofAddress, err := go_config.Config.GetString("pprof-address")
	if err != nil {
		go_logger.Logger.ErrorF("get config error - %s", err)
		return
	}
	if pprofEnable {
		s.pprofHttpServer = &http.Server{Addr: pprofAddress}
		s.wg.Add(1)
		go func() {
			go_logger.Logger.InfoF("started pprof server on %s, you can open url [http://%s/debug/pprof/] to enjoy!!", s.pprofHttpServer.Addr, s.pprofHttpServer.Addr)
			err := s.pprofHttpServer.ListenAndServe()
			if err != nil {
				go_logger.Logger.WarnF("pprof server start error - %s", err)
			}
			s.wg.Done()
		}()
	}
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
			go_logger.Logger.InfoF("TCP: new CONN(%s)", clientConn.RemoteAddr())
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
				go_logger.Logger.DebugF("Heartbeat: %s", listenerConn.listener.GetName())
				err := s.sendToListener(listenerConn, "PING", nil)
				if err != nil {
					listenerConn.pingErrCount++
					go_logger.Logger.WarnF("LISTENER(%s): ping error, count: %d. - %s", listenerConn.listener.GetName(), listenerConn.pingErrCount, err)
					if listenerConn.pingErrCount > 3 {
						go_logger.Logger.WarnF("LISTENER(%s): ping error too many, close this connection.", listenerConn.listener.GetName())
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
			goto exitHeartbeatLoop
		}
	}
exitHeartbeatLoop:
	timer.Stop()
	go_logger.Logger.Info("SHUTDOWN: heartbeat.")
	s.wg.Done()
}

func (s *Server) Exit() {
	close(s.finishChan)
}

func (s *Server) Clear() {
	s.tcpListener.Close()
	if s.pprofHttpServer != nil {
		s.pprofHttpServer.Shutdown(context.Background())
	}
	// 断开所有listener连接
	s.listeners.Range(func(key, value interface{}) bool {
		s.destroyListenerConn(value.(*ListenerConn).conn)
		return true
	})
	s.cancelFunc()
	s.wg.Wait()
}

func (s *Server) receiveMessageLoop(ctx context.Context, conn net.Conn) {
	for {
		select {
		case <-ctx.Done():
			goto exitMessageLoop
		default:
			packageData, err := protocol.ReadPackage(conn)
			if err != nil {
				if strings.HasSuffix(err.Error(), "use of closed network connection") {
					go_logger.Logger.WarnF("CONN(%s): read package error - '%s'", conn.RemoteAddr(), err)
					goto exitMessageLoop
				}
				if strings.HasSuffix(err.Error(), "EOF") {
					go_logger.Logger.WarnF("CONN(%s): read package error - '%s'", conn.RemoteAddr(), err)
					goto exitMessageLoop
				}
				go_logger.Logger.ErrorF("CONN(%s): read package error - '%s'", conn.RemoteAddr(), err)
				goto exitMessageLoop
			}
			go_logger.Logger.DebugF("CONN(%s): received package '%#v'", conn.RemoteAddr(), packageData)

			if packageData.Version != version.ProtocolVersion {
				go_logger.Logger.ErrorF("CONN(%s): bad protocol version", conn.RemoteAddr())
				sendErr := s.sendToListener(&ListenerConn{
					conn: conn,
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
					conn: conn,
				}, "TOKEN_ERROR", nil)
				if sendErr != nil {
					go_logger.Logger.WarnF("failed to exec ERROR command - %s", err)
				}
				goto exitMessageLoop
			}
			if packageData.ServerToken == s.clientToken { // client连接
				var zeroTime time.Time
				err := conn.SetReadDeadline(zeroTime) // client保持alive，因为没有心跳
				if err != nil {
					go_logger.Logger.WarnF("failed to set conn timeout - %s", err)
				}

				go_logger.Logger.InfoF("CONN(%s) connected.", conn.RemoteAddr())
				// 加上client id转发
				listenerConnI, ok := s.listeners.Load(packageData.ListenerName)
				if !ok {
					go_logger.Logger.ErrorF("CONN(%s): listener not found", conn.RemoteAddr())
					sendErr := s.sendToListener(&ListenerConn{
						conn: conn,
					}, "LISTENER_NOT_FOUND", nil)
					if sendErr != nil {
						go_logger.Logger.WarnF("failed to send LISTENER_NOT_FOUND - %s", err)
					}
					goto exitMessageLoop
				}
				listenerConn := listenerConnI.(*ListenerConn)
				// 如果是SHELL命令，则权限校验
				authPass := false
				if packageData.Command == "SHELL" {
					shellsI, ok := listenerConn.listener.ClientTokens[packageData.ListenerToken]
					if !ok {
						goto outAuthCheck
					}
					tokenAuthMap, ok := shellsI.(map[string]interface{})
					if !ok {
						goto outAuthCheck
					}
					shellsSlice, ok := tokenAuthMap["shell"].([]interface{})
					if !ok {
						goto outAuthCheck
					}
					for _, shellI := range shellsSlice {
						shellStr, err := go_reflect.Reflect.ToString(shellI)
						match, err := regexp.MatchString(shellStr, string(packageData.Params[0]))
						if err == nil && match == true {  // 正则校验
							authPass = true
							break
						}
					}
				}
			outAuthCheck:
				if authPass == false {
					go_logger.Logger.ErrorF("CONN(%s): UNAUTHORIZE.", conn.RemoteAddr())
					sendErr := s.sendToListener(&ListenerConn{
						conn: conn,
					}, "UNAUTHORIZE", nil)
					if sendErr != nil {
						go_logger.Logger.WarnF("failed to send UNAUTHORIZE - %s", err)
					}
					goto exitMessageLoop
				}

				uuidStr := uuid.New().String()
				s.clientConnCache.Store(uuidStr, conn)
				err = s.sendToListener(listenerConn, packageData.Command, append([][]byte{[]byte(uuidStr)}, packageData.Params...))
				if err != nil {
					go_logger.Logger.ErrorF("CONN(%s): send command to listener - %s", conn.RemoteAddr(), packageData.Command, err)
				}
				break
			} else {
				err := conn.SetReadDeadline(time.Now().Add(s.heartbeatInterval * 2)) // 这么久没收到数据，则报超时错
				if err != nil {
					go_logger.Logger.WarnF("failed to set conn timeout - %s", err)
				}

				// 处理listener命令
				listenerNameInterface, ok := s.connIdListenerNameMap.Load(conn.RemoteAddr().String())
				if !ok {  // 新连接
					if packageData.Command != "REGISTER" {
						go_logger.Logger.ErrorF("CONN(%s): must register first", conn.RemoteAddr())
						tempListenerConn := &ListenerConn{
							conn: conn,
						}
						sendErr := s.sendToListener(tempListenerConn, "ERROR", [][]byte{
							[]byte("must register first"),
						})
						if sendErr != nil {
							go_logger.Logger.WarnF("failed to exec REGISTER_FAIL command - %s", err)
						}
						goto exitMessageLoop
					}
					// 开始注册逻辑
					listenerConn, err := s.cmdRegister(conn, packageData)
					if err != nil {
						tempListenerConn := &ListenerConn{
							conn: conn,
						}
						sendErr := s.sendToListener(tempListenerConn, "REGISTER_FAIL", [][]byte{
							[]byte(err.Error()),
						})
						if sendErr != nil {
							go_logger.Logger.WarnF("failed to exec REGISTER_FAIL command - %s", err)
						}
					}
					err = s.sendToListener(listenerConn, "REGISTER_OK", nil)
					if err != nil {
						go_logger.Logger.WarnF("failed to exec REGISTER_OK command - %s", err)
					}
					break
				}
				// 老连接
				listenerName := listenerNameInterface.(string)
				val, ok := s.listeners.Load(listenerName)
				if !ok {
					go_logger.Logger.ErrorF("CONN(%s): listener not found", conn.RemoteAddr())
					goto exitMessageLoop
				}
				// 执行命令，不能使用协程，否则会造成顺序错乱
				s.execCommand(val.(*ListenerConn), packageData) // execCommand出错就关闭连接
			}
		}
	}
exitMessageLoop:
	time.Sleep(2 * time.Second) // 延迟，等待listner处理消息完成，避免立马断开导致listener不必要的重连
	conn.Close()
	go_logger.Logger.InfoF("CONN(%s) closed.", conn.RemoteAddr())
	s.wg.Done()
}

func (s *Server) execCommand(listenerConn *ListenerConn, packageData *protocol.ProtocolPackage) {
	if packageData.Command == "PONG" {
		listenerNameInterface, _ := s.connIdListenerNameMap.Load(listenerConn.conn.RemoteAddr().String())
		listenerName := listenerNameInterface.(string)
		go_logger.Logger.DebugF("LISTENER(%s): received PONG.", listenerName)
	} else if packageData.Command == "SHELL_RESULT" {
		clientId := string(packageData.Params[0])
		connI, ok := s.clientConnCache.Load(clientId)
		if !ok {
			go_logger.Logger.WarnF("client not found when send shell result to client, clientId: %s", clientId)
			s.sendToListener(listenerConn, "CLIENT_CLOSED", [][]byte{packageData.Params[0]})
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
			go_logger.Logger.WarnF("write to conn error when send shell result to client, clientId: %s - %s", clientId, err)
			go_logger.Logger.Warn("close client conn")
			s.clientConnCache.Delete(clientId)
			time.Sleep(2 * time.Second)
			clientConn.Close()
			return
		}
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
	go_logger.Logger.DebugF("remove listeners succeed. key: %s", listenerName)
	s.connIdListenerNameMap.Delete(connId)
	go_logger.Logger.DebugF("remove connIdListenerNameMap succeed. key: %s", connId)
	go_logger.Logger.InfoF("CONN(%s): closed.", conn.RemoteAddr())
}

func (s *Server) cmdRegister(conn net.Conn, packageData *protocol.ProtocolPackage) (*ListenerConn, error) {
	if len(packageData.Params) != 1 {
		return nil, fmt.Errorf("cmdRegister param length error. length: %d", len(packageData.Params))
	}
	// 保存连接
	if listenerConn, ok := s.listeners.Load(packageData.ListenerName); ok { // 已经存在的话，就断开老的连接
		s.destroyListenerConn(listenerConn.(*ListenerConn).conn)
	}
	clientTokensMap, err := go_json.Json.ParseBytesToMap(packageData.Params[0])
	if err != nil {
		return nil, fmt.Errorf("json.Unmarshal client tokens error - %s", err)
	}
	listenerConn := &ListenerConn{
		listener: &listener.Listener{
			Name: packageData.ListenerName,
			ClientTokens: clientTokensMap,
		},
		conn:     conn,
	}
	s.listeners.Store(packageData.ListenerName, listenerConn)
	go_logger.Logger.DebugF("save listeners succeed. key: %s", packageData.ListenerName)
	s.connIdListenerNameMap.Store(conn.RemoteAddr().String(), packageData.ListenerName)
	go_logger.Logger.DebugF("save connIdListenerNameMap succeed. key: %s, value: %s", conn.RemoteAddr().String(), packageData.ListenerName)
	return listenerConn, nil
}

func (s *Server) sendToListener(listenerConn *ListenerConn, command string, params [][]byte) error {
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
