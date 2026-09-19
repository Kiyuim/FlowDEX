package solmodel

import (
	. "github.com/klen-ygs/gorm-zero/gormc/sql"
	"gorm.io/gorm"
)

// avoid unused err
var _ = InitField
var _ UserPoolsModel = (*customUserPoolsModel)(nil)

type (
	// UserPoolsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customUserPoolsModel.
	UserPoolsModel interface {
		userPoolsModel
		customUserPoolsLogicModel
	}

	customUserPoolsLogicModel interface {
		WithSession(tx *gorm.DB) UserPoolsModel
	}

	customUserPoolsModel struct {
		*defaultUserPoolsModel
	}
)

func (c customUserPoolsModel) WithSession(tx *gorm.DB) UserPoolsModel {
	newModel := *c.defaultUserPoolsModel
	c.defaultUserPoolsModel = &newModel
	c.conn = tx
	return c
}

// NewUserPoolsModel returns a model for the database table.
func NewUserPoolsModel(conn *gorm.DB) UserPoolsModel {
	return &customUserPoolsModel{
		defaultUserPoolsModel: newUserPoolsModel(conn),
	}
}

func (m *defaultUserPoolsModel) customCacheKeys(data *UserPools) []string {
	if data == nil {
		return []string{}
	}
	return []string{}
}
