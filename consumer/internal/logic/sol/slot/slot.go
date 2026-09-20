package slot

import (
	"context"
	"errors"
	"fmt"
	"time"

	"dex/consumer/internal/svc"

	"github.com/gorilla/websocket"
	"github.com/zeromicro/go-zero/core/logx"
)

var ErrServiceStop = errors.New("service stop")

type SlotService struct {
	Conn *websocket.Conn
	sc   *svc.ServiceContext
	logx.Logger
	slotChan       chan uint64
	slowSlotChan   chan uint64
	startSlot      uint64        // 存量消费的起始 Slot
	endSlot        uint64        // 存量消费的结束 Slot
	historicalCh   chan uint64   // 存量 Slot 队列
	historicalDone chan struct{} // 存量消费完成信号
	errorCh        chan uint64   // 失败重试 Slot 队列
	realtimeCh     chan uint64   // 增量 Slot 队列

	ctx            context.Context
	cancel         func(err error)
	maxSlot        uint64
	lastQueuedSlot uint64 // highest slot handed to the workers; gaps below it are filled
}

func NewSlotService(sc *svc.ServiceContext, slotChan chan uint64, historyChan chan uint64) *SlotService {
	ctx, cancel := context.WithCancelCause(context.Background())
	return &SlotService{
		Logger:         logx.WithContext(context.Background()).WithFields(logx.Field("service", "slot")),
		sc:             sc,
		historicalDone: make(chan struct{}),
		historicalCh:   historyChan,
		realtimeCh:     slotChan,
		ctx:            ctx,
		cancel:         cancel,
	}
}

func (s *SlotService) Start() {
	// s.consumeHistoricalSlots()
}

func (s *SlotService) Stop() {
	s.Info("stop slot")
	s.cancel(ErrServiceStop)
	if s.Conn != nil {
		err := s.Conn.WriteMessage(websocket.TextMessage, []byte("{\"id\":1,\"jsonrpc\":\"2.0\",\"method\": \"slotUnsubscribe\", \"params\": [0]}\n"))
		if err != nil {
			s.Error("programUnsubscribe", err)
		}
		_ = s.Conn.Close()
	}
}

// consumeHistoricalSlots 消费存量 Slot
func (s *SlotService) consumeHistoricalSlots() {
	if s.startSlot <= 0 {
		s.startSlot = s.endSlot
	}
	//consumeHistoricalSlots startSlot is: 335998078
	// consumeHistoricalSlots endSlot is: 335998078
	fmt.Println("consumeHistoricalSlots startSlot is:", s.startSlot)
	fmt.Println("consumeHistoricalSlots endSlot is:", s.endSlot)
	for slot := s.startSlot; slot <= s.endSlot; slot++ {
		select {
		case <-s.ctx.Done():
			return
		case s.historicalCh <- slot: // 向通道发送 Slot
			// s.Infof("send slot: %v to historicalCh", slot)
			// 400 / 5
			time.Sleep(5 * time.Millisecond)
		}
	}
	// 存量消费完毕，关闭存量通道
	close(s.historicalCh)

	s.historicalDone <- struct{}{}

}
