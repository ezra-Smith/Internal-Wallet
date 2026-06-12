package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// FindConfigFile 通用配置文件查找函数
// 适用于所有微服务，自动在不同位置查找配置文件
func FindConfigFile(relativePath string) string {
	// 获取当前执行文件的路径
	execPath, err := os.Executable()
	if err != nil {
		execPath = os.Args[0] // 如果获取失败，使用第一个参数
	}

	// 获取可执行文件所在目录
	execDir := filepath.Dir(execPath)

	// 获取当前文件所在目录（源代码位置）
	_, filename, _, _ := runtime.Caller(1)
	sourceDir := filepath.Dir(filename)

	// 获取当前工作目录
	wd, _ := os.Getwd()

	// 获取服务名称
	serviceName := GetServiceName(sourceDir)

	// 定义可能的配置文件位置（按优先级排序）
	possiblePaths := []string{
		// 1. 当前工作目录（最常见，IDE运行时通常从这里开始）
		filepath.Join(wd, relativePath),

		// 2. 源代码目录下的相对位置（IDE编译后的位置）
		filepath.Join(sourceDir, relativePath),

		// 3. 可执行文件目录下的相对位置（命令行运行时）
		filepath.Join(execDir, relativePath),

		// 4. 当前工作目录的 services 结构（从项目根目录运行）
		filepath.Join(wd, "services", serviceName, "rpc", relativePath),

		// 5. 源代码向上查找项目根目录（从任意目录运行）
		FindProjectRootConfig(sourceDir, serviceName, relativePath),

		// 6. GOPATH 或工作空间模式
		findWorkspaceConfig(wd, serviceName, relativePath),
	}

	// 尝试每个可能的路径
	for _, path := range possiblePaths {
		if path == "" {
			continue
		}

		if _, err := os.Stat(path); err == nil {
			fmt.Printf("✓ 找到配置文件: %s\n", path)
			return path
		}
	}

	return "" // 未找到
}

// GetServiceName 从当前路径提取服务名称
func GetServiceName(sourceDir string) string {
	pathParts := strings.Split(sourceDir, string(filepath.Separator))

	// 查找 services 目录后的服务名
	for i, part := range pathParts {
		if part == "services" && i+1 < len(pathParts) {
			return pathParts[i+1]
		}
	}

	// 如果在 services 目录中，提取父目录名作为服务名
	for i := len(pathParts) - 1; i >= 0; i-- {
		if pathParts[i] == "rpc" && i > 0 {
			return pathParts[i-1]
		}
	}

	return "unknown"
}

// FindProjectRootConfig 从源代码目录向上查找项目根目录
func FindProjectRootConfig(sourceDir, serviceName, relativePath string) string {
	dir := sourceDir

	// 最多向上查找5级目录
	for i := 0; i < 5; i++ {
		// 检查是否有 go.mod 文件（项目根目录标识）
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			// 在项目根目录下查找配置文件
			configPath := filepath.Join(dir, relativePath)
			if _, err := os.Stat(configPath); err == nil {
				return configPath
			}

			// 在项目根目录的 services 结构中查找
			servicesConfigPath := filepath.Join(dir, "services", serviceName, "rpc", relativePath)
			if _, err := os.Stat(servicesConfigPath); err == nil {
				return servicesConfigPath
			}
		}

		// 检查是否有 services 目录
		if _, err := os.Stat(filepath.Join(dir, "services")); err == nil {
			// 尝试在 services/{服务名}/rpc 下查找
			servicesConfigPath := filepath.Join(dir, "services", serviceName, "rpc", relativePath)
			if _, err := os.Stat(servicesConfigPath); err == nil {
				return servicesConfigPath
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // 已经到达根目录
		}
		dir = parent
	}

	return ""
}

// findWorkspaceConfig 在工作空间模式中查找配置文件
func findWorkspaceConfig(wd, serviceName, relativePath string) string {
	// 尝试常见的工作空间结构
	workspacePaths := []string{
		filepath.Join(wd, "src", serviceName, "rpc", relativePath),
		filepath.Join(wd, "pkg", serviceName, "rpc", relativePath),
	}

	for _, path := range workspacePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// PrintConfigNotFoundError 打印配置文件未找到的错误信息和帮助
func PrintConfigNotFoundError(relativePath string) {
	fmt.Printf("❌ 配置文件 %s 未找到\n\n", relativePath)
	fmt.Printf("请尝试以下方法之一：\n")
	fmt.Printf("1. 在当前工作目录创建配置文件: %s\n", relativePath)
	fmt.Printf("2. 在服务根目录创建配置文件: services/{服务名}/rpc/%s\n", relativePath)
	fmt.Printf("3. 在项目根目录创建配置文件: %s\n", relativePath)
	fmt.Printf("4. 使用 -f 参数指定配置文件路径: -f /path/to/config.yaml\n")
	fmt.Printf("5. 从正确的目录启动服务\n\n")

	fmt.Printf("支持的配置文件位置：\n")
	fmt.Printf("- ./etc/account.yaml (当前工作目录)\n")
	fmt.Printf("- services/account/rpc/etc/account.yaml (服务目录)\n")
	fmt.Printf("- /path/to/project/etc/account.yaml (项目根目录)\n")
}
