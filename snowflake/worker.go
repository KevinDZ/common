package snowflake

import (
	"context"
	"errors"
	"fmt"
	"net"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// WorkerIDAllocator 基于 etcd 租约的 WorkerID 分配器
type WorkerIDAllocator struct {
	client *clientv3.Client
	prefix string // etcd 键前缀，如 "/snowflake/worker"
}

// Allocate 申请一个空闲的 WorkerID，通过 etcd 事务保证互斥
func (a *WorkerIDAllocator) Allocate(ctx context.Context) (int64, error) {
	for id := int64(0); id <= maxWorkerID; id++ {
		key := fmt.Sprintf("%s/%d", a.prefix, id)

		// 申请 60 秒租约，节点崩溃后租约到期自动释放
		lease, err := a.client.Grant(ctx, 60)
		if err != nil {
			return 0, err
		}

		// 事务：仅当 key 不存在时才写入，实现互斥分配
		txn := a.client.Txn(ctx).
			If(clientv3.Compare(clientv3.CreateRevision(key), "=", 0)).
			Then(clientv3.OpPut(key, "1", clientv3.WithLease(lease.ID))).
			Else()

		resp, err := txn.Commit()
		if err != nil {
			return 0, err
		}

		if resp.Succeeded {
			// 抢占成功，启动后台续租协程
			go a.keepAlive(ctx, lease.ID)
			return id, nil
		}
		// 被其他节点抢了，继续尝试下一个
	}
	return 0, errors.New("no available workerID")
}

// keepAlive 后台续租，保持 WorkerID 有效
func (a *WorkerIDAllocator) keepAlive(ctx context.Context, leaseID clientv3.LeaseID) {
	ch, err := a.client.KeepAlive(ctx, leaseID)
	if err != nil {
		return
	}
	for range ch {
		// 持续续租，直到 ctx 取消或连接断开
	}
}

// 降级方案：若 etcd 不可用，可 fallback 到本地文件缓存 WorkerID，或基于 IP 哈希取模
// Fallback：从本机 IP 最后一段推导 WorkerID（仅用于开发/测试环境）
func getWorkerIDFromIP() (int64, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return 0, err
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ip4 := ipnet.IP.To4(); ip4 != nil {
				return int64(ip4[3]) % (maxWorkerID + 1), nil
			}
		}
	}
	return 0, errors.New("no valid IPv4 found")
}
