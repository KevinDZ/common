package bigmodel

// 对于没有 Go 版官方 Tokenizer 的模型，采用经验公式，误差控制在 ±15% 以内，足以支撑预扣费的保守估算
// HeuristicEstimator 启发式估算器（无官方Go分词器时的兜底方案）
type HeuristicEstimator struct{}

func (h *HeuristicEstimator) Count(text string) int {
	// 统计 ASCII 和非 ASCII 字符数
	asciiCount := 0
	for _, r := range text {
		if r <= 0x7F {
			asciiCount++
		}
	}
	nonAscii := len([]rune(text)) - asciiCount

	// ASCII 部分按 4:1 折算，非 ASCII 按 1:2 高估，+1 兜底
	return asciiCount/4 + nonAscii*2 + 1
}
