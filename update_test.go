package main

import (
	"fmt"
	"os/exec"
	"testing"
)

 
func TestGetLatestGioVersion(t *testing.T) {
	version := getLatestGioVersion()

	// 下载模块
	cmd := exec.Command("go", "mod", "download", fmt.Sprintf("github.com/ddkwork/gio@%s", version))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("下载模块失败: %v\n输出: %s", err, string(output))
	} else {
		t.Logf("成功下载模块: github.com/ddkwork/gio@%s", version)
	}
}
