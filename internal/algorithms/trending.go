package algorithms

import (
	"math"
	"sort"
	"time"

	"github.com/sudip-kandel7/eveCollection/internal/db"
)

const decayRate = 0.8

type TrendingProduct struct {
	ProductID int32
	Points    float64
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func trendingPoints(stats []db.ProductStat) map[int32]float64 {
	points := make(map[int32]float64)
	today := dateOnly(time.Now())

	for _, stat := range stats {
		daysAgo := int(today.Sub(dateOnly(stat.Day.Time)).Hours() / 24)
		if daysAgo < 0 {
			daysAgo = 0
		}

		decay := math.Pow(decayRate, float64(daysAgo))

		raw := float64(stat.Views*1 + stat.Carts*3 + stat.Purchases*5)

		points[stat.ProductID] += raw * decay
	}

	return points
}

func TopTrending(stats []db.ProductStat, limit int) []TrendingProduct {
	points := trendingPoints(stats)

	ranked := make([]TrendingProduct, 0, len(points))
	for productID, score := range points {
		ranked = append(ranked, TrendingProduct{
			ProductID: productID,
			Points:    score,
		})
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Points == ranked[j].Points {
			return ranked[i].ProductID < ranked[j].ProductID
		}
		return ranked[i].Points > ranked[j].Points
	})

	if limit > 0 && len(ranked) > limit {
		ranked = ranked[:limit]
	}

	return ranked
}
