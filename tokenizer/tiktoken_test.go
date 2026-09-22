package tokenizer

import (
	"fmt"
	"testing"
	"tokenizer/estimator"
)

func TestTiktokenEstimator_Registry(t *testing.T) {
	// 1. 创建注册表（内部自动调用 registerDefaults）
	reg := estimator.NewRegistry()

	// 2. 根据模型名获取估算器
	est, err := reg.GetEstimator("gpt-4o")
	if err != nil {
		panic(err)
	}

	// 3. 直接估算单段文本
	tokens := est.Count("Hello, 你好世界！")
	fmt.Printf("Token 数: %d\n", tokens)

}

func TestTiktokenEstimator_OfflineRegistry(t *testing.T) {
	// 不要调用 NewRegistry()，它会自动执行 registerDefaults() 并联网
	// 手动构建一个不带网络请求的注册表
	reg := estimator.NewOfflineRegistry()

	// 注册 mock 估算器（不联网）
	reg.Register("gpt-4o", &mockEstimator{})
	reg.Register("gpt-4-turbo", &mockEstimator{})
	reg.Register("gpt-4", &mockEstimator{})
	reg.Register("gpt-3.5-turbo", &mockEstimator{})
	reg.Register("deepseek-chat", &mockEstimator{})
	reg.Register("deepseek-coder", &mockEstimator{})

	// 测试 GetEstimator 的路由逻辑
	models := []string{"gpt-4o", "gpt-4-turbo"}
	for _, model := range models {
		est, err := reg.GetEstimator(model)
		if err != nil {
			t.Errorf("GetEstimator(%s) error: %v", model, err)
		}
		if est == nil {
			t.Logf("GetEstimator(%s) returned nil", model)
		}
	}
}

type mockEstimator struct{}

func (m *mockEstimator) Count(text string) int { return 1 }
