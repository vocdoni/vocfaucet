package storage

import (
	"testing"
	"time"
)

func TestAddFundedUserWithWaitTime(t *testing.T) {
	// Create a new storage instance
	st, err := New("pebble", t.TempDir(), time.Second*2, []byte("prefix"))
	if err != nil {
		t.Fatalf("failed to create storage instance: %v", err)
	}
	defer st.Close()

	// Define test data
	userID := []byte("user123")
	authType := "email"

	// Add funded user with wait time
	err = st.AddFundedUserWithWaitTime(userID, authType)
	if err != nil {
		t.Fatalf("failed to add funded user: %v", err)
	}

	// Retrieve wait period end time
	funded, _ := st.CheckFundedUserWithWaitTime(userID, authType)
	if !funded {
		t.Fatalf("expected user to be funded, but it is not")
	}

	// Verify wait period end time
	time.Sleep(time.Second * 3)
	funded, _ = st.CheckFundedUserWithWaitTime(userID, authType)
	if funded {
		t.Fatalf("expected user to be not funded, but it is not")
	}

}
