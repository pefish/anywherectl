package client

import (
	"context"
	"flag"
	"github.com/pefish/anywherectl/internal/protocol"
	"github.com/pefish/anywherectl/internal/version"
	go_logger "github.com/pefish/go-logger"
	"log"
	"net"
	"os"
	"time"
)

type Client struct {
	finishChan chan <- bool
	cancelFunc      context.CancelFunc
}

func NewClient() *Client {
	return &Client{

	}
}

func (c *Client) DecorateFlagSet(flagSet *flag.FlagSet) {
	flagSet.String("action", "", "action you want to do")
	flagSet.String("data", "", "data of action")
	flagSet.String("listener-token", "", "token of listener")
	flagSet.String("listener-name", "", "listener name to control")
	flagSet.String("server-token", "", "token of server")
	flagSet.String("server-address", "0.0.0.0:8181", "server address to connect")
}

func (c *Client) ParseFlagSet(flagSet *flag.FlagSet) {
	err := flagSet.Parse(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
}

func (c *Client) Start(finishChan chan <- bool, flagSet *flag.FlagSet) {
	c.finishChan = finishChan

	serverToken := flagSet.Lookup("server-token").Value.(flag.Getter).Get().(string)
	if serverToken == "" {
		go_logger.Logger.Error("server token must be set")
		c.Exit()
		return
	}
	if len(serverToken) > 32 {
		go_logger.Logger.Error("server token too long")
		c.Exit()
		return
	}

	serverAddress := flagSet.Lookup("server-address").Value.(flag.Getter).Get().(string)

	listenerName := flagSet.Lookup("listener-name").Value.(flag.Getter).Get().(string)
	if listenerName == "" {
		go_logger.Logger.Error("listener name must be set")
		c.Exit()
		return
	}

	listenerToken := flagSet.Lookup("listener-token").Value.(flag.Getter).Get().(string)
	if listenerToken == "" {
		go_logger.Logger.Error("listener token must be set")
		c.Exit()
		return
	}

	action := flagSet.Lookup("action").Value.(flag.Getter).Get().(string)
	if action == "" {
		go_logger.Logger.Error("action must be set")
		c.Exit()
		return
	}

	data := flagSet.Lookup("data").Value.(flag.Getter).Get().(string)



	go_logger.Logger.InfoF("connecting server %s...", serverAddress)
	conn, err := net.Dial("tcp", serverAddress)
	if err != nil {
		go_logger.Logger.ErrorF("connect server err - %s", err)
		c.Exit()
		return
	}
	go_logger.Logger.InfoF("server '%s' connected!! start send action...", conn.RemoteAddr())

	// 开始接收消息
	ctx, cancel := context.WithCancel(context.Background())
	c.cancelFunc = cancel
	go c.receiveMessageLoop(ctx, conn)


	_, err = protocol.WritePackage(conn, &protocol.ProtocolPackage{
		Version:       version.ProtocolVersion,
		ServerToken:   serverToken,
		ListenerName:  listenerName,
		ListenerToken: listenerToken,
		Command:       "SHELL",
		Params:        []string{
			data,
		},
	})
	if err != nil {
		go_logger.Logger.Error("send command SHELL err - %s", err)
		c.Exit()
		return
	}
}

func (c *Client) receiveMessageLoop(ctx context.Context, conn net.Conn) {
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
				goto exit
			}
			go_logger.Logger.DebugF("received package '%#v'", packageData)
			if packageData.Command != "RESULT" {
				go_logger.Logger.ErrorF("received [%s] command, it is illegal.", packageData.Command)
				goto exit
			}
			go_logger.Logger.Info(packageData.Params[0])
			goto exit
		}

	}
exit:
	conn.Close()
	c.Exit()
}

func (c *Client) Exit() {
	close(c.finishChan)
}

func (c *Client) Clear() {
	c.cancelFunc()
}
