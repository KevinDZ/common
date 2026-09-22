package bigmodel

import (
	"github.com/pkoukk/tiktoken-go"
)

// TiktokenEstimator 基于 BPE 的精确估算器
type TiktokenEstimator struct {
	Enc *tiktoken.Tiktoken
}

func NewTiktokenEstimator(encodingName string) (*TiktokenEstimator, error) {
	var err error
	tiktoken := &TiktokenEstimator{}
	switch encodingName {
	case "o200k":
		err = tiktoken.GetTiktokenEncoding_o200k()
	case "cl100k":
		err = tiktoken.GetTiktokenEncoding_cl100k()
	}
	return tiktoken, err

}

func (t *TiktokenEstimator) GetTiktokenEncoding_o200k() error {
	// --- OpenAI 系列 (tiktoken) ---
	o200k, err := tiktoken.GetEncoding("o200k_base") // GPT-4o / GPT-4o-mini
	if err != nil {
		return err
	}
	t.Enc = o200k
	return nil
}

func (t *TiktokenEstimator) GetTiktokenEncoding_cl100k() error {
	cl100k, err := tiktoken.GetEncoding("cl100k_base") // GPT-3.5 / GPT-4 / GPT-4-Turbo
	if err != nil {
		return err
	}
	t.Enc = cl100k
	return nil
}

func (t *TiktokenEstimator) Count(text string) int {
	if t.Enc == nil {
		return len([]rune(text)) // fallback
	}
	tokens := t.Enc.Encode(text, nil, nil)
	return len(tokens)
}
