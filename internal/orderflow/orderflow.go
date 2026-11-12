package orderflow

var nextStatuses = map[string][]string{
	"pending":          {"placed"},
	"placed":           {"packed", "out_for_delivery", "delivered"},
	"packed":           {"out_for_delivery", "delivered"},
	"out_for_delivery": {"delivered"},
	"delivered":        {},
	"cancelled":        {},
}

func NextStatuses(status string) []string {
	statuses := nextStatuses[status]
	result := make([]string, len(statuses))
	copy(result, statuses)
	return result
}

func CanTransition(from, to string) bool {
	for _, status := range nextStatuses[from] {
		if status == to {
			return true
		}
	}
	return false
}
