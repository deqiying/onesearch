package config

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const maxSnapshotConfigBytes = 4 << 20

var errReadOnlySnapshot = errors.New("配置快照只读")

type readOnlySnapshot struct {
	raw      map[string]any
	env      map[string]string
	pathInfo PathInfo
	missing  bool
	err      error
}

// LoadReadOnlySnapshot 捕获单次请求的文件和环境视图，不初始化或写入配置。
// 即使读取失败也返回带元数据的非 nil Config；调用方必须处理返回的错误。
func LoadReadOnlySnapshot() (*Config, error) {
	// 完整环境只保存在私有字段，支持自定义 api_key_env，且不暴露到元数据。
	env := make(map[string]string)
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			if runtime.GOOS == "windows" {
				key = strings.ToUpper(key)
			}
			env[key] = value
		}
	}
	homeKey := "HOME"
	if runtime.GOOS == "windows" {
		homeKey = "USERPROFILE"
	} else if runtime.GOOS == "plan9" {
		homeKey = "home"
	}
	defDir := filepath.Join(".", ".onesearch")
	if home := env[homeKey]; home != "" {
		defDir = filepath.Join(home, ".config", "onesearch")
	}
	dir, source := defDir, "default"
	override := strings.TrimSpace(env[ConfigDirEnvName])
	if override != "" {
		dir, source = filepath.Clean(override), "environment"
	}
	path := filepath.Join(dir, "config.json")
	snapshot := &readOnlySnapshot{
		raw: map[string]any{}, env: env,
		pathInfo: PathInfo{
			OK: true, ConfigFile: path, ConfigDir: filepath.Dir(path),
			ConfigDirSource: source, DefaultConfigFile: filepath.Join(defDir, "config.json"),
			ConfigDirOverrideValue:          override,
			ConfigDirOverrideMatchesDefault: override != "" && samePath(override, defDir),
		},
	}
	c := &Config{ConfigFile: path, ConfigDirSource: source, snapshot: snapshot}
	fail := func(message string) (*Config, error) {
		// 不包装底层错误：PathError 和解析错误可能包含路径或凭据。
		snapshot.err = errors.New(message)
		c.InitializationError = message
		return c, snapshot.err
	}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		snapshot.missing = true
		c.MissingConfigFile = true
		return c, nil
	}
	snapshot.pathInfo.Exists = true
	if err != nil || !info.Mode().IsRegular() {
		return fail("无法读取配置快照")
	}
	if info.Size() > maxSnapshotConfigBytes {
		return fail("配置快照超过大小限制")
	}
	file, err := os.Open(path)
	if err != nil {
		return fail("无法读取配置快照")
	}
	// 即使 stat 后文件增长，读取和分配也不会超过上限加一个字节。
	body, readErr := io.ReadAll(io.LimitReader(file, maxSnapshotConfigBytes+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return fail("无法读取配置快照")
	}
	if len(body) > maxSnapshotConfigBytes {
		return fail("配置快照超过大小限制")
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return fail("配置快照不是有效 JSON object")
	}
	if !isRuntimeSchema(raw) || raw["schema_version"] != float64(1) {
		return fail("配置快照不是可识别的 runtime schema")
	}
	snapshot.raw = raw
	return c, nil
}

// 快照原始数据仅来自 JSON，因此递归复制 object 和 array 即可隔离所有可变值。
func cloneSnapshotMap(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = cloneSnapshotValue(value)
	}
	return out
}

func cloneSnapshotValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneSnapshotMap(typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = cloneSnapshotValue(item)
		}
		return out
	default:
		return value
	}
}
