package main

import (
	"encoding/binary"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/log"
)

const (
	fundedAddressPrefix = "a_"
)

type storage struct {
	kv                db.Database
	waitPeriodSeconds uint64
}

func newStorage(dataDir string, waitPeriod time.Duration) (*storage, error) {
	st := &storage{}
	var err error
	st.kv, err = metadb.New(db.TypePebble, filepath.Join(filepath.Clean(dataDir), "db"))
	if err != nil {
		return nil, err
	}
	st.waitPeriodSeconds = uint64(waitPeriod.Seconds())
	return st, nil
}

// addFundedAddress adds the given address to the funded addresses list, with the current time
// as the wait period end time.
func (st *storage) addFundedAddress(addr common.Address) error {
	tx := st.kv.WriteTx()
	defer tx.Discard()
	wp := uint64(time.Now().Unix()) + st.waitPeriodSeconds
	wpBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(wpBytes, wp)
	if err := tx.Set(append([]byte(fundedAddressPrefix), addr.Bytes()...), wpBytes); err != nil {
		log.Error(err)
	}
	return tx.Commit()
}

// checkIsFundedAddress checks if the given address is funded and returns true if it is, within
// the wait period time window. Otherwise, it returns false.
// The second return value is the wait period end time, if the address is funded.
func (st *storage) checkIsFundedAddress(addr common.Address) (bool, time.Time) {
	wpBytes, err := st.kv.Get(append([]byte(fundedAddressPrefix), addr.Bytes()...))
	if err != nil {
		return false, time.Time{}
	}
	wp := binary.LittleEndian.Uint64(wpBytes)
	return wp >= uint64(time.Now().Unix()), time.Unix(int64(wp), 0)
}
