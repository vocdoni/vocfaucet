package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"time"

	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/db/prefixeddb"
	"go.vocdoni.io/dvote/log"
)

type storage struct {
	kv                db.Database
	waitPeriodSeconds uint64
}

func newStorage(dbType string, dataDir string, waitPeriod time.Duration, dbPrefix []byte) (*storage, error) {
	if dbType != db.TypePebble && dbType != db.TypeLevelDB && dbType != db.TypeMongo {
		return nil, fmt.Errorf("invalid dbType: %q. Available types: %q %q %q", dbType, db.TypePebble, db.TypeLevelDB, db.TypeMongo)
	}
	log.Infow("create db storage", "type", dbType, "dir", dataDir, "prefix", hex.EncodeToString(dbPrefix))
	st := &storage{}
	var err error
	mdb, err := metadb.New(dbType, filepath.Join(filepath.Clean(dataDir), "db"))
	if err != nil {
		return nil, err
	}

	st.kv = prefixeddb.NewPrefixedDatabase(mdb, dbPrefix)
	st.waitPeriodSeconds = uint64(waitPeriod.Seconds())
	return st, nil
}

// addFunded adds the given text to the funded list, with the current time
// as the wait period end time.
func (st *storage) addFunded(text []byte, authType string) error {
	tx := st.kv.WriteTx()
	defer tx.Discard()
	key := append(text, []byte(authType)...)
	wp := uint64(time.Now().Unix()) + st.waitPeriodSeconds
	wpBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(wpBytes, wp)
	if err := tx.Set(key, wpBytes); err != nil {
		log.Error(err)
	}
	return tx.Commit()
}

// checkIsFunded checks if the given text is funded and returns true if it is, within
// the wait period time window. Otherwise, it returns false.
func (st *storage) checkIsFunded(text []byte, authType string) (bool, time.Time) {
	key := append(text, []byte(authType)...)
	wpBytes, err := st.kv.Get(key)
	if err != nil {
		return false, time.Time{}
	}
	wp := binary.LittleEndian.Uint64(wpBytes)
	return wp >= uint64(time.Now().Unix()), time.Unix(int64(wp), 0)
}
