package slot

import (
	"errors"
	"time"

	"dex/model/solmodel"
)

type SlotNotCompleteService struct {
	*SlotService
}

func NewSlotNotCompleteService(slotService *SlotService) *SlotNotCompleteService {
	return &SlotNotCompleteService{
		SlotService: slotService,
	}
}

func (s *SlotNotCompleteService) Start() {
	s.SlotNotCompleted()
}

func (s *SlotService) SlotNotCompleted() {
	// Re-queue blocks whose fetch failed (BlockFailed) onto the same worker
	// queue as live slots. This used to run once at startup, exit as soon as
	// it found nothing, and push to a channel no worker consumed — so a block
	// dropped by a transient RPC error (429, "not available") was lost for
	// good, along with any token create or trade inside it.
	const window = 20000 // slots (~1-2h on devnet); older losses aren't worth the RPC
	s.Infof("SlotNotCompleted: retry loop started")
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			s.Info("slotFailed stop succeed")
			return
		case <-ticker.C:
		}
		var since int64
		if s.maxSlot > window {
			since = int64(s.maxSlot) - window
		}
		slots, err := s.sc.BlockModel.FindProcessingSlots(s.ctx, since, 50)
		if err != nil && !errors.Is(err, solmodel.ErrNotFound) {
			s.Error("FindProcessingSlots err:", err)
			continue
		}
		for _, b := range slots {
			select {
			case <-s.ctx.Done():
				return
			case s.realtimeCh <- uint64(b.Slot):
				s.Infof("SlotNotCompleted: re-queued failed slot %v", b.Slot)
			}
		}
	}
}
