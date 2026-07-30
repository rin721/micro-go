package config_test

import (
	"crypto/sha256"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/rin721/micro-go/kernel/config"
)

type value struct {
	Items map[string]string `json:"items"`
}

func TestSnapshotValueReturnsDeepCopy(t *testing.T) {
	original := value{Items: map[string]string{"a": "b"}}
	data, _ := json.Marshal(original)
	typeOf := reflect.TypeOf(original)
	snapshot := config.NewSnapshot(1, time.Now(), []config.SnapshotEntry{{Type: typeOf, Data: data, Hash: sha256.Sum256(data)}})
	first, err := config.Value[value](snapshot)
	if err != nil {
		t.Fatal(err)
	}
	first.Items["a"] = "changed"
	second, err := config.Value[value](snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if second.Items["a"] != "b" {
		t.Fatalf("snapshot mutated: %#v", second)
	}
}
