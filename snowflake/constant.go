package snowflake

// 位段定义（可根据业务调整）
const (
	workerIDBits   = 10                          // 机器ID占10位，支持1024个节点
	sequenceBits   = 12                          // 序列号占12位，每毫秒最多4096个ID
	maxWorkerID    = -1 ^ (-1 << workerIDBits)   // 1023
	maxSequence    = -1 ^ (-1 << sequenceBits)   // 4095
	workerIDShift  = sequenceBits                // 机器ID左移12位
	timestampShift = sequenceBits + workerIDBits // 时间戳左移22位

	// 自定义纪元时间（2024-01-01 00:00:00 UTC），使ID更紧凑
	epoch int64 = 1704067200000

	// 时钟回拨容忍阈值（毫秒）
	maxClockBackwardMs = 5
)
