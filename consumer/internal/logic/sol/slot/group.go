package slot

import (
	"dex/consumer/internal/svc"
)

type SlotServiceGroup struct {
	*SlotService
	Ws           *SlotWsService
	NotCompleted *SlotNotCompleteService
}

func NewSlotServiceGroup(sc *svc.ServiceContext, slotChan chan uint64, historyChan chan uint64) *SlotServiceGroup {
	slotService := NewSlotService(sc, slotChan, historyChan)
	return &SlotServiceGroup{
		SlotService:  slotService,
		Ws:           NewSlotWsService(slotService),
		NotCompleted: NewSlotNotCompleteService(slotService),
	}
}

func (s *SlotServiceGroup) Start() {
	go s.NotCompleted.Start()
	s.Ws.Start()
}
