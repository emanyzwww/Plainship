package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/emanyzwww/Plainship/internal/builder"
	"github.com/emanyzwww/Plainship/internal/manifest"
	"github.com/emanyzwww/Plainship/internal/space"
	"github.com/emanyzwww/Plainship/internal/state"
)

// buildForTest 在 Space 上执行一次构建.
func buildForTest(t *testing.T, s *space.Space) *manifest.Manifest {
	t.Helper()
	res, err := builder.Build(s, nil)
	if err != nil {
		t.Fatalf("构建失败: %v", err)
	}
	return res.Manifest
}

// stateFor 读取同步状态.
func stateFor(root string) *state.SyncState {
	return state.LoadSyncState(root)
}

// saveStateFor 写入同步状态.
func saveStateFor(t *testing.T, root string, ss *state.SyncState) {
	t.Helper()
	if err := state.SaveSyncState(root, ss); err != nil {
		t.Fatal(err)
	}
}

// jsonMarshal 序列化.
func jsonMarshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

var (
	_ = os.Getenv
	_ = filepath.Join
)
