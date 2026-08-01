// Package config_test 从 Kernel 契约视角验证 Snapshot 的不可变值语义。
package config_test

import (
	"crypto/sha256"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/rin721/micro-go/internal/kernel/config"
)

type value struct {
	Items map[string]string `json:"items"`
}

// TestSnapshotValueReturnsDeepCopy 修改第一次读取结果后再次读取，证明 Snapshot 内部数据
// 不会被切片、map 或指针别名反向污染。
func TestSnapshotValueReturnsDeepCopy(t *testing.T) {
	original := value{Items: map[string]string{"a": "b"}}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
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
