package postgres

import (
	"database/sql"
	"fmt"

	pgutil "github.com/KyberNetwork/reserve-data/common/postgres"
	"github.com/KyberNetwork/reserve-data/reservesetting/common"
)

// SetGeneralData ...
func (s *Storage) SetGeneralData(data common.GeneralData) (uint64, error) {
	var (
		id uint64
	)
	tx, err := s.db.Beginx()
	if err != nil {
		return 0, err
	}
	defer pgutil.RollbackUnlessCommitted(tx)

	err = tx.NamedStmt(s.stmts.setGeneralData).Get(&id, data)
	if err != nil {
		return 0, fmt.Errorf("failed to set data, err=%s,", err)
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

// GetGeneralData ...
func (s *Storage) GetGeneralData(key string) (common.GeneralData, error) {
	var (
		data common.GeneralData
	)
	tx, err := s.db.Beginx()
	if err != nil {
		return data, err
	}
	defer pgutil.RollbackUnlessCommitted(tx)
	err = tx.Stmtx(s.stmts.getGeneralData).Get(&data, key)
	if err != nil {
		if err == sql.ErrNoRows {
			return data, common.ErrNotFound
		}
		return data, fmt.Errorf("failed to get data, err=%s,", err)
	}
	return data, nil
}
