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
		// Backfill must never delay live blocks: only re-queue when the live
		// queue is nearly drained, and a few at a time.
		if len(s.realtimeCh) > 5 {
			continue
		}
		var since int64
		if s.maxSlot > window {
			since = int64(s.maxSlot) - window
		}

		// Slots with NO row never reached a worker (slotSubscribe gaps before
		// gap-filling existed, reconnects, restarts). Scan a recent window and
		// enqueue the absent ones — a bounded batch per tick, oldest first.
		if s.maxSlot > 0 {
			scanFrom := int64(s.maxSlot) - 6000
			if scanFrom < since {
				scanFrom = since
			}
			if have, herr := s.sc.BlockModel.FindSlotsSince(s.ctx, scanFrom); herr == nil {
				present := make(map[int64]struct{}, len(have))
				for _, sl := range have {
					present[sl] = struct{}{}
				}
				queued := 0
				for sl := scanFrom; sl < int64(s.maxSlot)-50 && queued < 40; sl++ {
					if _, ok := present[sl]; ok {
						continue
					}
					select {
					case <-s.ctx.Done():
						return
					case s.realtimeCh <- uint64(sl):
						queued++
					}
				}
				if queued > 0 {
					s.Infof("SlotNotCompleted: enqueued %d absent slots from %d", queued, scanFrom)
				}
			} else {
				s.Error("FindSlotsSince err:", herr)
			}
		}

		slots, err := s.sc.BlockModel.FindProcessingSlots(s.ctx, since, 10)
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
