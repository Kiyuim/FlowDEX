package solmodel

import (
	. "github.com/klen-ygs/gorm-zero/gormc/sql"
	"gorm.io/gorm"
)

// avoid unused err
var _ = InitField
var _ UserTokensModel = (*customUserTokensModel)(nil)

type (
	// UserTokensModel is an interface to be customized, add more methods here,
	// and implement the added methods in customUserTokensModel.
	UserTokensModel interface {
		userTokensModel
		customUserTokensLogicModel
	}

	customUserTokensLogicModel interface {
		WithSession(tx *gorm.DB) UserTokensModel
	}

	customUserTokensModel struct {
		*defaultUserTokensModel
	}
)

func (c customUserTokensModel) WithSession(tx *gorm.DB) UserTokensModel {
	newModel := *c.defaultUserTokensModel
	c.defaultUserTokensModel = &newModel
	c.conn = tx
	return c
}

// NewUserTokensModel returns a model for the database table.
func NewUserTokensModel(conn *gorm.DB) UserTokensModel {
	return &customUserTokensModel{
		defaultUserTokensModel: newUserTokensModel(conn),
	}
}

func (m *defaultUserTokensModel) customCacheKeys(data *UserTokens) []string {
	if data == nil {
		return []string{}
	}
	return []string{}
}
