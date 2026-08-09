package log

import "math/rand/v2"

// sampleRand 返回并发安全的非确定性 [0,1) 伪随机数，供 ResultKeepSampler 默认使用。
func sampleRand() float64 {
	return rand.Float64()
}
