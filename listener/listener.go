package listener

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/pefish/anywherectl/internal/protocol"
	"github.com/pefish/anywherectl/internal/version"
	go_logger "github.com/pefish/go-logger"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"
	"sync"
	"time"
)

type Listener struct {
	Name            string
	serverToken     string
	serverAddress   string
	sendCommandLock sync.Mutex
	serverConn      net.Conn
	connExit        chan bool
	cancelFunc      context.CancelFunc
	finishChan      chan<- bool
	name            string
	isReconnectChan chan<- bool
}

func NewListener(name string) *Listener {
	return &Listener{
		Name:     name,
		connExit: make(chan bool, 1),
	}
}

func (s *Listener) DecorateFlagSet(flagSet *flag.FlagSet) {
	flagSet.String("config", "", "path to config file")
	flagSet.String("name", "pefish", "listener name")
	flagSet.String("server-token", "", "server token to connect. max length 32")
	flagSet.String("server-address", "0.0.0.0:8181", "server address to connect")
	flagSet.Bool("enable-pprof", false, "enable pprof")
	flagSet.String("pprof-address", "0.0.0.0:9191", "<addr>:<port> to listen on for pprof")
}

func (s *Listener) ParseFlagSet(flagSet *flag.FlagSet) {
	err := flagSet.Parse(os.Args[2:])
	if err != nil {
		log.Fatal(err)
	}
}

func (l *Listener) Start(finishChan chan<- bool, flagSet *flag.FlagSet) {
	l.finishChan = finishChan

	serverToken := flagSet.Lookup("server-token").Value.(flag.Getter).Get().(string)
	if serverToken == "" {
		go_logger.Logger.Error("server token must be set")
		l.Exit()
		return
	}
	if len(serverToken) > 32 {
		go_logger.Logger.Error("server token too long")
		l.Exit()
		return
	}
	l.serverToken = serverToken

	serverAddress := flagSet.Lookup("server-address").Value.(flag.Getter).Get().(string)
	l.serverAddress = serverAddress

	l.name = flagSet.Lookup("name").Value.(flag.Getter).Get().(string)
	// 连接服务器
	rm := NewReconnectManager()
	connChan, isReconnectChan := rm.Reconnect(l.serverAddress)
	l.isReconnectChan = isReconnectChan

	go func() {
		for {
			conn := <-connChan
			go_logger.Logger.InfoF("server '%s' connected!! start register...", conn.RemoteAddr())
			l.serverConn = conn

			// 开始接收消息
			ctx, cancel := context.WithCancel(context.Background())
			l.cancelFunc = cancel
			go l.receiveMessageLoop(ctx, conn)

			// 开始注册
			err := l.sendCommandToServer("REGISTER", []string{
				"cvc",
			})
			if err != nil {
				go_logger.Logger.Error("send command REGISTER err - %s", err)
				l.Exit()
				break
			}
		}
	}()

	pprofEnable := flagSet.Lookup("enable-pprof").Value.(flag.Getter).Get().(bool)
	pprofAddress := flagSet.Lookup("pprof-address").Value.(flag.Getter).Get().(string)
	if pprofEnable {
		go func() {
			go_logger.Logger.InfoF("starting pprof server on %s", pprofAddress)
			err := http.ListenAndServe(pprofAddress, nil)
			if err != nil {
				go_logger.Logger.WarnF("pprof server start error - %s", err)
			}
		}()
	}
}

func (l *Listener) receiveMessageLoop(ctx context.Context, conn net.Conn) {
	var zeroTime time.Time
	err := conn.SetDeadline(zeroTime)
	if err != nil {
		go_logger.Logger.WarnF("failed to set conn timeout - %s", err)
	}
	for {
		select {
		case <-ctx.Done():
			goto exit
		default:
			packageData, err := protocol.ReadPackage(conn)
			if err != nil {
				go_logger.Logger.DebugF("read command and params error - '%s'", err)
				if strings.Contains(err.Error(), "EOF") {
					go_logger.Logger.Error("connection disconnected!! start reconnecting...")
					l.isReconnectChan <- true
					goto exitMessageLoop
				}
				goto exit
			}
			go_logger.Logger.DebugF("received package '%#v'", packageData)
			err = l.execCommand(conn, packageData.Command, packageData.Params)
			if err != nil {
				go_logger.Logger.ErrorF("received [%s] command - %s", packageData.Command, err)
				goto exit
			}
		}

	}
exit:
	conn.Close()
	l.Exit()
exitMessageLoop:
	conn.Close()
}

func (l *Listener) execCommand(conn net.Conn, name string, params []string) error {
	if name == "PING" {
		err := l.sendCommandToServer("PONG", nil)
		if err != nil {
			go_logger.Logger.WarnF("failed to exec pong command - %s", err)
		}
	} else if name == "REGISTER_OK" {
		go_logger.Logger.Info("received REGISTER_OK.")
	} else if name == "REGISTER_FAIL" {
		return fmt.Errorf("register error - %s", params[0])
	} else if name == "TOKEN_ERROR" {
		return errors.New("token error")
	} else if name == "VERSION_ERROR" {
		return errors.New("version error")
	} else if name == "SHELL" {
		// 权限校验 TODO
		go_logger.Logger.InfoF("exec shell command %s", params[1])
		result := "nothing"
		resultTemp, err := ExecShell(params[1])
		if err != nil {
			go_logger.Logger.WarnF("exec shell command %s err - %s", params[1], err)
			result = err.Error()
		} else {
			result = resultTemp
		}
		err = l.sendCommandToServer("SHELL_RESULT", []string{params[0], result})
		if err != nil {
			go_logger.Logger.WarnF("failed to exec SHELL_RESULT command - %s", err)
		}
	} else {
		return errors.New("command error")
	}
	return nil
}

func (l *Listener) sendCommandToServer(command string, params []string) error {
	l.sendCommandLock.Lock()
	defer l.sendCommandLock.Unlock()

	_, err := protocol.WritePackage(l.serverConn, &protocol.ProtocolPackage{
		Version:       version.ProtocolVersion,
		ServerToken:   l.serverToken,
		ListenerName:  l.name,
		ListenerToken: "",
		Command:       command,
		Params:        params,
	})
	return err
}

func (s *Listener) Exit() {
	close(s.finishChan)
}

func (s *Listener) Clear() {
	s.cancelFunc()
}
