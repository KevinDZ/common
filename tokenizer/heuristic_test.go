package tokenizer

import (
	"testing"
	"tokenizer/bigmodel"
)

func TestHeuristicEstimator_Count(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "空字符串",
			input:    "",
			expected: 1, // 0/4 + 0*2 + 1 = 1
		},
		{
			name:     "纯ASCII-4个字符",
			input:    "abcd",
			expected: 2, // 4/4 + 0*2 + 1 = 2
		},
		{
			name:     "纯ASCII-8个字符",
			input:    "abcdefgh",
			expected: 3, // 8/4 + 0*2 + 1 = 3
		},
		{
			name:     "纯ASCII-不足4个字符",
			input:    "ab",
			expected: 1, // 2/4=0(整数除法) + 0*2 + 1 = 1
		},
		{
			name:     "纯中文-1个字",
			input:    "你",
			expected: 3, // 0/4 + 1*2 + 1 = 3
		},
		{
			name:     "纯中文-3个字",
			input:    "你好啊",
			expected: 7, // 0/4 + 3*2 + 1 = 7
		},
		{
			name:     "中英文混合",
			input:    "Hi你",
			expected: 3, // 2/4=0 + 1*2 + 1 = 3
		},
		{
			name:     "包含Emoji",
			input:    "Hi😀",
			expected: 3, // 2/4=0 + 1*2 + 1 = 3（😀是非ASCII）
		},
		{
			name:     "长文本混合",
			input:    "Hello你好World世界",
			expected: 11, // 10/4=2 + 4*2 + 1 = 11
		},
	}

	est := &bigmodel.HeuristicEstimator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := est.Count(tt.input)
			if got != tt.expected {
				t.Errorf("Count(%q) = %d; want %d", tt.input, got, tt.expected)
			}
			t.Logf("Count(%q) = %d", tt.input, got)
		})
	}
}
