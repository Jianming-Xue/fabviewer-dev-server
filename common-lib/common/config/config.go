package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// DefaultConfigFile 是未设置 {appName}_CFG 环境变量时的缺省配置文件路径
// （相对工作目录，即业务服务的项目根目录）。
const DefaultConfigFile = "application.yml"

// ErrInvalidConfigFile 表示配置文件不是受支持的格式（仅支持 yml/yaml/json）。
var ErrInvalidConfigFile = errors.New("非法的配置文件")

// LoadConfig 加载业务服务的配置文件并返回嵌套 map。文件位置取环境变量
// {appName}_CFG（如 appName 为 security 时即 security_CFG），未设置时用
// 工作目录下的 application.yml。加载完成后逐叶子应用环境变量覆盖（如
// security.log.level），未设置对应环境变量时保留文件原值。
func LoadConfig(appName string) (map[string]any, error) {
	path := os.Getenv(appName + "_CFG")
	if path == "" {
		path = DefaultConfigFile
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yml", ".yaml", ".json":
	default:
		return nil, fmt.Errorf("%w: %s", ErrInvalidConfigFile, path)
	}

	//nolint:gosec //G304：路径来自环境变量 {appName}_CFG 或缺省值，配置加载器的固有行为
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var m map[string]any
	if ext == ".json" {
		err = json.Unmarshal(data, &m)
	} else {
		err = yaml.Unmarshal(data, &m)
	}
	if err != nil {
		return nil, fmt.Errorf("配置文件 %s 解析失败: %w", path, err)
	}
	if m == nil {
		m = map[string]any{} // 空文件：等价于空配置，后续读取按未命中处理
	}

	applyEnvOverrides(appName, m)
	return m, nil
}

// applyEnvOverrides 沿嵌套 map 的每个叶子路径查环境变量并就地覆盖：
// 从叶子 key 回溯到顶层 key 以点连接，再加上 appName 前缀组成环境变量名
// （如 log.file 下的 maxSizeMB 叶子 → security.log.file.maxSizeMB）。
// 环境变量存在即覆盖（值为字符串原样放入 map，类型转换推迟到绑定阶段），
// 不存在则保留原值。
func applyEnvOverrides(appName string, m map[string]any) {
	overrideFromEnv(appName, m)
}

// overrideFromEnv 递归处理 prefix 前缀下的 map：子 map 继续下钻，叶子值
// 查 "<prefix>.<key>" 环境变量决定是否覆盖。
func overrideFromEnv(prefix string, m map[string]any) {
	for k, v := range m {
		if child, ok := v.(map[string]any); ok {
			overrideFromEnv(prefix+"."+k, child)
			continue
		}
		if val, ok := os.LookupEnv(prefix + "." + k); ok {
			m[k] = val
		}
	}
}
