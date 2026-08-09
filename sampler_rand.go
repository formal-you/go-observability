package log

import (
	"math/rand"
	"sync"
)

// sampleRand 返回 [0,1) 伪随机，供 ResultKeepSampler 默认使用。
// 使用独立源 + 互斥，避免全局 rand 竞态。
var (
	sampleRandMu sync.Mutex
	sampleRandR  = rand.New(rand.NewSource(1)) // 确定性种子；生产可换 time 种子见 NewResultKeepSampler
)

func sampleRand() float64 {
	sampleRandMu.Lock()
	v := sampleRandR.Float64()
	sampleRandMu.Unlock()
	return v
}

// SeedSampleRand 设置默认采样随机源种子（测试可复现；进程级）。
func SeedSampleRand(seed int64) {
	sampleRandMu.Lock()
	sampleRandR = rand.New(rand.NewSource(seed))
	sampleRandMu.Unlock()
}
