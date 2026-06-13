package models

type ListOptions struct {
	Status   string
	Provider string
	Repo     string
	Page     int
	PageSize int
}

type AggregateStats struct {
	Running    int
	GateFailed int
	Findings   int
}
