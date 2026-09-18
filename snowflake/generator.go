package snowflake

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Generator Snowflake ID生成器
type Generator struct {
	mu            sync.Mutex // 保护并发安全
	lastTimestamp int64      // 上次生成ID的时间戳（毫秒）
	workerID      int64      // 机器ID（0~1023）
	sequence      int64      // 当前毫秒内的序列号
	logicalClock  int64      // 逻辑时钟（用于回拨时降级）
}

// NewGenerator 创建生成器，workerID 由外部传入（通过 etcd 分配或配置）
func NewGenerator(workerID int64) (*Generator, error) {
	if workerID < 0 || workerID > maxWorkerID {
		return nil, fmt.Errorf("workerID %d out of range [0, %d]", workerID, maxWorkerID)
	}
	return &Generator{
		workerID:      workerID,
		lastTimestamp: -1, // 初始化为 -1，首次调用时直接通过
	}, nil
}

// NextID 生成下一个全局唯一ID
func (g *Generator) NextID() (int64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := currentMillis()

	// ===== 第一层：时钟回拨检测 =====
	if now < g.lastTimestamp {
		offset := g.lastTimestamp - now

		// 小幅度回拨（≤5ms）：阻塞等待时钟追平
		if offset <= maxClockBackwardMs {
			time.Sleep(time.Duration(offset) * time.Millisecond)
			now = currentMillis()
			if now < g.lastTimestamp {
				// 等待后仍然回拨，进入第二层
				offset = g.lastTimestamp - now
			}
		}

		// 仍然回拨：进入第二层 —— 使用逻辑时钟
		if now < g.lastTimestamp {
			// 逻辑时钟 = 上次时间戳 + 1，保证单调递增
			g.logicalClock++
			now = g.lastTimestamp + g.logicalClock
		}
	} else {
		// 时钟正常，重置逻辑时钟
		g.logicalClock = 0
	}

	// ===== 序列号处理 =====
	if now == g.lastTimestamp {
		g.sequence = (g.sequence + 1) & maxSequence
		if g.sequence == 0 {
			// 序列号溢出：等待下一毫秒
			for now <= g.lastTimestamp {
				now = currentMillis()
			}
			g.sequence = 0
		}
	} else {
		g.sequence = 0
	}

	g.lastTimestamp = now

	// ===== 拼接 ID =====
	id := ((now - epoch) << timestampShift) |
		(g.workerID << workerIDShift) |
		g.sequence

	return id, nil
}

// currentMillis 获取当前毫秒级时间戳
func currentMillis() int64 {
	return time.Now().UnixNano() / 1e6
}

// AtomicGenerator 无锁版本的 Snowflake 生成器
// 适用于高并发场景，通过 CAS (Compare And Swap) 避免互斥锁的上下文切换开销
type AtomicGenerator struct {
	workerID      int64
	lastTimestamp int64 // 使用 int64 存储，便于 atomic 操作
	sequence      int64
}

// NextID 生成下一个唯一 ID（无锁版本）
func (g *AtomicGenerator) NextID() (int64, error) {
	for {
		now := currentMillis()
		last := atomic.LoadInt64(&g.lastTimestamp)

		if now < last {
			// 遇到时钟回拨，简单自旋等待时钟追平（生产环境可结合逻辑时钟优化）
			time.Sleep(time.Millisecond)
			continue
		}

		if now == last {
			// 同一毫秒内，通过 CAS 递增序列号
			seq := atomic.AddInt64(&g.sequence, 1)
			if seq > maxSequence {
				// 序列号溢出，等待下一毫秒
				time.Sleep(time.Millisecond)
				continue
			}
			// 尝试更新 lastTimestamp（防止跨毫秒时 sequence 未重置的竞争）
			if atomic.CompareAndSwapInt64(&g.lastTimestamp, last, now) {
				atomic.StoreInt64(&g.sequence, 0)
			}
			return g.packID(now, seq)
		} else {
			// 进入新的毫秒，重置序列号为 0
			atomic.StoreInt64(&g.sequence, 0)
			atomic.StoreInt64(&g.lastTimestamp, now)
			return g.packID(now, 0)
		}
	}
}

// packID 组装最终的 64 位 ID
func (g *AtomicGenerator) packID(timestamp int64, sequence int64) (int64, error) {
	if timestamp < epoch {
		return 0, errors.New("clock moved backwards")
	}
	return ((timestamp - epoch) << timestampShift) |
		(g.workerID << workerIDShift) |
		sequence, nil
}

// ValidateClock 启动期时钟校验
// lastSavedTimestamp: 从持久化存储（如 Redis/ZK/本地文件）中读取的上次运行最大时间戳
// maxBackwardMs: 允许的最大回拨毫秒数阈值，超过则拒绝启动
func ValidateClock(lastSavedTimestamp int64, maxBackwardMs int64) error {
	if lastSavedTimestamp <= 0 {
		// 首次启动，没有历史时间戳，无需校验
		return nil
	}

	now := currentMillis()
	if now >= lastSavedTimestamp {
		// 当前时间正常，大于等于上次保存的时间
		return nil
	}

	// 发生时钟回拨，计算回拨幅度
	backwardMs := lastSavedTimestamp - now
	if backwardMs <= maxBackwardMs {
		// 回拨幅度在容忍阈值内，记录告警但允许启动
		fmt.Printf("[WARN] clock moved backwards by %d ms, within tolerance\n", backwardMs)
		return nil
	}

	// 回拨幅度过大，拒绝启动
	return fmt.Errorf("clock moved backwards by %d ms (threshold: %d ms), refusing to start", backwardMs, maxBackwardMs)
}
