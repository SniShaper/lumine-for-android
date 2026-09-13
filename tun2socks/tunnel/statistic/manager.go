package statistic

import (
	"sync"
	"time"

	"go.uber.org/atomic"
)

var DefaultManager *Manager

func init() {
	DefaultManager = NewManager()
}

type Manager struct {
	connections   sync.Map
	uploadTemp    *atomic.Int64
	downloadTemp  *atomic.Int64
	uploadBlip    *atomic.Int64
	downloadBlip  *atomic.Int64
	uploadTotal   *atomic.Int64
	downloadTotal *atomic.Int64
	done          chan struct{}
	closeOnce     sync.Once
}

// NewManager returns a fully initialized Manager with its one-second
// statistics loop running. A zero-value Manager panics on the first
// statistic push (nil atomic pointers), so always use this constructor.
func NewManager() *Manager {
	m := &Manager{
		uploadTemp:    atomic.NewInt64(0),
		downloadTemp:  atomic.NewInt64(0),
		uploadBlip:    atomic.NewInt64(0),
		downloadBlip:  atomic.NewInt64(0),
		uploadTotal:   atomic.NewInt64(0),
		downloadTotal: atomic.NewInt64(0),
		done:          make(chan struct{}),
	}
	go m.handle()
	return m
}

// Close stops the internal statistics ticker. It is safe to call
// multiple times. Statistics methods remain usable afterwards; only the
// per-second blip refresh stops.
func (m *Manager) Close() {
	m.closeOnce.Do(func() {
		close(m.done)
	})
}

func (m *Manager) Join(c tracker) {
	m.connections.Store(c.ID(), c)
}

func (m *Manager) Leave(c tracker) {
	m.connections.Delete(c.ID())
}

func (m *Manager) PushUploaded(size int64) {
	m.uploadTemp.Add(size)
	m.uploadTotal.Add(size)
}

func (m *Manager) PushDownloaded(size int64) {
	m.downloadTemp.Add(size)
	m.downloadTotal.Add(size)
}

func (m *Manager) Now() (up int64, down int64) {
	return m.uploadBlip.Load(), m.downloadBlip.Load()
}

func (m *Manager) Snapshot() *Snapshot {
	var connections []tracker
	m.connections.Range(func(key, value any) bool {
		connections = append(connections, value.(tracker))
		return true
	})

	return &Snapshot{
		UploadTotal:   m.uploadTotal.Load(),
		DownloadTotal: m.downloadTotal.Load(),
		Connections:   connections,
	}
}

func (m *Manager) ResetStatistic() {
	m.uploadTemp.Store(0)
	m.uploadBlip.Store(0)
	m.uploadTotal.Store(0)
	m.downloadTemp.Store(0)
	m.downloadBlip.Store(0)
	m.downloadTotal.Store(0)
}

func (m *Manager) handle() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.uploadBlip.Store(m.uploadTemp.Load())
			m.uploadTemp.Store(0)
			m.downloadBlip.Store(m.downloadTemp.Load())
			m.downloadTemp.Store(0)
		case <-m.done:
			return
		}
	}
}

type Snapshot struct {
	DownloadTotal int64     `json:"downloadTotal"`
	UploadTotal   int64     `json:"uploadTotal"`
	Connections   []tracker `json:"connections"`
}
