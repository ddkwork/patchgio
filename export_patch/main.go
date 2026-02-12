package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)


func main() {
	// 【坑1】必须切换到可执行文件所在目录
	// 否则 git 命令会在当前工作目录执行，找不到正确的仓库
	if exe, err := os.Executable(); err == nil {
		os.Chdir(filepath.Dir(exe))
	}

	// 【坑2】禁止邮件头编码
	// true: Subject 会变成 =?UTF-8?q?=E5=8D=87=E7=BA=A7... 这种转义格式
	// false: Subject 保持原始中文，如 "升级现代化代码"
	exec.Command("git", "config", "format.encodeEmailHeaders", "false").Run()
	exec.Command("git", "config", "core.quotepath", "false").Run()
	exec.Command("git", "config", "i18n.logOutputEncoding", "utf-8").Run()

	// 【坑3】首次运行时 origin/main 不存在会报错
	// git rev-list --reverse origin/main..HEAD 需要 origin/main 的本地引用
	// 如果从未 fetch 过，origin/main 是 unknown revision
	if !branchExists("origin/main") {
		fmt.Println("首次运行，正在 fetch origin...")
		exec.Command("git", "fetch", "origin").Run()
	}

	// 创建 patches 目录
	os.MkdirAll("patches", 0o755)

	// 清空旧补丁
	files, _ := filepath.Glob("patches/*.patch")
	for _, f := range files {
		os.Remove(f)
	}

	// 获取提交列表
	commits := getCommits()

	for i, hash := range commits {
		subject := getSubject(hash)
		safe := sanitizeFilename(subject)
		if len(safe) > 50 {
			safe = safe[:50]
		}

		num := fmt.Sprintf("%04d", i+1)

		// 【坑4】绝不能用 --stdout + os.WriteFile！
		// 错误写法: exec.Command("git", "format-patch", "-1", hash, "--stdout").Output() 然后 os.WriteFile
		// 这样虽然能写入，但 git format-patch --stdout 输出的是正确的 UTF-8，
		// 问题在于 Windows 下可能遇到编码问题
		//
		// 正确做法: 让 git 直接写入文件 (-o patches)
		// git 原生输出就是正确的 UTF-8 邮件格式
		tmpFile, err := exec.Command("git", "format-patch", "-1", hash, "-o", "patches").Output()
		if err != nil {
			fmt.Fprintf(os.Stderr, "导出失败 %s: %v\n", hash, err)
			continue
		}

		// git format-patch -o 返回生成的文件名（如 patches/0001-xxx.patch）
		// 我们需要重命名为自定义格式
		oldPath := strings.TrimSpace(string(tmpFile))
		newPath := fmt.Sprintf("patches/%s-%s.patch", num, safe)
		if err := os.Rename(oldPath, newPath); err != nil {
			fmt.Fprintf(os.Stderr, "重命名失败 %s -> %s: %v\n", oldPath, newPath, err)
			continue
		}

		fmt.Println(newPath)
	}
}

// 检查分支/引用是否存在
func branchExists(ref string) bool {
	err := exec.Command("git", "rev-parse", "--verify", ref).Run()
	return err == nil
}

func getCommits() []string {
	out, err := exec.Command("git", "rev-list", "--reverse", "origin/main..HEAD").Output()
	if err != nil {
		// 如果 origin/main 还是不存在，尝试 master
		out, err = exec.Command("git", "rev-list", "--reverse", "origin/master..HEAD").Output()
		if err != nil {
			panic(err)
		}
	}
	return strings.Fields(string(out))
}

func getSubject(hash string) string {
	out, err := exec.Command("git", "log", "-1", "--format=%s", hash).Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func sanitizeFilename(s string) string {
	// 替换非法字符
	re := regexp.MustCompile(`[\\/:*?"<>|]`)
	s = re.ReplaceAllString(s, "-")
	// 移除前后空格
	s = strings.TrimSpace(s)
	return s
}
