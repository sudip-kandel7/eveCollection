package algorithms

import (
	"sort"

	"github.com/sudip-kandel7/eveCollection/internal/db"
)

const (
	categoryPoints = 7.0
	tagPoints      = 2.0
	soldPoints     = 5.0
	ratingPoints   = 3.0
	ratingPrior    = 5.0
)

type Recommendation struct {
	ProductID int32
	Score     float64
}

func RecommendationPoints(products []db.GetRecommendationFeaturesRow, productTagIDs []int32, categoryID int32, limit int) []Recommendation {
	targetTags := make(map[int32]struct{}, len(productTagIDs))
	for _, tagID := range productTagIDs {
		targetTags[tagID] = struct{}{}
	}

	C := averageRating(products)
	highest := highestSold(products)

	recommendations := make([]Recommendation, 0, len(products))
	for _, product := range products {
		recommendation := recommendationScore(product, targetTags, categoryID, C, highest)
		if recommendation.Score == 0 {
			continue
		}
		recommendations = append(recommendations, recommendation)
	}

	sortRecommendations(recommendations)

	if limit > 0 && len(recommendations) > limit {
		recommendations = recommendations[:limit]
	}

	return recommendations
}

func averageRating(products []db.GetRecommendationFeaturesRow) float64 {
	if len(products) == 0 {
		return 0
	}

	var total float64
	for _, product := range products {
		total += product.Rating
	}
	return total / float64(len(products))
}

func highestSold(products []db.GetRecommendationFeaturesRow) int32 {
	var highest int32
	for _, product := range products {
		if product.Sold > highest {
			highest = product.Sold
		}
	}
	return highest
}

func recommendationScore(product db.GetRecommendationFeaturesRow, targetTags map[int32]struct{}, categoryID int32, C float64, highestSold int32) Recommendation {
	matchedTags := matchedTagCount(product.TagIds, targetTags)

	score := float64(matchedTags) * tagPoints
	if product.CategoryID == categoryID {
		score += categoryPoints
	}
	if highestSold > 0 {
		score += (float64(product.Sold) / float64(highestSold)) * soldPoints
	}

	weightedRating := product.Rating
	if product.ReviewCount > 0 || C > 0 {
		weightedRating = (float64(product.ReviewCount)*product.Rating + ratingPrior*C) / (float64(product.ReviewCount) + ratingPrior)
	}
	score += (weightedRating / 5.0) * ratingPoints

	return Recommendation{
		ProductID: product.ID,
		Score:     score,
	}
}

func matchedTagCount(productTagIDs []int32, targetTags map[int32]struct{}) int {
	matched := 0
	for _, tagID := range productTagIDs {
		if _, ok := targetTags[tagID]; ok {
			matched++
		}
	}
	return matched
}

func sortRecommendations(recommendations []Recommendation) {
	sort.Slice(recommendations, func(i, j int) bool {
		if recommendations[i].Score == recommendations[j].Score {
			return recommendations[i].ProductID < recommendations[j].ProductID
		}
		return recommendations[i].Score > recommendations[j].Score
	})
}
