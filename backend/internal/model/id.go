package model

import (
	"fmt"
	"sync/atomic"
	"time"
)

var idCounter uint64

func NewID(prefix string) string {
	value := atomic.AddUint64(&idCounter, 1)
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixMilli(), value)
}
