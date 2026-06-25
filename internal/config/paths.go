package config

import (
	"os"
	"path/filepath"
)

// AppDirName 是 Suna 默认数据目录名；所有默认运行态路径都从 DefaultDataDir 派生。
const AppDirName = ".suna"

func DefaultDataDir() string {
	homeDir, _ := os.UserHomeDir()
	if homeDir == "" {
		return AppDirName
	}
	return filepath.Join(homeDir, AppDirName)
}

func DataDirConfigPath(dataDir string) string      { return filepath.Join(dataDir, "config.toml") }
func DataDirCredentialsPath(dataDir string) string { return filepath.Join(dataDir, "credentials.toml") }
func DataDirLogsDir(dataDir string) string         { return filepath.Join(dataDir, "logs") }
func DataDirLogPath(dataDir string) string         { return filepath.Join(DataDirLogsDir(dataDir), "app.log") }
func DataDirSkillsDir(dataDir string) string       { return filepath.Join(dataDir, "skills") }
func DataDirDBPath(dataDir string) string          { return filepath.Join(dataDir, "memory.db") }
func DataDirPIDPath(dataDir string) string         { return filepath.Join(dataDir, "sunad.pid") }
func DataDirSocketPath(dataDir string) string      { return filepath.Join(dataDir, "sunad.sock") }
func DataDirAttachmentsDir(dataDir string) string  { return filepath.Join(dataDir, "attachments") }

func DefaultConfigPath() string      { return DataDirConfigPath(DefaultDataDir()) }
func DefaultCredentialsPath() string { return DataDirCredentialsPath(DefaultDataDir()) }
func DefaultLogsDir() string         { return DataDirLogsDir(DefaultDataDir()) }
func DefaultLogPath() string         { return DataDirLogPath(DefaultDataDir()) }
func DefaultSkillsDir() string       { return DataDirSkillsDir(DefaultDataDir()) }
func DefaultDBPath() string          { return DataDirDBPath(DefaultDataDir()) }
func DefaultPIDPath() string         { return DataDirPIDPath(DefaultDataDir()) }
func DefaultSocketPath() string      { return DataDirSocketPath(DefaultDataDir()) }
func DefaultAttachmentsDir() string  { return DataDirAttachmentsDir(DefaultDataDir()) }

func (c *Config) DBPath() string          { return DataDirDBPath(c.DataDir) }
func (c *Config) ConfigPath() string      { return DataDirConfigPath(c.DataDir) }
func (c *Config) CredentialsPath() string { return DataDirCredentialsPath(c.DataDir) }
func (c *Config) LogsDir() string         { return DataDirLogsDir(c.DataDir) }
func (c *Config) LogPath() string         { return DataDirLogPath(c.DataDir) }
func (c *Config) SkillsDir() string       { return DataDirSkillsDir(c.DataDir) }
func (c *Config) PIDPath() string         { return DataDirPIDPath(c.DataDir) }
func (c *Config) SocketPath() string      { return DataDirSocketPath(c.DataDir) }
func (c *Config) AttachmentsDir() string  { return DataDirAttachmentsDir(c.DataDir) }

// ProjectDirName 是 Suna 项目级配置的目录名，与全局 AppDirName 保持风格一致。
// 项目级 config.toml 默认路径为 <cwd>/.suna/config.toml。
const ProjectDirName = ".suna"
const ProjectConfigFileName = "config.toml"

// FindProjectConfigPath 从 cwd 向上查找最近的 .suna/config.toml，找到即返回。
// 遍历到文件系统根目录才终止；任何一级找到即立即返回。
// 没找到或 cwd 为空时返回空字符串。
func FindProjectConfigPath(cwd string) string {
	if cwd == "" {
		return ""
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return ""
	}
	dir := abs
	for {
		candidate := filepath.Join(dir, ProjectDirName, ProjectConfigFileName)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
