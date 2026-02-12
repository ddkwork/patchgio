package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/ddkwork/golibrary/std/mylog"
	"github.com/ddkwork/golibrary/std/stream"
)

const (
	gioRepoDir    = "gio"
	gvcodeRepoDir = "gvcode"
)

var (
	GH_USER         = os.Getenv("GH_USER")
	GIO_UPSTREAM    = os.Getenv("GIO_UPSTREAM")
	GVCODE_UPSTREAM = os.Getenv("GVCODE_UPSTREAM")
	GH_TOKEN        = os.Getenv("GH_TOKEN")
)

func main() {
	if stream.IsRunningOnGitHubActions() {
		stream.RunCommand("git", "config", "--global", "user.email", "action@github.com")
		stream.RunCommand("git", "config", "--global", "user.name", "GitHub Action")
	}

	root, _ := os.Getwd()

	sync.OnceFunc(func() {
		fmt.Println("=== 开始更新 gvcode 仓库 ===")
		cloneRepo(gvcodeRepoDir, GVCODE_UPSTREAM)
		gitignoreAppendCmd(gvcodeRepoDir)
		runModernizeCommands(gvcodeRepoDir)
		applyPatches(gvcodeRepoDir, filepath.Join(root, "export_patch", "gvcode"))
		fmt.Println("=== 更新 gvcode 依赖 ===")
		os.WriteFile(filepath.Join(gvcodeRepoDir, "go.mod"), []byte(`
module github.com/oligo/gvcode

go 1.26

`), 0o644)
		updateAllDeps(gvcodeRepoDir, false)
		downgradeBugDeps(gvcodeRepoDir)
		commitDepsChanges(gvcodeRepoDir)
		pushRepo(gvcodeRepoDir)
		os.Chdir(root)
	})()

	sync.OnceFunc(func() {
		fmt.Println("=== 开始更新 gio 仓库 ===")

		cloneRepo(gioRepoDir, GIO_UPSTREAM)
		gitignoreAppendCmd(gioRepoDir)

		fmt.Println("=== 合并 gio 子模块 ===")
		gioDeps := []Dependency{
			{"https://github.com/gioui/gio-cmd", "cmd", "cmd"},
			{"https://github.com/gioui/gio-x", "x", "x"},
			{"https://github.com/gioui/gio-example", "example", "example"},
			{"https://git.sr.ht/~eliasnaur/gio-shader", "shader", "shader"},
			{"https://git.sr.ht/~eliasnaur/gio-cpu", "cpu", "cpu"},
		}
		if stream.IsRunningOnGitHubActions() {
			os.Remove("export_patch/giosub/0001-添加子模块.patch")
			for _, dep := range gioDeps {
				depFullPath := filepath.Join(gioRepoDir, dep.Dir)
				fmt.Printf("克隆子模块: %s -> %s\n", dep.URL, dep.Dir)
				stream.RunCommand("git", "clone", dep.URL, depFullPath)
			}

			for _, dep := range gioDeps {
				depFullPath := filepath.Join(gioRepoDir, dep.Dir)
				gitDir := filepath.Join(depFullPath, ".git")
				os.RemoveAll(gitDir)
				os.Remove(filepath.Join(depFullPath, "go.mod"))
				os.Remove(filepath.Join(depFullPath, "go.sum"))
			}
			if !gitAddWithDir(gioRepoDir) {
				return
			}
			gitCommitWithDir(gioRepoDir, "合并所有子模块")
		}
		applyPatches(gioRepoDir, filepath.Join(root, "export_patch", "giosub"))

		fmt.Println("=== 更新 gio 依赖 ===")
		os.WriteFile(filepath.Join(gioRepoDir, "go.mod"), []byte(`
module gioui.org

go 1.26

`), 0o644)
		updateAllDeps(gioRepoDir, true)
		downgradeBugDeps(gioRepoDir)
		commitDepsChanges(gioRepoDir)
		runModernizeCommands(gioRepoDir)

		applyPatches(gioRepoDir, filepath.Join(root, "export_patch", "gio"))

		pushRepo(gioRepoDir)
		os.Chdir(root)

	})()

	fmt.Println("=== 所有更新完成 ===")
}

func init() {
	if GH_USER == "" {
		GH_USER = "ddkwork"
	}
	if GIO_UPSTREAM == "" {
		GIO_UPSTREAM = "https://github.com/gioui/gio"
	}
	if GVCODE_UPSTREAM == "" {
		GVCODE_UPSTREAM = "https://github.com/oligo/gvcode"
	}
}

type Dependency struct {
	URL  string
	Dir  string
	Name string
}

func downgradeBugDeps(repoDir string) {
	if _, err := os.Stat(filepath.Join(repoDir, "go.mod")); err == nil {
		stream.RunCommandWithDir(repoDir, "go", "get", "github.com/go-text/typesetting@v0.3.0")
	}
}

func updateAllDeps(repoDir string, runTidy bool) {
	if runTidy {
		stream.RunCommandWithDir(repoDir, "go", "mod", "tidy")
	}
	stream.RunCommandWithDir(repoDir, "go", "get", "-u", "all")
}

func commitDepsChanges(repoDir string) {
	if !gitAddWithDir(repoDir) {
		return
	}
	gitCommitWithDir(repoDir, "更新依赖")
	runModernizeCommands(repoDir)
}

func applyPatches(repoDir, patchesDir string) {
	files, err := filepath.Glob(filepath.Join(patchesDir, "*.patch"))
	if err != nil || len(files) == 0 {
		panic("没有发现补丁文件")
	}
	mylog.Struct(files)
	for _, patchFile := range files {
		patchName := filepath.Base(patchFile)
		fmt.Println("应用补丁:", patchName)
		stream.RunCommandWithDir(repoDir, "git", "apply", "--whitespace=fix", patchFile)
		if !gitAddWithDir(repoDir) {
			return
		}

		// 移除补丁文件名中的编号前缀用于提交信息
		commitMsg := stream.BaseName(regexp.MustCompile(`^\d{4}[-_]`).ReplaceAllString(patchName, ""))
		gitCommitWithDir(repoDir, commitMsg)
	}
}

func pushRepo(repoDir string) {
	if GH_TOKEN == "" {
		fmt.Println("警告: GH_TOKEN 环境变量未设置，跳过推送")
		return
	}
	pushURL := fmt.Sprintf("https://%s:%s@github.com/%s/%s.git", GH_USER, GH_TOKEN, GH_USER, repoDir)
	stream.RunCommandWithDir(repoDir, "git", "push", "--force", pushURL, "HEAD:main")
}

func cloneRepo(repoDir, upstreamURL string) {
	if stream.IsRunningOnGitHubActions() {
		os.RemoveAll(repoDir)
	}
	// 本地环境：检查目录是否存在
	if stream.IsDir(repoDir) {
		fmt.Printf("=== 仓库 %s 已存在，跳过克隆 ===\n", repoDir)
		switch repoDir {
		case "gvcode":
			stream.RunCommandWithDir(gvcodeRepoDir, "git", "reset", "--hard", "daa83c26d0105e051da174612311857c01441954")
		case "gio":
			stream.RunCommandWithDir(gioRepoDir, "git", "reset", "--hard", "99647591f6b9df438999841c898f89ec629ea818")
		}
		return
	}
	fmt.Printf("=== 克隆 %s 仓库 ===\n", repoDir)
	stream.RunCommand("git", "clone", upstreamURL, repoDir)
}

func runModernizeCommands(repoDir string) {
	stream.Fmt(repoDir)
	if !gitAddWithDir(repoDir) {
		return
	}
	gitCommitWithDir(repoDir, "格式化代码")

	stream.Fix(repoDir)
	if !gitAddWithDir(repoDir) {
		mylog.Warning("没有检测到旧语法需要升级")
		return
	}
	gitCommitWithDir(repoDir, "升级现代化代码")
}

func gitignoreAppendCmd(repoDir string) {
	path := filepath.Join(repoDir, ".gitignore")
	stream.WriteAppend(path, ".idea\n")
	stream.WriteAppend(path, "*.cmd\n")
	stream.WriteAppend(path, "*.log\n")
	stream.WriteAppend(path, "*.ps1\n")

	// 清理空行和重复内容的行
	cleanGitignore(path)
}

func cleanGitignore(path string) {
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}

	lines := strings.Split(string(content), "\n")
	seen := make(map[string]bool)
	var cleanedLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// 跳过空行
		if trimmed == "" {
			continue
		}
		// 跳过重复行
		if !seen[trimmed] {
			seen[trimmed] = true
			cleanedLines = append(cleanedLines, trimmed)
		}
	}

	// 写回文件
	cleanedContent := strings.Join(cleanedLines, "\n") + "\n"
	os.WriteFile(path, []byte(cleanedContent), 0o644)
}

func gitAddWithDir(dir string) bool {
	stream.RunCommandWithDir(dir, "git", "add", ".")
	// 检查是否有变更：通过 git status --porcelain 检查
	statusOutput := stream.RunCommandWithDirSilent(dir, "git", "status", "--porcelain")
	return statusOutput.String() != ""
}

func gitCommitWithDir(dir string, msg string) {
	stream.RunCommandWithDir(dir, "git", "commit", "-m", msg)
}
