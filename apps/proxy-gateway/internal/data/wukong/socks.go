package wukong

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/go-gost/gosocks5"
)

type probeStage string

const (
	stageTCP     probeStage = "TCP"
	stageSOCKS   probeStage = "SOCKS5"
	stageConnect probeStage = "CONNECT"
	stageHTTPS   probeStage = "HTTPS"
)

func (p *Pool) dial(ctx context.Context, entry string, sid int64, target string) (*net.TCPConn, probeStage, error) {
	var addr gosocks5.Addr
	if err := addr.ParseFrom(target); err != nil {
		return nil, stageConnect, err
	}
	raw, err := (&net.Dialer{}).DialContext(ctx, "tcp", entry)
	if err != nil {
		return nil, stageTCP, err
	}
	conn := raw.(*net.TCPConn)
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	stage, err := p.negotiate(conn, sid, &addr)
	if !stop() || err != nil {
		_ = conn.Close()
		if err == nil {
			err = ctx.Err()
		}
		return nil, stage, err
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, stage, nil
}

func (p *Pool) negotiate(conn net.Conn, sid int64, addr *gosocks5.Addr) (probeStage, error) {
	if _, err := conn.Write([]byte{5, 1, gosocks5.MethodUserPass}); err != nil {
		return stageSOCKS, err
	}
	var method [2]byte
	if _, err := io.ReadFull(conn, method[:]); err != nil {
		return stageSOCKS, err
	}
	if method != [2]byte{5, gosocks5.MethodUserPass} {
		return stageSOCKS, errors.New("Wukong did not select SOCKS5 username/password")
	}
	username := fmt.Sprintf("%s-res-any-sid-%09d-sidtime-3600", p.username, sid)
	if err := gosocks5.NewUserPassRequest(gosocks5.UserPassVer, username, p.password).Write(conn); err != nil {
		return stageSOCKS, err
	}
	response, err := gosocks5.ReadUserPassResponse(conn)
	if err != nil {
		return stageSOCKS, err
	}
	if response.Status != 0 {
		return stageSOCKS, errors.New("Wukong SOCKS5 authentication failed")
	}
	if err := gosocks5.NewRequest(gosocks5.CmdConnect, addr).Write(conn); err != nil {
		return stageConnect, err
	}
	reply, err := gosocks5.ReadReply(conn)
	if err != nil {
		return stageConnect, err
	}
	if reply.Rep != gosocks5.Succeeded {
		return stageConnect, errors.New("Wukong SOCKS5 CONNECT rejected")
	}
	return stageHTTPS, nil
}
