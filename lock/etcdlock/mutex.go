package etcdlock

import (
	"fmt"
	"strings"
	"time"

	"github.com/projecteru2/core/types"
	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/clientv3/concurrency"
	"golang.org/x/net/context"
)

// Mutex is etcdv3 lock
type Mutex struct {
	timeout time.Duration
	mutex   *concurrency.Mutex
	session *concurrency.Session
}

// New new a lock
func New(cli *clientv3.Client, key string, ttl time.Duration) (*Mutex, error) {
	if key == "" {
		return nil, types.ErrKeyIsEmpty
	}

	if !strings.HasPrefix(key, "/") {
		key = fmt.Sprintf("/%s", key)
	}

	session, err := concurrency.NewSession(cli, concurrency.WithTTL(int(ttl.Seconds())))
	if err != nil {
		return nil, err
	}

	mutex := &Mutex{mutex: concurrency.NewMutex(session, key), session: session}
	mutex.timeout = ttl
	return mutex, nil
}

// Lock get locked
func (m *Mutex) Lock(ctx context.Context) error {
	lockCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()
	return m.mutex.Lock(lockCtx)
}

// Unlock unlock
func (m *Mutex) Unlock(ctx context.Context) error {
	defer m.session.Close()
	// 一定要释放
	lockCtx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()
	return m.mutex.Unlock(lockCtx)
}
