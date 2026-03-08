package main

import (
	"strings"
	"testing"
)

// TestGetLatestGioVersion 测试 getLatestGioVersion 函数
func TestGetLatestGioVersion(t *testing.T) {
	// 测试函数是否能够正常调用
	// 注意：这个测试会调用真实的 GitHub API
	// 在 CI/CD 环境中可能需要 mock API 响应
	version := getLatestGioVersion()

	// 验证版本号格式
	if !strings.HasPrefix(version, "v0.0.0-00010101000000-") {
		t.Errorf("Expected version to start with 'v0.0.0-00010101000000-', got %s", version)
	}

	// 验证版本号包含两个部分（固定时间和 SHA）
	parts := strings.Split(version, "-")
	if len(parts) != 3 {
		t.Errorf("Expected version to have 3 parts separated by '-', got %d parts in %s", len(parts), version)
	}

	// 验证时间部分是否为固定值
	if parts[1] != "00010101000000" {
		t.Errorf("Expected time part to be '00010101000000', got %s", parts[1])
	}

	// 验证 SHA 部分长度是否为 12
	if len(parts[2]) != 12 {
		t.Errorf("Expected SHA part to have 12 characters, got %d in %s", len(parts[2]), parts[2])
	}
}
