package ai

import (
	"sync"

	"github.com/pkoukk/tiktoken-go"
	tiktoken_loader "github.com/pkoukk/tiktoken-go-loader"
)

func init() {
	// Encodings are embedded in the binary instead of downloaded at runtime.
	tiktoken.SetBpeLoader(tiktoken_loader.NewOfflineLoader())
}

// tokensPerMessage is the per-message overhead used by the Python version.
const tokensPerMessage = 8

type TokenCounter struct {
	mu       sync.Mutex
	encoders map[string]*tiktoken.Tiktoken
}

func NewTokenCounter() *TokenCounter {
	return &TokenCounter{encoders: map[string]*tiktoken.Tiktoken{}}
}

func (c *TokenCounter) encoder(model string) *tiktoken.Tiktoken {
	c.mu.Lock()
	defer c.mu.Unlock()

	if enc, ok := c.encoders[model]; ok {
		return enc
	}

	enc, err := tiktoken.EncodingForModel(model)
	if err != nil {
		// Unknown (newer) models use the latest encoding.
		enc, err = tiktoken.GetEncoding("o200k_base")
		if err != nil {
			panic(err) // embedded asset, cannot fail
		}
	}
	c.encoders[model] = enc
	return enc
}

// Count returns the approximate number of prompt tokens for messages.
func (c *TokenCounter) Count(model string, messages ...ChatMessage) int {
	enc := c.encoder(model)
	n := 0
	for _, m := range messages {
		n += tokensPerMessage + len(enc.Encode(m.Role, nil, nil)) + len(enc.Encode(m.Content, nil, nil))
	}
	return n
}
