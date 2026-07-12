package studentwordtask

import (
	"hash/fnv"
	"math/rand"
)

// rand.go 提供确定性随机源：按日期（+可选盐）生成种子，
// 保证同一天多次生成今日任务结果一致，避免刷新首页时新词池/抽查抖动。
//
// 注意：这是任务生成内部的稳定性需求，不是安全随机。

// seededRand 封装一个基于种子的伪随机源，提供 nextIntN(max)。
type seededRand struct {
	r *rand.Rand
}

// newSeededRand 用字符串种子构造确定性随机源。
func newSeededRand(seed string) *seededRand {
	h := fnv.New64a()
	_, _ = h.Write([]byte(seed))
	src := rand.NewSource(int64(h.Sum64()))
	return &seededRand{r: rand.New(src)}
}

// nextIntN 返回 [0,max) 的伪随机整数。
func (s *seededRand) nextIntN(max int) int {
	if max <= 0 {
		return 0
	}
	return s.r.Intn(max)
}
