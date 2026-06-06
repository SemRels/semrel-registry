package service

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SemRels/semrel-registry/api/database"
)

type MetricType string

const (
	MetricTypeView     MetricType = "view"
	MetricTypeDownload MetricType = "download"
)

type MetricEvent struct {
	PluginID   int64
	VersionID  int64
	Type       MetricType
	Source     string
	OccurredAt time.Time
}

type MetricsRecorder interface {
	Record(event MetricEvent)
	Close(ctx context.Context) error
}

type MetricsConfig struct {
	BufferSize    int
	BatchSize     int
	FlushInterval time.Duration
}

type NoopMetricsRecorder struct{}

func NewNoopMetricsRecorder() MetricsRecorder {
	return NoopMetricsRecorder{}
}

// Record intentionally does nothing for storage backends that do not persist metrics.
func (NoopMetricsRecorder) Record(_ MetricEvent) {}

func (NoopMetricsRecorder) Close(_ context.Context) error { return nil }

type asyncMetricsRecorder struct {
	db       *database.Database
	events   chan MetricEvent
	batch    int
	flushDur time.Duration
	dropped  atomic.Int64

	stopCh chan struct{}
	doneCh chan struct{}
	once   sync.Once
}

func NewAsyncMetricsRecorder(db *database.Database, cfg MetricsConfig) MetricsRecorder {
	if db == nil || db.Pool() == nil {
		return NewNoopMetricsRecorder()
	}

	if cfg.BufferSize < 1 {
		cfg.BufferSize = 2048
	}
	if cfg.BatchSize < 1 {
		cfg.BatchSize = 200
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 2 * time.Second
	}

	r := &asyncMetricsRecorder{
		db:       db,
		events:   make(chan MetricEvent, cfg.BufferSize),
		batch:    cfg.BatchSize,
		flushDur: cfg.FlushInterval,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}

	go r.run()
	return r
}

func (r *asyncMetricsRecorder) Record(event MetricEvent) {
	if event.PluginID <= 0 {
		return
	}
	if event.Type != MetricTypeView && event.Type != MetricTypeDownload {
		return
	}
	if event.Source == "" {
		event.Source = "api"
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}

	select {
	case r.events <- event:
	default:
		r.dropped.Add(1)
	}
}

func (r *asyncMetricsRecorder) Close(ctx context.Context) error {
	r.once.Do(func() {
		close(r.stopCh)
	})

	select {
	case <-r.doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *asyncMetricsRecorder) run() {
	defer close(r.doneCh)

	ticker := time.NewTicker(r.flushDur)
	defer ticker.Stop()

	buffer := make([]MetricEvent, 0, r.batch)
	flush := func() {
		if len(buffer) == 0 {
			return
		}
		if err := r.flushBatch(buffer); err != nil {
			log.Printf("metrics flush warning: %v", err)
		}
		buffer = buffer[:0]
	}

	for {
		select {
		case <-r.stopCh:
			drain := true
			for drain {
				select {
				case ev := <-r.events:
					buffer = append(buffer, ev)
					if len(buffer) >= r.batch {
						flush()
					}
				default:
					drain = false
				}
			}
			flush()
			if dropped := r.dropped.Load(); dropped > 0 {
				log.Printf("metrics recorder dropped %d events", dropped)
			}
			return
		case ev := <-r.events:
			buffer = append(buffer, ev)
			if len(buffer) >= r.batch {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

type metricCounter struct {
	views     int64
	downloads int64
}

type dailyMetricKey struct {
	day       string
	pluginID  int64
	versionID int64
	typeName  MetricType
}

func (r *asyncMetricsRecorder) flushBatch(events []MetricEvent) error {
	tx, err := r.db.BeginTx()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	pluginTotals := make(map[int64]*metricCounter)
	versionTotals := make(map[int64]*metricCounter)
	dailyPlugin := make(map[dailyMetricKey]int64)
	dailyVersion := make(map[dailyMetricKey]int64)

	for _, ev := range events {
		_, execErr := tx.Exec(ctx, `
INSERT INTO metric_events (plugin_id, version_id, metric_type, source, occurred_at)
VALUES ($1, $2, $3, $4, $5)
`, ev.PluginID, nullableVersionID(ev.VersionID), string(ev.Type), ev.Source, ev.OccurredAt.UTC())
		if execErr != nil {
			return execErr
		}

		pc := pluginTotals[ev.PluginID]
		if pc == nil {
			pc = &metricCounter{}
			pluginTotals[ev.PluginID] = pc
		}
		if ev.Type == MetricTypeDownload {
			pc.downloads++
		} else {
			pc.views++
		}

		day := ev.OccurredAt.UTC().Format("2006-01-02")
		pKey := dailyMetricKey{day: day, pluginID: ev.PluginID, typeName: ev.Type}
		dailyPlugin[pKey]++

		if ev.VersionID > 0 {
			vc := versionTotals[ev.VersionID]
			if vc == nil {
				vc = &metricCounter{}
				versionTotals[ev.VersionID] = vc
			}
			if ev.Type == MetricTypeDownload {
				vc.downloads++
			} else {
				vc.views++
			}

			vKey := dailyMetricKey{day: day, pluginID: ev.PluginID, versionID: ev.VersionID, typeName: ev.Type}
			dailyVersion[vKey]++
		}
	}

	for pluginID, counter := range pluginTotals {
		_, execErr := tx.Exec(ctx, `
UPDATE plugins
SET views = views + $2,
    downloads = downloads + $3,
    updated_at = NOW()
WHERE id = $1
`, pluginID, counter.views, counter.downloads)
		if execErr != nil {
			return execErr
		}
	}

	for versionID, counter := range versionTotals {
		_, execErr := tx.Exec(ctx, `
UPDATE plugin_versions
SET views = views + $2,
    downloads = downloads + $3
WHERE id = $1
`, versionID, counter.views, counter.downloads)
		if execErr != nil {
			return execErr
		}
	}

	for key, count := range dailyPlugin {
		_, execErr := tx.Exec(ctx, `
INSERT INTO metric_daily_plugin (day, plugin_id, metric_type, count)
VALUES ($1, $2, $3, $4)
ON CONFLICT (day, plugin_id, metric_type)
DO UPDATE SET count = metric_daily_plugin.count + EXCLUDED.count,
              updated_at = NOW()
`, key.day, key.pluginID, string(key.typeName), count)
		if execErr != nil {
			return execErr
		}
	}

	for key, count := range dailyVersion {
		_, execErr := tx.Exec(ctx, `
INSERT INTO metric_daily_version (day, plugin_id, version_id, metric_type, count)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (day, version_id, metric_type)
DO UPDATE SET count = metric_daily_version.count + EXCLUDED.count,
              updated_at = NOW()
`, key.day, key.pluginID, key.versionID, string(key.typeName), count)
		if execErr != nil {
			return execErr
		}
	}

	if commitErr := tx.Commit(ctx); commitErr != nil {
		return commitErr
	}
	return nil
}

func nullableVersionID(versionID int64) interface{} {
	if versionID <= 0 {
		return nil
	}
	return versionID
}
