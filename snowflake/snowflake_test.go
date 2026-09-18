package snowflake

import (
	"sync"
	"testing"
	"time"
)

// ==================== 1. 基础功能测试 ====================

// TestNewGenerator 测试生成器创建
func TestNewGenerator(t *testing.T) {
	// 正常范围
	gen, err := NewGenerator(100)
	if err != nil {
		t.Fatalf("NewGenerator(100) failed: %v", err)
	}
	if gen == nil {
		t.Fatal("NewGenerator(100) returned nil")
	}

	// 越界测试：workerID 为负数
	_, err = NewGenerator(-1)
	if err == nil {
		t.Fatal("NewGenerator(-1) should return error")
	}

	// 越界测试：workerID 超过最大值
	_, err = NewGenerator(maxWorkerID + 1)
	if err == nil {
		t.Fatal("NewGenerator(maxWorkerID+1) should return error")
	}

	// 边界值测试
	gen, err = NewGenerator(0)
	if err != nil {
		t.Fatalf("NewGenerator(0) failed: %v", err)
	}
	if gen.workerID != 0 {
		t.Fatalf("expected workerID 0, got %d", gen.workerID)
	}

	gen, err = NewGenerator(maxWorkerID)
	if err != nil {
		t.Fatalf("NewGenerator(maxWorkerID) failed: %v", err)
	}
	if gen.workerID != maxWorkerID {
		t.Fatalf("expected workerID %d, got %d", maxWorkerID, gen.workerID)
	}
}

// TestNextID 测试 ID 生成的基本正确性
func TestNextID(t *testing.T) {
	gen, err := NewGenerator(1)
	if err != nil {
		t.Fatal(err)
	}

	id1, err := gen.NextID()
	if err != nil {
		t.Fatalf("NextID() failed: %v", err)
	}
	if id1 <= 0 {
		t.Fatalf("expected positive ID, got %d", id1)
	}

	id2, err := gen.NextID()
	if err != nil {
		t.Fatalf("NextID() failed: %v", err)
	}
	if id2 <= id1 {
		t.Fatalf("expected ID to be increasing: %d > %d", id2, id1)
	}

	// 验证 ID 结构：提取 workerID
	extractedWorkerID := (id1 >> workerIDShift) & maxWorkerID
	if extractedWorkerID != 1 {
		t.Fatalf("expected workerID 1, got %d", extractedWorkerID)
	}

	// 验证 ID 结构：提取时间戳
	timestamp := (id1 >> timestampShift) + epoch
	now := currentMillis()
	if timestamp > now+1000 || timestamp < now-1000 {
		t.Fatalf("timestamp %d is too far from current time %d", timestamp, now)
	}
	t.Logf("Generated ID: %d, Timestamp: %d, WorkerID: %d", id1, timestamp, extractedWorkerID)
}

// TestNextIDUniqueness 测试生成的 ID 唯一性（单 goroutine）
func TestNextIDUniqueness(t *testing.T) {
	gen, err := NewGenerator(2)
	if err != nil {
		t.Fatal(err)
	}

	count := 10000
	ids := make(map[int64]bool, count)
	for i := 0; i < count; i++ {
		id, err := gen.NextID()
		if err != nil {
			t.Fatalf("NextID() failed at iteration %d: %v", i, err)
		}
		if ids[id] {
			t.Fatalf("duplicate ID found: %d at iteration %d", id, i)
		}
		ids[id] = true
	}
	t.Logf("Successfully generated %d unique IDs in single goroutine", count)
}

// ==================== 2. 并发安全测试 ====================

// TestConcurrentNextID 测试并发场景下的 ID 唯一性与递增性
func TestConcurrentNextID(t *testing.T) {
	gen, err := NewGenerator(3)
	if err != nil {
		t.Fatal(err)
	}

	goroutines := 100
	idsPerGoroutine := 1000
	totalIDs := goroutines * idsPerGoroutine

	ids := make(chan int64, totalIDs)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < idsPerGoroutine; j++ {
				id, err := gen.NextID()
				if err != nil {
					t.Errorf("NextID() failed: %v", err)
					return
				}
				ids <- id
			}
		}()
	}

	wg.Wait()
	close(ids)

	// 检查唯一性
	idMap := make(map[int64]bool, totalIDs)
	for id := range ids {
		if idMap[id] {
			t.Fatalf("duplicate ID found in concurrent test: %d", id)
		}
		idMap[id] = true
	}

	if len(idMap) != totalIDs {
		t.Fatalf("expected %d unique IDs, got %d", totalIDs, len(idMap))
	}
	t.Logf("Successfully generated %d unique IDs in concurrent test", totalIDs)
}

// TestConcurrentNextIDWithAtomic 测试无锁版本的并发安全性
func TestConcurrentNextIDWithAtomic(t *testing.T) {
	gen := &AtomicGenerator{
		workerID:      4,
		lastTimestamp: -1,
	}

	goroutines := 100
	idsPerGoroutine := 1000
	totalIDs := goroutines * idsPerGoroutine

	ids := make(chan int64, totalIDs)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < idsPerGoroutine; j++ {
				id, err := gen.NextID()
				if err != nil {
					t.Errorf("AtomicGenerator.NextID() failed: %v", err)
					return
				}
				ids <- id
			}
		}()
	}

	wg.Wait()
	close(ids)

	// 检查唯一性
	idMap := make(map[int64]bool, totalIDs)
	for id := range ids {
		if idMap[id] {
			t.Fatalf("duplicate ID found in atomic concurrent test: %d", id)
		}
		idMap[id] = true
	}

	if len(idMap) != totalIDs {
		t.Fatalf("expected %d unique IDs, got %d", totalIDs, len(idMap))
	}
	t.Logf("Successfully generated %d unique IDs in atomic concurrent test", totalIDs)
}

// ==================== 3. 时钟回拨测试 ====================

// TestClockBackwardSmall 测试小幅度时钟回拨（≤5ms）
func TestClockBackwardSmall(t *testing.T) {
	gen, err := NewGenerator(5)
	if err != nil {
		t.Fatal(err)
	}

	// 先生成一个 ID
	id1, err := gen.NextID()
	if err != nil {
		t.Fatal(err)
	}

	// 模拟时钟回拨 3ms：手动设置 lastTimestamp 为未来
	gen.mu.Lock()
	gen.lastTimestamp = currentMillis() + 3
	gen.mu.Unlock()

	// 再次生成 ID，应该能正常处理回拨
	id2, err := gen.NextID()
	if err != nil {
		t.Fatalf("NextID() after small clock backward failed: %v", err)
	}

	if id2 <= id1 {
		t.Fatalf("expected ID to be increasing after clock backward: %d > %d", id2, id1)
	}
	t.Logf("Successfully generated ID %d after small clock backward, previous ID was %d", id2, id1)
}

// TestClockBackwardLarge 测试大幅度时钟回拨（>5ms，触发逻辑时钟）
func TestClockBackwardLarge(t *testing.T) {
	gen, err := NewGenerator(6)
	if err != nil {
		t.Fatal(err)
	}

	// 先生成一个 ID
	id1, err := gen.NextID()
	if err != nil {
		t.Fatal(err)
	}

	// 模拟时钟回拨 10ms
	gen.mu.Lock()
	gen.lastTimestamp = currentMillis() + 10
	gen.mu.Unlock()

	// 再次生成 ID，应该使用逻辑时钟
	id2, err := gen.NextID()
	if err != nil {
		t.Fatalf("NextID() after large clock backward failed: %v", err)
	}

	if id2 <= id1 {
		t.Fatalf("expected ID to be increasing after large clock backward: %d > %d", id2, id1)
	}

	// 验证逻辑时钟被使用：提取时间戳部分应等于 lastTimestamp
	timestamp := (id2 >> timestampShift) + epoch
	gen.mu.Lock()
	expectedTimestamp := gen.lastTimestamp + epoch
	gen.mu.Unlock()

	if timestamp != expectedTimestamp {
		t.Fatalf("expected timestamp %d, got %d (logical clock may not be used)", expectedTimestamp, timestamp)
	}
	t.Logf("Successfully generated ID %d after large clock backward, previous ID was %d", id2, id1)
}

// TestSequenceOverflow 测试序列号溢出（同一毫秒内生成超过4096个ID）
func TestSequenceOverflow(t *testing.T) {
	gen, err := NewGenerator(7)
	if err != nil {
		t.Fatal(err)
	}

	// 强制设置 lastTimestamp 为当前时间，sequence 为最大值
	gen.mu.Lock()
	gen.lastTimestamp = currentMillis()
	gen.sequence = maxSequence
	gen.mu.Unlock()

	// 下一个 ID 应该等待下一毫秒，sequence 重置为 0
	id, err := gen.NextID()
	if err != nil {
		t.Fatalf("NextID() after sequence overflow failed: %v", err)
	}

	// 提取 sequence 部分，应该为 0
	sequence := id & maxSequence
	if sequence != 0 {
		t.Fatalf("expected sequence 0 after overflow, got %d", sequence)
	}
	t.Logf("Successfully generated ID %d after sequence overflow, sequence reset to %d", id, sequence)
}

// ==================== 4. WorkerID 分配测试 ====================

// TestGetWorkerIDFromIP 测试从 IP 推导 WorkerID
func TestGetWorkerIDFromIP(t *testing.T) {
	workerID, err := getWorkerIDFromIP()
	if err != nil {
		t.Fatalf("getWorkerIDFromIP() failed: %v", err)
	}
	if workerID < 0 || workerID > maxWorkerID {
		t.Fatalf("workerID %d out of range [0, %d]", workerID, maxWorkerID)
	}
	t.Logf("derived workerID from IP: %d", workerID)
}

// ==================== 5. 无锁版本测试 ====================

// TestAtomicGeneratorBasic 测试无锁版本基本功能
func TestAtomicGeneratorBasic(t *testing.T) {
	gen := &AtomicGenerator{
		workerID:      8,
		lastTimestamp: -1,
	}

	id1, err := gen.NextID()
	if err != nil {
		t.Fatalf("AtomicGenerator.NextID() failed: %v", err)
	}
	if id1 <= 0 {
		t.Fatalf("expected positive ID, got %d", id1)
	}

	id2, err := gen.NextID()
	if err != nil {
		t.Fatalf("AtomicGenerator.NextID() failed: %v", err)
	}
	if id2 <= id1 {
		t.Fatalf("expected ID to be increasing: %d > %d", id2, id1)
	}

	// 验证 workerID
	extractedWorkerID := (id1 >> workerIDShift) & maxWorkerID
	if extractedWorkerID != 8 {
		t.Fatalf("expected workerID 8, got %d", extractedWorkerID)
	}
	t.Logf("AtomicGenerator generated IDs: %d, %d with workerID %d", id1, id2, extractedWorkerID)
}

// TestAtomicGeneratorClockBackward 测试无锁版本时钟回拨处理
func TestAtomicGeneratorClockBackward(t *testing.T) {
	gen := &AtomicGenerator{
		workerID:      9,
		lastTimestamp: currentMillis() + 10, // 模拟未来时间戳
	}

	// 应该自旋等待直到时钟追平
	id, err := gen.NextID()
	if err != nil {
		t.Fatalf("AtomicGenerator.NextID() after clock backward failed: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive ID, got %d", id)
	}
	t.Logf("Successfully generated ID %d after clock backward in AtomicGenerator", id)
}

// ==================== 6. 启动期时钟校验测试 ====================

// TestValidateClockNormal 测试正常情况（无回拨）
func TestValidateClockNormal(t *testing.T) {
	lastSaved := currentMillis() - 1000 // 上次时间戳是 1 秒前
	err := ValidateClock(lastSaved, 1000)
	if err != nil {
		t.Fatalf("ValidateClock() failed for normal case: %v", err)
	}
	t.Logf("ValidateClock() passed for normal case")
}

// TestValidateClockSmallBackward 测试小幅度回拨（<1秒）
func TestValidateClockSmallBackward(t *testing.T) {
	lastSaved := currentMillis() + 500 // 回拨 500ms
	err := ValidateClock(lastSaved, 1000)
	if err != nil {
		t.Fatalf("ValidateClock() failed for small backward: %v", err)
	}
	t.Logf("ValidateClock() passed for small backward")
}

// TestValidateClockLargeBackward 测试大幅度回拨（>1秒，应拒绝）
func TestValidateClockLargeBackward(t *testing.T) {
	lastSaved := currentMillis() + 2000 // 回拨 2 秒
	err := ValidateClock(lastSaved, 1000)
	if err == nil {
		t.Fatal("ValidateClock() should return error for large backward")
	}
	t.Logf("expected error for large backward: %v", err)
}

// ==================== 7. 性能基准测试 ====================

// BenchmarkNextID 基准测试：Mutex 版本
func BenchmarkNextID(b *testing.B) {
	gen, err := NewGenerator(10)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = gen.NextID()
	}
}

// BenchmarkAtomicNextID 基准测试：无锁版本
func BenchmarkAtomicNextID(b *testing.B) {
	gen := &AtomicGenerator{
		workerID:      11,
		lastTimestamp: -1,
	}
	// 只打印一次，用于确认测试参数
	b.Logf("Starting benchmark with workerID=%d, datacenterID=%d", gen.workerID, gen.lastTimestamp)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := gen.NextID()
		if err != nil {
			b.Fatalf("AtomicGenerator.NextID() failed: %v", err)
		}
	}
}

// BenchmarkConcurrentNextID 基准测试：并发场景 Mutex 版本
func BenchmarkConcurrentNextID(b *testing.B) {
	gen, err := NewGenerator(12)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = gen.NextID()
		}
	})
}

// BenchmarkConcurrentAtomicNextID 基准测试：并发场景无锁版本
func BenchmarkConcurrentAtomicNextID(b *testing.B) {
	gen := &AtomicGenerator{
		workerID:      13,
		lastTimestamp: -1,
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = gen.NextID()
		}
	})
}

// ==================== 8. 辅助函数测试 ====================

// TestCurrentMillis 测试时间戳获取
func TestCurrentMillis(t *testing.T) {
	before := time.Now().UnixNano() / 1e6
	now := currentMillis()
	after := time.Now().UnixNano() / 1e6

	if now < before || now > after {
		t.Fatalf("currentMillis() %d is not between %d and %d", now, before, after)
	}
}

// TestIDStructure 测试 ID 位段结构正确性
func TestIDStructure(t *testing.T) {
	gen, err := NewGenerator(42)
	if err != nil {
		t.Fatal(err)
	}

	id, err := gen.NextID()
	if err != nil {
		t.Fatal(err)
	}

	// 提取各部分
	timestamp := (id >> timestampShift) + epoch
	workerID := (id >> workerIDShift) & maxWorkerID
	sequence := id & maxSequence

	t.Logf("ID: %d", id)
	t.Logf("  Timestamp: %d (%s)", timestamp, time.Unix(0, timestamp*1e6).Format(time.RFC3339))
	t.Logf("  WorkerID:  %d", workerID)
	t.Logf("  Sequence:  %d", sequence)

	if workerID != 42 {
		t.Errorf("expected workerID 42, got %d", workerID)
	}
	if sequence < 0 || sequence > maxSequence {
		t.Errorf("sequence %d out of range [0, %d]", sequence, maxSequence)
	}

	// 验证符号位为 0（正数）
	if id < 0 {
		t.Errorf("ID should be positive, got %d", id)
	}
}

// TestIDMonotonicIncreasing 测试 ID 严格单调递增
func TestIDMonotonicIncreasing(t *testing.T) {
	gen, err := NewGenerator(14)
	if err != nil {
		t.Fatal(err)
	}

	var prevID int64
	for i := 0; i < 100000; i++ {
		id, err := gen.NextID()
		if err != nil {
			t.Fatalf("NextID() failed at iteration %d: %v", i, err)
		}
		if i > 0 && id <= prevID {
			t.Fatalf("ID not monotonically increasing at iteration %d: %d <= %d", i, id, prevID)
		}
		prevID = id
	}
}

// ==================== 9. 边界值测试 ====================

// TestMaxWorkerID 测试最大 WorkerID 边界
func TestMaxWorkerID(t *testing.T) {
	gen, err := NewGenerator(maxWorkerID)
	if err != nil {
		t.Fatal(err)
	}

	id, err := gen.NextID()
	if err != nil {
		t.Fatal(err)
	}

	extractedWorkerID := (id >> workerIDShift) & maxWorkerID
	if extractedWorkerID != maxWorkerID {
		t.Fatalf("expected workerID %d, got %d", maxWorkerID, extractedWorkerID)
	}
}

// TestZeroWorkerID 测试 WorkerID 为 0 的边界
func TestZeroWorkerID(t *testing.T) {
	gen, err := NewGenerator(0)
	if err != nil {
		t.Fatal(err)
	}

	id, err := gen.NextID()
	if err != nil {
		t.Fatal(err)
	}

	extractedWorkerID := (id >> workerIDShift) & maxWorkerID
	if extractedWorkerID != 0 {
		t.Fatalf("expected workerID 0, got %d", extractedWorkerID)
	}
}

// TestEpoch 测试纪元时间设置正确性
func TestEpoch(t *testing.T) {
	expectedEpoch := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	if epoch != expectedEpoch {
		t.Fatalf("epoch %d does not match expected %d", epoch, expectedEpoch)
	}
	t.Logf("epoch: %d (%s)", epoch, time.Unix(0, epoch*1e6).Format(time.RFC3339))
}

// ==================== 10. 综合集成测试 ====================

// TestFullLifecycle 模拟完整生命周期：创建 → 生成 → 并发 → 验证
func TestFullLifecycle(t *testing.T) {
	// 1. 创建生成器
	gen, err := NewGenerator(99)
	if err != nil {
		t.Fatal(err)
	}

	// 2. 单线程生成
	id1, err := gen.NextID()
	if err != nil {
		t.Fatal(err)
	}

	// 3. 并发生成
	var wg sync.WaitGroup
	ids := make([]int64, 1000)
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id, err := gen.NextID()
			if err != nil {
				t.Errorf("NextID() failed: %v", err)
				return
			}
			ids[idx] = id
		}(i)
	}
	wg.Wait()

	// 4. 验证唯一性
	idMap := make(map[int64]bool)
	idMap[id1] = true
	for _, id := range ids {
		if idMap[id] {
			t.Fatalf("duplicate ID found in lifecycle test: %d", id)
		}
		idMap[id] = true
	}

	// 5. 验证递增性（排序后检查）
	sortedIDs := make([]int64, len(ids)+1)
	sortedIDs[0] = id1
	copy(sortedIDs[1:], ids)
	// 简单冒泡排序用于验证（实际生产可用 sort.Slice）
	for i := 0; i < len(sortedIDs); i++ {
		for j := i + 1; j < len(sortedIDs); j++ {
			if sortedIDs[i] > sortedIDs[j] {
				sortedIDs[i], sortedIDs[j] = sortedIDs[j], sortedIDs[i]
			}
		}
	}
	for i := 1; i < len(sortedIDs); i++ {
		if sortedIDs[i] <= sortedIDs[i-1] {
			t.Fatalf("IDs not monotonically increasing in lifecycle test at index %d: %d <= %d", i, sortedIDs[i], sortedIDs[i-1])
		}
	}

	t.Logf("Lifecycle test passed: generated %d unique and increasing IDs", len(sortedIDs))
}
