package solmodel

import (
	"context"
	"fmt"
	"sync"

	. "github.com/klen-ygs/gorm-zero/gormc/sql"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// avoid unused err
var _ = InitField
var _ SolAccountModel = (*customSolAccountModel)(nil)
var solSolAccountModelLock sync.Mutex

type (
	// SolAccountModel is an interface to be customized, add more methods here,
	// and implement the added methods in customSolAccountModel.
	SolAccountModel interface {
		solAccountModel
		customSolAccountLogicModel
	}

	customSolAccountLogicModel interface {
		WithSession(tx *gorm.DB) SolAccountModel
		BatchInsertAccounts(ctx context.Context, accounts []*SolAccount) error
	}

	customSolAccountModel struct {
		*defaultSolAccountModel
	}
)

func (c customSolAccountModel) WithSession(tx *gorm.DB) SolAccountModel {
	newModel := *c.defaultSolAccountModel
	c.defaultSolAccountModel = &newModel
	c.conn = tx
	return c
}

// NewSolAccountModel returns a model for the database table.
func NewSolAccountModel(conn *gorm.DB) SolAccountModel {
	return &customSolAccountModel{
		defaultSolAccountModel: newSolAccountModel(conn),
	}
}

func (m *defaultSolAccountModel) customCacheKeys(data *SolAccount) []string {
	if data == nil {
		return []string{}
	}
	return []string{}
}

func (m *defaultSolAccountModel) BatchInsertAccounts(ctx context.Context, accounts []*SolAccount) error {

	if len(accounts) == 0 {
		return nil
	}

	solSolAccountModelLock.Lock()
	defer solSolAccountModelLock.Unlock()

	err := m.conn.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "address"}}, // 用于匹配唯一键
		DoUpdates: clause.Assignments(map[string]interface{}{
			"balance":    gorm.Expr("CASE WHEN VALUES(slot) > slot THEN VALUES(balance) ELSE balance END"),
			"slot":       gorm.Expr("CASE WHEN VALUES(slot) > slot THEN VALUES(slot) ELSE slot END"),
			"updated_at": gorm.Expr("CASE WHEN VALUES(slot) > slot THEN VALUES(updated_at) ELSE updated_at END"),
		}),
	}).CreateInBatches(accounts, 10).Error

	return errors.Wrap(err, fmt.Sprintf("BatchInsertAccounts error after get lock: %v", len(accounts)))
}
