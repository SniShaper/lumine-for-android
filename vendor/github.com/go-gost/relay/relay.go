package relay

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

const (
	Version1 = 0x01
)

type CmdType uint8

// request commands
const (
	CmdConnect   CmdType = 0x01
	CmdBind      CmdType = 0x02
	CmdAssociate CmdType = 0x03
	CmdMask      CmdType = 0x0F

	// FUDP is a command flag indicating that the request is UDP-oriented.
	// DEPRECATED by network feature.
	FUDP CmdType = 0x80
)

// response status list
const (
	StatusOK                  = 0x00
	StatusBadRequest          = 0x01
	StatusUnauthorized        = 0x02
	StatusForbidden           = 0x03
	StatusTimeout             = 0x04
	StatusServiceUnavailable  = 0x05
	StatusHostUnreachable     = 0x06
	StatusNetworkUnreachable  = 0x07
	StatusInternalServerError = 0x08
)

var (
	ErrBadVersion = errors.New("bad version")
)

// Request is a relay client request.
//
// Protocol spec:
//
//	+-----+-------------+----+---+-----+----+
//	| VER |  CMD/FLAGS  | FEALEN | FEATURES |
//	+-----+-------------+----+---+-----+----+
//	|  1  |      1      |    2   |    VAR   |
//	+-----+-------------+--------+----------+
//
//	VER - protocol version, 1 byte.
//	CMD/FLAGS - command (low 4-bit) and flags (high 4-bit), 1 byte.
//	FEALEN - length of features, 2 bytes.
//	FEATURES - feature list.
type Request struct {
	Version  uint8
	Cmd      CmdType
	Features []Feature
}

func (req *Request) ReadFrom(r io.Reader) (n int64, err error) {
	var header [4]byte
	nn, err := io.ReadFull(r, header[:])
	n += int64(nn)
	if err != nil {
		return
	}

	if header[0] != Version1 {
		err = ErrBadVersion
		return
	}
	req.Version = header[0]
	req.Cmd = CmdType(header[1])

	flen := int(binary.BigEndian.Uint16(header[2:]))

	if flen == 0 {
		return
	}
	bf := make([]byte, flen)
	nn, err = io.ReadFull(r, bf)
	n += int64(nn)
	if err != nil {
		return
	}
	req.Features, err = readFeatures(bf)
	return
}

func (req *Request) WriteTo(w io.Writer) (n int64, err error) {
	var buf bytes.Buffer

	buf.WriteByte(req.Version)
	buf.WriteByte(byte(req.Cmd))
	buf.Write([]byte{0, 0}) // placeholder for features length
	n += 4

	flen := 0
	for _, f := range req.Features {
		var b []byte
		b, err = f.Encode()
		if err != nil {
			return
		}
		binary.Write(&buf, binary.BigEndian, f.Type())
		binary.Write(&buf, binary.BigEndian, uint16(len(b)))
		flen += featureHeaderLen
		nn, _ := buf.Write(b)
		flen += nn
	}
	n += int64(flen)
	if flen > 0xFFFF {
		err = errors.New("features maximum length exceeded")
		return
	}

	b := buf.Bytes()
	binary.BigEndian.PutUint16(b[2:4], uint16(flen))

	return buf.WriteTo(w)
}

// Response is a relay server response.
//
// Protocol spec:
//
//	+-----+--------+----+---+-----+----+
//	| VER | STATUS | FEALEN | FEATURES |
//	+-----+--------+----+---+-----+----+
//	|  1  |    1   |    2   |    VAR   |
//	+-----+--------+--------+----------+
//
//	VER - protocol version, 1 byte.
//	STATUS - server status, 1 byte.
//	FEALEN - length of features, 2 bytes.
//	FEATURES - feature list.
type Response struct {
	Version  uint8
	Status   uint8
	Features []Feature
}

func (resp *Response) ReadFrom(r io.Reader) (n int64, err error) {
	var header [4]byte
	nn, err := io.ReadFull(r, header[:])
	n += int64(nn)
	if err != nil {
		return
	}

	if header[0] != Version1 {
		err = ErrBadVersion
		return
	}
	resp.Version = header[0]
	resp.Status = header[1]

	flen := int(binary.BigEndian.Uint16(header[2:]))

	if flen == 0 {
		return
	}
	bf := make([]byte, flen)
	nn, err = io.ReadFull(r, bf)
	n += int64(nn)
	if err != nil {
		return
	}

	resp.Features, err = readFeatures(bf)
	return
}

func (resp *Response) WriteTo(w io.Writer) (n int64, err error) {
	var buf bytes.Buffer

	buf.WriteByte(resp.Version)
	buf.WriteByte(resp.Status)
	buf.Write([]byte{0, 0}) // placeholder for features length
	n += 4

	flen := 0
	for _, f := range resp.Features {
		var b []byte
		b, err = f.Encode()
		if err != nil {
			return
		}
		binary.Write(&buf, binary.BigEndian, f.Type())
		binary.Write(&buf, binary.BigEndian, uint16(len(b)))
		flen += featureHeaderLen
		nn, _ := buf.Write(b)
		flen += nn
	}
	n += int64(flen)
	if flen > 0xFFFF {
		err = errors.New("features maximum length exceeded")
		return
	}

	b := buf.Bytes()
	binary.BigEndian.PutUint16(b[2:4], uint16(flen))

	return buf.WriteTo(w)
}

func readFeatures(b []byte) (fs []Feature, err error) {
	if len(b) == 0 {
		return
	}
	br := bytes.NewReader(b)
	for br.Len() > 0 {
		var f Feature
		f, err = ReadFeature(br)
		if err != nil {
			return
		}
		fs = append(fs, f)
	}
	return
}
