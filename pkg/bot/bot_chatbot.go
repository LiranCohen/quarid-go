package bot

import (
	"github.com/enmand/quarid-go/pkg/adapter"
	"github.com/enmand/quarid-go/pkg/logger"
	//"github.com/enmand/quarid-go/pkg/config"
)

type MsgService struct {
	Services map[string]MsgHandler
}

type Service interface {
	Name() string
	Handler() MsgHandler
}

type PrvMsg struct {
	Prefix  string
	Source  string
	Message string
}

type MsgHandler func(*PrvMsg, adapter.Responder)

func (ms *MsgService) initialize(services ...Service) adapter.HandlerFunc {
	ms.Services = make(map[string]MsgHandler)
	for _, service := range services {
		ms.Services[service.Name()] = service.Handler()
		logger.Log.Info(ms.Services)
	}
	return func(ev *adapter.Event, c adapter.Responder) {}
}

type ChanBot struct {
	Name string
}

func (cb *ChanBot) Name() string {
	return cb.Name
}

//func (pm *PrvMsg) cmdChanOP() {

//}

//func (pm *PrvMsg) cmdAddOp() {

//}

//func (pm *PrvMsg) cmdDropOp() {

//}

//func (pm *PrvMsg) cmdAddAdmin() {

//}

//func (pm *PrvMsg) cmdAddAdmin() {

//}
