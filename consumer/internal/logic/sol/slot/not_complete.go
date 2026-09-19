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
	slot := s.sc.Config.Sol.StartBlock

	if slot == 0 {
		block, err := s.sc.BlockModel.FindFirstFailBlock(s.ctx)
		if err != nil {
			s.Errorf("SlotNotCompleted:FindFirstFailBlock %v", err)
			slot = 0
		} else {
			slot = uint64(block.Slot)
		}
	}

	s.Infof("SlotNotCompleted: start slot: %v, startBlock: %v", slot, s.sc.Config.Sol.StartBlock)

	var checkTicker = time.NewTicker(time.Millisecond * 5000)
	var sendTicker = time.NewTicker(time.Millisecond * 1000)
	defer checkTicker.Stop()
	defer sendTicker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			s.Info("slotFailed stop succeed")
			return
		case <-checkTicker.C:
			slots, err := s.sc.BlockModel.FindProcessingSlots(s.ctx, int64(slot-100), 50)
			s.Infof("FindProcessingSlots err: %v, size: %v", err, len(slots))
			switch {
			case errors.Is(err, solmodel.ErrNotFound) || len(slots) == 0:
				return
			case err == nil:
			default:
				s.Error("FindProcessingSlot err:", err)
			}
			for _, slot := range slots {
				select {
				case <-s.ctx.Done():
					return
				case <-sendTicker.C:
					s.Infof("SlotNotCompleted: push slot: %v to err chain, start Block: %v", slot.Slot, s.sc.Config.Sol.StartBlock)

					s.errorCh <- uint64(slot.Slot)
				}
			}
		}
	}
}
