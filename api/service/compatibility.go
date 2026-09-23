package service

import (
	"context"
	"math"

	appErrors "github.com/SemRels/semrel-registry/api/internal"
	"github.com/SemRels/semrel-registry/api/models"
	"github.com/SemRels/semrel-registry/api/repository"
)

// maxCompatibilityScan bounds the rows a compatibility query examines.
//
// The filter cannot run in SQL, so the alternative to a bound is an unbounded
// fetch that degrades with the catalogue. At this size the whole catalogue fits
// comfortably; past it the answer is to precompute the numeric bounds of each
// range into columns at publish time and filter in the database.
const maxCompatibilityScan = 2000

// listCompatible answers a listing filtered by `compatibleWith`, a concrete
// semrel core version such as "0.27.1".
//
// A plugin matches when the release a client would actually install — its
// latest stable, non-yanked version — declares a range that admits that core
// version. Plugins that declare no range match: the metadata is optional and
// most of the catalogue predates it, so treating "unknown" as "incompatible"
// would hide almost everything.
func (s *PluginService) listCompatible(ctx context.Context, params ListPluginsParams, filters []repository.Filter) (PluginListResult, error) {
	if _, err := ParseVersion(params.CompatibleWith); err != nil {
		return PluginListResult{}, &appErrors.ValidationError{
			Field: "compatibleWith",
			Issue: "must be a semantic version such as 0.27.1",
		}
	}

	candidates, err := s.repo.GetAll(ctx, maxCompatibilityScan, 0, filters...)
	if err != nil {
		return PluginListResult{}, err
	}

	matched := make([]models.Plugin, 0, len(candidates))
	for _, plugin := range candidates {
		ok, rangeErr := SatisfiesRange(params.CompatibleWith, plugin.LatestSemrelCore)
		if rangeErr != nil {
			// A plugin that declares an unparseable range is a publishing
			// mistake, not a reason to fail the whole listing. Excluding it is
			// the conservative answer for a compatibility question.
			continue
		}
		if ok {
			matched = append(matched, plugin)
		}
	}

	total := int64(len(matched))
	pages := 0
	if total > 0 {
		pages = int(math.Ceil(float64(total) / float64(params.Limit)))
	}

	offset := (params.Page - 1) * params.Limit
	if offset >= len(matched) {
		matched = nil
	} else {
		end := offset + params.Limit
		if end > len(matched) {
			end = len(matched)
		}
		matched = matched[offset:end]
	}

	return PluginListResult{
		Data: matched,
		Pagination: Pagination{
			Page:  params.Page,
			Limit: params.Limit,
			Total: total,
			Pages: pages,
		},
	}, nil
}
