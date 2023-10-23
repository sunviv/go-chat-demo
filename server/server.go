package server

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/sirupsen/logrus"
)

type ServerOption struct {
	writeWait time.Duration
	readWait  time.Duration
}

type Server struct {
	once    sync.Once
	options ServerOption
	id      string
	address string
	sync.Mutex
	users map[string]net.Conn
}

func NewServer(id, address string) *Server {
	return newServer(id, address)
}

func newServer(id, address string) *Server {
	return &Server{
		id:      id,
		address: address,
		users:   make(map[string]net.Conn, 100),
		options: ServerOption{
			writeWait: time.Second * 10,
			readWait:  time.Minute * 2,
		},
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	log := logrus.WithFields(logrus.Fields{
		"module": "Server",
		"listen": s.address,
		"id":     s.id,
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			conn.Close()
			return
		}

		user := r.URL.Query().Get("user")
		if user == "" {
			conn.Close()
			return
		}

		old, ok := s.addUser(user, conn)
		if ok {
			old.Close()
			log.Infof("close old connection %v", old.RemoteAddr())
		}
		log.Infof("user %s in from %v", user, conn.RemoteAddr())

		go func(user string, conn net.Conn) {
			err := s.readLoop(user, conn)
			if err != nil {
				log.Warn("readloop - ", err)
			}
			conn.Close()
			s.delUser(user)
			log.Infof("connection of %s closed", user)
		}(user, conn)
	})
	log.Infoln("started")
	return http.ListenAndServe(s.address, mux)
}

func (s *Server) Shutdown() {
	s.once.Do(func() {
		s.Lock()
		defer s.Unlock()
		for _, conn := range s.users {
			conn.Close()
		}
	})
}

func (s *Server) addUser(user string, conn net.Conn) (net.Conn, bool) {
	s.Lock()
	defer s.Unlock()
	old, ok := s.users[user]
	s.users[user] = conn
	return old, ok
}

func (s *Server) delUser(user string) {
	s.Lock()
	defer s.Unlock()
	delete(s.users, user)
}

func (s *Server) readLoop(user string, conn net.Conn) error {
	for {
		_ = conn.SetReadDeadline(time.Now().Add(s.options.readWait))

		frame, err := ws.ReadFrame(conn)
		if err != nil {
			return err
		}
		if frame.Header.OpCode == ws.OpPing {
			_ = wsutil.WriteServerMessage(conn, ws.OpPong, nil)
			logrus.Info("write a pong...")
			continue
		}
		if frame.Header.OpCode == ws.OpClose {
			return errors.New("remote sid close the conn")
		}
		logrus.Info(frame.Header)
		if frame.Header.Masked {
			ws.Cipher(frame.Payload, frame.Header.Mask, 0)
		}
		if frame.Header.OpCode == ws.OpText {
			go s.handle(user, string(frame.Payload))
		} else if frame.Header.OpCode == ws.OpBinary {
			go s.handleBinary(user, frame.Payload)
		}
	}
}

func (s *Server) handle(user string, message string) {
	logrus.Infof("recv message %s from %s", message, user)
	s.Lock()
	defer s.Unlock()
	broadcast := fmt.Sprintf("%s <-- FROM %s", message, user)
	for u, conn := range s.users {
		if u == user {
			continue
		}
		logrus.Infof("send to %s : %s", u, broadcast)
		err := s.writeText(conn, broadcast)
		if err != nil {
			logrus.Errorf("write to %s failed, error: %v", user, err)
		}
	}
}

const (
	CommandPing = 100
	CommandPong = 101
)

func (s *Server) handleBinary(user string, message []byte) {
	logrus.Infof("recv message %v from %s", message, user)
	s.Lock()
	defer s.Unlock()
	i := 0
	command := binary.BigEndian.Uint16(message[i : i+2])
	i += 2
	payloadLen := binary.BigEndian.Uint32(message[i : i+4])
	logrus.Infof("command: %v payloadLen: %v", command, payloadLen)
	if command == CommandPing {
		u := s.users[user]
		err := wsutil.WriteServerBinary(u, []byte{0, CommandPong, 0, 0, 0, 0})
		if err != nil {
			logrus.Errorf("write to %s failed, error: %v", user, err)
		}
	}
}

func (s *Server) writeText(conn net.Conn, message string) error {
	f := ws.NewTextFrame([]byte(message))
	err := conn.SetWriteDeadline(time.Now().Add(s.options.writeWait))
	if err != nil {
		return err
	}
	return ws.WriteFrame(conn, f)
}
