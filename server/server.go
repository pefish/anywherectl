package server

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"github.com/pefish/anywherectl/internal/protocol"
	"github.com/pefish/anywherectl/internal/version"
	"github.com/pefish/anywherectl/listener"
	go_logger "github.com/pefish/go-logger"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

type ListenerConn struct {
	listener *listener.Listener
	conn     net.Conn
}

type Server struct {
	wg                sync.WaitGroup           // 退出时等待所有连接handle完毕
	listeners         map[string]*ListenerConn // 连接到server的所有listener
	heartbeatInterval time.Duration            // listener的心跳间隔
	listenerToken     string                   // listener连接是需要的token
	clientToken       string                   // client连接时需要的token
}

func NewServer() *Server {
	return &Server{
		wg:                sync.WaitGroup{},
		listeners:         make(map[string]*ListenerConn),
		heartbeatInterval: 5 * time.Second,
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

func (s *Server) Start(flagSet *flag.FlagSet) error {
	listenerToken := flagSet.Lookup("listener-token").Value.(flag.Getter).Get().(string)
	if listenerToken == "" {
		return errors.New("listener token must be set")
	}
	s.listenerToken = listenerToken

	clientToken := flagSet.Lookup("client-token").Value.(flag.Getter).Get().(string)
	if clientToken == "" {
		return errors.New("client token must be set")
	}
	s.clientToken = clientToken

	tcpAddress := flagSet.Lookup("tcp-address").Value.(flag.Getter).Get().(string)
	tcpListener, err := net.Listen("tcp", tcpAddress)
	if err != nil {
		return errors.New(fmt.Sprintf("listen (%s) failed - %s", tcpAddress, err))
	}
	go_logger.Logger.InfoF("TCP: listening on %s", tcpListener.Addr())

	// 开启心跳协程
	go s.heartbeat()

	for {
		clientConn, err := tcpListener.Accept()
		if err != nil {
			if err, ok := err.(net.Error); ok && err.Temporary() {
				continue
			}
			break
		}

		s.wg.Add(1)
		go func() {
			go_logger.Logger.InfoF("TCP: new client(%s)", clientConn.RemoteAddr())
			s.Handle(clientConn)
			go_logger.Logger.InfoF("CONN(%s): closed.", clientConn.RemoteAddr())
			s.wg.Done()
		}()
	}

	go_logger.Logger.InfoF("TCP: closing %s", tcpListener.Addr())

	return nil
}

func (s *Server) heartbeat() {
	for _, listenerConn := range s.listeners {
		// TODO
		go_logger.Logger.InfoF("Heartbeat: %s", listenerConn.listener.Name)
	}
}

func (s *Server) Stop() {
	s.wg.Wait()
}

func (s *Server) Handle(conn net.Conn) {
	for {
		//reader := bufio.NewReader(conn)
		// 校验版本号
		protocolVersionBuf := make([]byte, 4)
		err := conn.SetReadDeadline(time.Now().Add(s.heartbeatInterval * 2)) // 设置tcp连接的读超时。有心跳就不会导致超时
		if err != nil {
			go_logger.Logger.WarnF("failed to set conn timeout - %s", err)
		}
		//_, err := reader.Read(protocolVersionBuf) // 带有缓冲的reader会先准备好一个buffer，读到的塞进去，没读到的是0，这里不够4个字节不会阻塞
		_, err = io.ReadFull(conn, protocolVersionBuf) // ReadFull保证精确的读出4个字节，没有4个字节的话会阻塞，阻塞时间不能超过上面设置的超时时间
		if err != nil {
			// 客户端连接断开，会抛ErrUnexpectedEOF错
			go_logger.Logger.ErrorF("failed to read protocol version - %s", err)
			break
		}
		protocolVersion := strings.TrimSpace(string(protocolVersionBuf))
		go_logger.Logger.InfoF("CONN(%s): received protocol version '%s'", conn.RemoteAddr(), protocolVersion)
		if protocolVersion != version.ProtocolVersion {
			go_logger.Logger.ErrorF("CONN(%s): bad protocol version '%s'", conn.RemoteAddr(), protocolVersion)
			break
		}

		// 校验token，区分出是listener还是client
		tokenBuf := make([]byte, 32)
		_, err = io.ReadFull(conn, tokenBuf)
		if err != nil {
			go_logger.Logger.ErrorF("failed to read token - %s", err)
			break
		}
		tokenStr := strings.TrimSpace(string(tokenBuf))
		go_logger.Logger.DebugF("CONN(%s): received token '%s'", conn.RemoteAddr(), tokenStr)
		if tokenStr != s.listenerToken && tokenStr != s.clientToken {
			go_logger.Logger.ErrorF("CONN(%s): bad token '%s'", conn.RemoteAddr(), tokenStr)
			break
		}
		if tokenStr == s.clientToken { // client连接
			// TODO
			go_logger.Logger.Info("client connected.")
			break // client都是一次性命令，tcp连接处理完了就关掉
		}
		go_logger.Logger.InfoF("listener connected.")

		// 读出命令
		commandStr, params, err := protocol.ReadCommandAndParams(conn)
		if err != nil {
			go_logger.Logger.ErrorF("LISTENER(%s): read command and params error - '%s'", conn.RemoteAddr(), err)
			break
		}
		go_logger.Logger.InfoF("LISTENER(%s): received command '%s'", conn.RemoteAddr(), commandStr)
		go_logger.Logger.InfoF("LISTENER(%s): received param message '%#v'", conn.RemoteAddr(), params)
		// 执行命令
		err = s.execCommand(conn, commandStr, params)
		if err != nil { // REGISTER命令失败则关闭连接，其他命令不关闭连接
			go_logger.Logger.ErrorF("LISTENER(%s): failed to execCommand command - %s", conn.RemoteAddr(), err)
			break
		}
	}

	err := conn.Close()
	if err != nil {
		go_logger.Logger.WarnF("failed to close conn - %s", err)
	}
}

func (s *Server) execCommand(conn net.Conn, name string, params []string) error {
	if name == "REGISTER" {
		err := s.cmdRegister(conn, params)
		if err != nil {
			return err
		}
		err = s.sendCommand(conn, "RESPONSE_REGISTER", []string{"OK_REGISTER"})
		if err != nil {
			go_logger.Logger.WarnF("failed to execCommand command - %s", err)
		}
	} else if name == "RESPONSE_TEST" {
		// 要屏蔽错误，防止连接被关闭 TODO
	} else {
		return errors.New("command error")
	}
	return nil
}

func (s *Server) cmdRegister(conn net.Conn, params []string) error {
	if len(params) != 2 {
		return errors.New("cmdRegister param length error")
	}
	name := params[0]
	//clientTokensStr := params[1]
	// 保存连接
	if listenerConn, ok := s.listeners[name]; ok { // 已经存在的话，就断开老的连接
		err := listenerConn.conn.Close()
		if err != nil {
			go_logger.Logger.WarnF("failed to close old listener conn - %s", err)
		}
		delete(s.listeners, name)
	}
	s.listeners[name] = &ListenerConn{
		listener: listener.NewListener(name),
		conn:     conn,
	}
	return nil
}

func (s *Server) sendCommand(conn net.Conn, command string, params []string) error {
	commandBuf := bytes.Repeat([]byte(" "), 32)
	copy(commandBuf, command)
	_, err := conn.Write(commandBuf)
	if err != nil {
		return fmt.Errorf("write command to conn err - %s", err)
	}

	paramsStr := strings.Join(params, " ")
	paramsSize := len(paramsStr)
	err = binary.Write(conn, binary.BigEndian, int32(paramsSize))
	if err != nil {
		return fmt.Errorf("write params size to conn err - %s", err)
	}
	paramsBuf := make([]byte, paramsSize)
	copy(paramsBuf, paramsStr)
	_, err = conn.Write(paramsBuf)
	if err != nil {
		return fmt.Errorf("write params to conn err - %s", err)
	}

	return nil
}
