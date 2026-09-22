package estimator

import (
	"fmt"
	"strings"
	"sync"

	"tokenizer/bigmodel" // 无官方Go库，用启发式兜底 系列
	// OpenAI 系列
)

// Registry 多模型Tokenizer注册表
type Registry struct {
	mu         sync.RWMutex
	estimators map[string]Estimator // key: 模型前缀或完整名
}

func NewRegistry() *Registry {
	r := &Registry{
		estimators: make(map[string]Estimator),
	}
	r.registerDefaults()
	return r
}

func NewOfflineRegistry() *Registry {
	r := &Registry{
		estimators: make(map[string]Estimator),
	}
	return r
}

// GetEstimator 根据模型名获取对应的估算器（前缀匹配）
func (r *Registry) GetEstimator(model string) (Estimator, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 精确匹配优先
	if est, ok := r.estimators[model]; ok {
		return est, nil
	}
	// 前缀匹配
	for prefix, est := range r.estimators {
		if strings.HasPrefix(model, prefix) {
			return est, nil
		}
	}
	return nil, fmt.Errorf("unsupported model: %s", model)
}

// Register 注册模型估算器
func (r *Registry) Register(modelPrefix string, est Estimator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.estimators[modelPrefix] = est
}

// registerDefaults 注册主流模型的默认估算器
func (r *Registry) registerDefaults() {
	o200kEstimator, err := bigmodel.NewTiktokenEstimator("o200k")
	if err != nil {
		panic(fmt.Sprintf("failed to create o200k estimator: %v", err))
	}
	cl100kEstimator, err := bigmodel.NewTiktokenEstimator("cl100k")
	if err != nil {
		panic(fmt.Sprintf("failed to create cl100k estimator: %v", err))
	}

	r.Register("gpt-4o", o200kEstimator)
	r.Register("gpt-4-turbo", cl100kEstimator)
	r.Register("gpt-4", cl100kEstimator)
	r.Register("gpt-3.5-turbo", cl100kEstimator)

	// --- DeepSeek (底层兼容 tiktoken cl100k_base) ---
	r.Register("deepseek-chat", cl100kEstimator)
	r.Register("deepseek-coder", cl100kEstimator)

	// --- Claude / Anthropic (无官方Go库，用启发式兜底) ---
	r.Register("claude", &bigmodel.HeuristicEstimator{})

	// --- Gemini (无官方Go库，用启发式兜底) ---
	r.Register("gemini", &bigmodel.HeuristicEstimator{})

	// --- 通义千问 Qwen (无官方Go库，用启发式兜底) ---
	r.Register("qwen", &bigmodel.HeuristicEstimator{})
}
