package database

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"testing"
)

// The first migration must stay byte for byte the file core shipped
// (unlocked-core's modules/learningrecord/migrations, release 1.10.0): every
// site that ran it inside core has it in the ledger, so a change here would
// never run there and the sites would drift apart. Add a new migration
// instead.
func TestMigrationsUnchanged(t *testing.T) {
	want := map[string]string{
		"migrations/00001_move_into_learning_record_schema.sql": "f3606a92d4cd5895b80740e8bc6583bdd8efa6c27f41bc123cce4d22e4b9ca36",
	}
	for path, sum := range want {
		b, err := fs.ReadFile(migrations, path)
		if err != nil {
			t.Fatal(err)
		}
		got := sha256.Sum256(b)
		if hex.EncodeToString(got[:]) != sum {
			t.Errorf("%s changed: never edit a merged migration, add a new one", path)
		}
	}
}
