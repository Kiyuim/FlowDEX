package solmodel

import (
	"context"
	"dex/pkg/constants"
	"errors"

	"github.com/klen-ygs/gorm-zero/gormc"
	. "github.com/klen-ygs/gorm-zero/gormc/sql"
	"gorm.io/gorm"
)

// avoid unused err
var _ = InitField
var _ BlockModel = (*customBlockModel)(nil)

type (
	// BlockModel is an interface to be customized, add more methods here,
	// and implement the added methods in customBlockModel.
	BlockModel interface {
		blockModel
		customBlockLogicModel
	}

	customBlockLogicModel interface {
		WithSession(tx *gorm.DB) BlockModel
		FindOneByNearSlot(ctx context.Context, slot int64) (*Block, error)
		FindLastSuccessBlock(ctx context.Context) (*Block, error)
		FindFirstFailBlock(ctx context.Context) (*Block, error)
		FindProcessingSlots(ctx context.Context, slot int64, limit int) ([]*Block, error)
		FindSlotsSince(ctx context.Context, since int64) ([]int64, error)
	}

	customBlockModel struct {
		*defaultBlockModel
	}
)

func (c customBlockModel) WithSession(tx *gorm.DB) BlockModel {
	newModel := *c.defaultBlockModel
	c.defaultBlockModel = &newModel
	c.conn = tx
	return c
}

// NewBlockModel returns a model for the database table.
func NewBlockModel(conn *gorm.DB) BlockModel {
	return &customBlockModel{
		defaultBlockModel: newBlockModel(conn),
	}
}

func (m *defaultBlockModel) customCacheKeys(data *Block) []string {
	if data == nil {
		return []string{}
	}
	return []string{}
}

func (m *defaultBlockModel) FindOneByNearSlot(ctx context.Context, slot int64) (*Block, error) {
	var resp Block
	err := m.conn.WithContext(ctx).Model(&Block{}).Where("`slot` < ? and `status` = ?", slot, constants.BlockProcessed).Order("slot desc").First(&resp).Error
	return &resp, err
}

func (m *defaultBlockModel) FindLastSuccessBlock(ctx context.Context) (*Block, error) {
	var resp Block
	err := m.conn.WithContext(ctx).Model(&Block{}).Where("`status` = ?", constants.BlockProcessed).
		Order("slot desc").First(&resp).Error
	if errors.Is(err, gormc.ErrNotFound) {
		return nil, err
	}
	return &resp, err
}

func (m *defaultBlockModel) FindFirstFailBlock(ctx context.Context) (*Block, error) {
	var resp Block
	err := m.conn.WithContext(ctx).Model(&Block{}).Where("`status` = ?", constants.BlockFailed).First(&resp).Error
	if err == gormc.ErrNotFound {
		return nil, err
	}
	return &resp, err
}

func (m *defaultBlockModel) FindProcessingSlots(ctx context.Context, slot int64, limit int) ([]*Block, error) {
	var resp []*Block
	err := m.conn.WithContext(ctx).
		Where("status = ? and slot >= ?", constants.BlockFailed, slot).
		Order("slot desc").
		Limit(limit).
		Find(&resp).Error

	switch err {
	case nil:
		return resp, nil
	case gormc.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

// FindSlotsSince returns every slot number with a block row at or above
// `since`, so callers can detect slots that never entered the pipeline.
func (m *defaultBlockModel) FindSlotsSince(ctx context.Context, since int64) ([]int64, error) {
	var slots []int64
	err := m.conn.WithContext(ctx).Model(&Block{}).Where("slot >= ?", since).Order("slot asc").Pluck("slot", &slots).Error
	return slots, err
}
