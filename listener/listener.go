package listener

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"github.com/pefish/anywherectl/internal/protocol"
	"github.com/pefish/anywherectl/internal/version"
	go_logger "github.com/pefish/go-logger"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

type Listener struct {
	Name        string
	serverToken string
	serverAddress string
}

func NewListener(name string) *Listener {
	return &Listener{
		Name: name,
	}
}

func (s *Listener) DecorateFlagSet(flagSet *flag.FlagSet) {
	flagSet.String("config", "", "path to config file")
	flagSet.String("server-token", "", "server token to connect. max length 32")
	flagSet.String("server-address", "0.0.0.0:8181", "server address to connect")
}

func (s *Listener) ParseFlagSet(flagSet *flag.FlagSet) {
	err := flagSet.Parse(os.Args[2:])
	if err != nil {
		log.Fatal(err)
	}
}

func (l *Listener) Start(flagSet *flag.FlagSet) error {

	serverToken := flagSet.Lookup("server-token").Value.(flag.Getter).Get().(string)
	if serverToken == "" {
		return errors.New("server token must be set")
	}
	if len(serverToken) > 32 {
		return errors.New("server token too long")
	}
	l.serverToken = serverToken

	serverAddress := flagSet.Lookup("server-address").Value.(flag.Getter).Get().(string)
	l.serverAddress = serverAddress
	// 连接服务器
	conn, err := net.Dial("tcp", l.serverAddress)
	if err != nil {
		return fmt.Errorf("connect server err - %s", err)
	}
	versionBuf := bytes.Repeat([]byte(" "), 4)
	copy(versionBuf, version.ProtocolVersion)
	_, err = conn.Write(versionBuf)
	if err != nil {
		return fmt.Errorf("write version to conn err - %s", err)
	}

	tokenBuf := bytes.Repeat([]byte(" "), 32)
	copy(tokenBuf, l.serverToken)
	_, err = conn.Write(tokenBuf)
	if err != nil {
		return fmt.Errorf("write token to conn err - %s", err)
	}

	commandBuf := bytes.Repeat([]byte(" "), 32)
	copy(commandBuf, "REGISTER")
	_, err = conn.Write(commandBuf)
	if err != nil {
		return fmt.Errorf("write command to conn err - %s", err)
	}

	params := strings.Join([]string{
		"test_listener",
		"你好",
	}, " ")
	paramsSize := len(params)
	fmt.Println("参数长度：", paramsSize)
	err = binary.Write(conn, binary.BigEndian, int32(paramsSize))
	if err != nil {
		return fmt.Errorf("write params size to conn err - %s", err)
	}
	paramsBuf := make([]byte, paramsSize)
	copy(paramsBuf, params)
	_, err = conn.Write(paramsBuf)
	if err != nil {
		return fmt.Errorf("write params to conn err - %s", err)
	}
	go_logger.Logger.InfoF("connect server succeed!!! start receiving...")
	// 开始接收消息
	err = conn.SetReadDeadline(time.Now().Add(10 * time.Second)) // 设置tcp连接的读超时
	if err != nil {
		go_logger.Logger.WarnF("failed to set conn timeout - %s", err)
	}
	for {
		commandStr, params, err := protocol.ReadCommandAndParams(conn)
		if err != nil {
			go_logger.Logger.ErrorF("read command and params error - %s", err)
			break
		}
		go_logger.Logger.InfoF("received command '%s'", commandStr)
		go_logger.Logger.InfoF("received param message '%#v'", params)
	}

	err = conn.Close()
	if err != nil {
		return fmt.Errorf("conn close err - %s", err)
	}

	return nil
}

func (s *Listener) Stop() {

}
