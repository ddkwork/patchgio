package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	upstreamURL = "https://github.com/gioui/gio"
	forkURL     = "https://github.com/ddkwork/gio"
	ghUser      = "ddkwork"
	ghToken     = os.Getenv("GH_TOKEN")
)

type Dependency struct {
	URL  string
	Dir  string
	Name string
}

func main() {
	fmt.Println("开始更新 fork 仓库...")

	// 设置 git 配置
	runCmd("git", "config", "--global", "user.email", "action@github.com")
	runCmd("git", "config", "--global", "user.name", "GitHub Action")

	// 克隆上游仓库
	fmt.Println("\n1. 克隆上游仓库...")
	runCmd("git", "clone", "--depth=1", upstreamURL, "gio")
	os.Chdir("gio")

	// 复制补丁文件
	fmt.Println("\n2. 复制补丁文件...")
	runCmd("cp", "../dropfile.patch", ".")
	runCmd("cp", "../使用矩阵跟踪锚点绝对坐标.patch", ".")

	// 提交补丁文件
	commitFiles("添加补丁文件")

	// 应用第一个补丁
	fmt.Println("\n3. 应用 dropfile.patch...")
	runCmd("git", "apply", "dropfile.patch")
	commitFiles("拖放文件支持")

	// 应用第二个补丁
	fmt.Println("应用 使用矩阵跟踪锚点绝对坐标.patch...")
	runCmd("git", "apply", "使用矩阵跟踪锚点绝对坐标.patch")
	commitFiles("使用矩阵跟踪锚点绝对坐标")

	// 克隆依赖
	deps := []Dependency{
		{"https://github.com/gioui/gio-cmd", "cmd", "cmd"},
		{"https://github.com/gioui/gio-x", "x", "x"},
		{"https://github.com/gioui/gio-example", "example", "example"},
		{"https://git.sr.ht/~eliasnaur/gio-shader", "shader", "shader"},
		{"https://git.sr.ht/~eliasnaur/gio-cpu", "cpu", "cpu"},
	}

	for _, dep := range deps {
		cloneAndAddDependency(dep)
	}

	// 设置远程地址并推送
	fmt.Println("\n8. 设置远程地址并推送...")
	runCmd("git", "remote", "set-url", "origin", forkURL)

	pushURL := fmt.Sprintf("https://%s:%s@github.com/%s/gio.git", ghUser, ghToken, ghUser)
	fmt.Printf("强制推送到 %s...\n", pushURL)
	runCmd("git", "push", "--force", pushURL, "main")

	fmt.Println("\n更新完成!")
}

func cloneAndAddDependency(dep Dependency) {
	fmt.Printf("\n克隆 %s 到 %s 目录...\n", dep.URL, dep.Dir)

	// 克隆
	runCmd("git", "clone", dep.URL, dep.Dir)

	// 检查并删除 .git 目录
	gitDir := filepath.Join(dep.Dir, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		fmt.Printf("  删除 %s\n", gitDir)
		os.RemoveAll(gitDir)
	} else {
		fmt.Printf("  %s 不存在 (预期中)\n", gitDir)
	}

	// 检查并删除 go.mod 文件
	goMod := filepath.Join(dep.Dir, "go.mod")
	if _, err := os.Stat(goMod); err == nil {
		fmt.Printf("  删除 %s\n", goMod)
		os.Remove(goMod)
	} else {
		fmt.Printf("  %s 不存在 (可能正常)\n", goMod)
	}

	// 检查并删除 go.sum 文件
	goSum := filepath.Join(dep.Dir, "go.sum")
	if _, err := os.Stat(goSum); err == nil {
		fmt.Printf("  删除 %s\n", goSum)
		os.Remove(goSum)
	} else {
		fmt.Printf("  %s 不存在 (可能正常)\n", goSum)
	}

	// 检查目录内容
	fmt.Printf("  检查 %s 目录内容:\n", dep.Dir)
	entries, err := os.ReadDir(dep.Dir)
	if err != nil {
		fmt.Printf("  无法读取目录: %v\n", err)
	} else {
		for _, entry := range entries {
			fmt.Printf("    - %s\n", entry.Name())
		}
	}

	// 运行 go mod tidy
	fmt.Println("  运行 go mod tidy...")
	runCmd("go", "mod", "tidy")

	// 提交
	msg := fmt.Sprintf("添加 %s 目录", dep.Name)
	commitFiles(msg)
}

func commitFiles(msg string) {
	timestamp := time.Now().Format("2006-01-02 15:04")
	commitMsg := fmt.Sprintf("%s %s", msg, timestamp)

	fmt.Printf("  提交: %s\n", commitMsg)
	runCmd("git", "add", ".")
	runCmd("git", "commit", "-m", commitMsg)
}

func runCmd(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if strings.HasPrefix(name, "git") || strings.HasPrefix(name, "go") {
		cmd.Dir, _ = os.Getwd()
	}

	fmt.Printf("> %s %s\n", name, strings.Join(args, " "))
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "命令执行失败: %v\n", err)
		os.Exit(1)
	}
}
