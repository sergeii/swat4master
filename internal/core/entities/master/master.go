package master

import (
	"fmt"
)

var (
	ResponseChallenge   = []byte{0x44, 0x3d, 0x73, 0x7e, 0x6a, 0x59}
	ResponseIsAvailable = []byte{0xfe, 0xfd, 0x09, 0x00, 0x00, 0x00, 0x00}
)

type Msg uint8

const (
	MsgChallenge Msg = 0x01
	MsgHeartbeat Msg = 0x03
	MsgKeepalive Msg = 0x08
	MsgAvailable Msg = 0x09
)

func (msg Msg) String() string {
	switch msg {
	case MsgChallenge:
		return "challenge"
	case MsgHeartbeat:
		return "heartbeat"
	case MsgKeepalive:
		return "keepalive"
	case MsgAvailable:
		return "available"
	}
	return fmt.Sprintf("0x%02x", uint8(msg))
}
