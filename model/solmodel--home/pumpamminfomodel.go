package solmodel__home

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ PumpAmmInfoModel = (*customPumpAmmInfoModel)(nil)

type (
	// PumpAmmInfoModel is an interface to be customized, add more methods here,
	// and implement the added methods in customPumpAmmInfoModel.
	PumpAmmInfoModel interface {
		pumpAmmInfoModel
		withSession(session sqlx.Session) PumpAmmInfoModel
	}

	customPumpAmmInfoModel struct {
		*defaultPumpAmmInfoModel
	}
)

// NewPumpAmmInfoModel returns a model for the database table.
func NewPumpAmmInfoModel(conn sqlx.SqlConn) PumpAmmInfoModel {
	return &customPumpAmmInfoModel{
		defaultPumpAmmInfoModel: newPumpAmmInfoModel(conn),
	}
}

func (m *customPumpAmmInfoModel) withSession(session sqlx.Session) PumpAmmInfoModel {
	return NewPumpAmmInfoModel(sqlx.NewSqlConnFromSession(session))
}
