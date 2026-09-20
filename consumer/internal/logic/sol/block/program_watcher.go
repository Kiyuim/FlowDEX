package block

import (
	"context"
	"strings"
	"sync"
	"time"

	"dex/consumer/internal/svc"
	"dex/model/solmodel"
	"dex/pkg/constants"

	"github.com/blocto/solana-go-sdk/client"
	"github.com/blocto/solana-go-sdk/rpc"
	"github.com/zeromicro/go-zero/core/logx"
)

// ProgramWatcher indexes every transaction of the programs this DEX
// launches tokens on (PumpMeteora, PumpMeteora V2) by signature, on top of
// the slot scanner. The scanner has to fetch every block on the chain and
// is bounded by RPC throughput — under load it fell 2–3 minutes behind and
// lost slots outright — while these programs see a handful of transactions
// per minute. Polling getSignaturesForAddress per program and fetching
// only those transactions costs ~1 RPC call per trade and lands a trade or
// token create in the index within a couple of seconds, for every token on
// these programs, independent of scanner backlog. Trade persistence
// de-duplicates on tx_hash, so both paths seeing the same transaction is
// harmless. The block row is deliberately not written here (see
// processBlockInfo) so the scanner's absent-slot backfill stays correct.
type ProgramWatcher struct {
	*BlockService
	programs []string
	seen     map[string]struct{}
	seenMu   sync.Mutex
	started  time.Time
	rpc      *client.Client // nil → round-robin consumer clients
}

const (
	watcherPollInterval = 3 * time.Second
	watcherSigLimit     = 25
	watcherSeenCap      = 5000
)

func NewProgramWatcher(sc *svc.ServiceContext) *ProgramWatcher {
	bs := NewBlockService(sc, "program-watcher", make(chan uint64), 0)
	w := &ProgramWatcher{
		BlockService: bs,
		programs:     []string{ProgramStrPumpMeteora, ProgramStrPumpMeteoraOpt},
		seen:         make(map[string]struct{}),
		started:      time.Now(),
	}
	// The watcher makes ~1 call/s; give it the primary (SOL_NODE_URL) endpoint
	// rather than the scanner's CONSUMER_SOL_NODE_URL, which is the one being
	// hammered with getBlock and rate-limited.
	if u := strings.TrimSpace(sc.Config.Sol.NodeUrlEnv); u != "" {
		if i := strings.Index(u, ","); i > 0 {
			u = u[:i]
		}
		w.rpc = client.NewClient(u)
	}
	return w
}

func (w *ProgramWatcher) Start() {
	w.Infof("program watcher: watching %v", w.programs)
	ticker := time.NewTicker(watcherPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			for _, program := range w.programs {
				w.poll(program)
			}
		}
	}
}

func (w *ProgramWatcher) Stop() {
	w.cancel(constants.ErrServiceStop)
}

func (w *ProgramWatcher) markSeen(sig string) bool {
	w.seenMu.Lock()
	defer w.seenMu.Unlock()
	if _, ok := w.seen[sig]; ok {
		return false
	}
	if len(w.seen) >= watcherSeenCap {
		w.seen = make(map[string]struct{})
	}
	w.seen[sig] = struct{}{}
	return true
}

func (w *ProgramWatcher) poll(program string) {
	ctx, cancel := context.WithTimeout(w.ctx, 20*time.Second)
	defer cancel()

	c := w.rpc
	if c == nil {
		c = w.sc.GetSolClient()
	}
	var sigs rpc.GetSignaturesForAddress
	var err error
	for attempt := 1; attempt <= 4; attempt++ {
		sigs, err = c.GetSignaturesForAddressWithConfig(ctx, program, client.GetSignaturesForAddressConfig{
			Limit:      watcherSigLimit,
			Commitment: rpc.CommitmentConfirmed,
		})
		if err == nil || !strings.Contains(err.Error(), "429") {
			break
		}
		time.Sleep(time.Duration(400*attempt) * time.Millisecond)
	}
	if err != nil {
		w.Errorf("program watcher: getSignaturesForAddress %s: %v", program, err)
		return
	}

	// Oldest first so trades are persisted in chain order.
	for i := len(sigs) - 1; i >= 0; i-- {
		sg := sigs[i]
		if sg.Err != nil {
			continue
		}
		// No age cutoff: the last N signatures per program are cheap to
		// re-check (seen-set + DB dedupe), and a create that landed just
		// before a restart must still be picked up.
		if !w.markSeen(sg.Signature) {
			continue
		}
		w.indexSignature(ctx, c, sg)
	}
}

func (w *ProgramWatcher) indexSignature(ctx context.Context, c *client.Client, sg rpc.SignatureWithStatus) {
	var tx *client.Transaction
	var err error
	for attempt := 1; attempt <= 4; attempt++ {
		tx, err = c.GetTransactionWithConfig(ctx, sg.Signature, client.GetTransactionConfig{Commitment: rpc.CommitmentConfirmed})
		if err == nil || !strings.Contains(err.Error(), "429") {
			break
		}
		time.Sleep(time.Duration(400*attempt) * time.Millisecond)
	}
	if err != nil || tx == nil || tx.Meta == nil {
		w.Errorf("program watcher: getTransaction %s: %v", sg.Signature, err)
		w.seenMu.Lock()
		delete(w.seen, sg.Signature) // retry next poll
		w.seenMu.Unlock()
		return
	}
	if tx.Meta.Err != nil {
		return
	}

	blockTime := time.Now()
	if tx.BlockTime != nil {
		blockTime = time.Unix(*tx.BlockTime, 0)
	}
	bt := blockTime
	slot := int64(tx.Slot)
	block := &solmodel.Block{
		Slot:      slot,
		BlockTime: blockTime,
		Status:    constants.BlockProcessed,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	blockInfo := &client.Block{
		BlockTime:  &bt,
		ParentSlot: tx.Slot - 1,
		Transactions: []client.BlockTransaction{{
			Meta:        tx.Meta,
			Transaction: tx.Transaction,
			AccountKeys: tx.AccountKeys,
		}},
	}

	w.Logger = logx.WithContext(w.ctx).WithFields(logx.Field("service", "program-watcher"), logx.Field("slot", slot))
	w.Infof("program watcher: indexing %s (slot %d)", sg.Signature, slot)
	w.processBlockInfo(ctx, block, blockInfo, slot, time.Now(), false)
}
