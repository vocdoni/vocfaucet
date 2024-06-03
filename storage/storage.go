package storage

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

// Storage is a key-value storage for the faucet.
type Storage struct {
	kv                db.Database
	waitPeriodSeconds uint64
}

// New creates a new storage instance.
func New(dbType string, dataDir string, waitPeriod time.Duration, dbPrefix []byte) (*Storage, error) {
	if dbType != db.TypePebble && dbType != db.TypeLevelDB && dbType != db.TypeMongo {
		return nil, fmt.Errorf("invalid dbType: %q. Available types: %q %q %q",
			dbType, db.TypePebble, db.TypeLevelDB, db.TypeMongo)
	}
	log.Infow("create db storage", "type", dbType, "dir", dataDir, "prefix", hex.EncodeToString(dbPrefix))
	st := &Storage{}
	var err error
	mdb, err := metadb.New(dbType, filepath.Join(filepath.Clean(dataDir), "db"))
	if err != nil {
		return nil, err
	}

	st.kv = prefixeddb.NewPrefixedDatabase(mdb, dbPrefix)
	st.waitPeriodSeconds = uint64(waitPeriod.Seconds())
	return st, nil
}

// Set sets the given key to the given value.
func (st *Storage) Set(key, value []byte) error {
	tx := st.kv.WriteTx()
	defer tx.Discard()
	if err := tx.Set(key, value); err != nil {
		return fmt.Errorf("failed to set key %s: %w", key, err)
	}
	return tx.Commit()
}

// Get gets the value for the given key.
func (st *Storage) Get(key []byte) ([]byte, error) {
	return st.kv.Get(key)
}

// Close closes the storage.
func (st *Storage) Close() error {
	return st.kv.Close()
}

// AddFundedUserWithWaitTime adds the given userID to the funded list, with the current time
// as the wait period end time.
func (st *Storage) AddFundedUserWithWaitTime(userID []byte, authType string) error {
	tx := st.kv.WriteTx()
	defer tx.Discard()
	key := append(userID, []byte(authType)...)
	wp := uint64(time.Now().Unix()) + st.waitPeriodSeconds
	wpBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(wpBytes, wp)
	if err := tx.Set(key, wpBytes); err != nil {
		log.Error(err)
	}
	return tx.Commit()
}

// CheckFundedUserWithWaitTime checks if the given text is funded and returns true if it is, within
// the wait period time window. Otherwise, it returns false.
func (st *Storage) CheckFundedUserWithWaitTime(userID []byte, authType string) (bool, time.Time) {
	key := append(userID, []byte(authType)...)
	wpBytes, err := st.kv.Get(key)
	if err != nil {
		return false, time.Time{}
	}
	wp := binary.LittleEndian.Uint64(wpBytes)
	return wp >= uint64(time.Now().Unix()), time.Unix(int64(wp), 0)
}
