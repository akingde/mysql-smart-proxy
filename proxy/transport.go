package server

import (
	"fmt"
	"io"
	"net"

	"github.com/compasses/mysql-load-balancer/backend"
	"github.com/compasses/mysql-load-balancer/core/golog"
	"github.com/compasses/mysql-load-balancer/mysql"
)

type Transport struct {
	Client    TransPipe
	Server    TransPipe
	Quit      chan bool
	backend   *backend.BackendConn
	clientend *ClientConn
}

type TransPipe struct {
	pipe      net.Conn
	info      string
	errMsg    chan string
	RoundTrip chan int
	quit      bool
	cid       uint32
	direct    int // 0 from client, 1 from server
}

const (
	readBuf      int = 1024
	readLargeBuf int = 1024 * 1024 * 16
)

func NewTransport(c *ClientConn) (*Transport, error) {
	//got backend connection
	backConn, err := c.GetBackendConn("node1")
	if err != nil {
		golog.Error("Transport", "NewTransport", "no backend connection available", c.connectionId, err.Error())
		return nil, err
	}
	backConn.UseDB("ESHOPDB16")
	t := new(Transport)

	t.Client = TransPipe{
		pipe:   c.c,
		info:   c.Info(),
		cid:    c.connectionId,
		direct: 0,
	}

	t.Server = TransPipe{
		pipe:   backConn.Conn.GetTCPConnect(),
		info:   backConn.Info(),
		cid:    backConn.Conn.ConnectionId(),
		direct: 1,
	}

	t.backend = backConn
	t.clientend = c

	return t, nil
}

func (trans *Transport) Run() {
	defer trans.backend.Close()
	golog.Info("Transport", "Run", "Start transfer", trans.Client.cid, "backend cid", trans.Server.cid, trans.Client.info, trans.Server.info)

	for {
		data, err := trans.Client.ReadClientRaw()
		if err != nil {
			golog.Warn("Transport", "Run", "client error", trans.Client.cid, err.Error())
			return
		}

		if len(data) > 4 {
			cmd := data[4]
			switch cmd {
			case mysql.COM_QUIT:
				golog.Info("Transport", "Run", "client quit", trans.Client.cid)
				return
			case mysql.COM_PING:
				trans.clientend.writeOK(nil)
				golog.Warn("Transport", "Run", "client ping", uint32(cmd))
				continue
				// case mysql.COM_INIT_DB:
				// 	if err := trans.clientend.useDB(hack.String(data)); err != nil {
				// 		return //err
				// 	} else {
				// 		trans.clientend.writeOK(nil)
				// 	}
				// 	golog.Warn("Transport", "Run", "client change DB", uint32(cmd), string(data[5:]))
				// 	continue
				//case mysql.COM_QUERY:
			}
			golog.Debug("Transport", "Run", "client command", uint32(cmd), string(data[5:]))
		}

		//send to server
		err = trans.Server.Write(data)
		if err != nil {
			golog.Warn("Transport", "Run", "server write error", trans.Server.cid, err.Error())
			return
		}

		//read response from server
		data, err = trans.Server.ReadServerRaw()
		golog.Debug("Transport", "Run", "server read ", trans.Server.cid, data)

		if err != nil {
			golog.Warn("Transport", "Run", "server read error", trans.Server.cid, err.Error())
			return
		}

		// send to client
		err = trans.Client.Write(data)
		if err != nil {
			golog.Warn("Transport", "Run", "client write error", trans.Client.cid, err.Error())
			return
		}
	}
}

func (trans *TransPipe) ReadPacket() ([]byte, error) {
	header := []byte{0, 0, 0, 0}

	if _, err := io.ReadFull(trans.pipe, header); err != nil {
		return nil, mysql.ErrBadConn
	}
	length := int(uint32(header[0]) | uint32(header[1])<<8 | uint32(header[2])<<16)
	if length < 1 {
		return nil, fmt.Errorf("invalid payload length %d", length)
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(trans.pipe, data); err != nil {
		return nil, mysql.ErrBadConn
	} else {
		if length < mysql.MaxPayloadLen {
			return append(header[:], data...), nil
		}
		var buf []byte
		buf, err = trans.ReadPacket()
		if err != nil {
			return nil, mysql.ErrBadConn
		} else {
			header = append(header[:], data...)
			return append(header, buf...), nil
		}
	}
}

func (trans *TransPipe) ReadHeader() ([]byte, error) {
	header := []byte{0, 0, 0, 0}

	if _, err := io.ReadFull(trans.pipe, header); err != nil {
		return nil, mysql.ErrBadConn
	}
	return header[:], nil
}

func (trans *TransPipe) ReadClientRaw() ([]byte, error) {
	return trans.ReadPacket()
}

func (trans *TransPipe) ReadServerColumns() ([]byte, error) {
	//just read packet
	var result []byte
	for {
		data, err := trans.ReadPacket()
		if err != nil {
			return nil, err
		}

		// EOF Packet
		if mysql.IsEOFPacket(data[4:]) {
			result = append(result, data...)
			return result, nil
		}
		result = append(result, data...)
	}
}

func (trans *TransPipe) ReadServerRows() ([]byte, error) {
	//now just same to read columns
	return trans.ReadServerColumns()
}

func (trans *TransPipe) ReadServerRaw() ([]byte, error) {
	data, err := trans.ReadPacket()
	if err != nil {
		return nil, err
	}

	if data[4] == mysql.OK_HEADER {
		return data, nil
	}

	// must be a result set
	//get column count
	_, _, n := mysql.LengthEncodedInt(data[4:])
	if n-len(data[4:]) != 0 {
		return nil, mysql.ErrMalformPacket
	}
	//read result columns
	cols, err := trans.ReadServerColumns()
	if err != nil {
		return nil, err
	}

	//read result rows
	rows, err := trans.ReadServerRows()
	if err != nil {
		return nil, err
	}
	data = append(data, cols...)
	return append(data, rows...), nil
}

func (trans *TransPipe) Write(data []byte) error {
	_, err := trans.pipe.Write(data)
	return err
}

func (t *TransPipe) PipeError(err error) {
	if err != io.EOF {
		golog.Warn("Server", "PipeError", t.info, t.cid, err)
	}
	t.RoundTrip <- 0
}
