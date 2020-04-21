package test

import (
	"io"
	"net"
	"time"
)

type FakeConn struct {
	ReadFunc             func([]byte) (int, error)
	WriteFunc            func([]byte) (int, error)
	CloseFunc            func() error
	LocalAddrFunc        func() net.Addr
	RemoteAddrFunc       func() net.Addr
	SetDeadlineFunc      func(time.Time) error
	SetReadDeadlineFunc  func(time.Time) error
	SetWriteDeadlineFunc func(time.Time) error

	Reader io.Reader
	Writer io.Writer
}

func (f FakeConn) Read(b []byte) (int, error)         { return f.ReadFunc(b) }
func (f FakeConn) Write(b []byte) (int, error)        { return f.WriteFunc(b) }
func (f FakeConn) Close() error                       { return f.CloseFunc() }
func (f FakeConn) LocalAddr() net.Addr                { return f.LocalAddrFunc() }
func (f FakeConn) RemoteAddr() net.Addr               { return f.RemoteAddrFunc() }
func (f FakeConn) SetDeadline(t time.Time) error      { return f.SetDeadlineFunc(t) }
func (f FakeConn) SetReadDeadline(t time.Time) error  { return f.SetReadDeadlineFunc(t) }
func (f FakeConn) SetWriteDeadline(t time.Time) error { return f.SetWriteDeadlineFunc(t) }

type fakeNetAddr struct{}

func (fakeNetAddr) Network() string { return "" }
func (fakeNetAddr) String() string  { return "" }

func NewFakeConn() *FakeConn {
	netAddr := fakeNetAddr{}
	return &FakeConn{
		ReadFunc:             func(b []byte) (int, error) { return 0, nil },
		WriteFunc:            func(b []byte) (int, error) { return len(b), nil },
		CloseFunc:            func() error { return nil },
		LocalAddrFunc:        func() net.Addr { return netAddr },
		RemoteAddrFunc:       func() net.Addr { return netAddr },
		SetDeadlineFunc:      func(time.Time) error { return nil },
		SetWriteDeadlineFunc: func(time.Time) error { return nil },
		SetReadDeadlineFunc:  func(time.Time) error { return nil },
	}
}
