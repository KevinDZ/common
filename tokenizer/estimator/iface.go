package estimator

// Estimator Token估算器接口
type Estimator interface {
	Count(text string) int // 本地估算文本Token数
}
