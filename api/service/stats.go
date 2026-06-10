package service

import (
	"context"
	"fmt"
	"time"

	"github.com/SemRels/semrel-registry/api/database"
)

type StatsSeriesPoint struct {
	Period    string `json:"period"`
	Views     int64  `json:"views"`
	Downloads int64  `json:"downloads"`
}

type TopPluginStat struct {
	PluginID  int64  `json:"pluginId"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
	Views     int64  `json:"views"`
	Downloads int64  `json:"downloads"`
	Category  string `json:"category"`
}

type TopVersionStat struct {
	VersionID  int64  `json:"versionId"`
	PluginID   int64  `json:"pluginId"`
	Namespace  string `json:"namespace,omitempty"`
	PluginName string `json:"pluginName"`
	Version    string `json:"version"`
	Views      int64  `json:"views"`
	Downloads  int64  `json:"downloads"`
}

type RegistryStats struct {
	TotalPlugins   int64                         `json:"totalPlugins"`
	Categories     map[string]int64              `json:"categories"`
	TotalViews     int64                         `json:"totalViews"`
	TotalDownloads int64                         `json:"totalDownloads"`
	TopPlugins     []TopPluginStat               `json:"topPlugins"`
	TopVersions    []TopVersionStat              `json:"topVersions"`
	Series         map[string][]StatsSeriesPoint `json:"series"`
	Timestamp      time.Time                     `json:"timestamp"`
}

type RegistryStatsProvider interface {
	GetRegistryStats(ctx context.Context) (RegistryStats, error)
}

type NoopRegistryStatsProvider struct{}

func NewNoopRegistryStatsProvider() RegistryStatsProvider {
	return NoopRegistryStatsProvider{}
}

func (NoopRegistryStatsProvider) GetRegistryStats(_ context.Context) (RegistryStats, error) {
	return RegistryStats{Categories: map[string]int64{}, Series: map[string][]StatsSeriesPoint{}}, nil
}

type PostgresRegistryStatsProvider struct {
	db *database.Database
}

func NewPostgresRegistryStatsProvider(db *database.Database) RegistryStatsProvider {
	if db == nil || db.Pool() == nil {
		return NewNoopRegistryStatsProvider()
	}
	return &PostgresRegistryStatsProvider{db: db}
}

func (p *PostgresRegistryStatsProvider) GetRegistryStats(ctx context.Context) (RegistryStats, error) {
	stats := RegistryStats{
		Categories: map[string]int64{},
		Series:     map[string][]StatsSeriesPoint{},
		Timestamp:  time.Now().UTC(),
	}

	if err := p.loadTotals(ctx, &stats); err != nil {
		return RegistryStats{}, err
	}
	if err := p.loadCategories(ctx, &stats); err != nil {
		return RegistryStats{}, err
	}
	if err := p.loadTopPlugins(ctx, &stats); err != nil {
		return RegistryStats{}, err
	}
	if err := p.loadTopVersions(ctx, &stats); err != nil {
		return RegistryStats{}, err
	}

	daySeries, err := p.querySeries(ctx, "day", 14)
	if err != nil {
		return RegistryStats{}, err
	}
	stats.Series["day"] = daySeries

	weekSeries, err := p.querySeries(ctx, "week", 12)
	if err != nil {
		return RegistryStats{}, err
	}
	stats.Series["week"] = weekSeries

	monthSeries, err := p.querySeries(ctx, "month", 12)
	if err != nil {
		return RegistryStats{}, err
	}
	stats.Series["month"] = monthSeries

	return stats, nil
}

func (p *PostgresRegistryStatsProvider) loadTotals(ctx context.Context, stats *RegistryStats) error {
	row := p.db.Pool().QueryRow(ctx, `
SELECT COUNT(*)::BIGINT,
       COALESCE(SUM(views), 0)::BIGINT,
       COALESCE(SUM(downloads), 0)::BIGINT
FROM plugins
WHERE deleted_at IS NULL
`)
	if err := row.Scan(&stats.TotalPlugins, &stats.TotalViews, &stats.TotalDownloads); err != nil {
		return fmt.Errorf("query totals: %w", err)
	}
	return nil
}

func (p *PostgresRegistryStatsProvider) loadCategories(ctx context.Context, stats *RegistryStats) error {
	rows, err := p.db.Pool().Query(ctx, `
SELECT category, COUNT(*)::BIGINT
FROM plugins
WHERE deleted_at IS NULL
GROUP BY category
ORDER BY category ASC
`)
	if err != nil {
		return fmt.Errorf("query categories: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var category string
		var count int64
		if scanErr := rows.Scan(&category, &count); scanErr != nil {
			return fmt.Errorf("scan category: %w", scanErr)
		}
		stats.Categories[category] = count
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate categories: %w", err)
	}
	return nil
}

func (p *PostgresRegistryStatsProvider) loadTopPlugins(ctx context.Context, stats *RegistryStats) error {
	rows, err := p.db.Pool().Query(ctx, `
SELECT id,
       COALESCE(namespace, ''),
       name,
       COALESCE(views, 0)::BIGINT,
       COALESCE(downloads, 0)::BIGINT,
       category
FROM plugins
WHERE deleted_at IS NULL
ORDER BY COALESCE(downloads, 0) DESC, COALESCE(views, 0) DESC, name ASC
LIMIT 10
`)
	if err != nil {
		return fmt.Errorf("query top plugins: %w", err)
	}
	defer rows.Close()

	top := make([]TopPluginStat, 0, 10)
	for rows.Next() {
		var item TopPluginStat
		if scanErr := rows.Scan(&item.PluginID, &item.Namespace, &item.Name, &item.Views, &item.Downloads, &item.Category); scanErr != nil {
			return fmt.Errorf("scan top plugin: %w", scanErr)
		}
		top = append(top, item)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate top plugins: %w", err)
	}
	stats.TopPlugins = top
	return nil
}

func (p *PostgresRegistryStatsProvider) loadTopVersions(ctx context.Context, stats *RegistryStats) error {
	rows, err := p.db.Pool().Query(ctx, `
SELECT pv.id,
       pv.plugin_id,
       COALESCE(pl.namespace, ''),
       pl.name,
       pv.version,
       COALESCE(pv.views, 0)::BIGINT,
       COALESCE(pv.downloads, 0)::BIGINT
FROM plugin_versions pv
JOIN plugins pl ON pl.id = pv.plugin_id
WHERE pl.deleted_at IS NULL
ORDER BY COALESCE(pv.downloads, 0) DESC, COALESCE(pv.views, 0) DESC, pv.created_at DESC
LIMIT 10
`)
	if err != nil {
		return fmt.Errorf("query top versions: %w", err)
	}
	defer rows.Close()

	top := make([]TopVersionStat, 0, 10)
	for rows.Next() {
		var item TopVersionStat
		if scanErr := rows.Scan(&item.VersionID, &item.PluginID, &item.Namespace, &item.PluginName, &item.Version, &item.Views, &item.Downloads); scanErr != nil {
			return fmt.Errorf("scan top version: %w", scanErr)
		}
		top = append(top, item)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate top versions: %w", err)
	}
	stats.TopVersions = top
	return nil
}

func (p *PostgresRegistryStatsProvider) querySeries(ctx context.Context, grain string, periods int) ([]StatsSeriesPoint, error) {
	if periods <= 0 {
		return []StatsSeriesPoint{}, nil
	}

	var query string
	switch grain {
	case "day":
		query = `
WITH gs AS (
  SELECT generate_series::date AS day
  FROM generate_series(CURRENT_DATE - ($1::INT - 1), CURRENT_DATE, '1 day'::interval)
)
SELECT TO_CHAR(gs.day, 'YYYY-MM-DD') AS period,
       COALESCE(SUM(CASE WHEN m.metric_type = 'view' THEN m.count END), 0)::BIGINT AS views,
       COALESCE(SUM(CASE WHEN m.metric_type = 'download' THEN m.count END), 0)::BIGINT AS downloads
FROM gs
LEFT JOIN metric_daily_plugin m ON m.day = gs.day
GROUP BY gs.day
ORDER BY gs.day DESC
LIMIT $1
`
	case "week":
		query = `
WITH gs AS (
  SELECT DATE_TRUNC('week', gs::date) AS week_start
  FROM generate_series(
    DATE_TRUNC('week', CURRENT_DATE) - (($1::INT - 1) * INTERVAL '1 week'),
    DATE_TRUNC('week', CURRENT_DATE),
    '1 week'::interval
  ) AS gs
)
SELECT TO_CHAR(gs.week_start, 'IYYY-"W"IW') AS period,
       COALESCE(SUM(CASE WHEN m.metric_type = 'view' THEN m.count END), 0)::BIGINT AS views,
       COALESCE(SUM(CASE WHEN m.metric_type = 'download' THEN m.count END), 0)::BIGINT AS downloads
FROM gs
LEFT JOIN metric_daily_plugin m ON DATE_TRUNC('week', m.day) = gs.week_start
GROUP BY gs.week_start
ORDER BY gs.week_start DESC
LIMIT $1
`
	case "month":
		query = `
WITH gs AS (
  SELECT DATE_TRUNC('month', gs::date) AS month_start
  FROM generate_series(
    DATE_TRUNC('month', CURRENT_DATE) - (($1::INT - 1) * INTERVAL '1 month'),
    DATE_TRUNC('month', CURRENT_DATE),
    '1 month'::interval
  ) AS gs
)
SELECT TO_CHAR(gs.month_start, 'YYYY-MM') AS period,
       COALESCE(SUM(CASE WHEN m.metric_type = 'view' THEN m.count END), 0)::BIGINT AS views,
       COALESCE(SUM(CASE WHEN m.metric_type = 'download' THEN m.count END), 0)::BIGINT AS downloads
FROM gs
LEFT JOIN metric_daily_plugin m ON DATE_TRUNC('month', m.day) = gs.month_start
GROUP BY gs.month_start
ORDER BY gs.month_start DESC
LIMIT $1
`
	default:
		return nil, fmt.Errorf("unsupported series grain: %s", grain)
	}

	rows, err := p.db.Pool().Query(ctx, query, periods)
	if err != nil {
		return nil, fmt.Errorf("query %s series: %w", grain, err)
	}
	defer rows.Close()

	series := make([]StatsSeriesPoint, 0, periods)
	for rows.Next() {
		var point StatsSeriesPoint
		if scanErr := rows.Scan(&point.Period, &point.Views, &point.Downloads); scanErr != nil {
			return nil, fmt.Errorf("scan %s series point: %w", grain, scanErr)
		}
		series = append(series, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s series: %w", grain, err)
	}
	return series, nil
}
