package instances

import (
	"net"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	"github.com/sergeii/swat4master/internal/core/entities/instance"
)

type storedItem struct {
	ID   string `json:"id"`
	IP   net.IP `json:"ip"`
	Port int    `json:"port"`
}

func newStoredItem(id string, addr addr.Addr) storedItem {
	return storedItem{
		ID:   encodeID(id),
		IP:   addr.GetIP(),
		Port: addr.Port,
	}
}

func (i storedItem) convert() (instance.Instance, error) {
	id, err := decodeID(i.ID)
	if err != nil {
		return instance.Blank, err
	}
	return instance.New(id, i.IP, i.Port)
}
